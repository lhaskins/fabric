/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/tedsuo/ifrit"
)

const KafkaDefaultImage = "hyperledger/fabric-kafka:latest"

// DefaultNamer is the default naming function.
var KafkaDefaultNamer NameFunc = UniqueName

// Kafka manages the execution of an instance of a dockerized CounchDB
// for tests.
type Kafka struct {
	Client        *docker.Client
	Image         string
	HostIP        string
	HostPort      int
	ContainerPort docker.Port
	Name          string
	StartTimeout  time.Duration

	KafkaMessageMaxBytes              int
	KafkaReplicaFetchMaxBytes         int
	KafkaUncleanLeaderElectionEnable  bool
	KafkaDefaultReplicationFactor     int
	KafkaMinInsyncReplicas            int
	KafkaBrokerID                     int
	KafkaZookeeperConnect             string
	KafkaReplicaFetchResponseMaxBytes int

	ErrorStream  io.Writer
	OutputStream io.Writer

	NetworkID        string
	NetworkName      string
	ContainerID      string
	HostAddress      string
	ContainerAddress string
	Address          string

	mutex   sync.Mutex
	stopped bool
}

// Run runs a Kafka container. It implements the ifrit.Runner interface
func (k *Kafka) Run(sigCh <-chan os.Signal, ready chan<- struct{}) error {
	if k.Image == "" {
		k.Image = KafkaDefaultImage
	}

	if k.Name == "" {
		k.Name = KafkaDefaultNamer()
	}

	if k.HostIP == "" {
		k.HostIP = "127.0.0.1"
	}

	if k.ContainerPort == docker.Port("") {
		k.ContainerPort = docker.Port("9092/tcp")
	}

	if k.StartTimeout == 0 {
		k.StartTimeout = DefaultStartTimeout
	}

	if k.Client == nil {
		client, err := docker.NewClientFromEnv()
		if err != nil {
			return err
		}
		k.Client = client
	}

	if k.KafkaDefaultReplicationFactor == 0 {
		k.KafkaDefaultReplicationFactor = 1
	}

	if k.KafkaMinInsyncReplicas == 0 {
		k.KafkaMinInsyncReplicas = 1
	}

	if k.KafkaBrokerID == 0 {
		k.KafkaBrokerID = 0
	}

	if k.KafkaZookeeperConnect == "" {
		k.KafkaZookeeperConnect = "zookeeper:2181"
	}

	if k.KafkaMessageMaxBytes == 0 {
		k.KafkaMessageMaxBytes = 1000012
	}

	if k.KafkaReplicaFetchMaxBytes == 0 {
		k.KafkaReplicaFetchMaxBytes = 1048576
	}

	if k.KafkaReplicaFetchResponseMaxBytes == 0 {
		k.KafkaReplicaFetchResponseMaxBytes = 10485760
	}

	hostConfig := &docker.HostConfig{
		PortBindings: map[docker.Port][]docker.PortBinding{
			k.ContainerPort: []docker.PortBinding{{
				HostIP:   k.HostIP,
				HostPort: strconv.Itoa(k.HostPort),
			}},
		},
	}

	config := &docker.Config{
		Image: k.Image,
		Env: []string{
			"KAFKA_LOG_RETENTION_MS=-1",
			fmt.Sprintf("KAFKA_MESSAGE_MAX_BYTES=%d", k.KafkaMessageMaxBytes),
			fmt.Sprintf("KAFKA_REPLICA_FETCH_MAX_BYTES=%d", k.KafkaReplicaFetchMaxBytes),
			fmt.Sprintf("KAFKA_UNCLEAN_LEADER_ELECTION_ENABLE=%s", strconv.FormatBool(k.KafkaUncleanLeaderElectionEnable)),
			fmt.Sprintf("KAFKA_DEFAULT_REPLICATION_FACTOR=%d", k.KafkaDefaultReplicationFactor),
			fmt.Sprintf("KAFKA_MIN_INSYNC_REPLICAS=%d", k.KafkaMinInsyncReplicas),
			fmt.Sprintf("KAFKA_BROKER_ID=%d", k.KafkaBrokerID),
			fmt.Sprintf("KAFKA_ZOOKEEPER_CONNECT=%s", k.KafkaZookeeperConnect),
			fmt.Sprintf("KAFKA_REPLICA_FETCH_RESPONSE_MAX_BYTES=%d", k.KafkaReplicaFetchResponseMaxBytes),
		},
	}

	networkingConfig := &docker.NetworkingConfig{
		EndpointsConfig: map[string]*docker.EndpointConfig{
			k.NetworkName: &docker.EndpointConfig{
				NetworkID: k.NetworkID,
			},
		},
	}

	container, err := k.Client.CreateContainer(
		docker.CreateContainerOptions{
			Name:             k.Name,
			Config:           config,
			HostConfig:       hostConfig,
			NetworkingConfig: networkingConfig,
		})
	if err != nil {
		return err
	}
	k.ContainerID = container.ID

	err = k.Client.StartContainer(container.ID, nil)
	if err != nil {
		return err
	}
	defer k.Stop()

	container, err = k.Client.InspectContainer(container.ID)
	if err != nil {
		return err
	}

	k.HostAddress = net.JoinHostPort(
		container.NetworkSettings.Ports[k.ContainerPort][0].HostIP,
		container.NetworkSettings.Ports[k.ContainerPort][0].HostPort,
	)
	k.ContainerAddress = net.JoinHostPort(
		container.NetworkSettings.Networks[k.NetworkName].IPAddress,
		k.ContainerPort.Port(),
	)

	logContext, cancelLogs := context.WithCancel(context.Background())
	go func() {
		k.streamLogs(logContext)
		// TODO: report the error or do something crazy like having another case statement in the select
	}()
	defer cancelLogs()

	containerExit := k.wait()
	ctx, cancel := context.WithTimeout(context.Background(), k.StartTimeout)
	defer func() {
		cancel()
	}()

	select {
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "database in container %s did not start", k.ContainerID)
	case <-containerExit:
		return errors.New("container exited before ready")
	case <-k.ready(ctx, k.ContainerAddress):
		k.Address = k.ContainerAddress
	case <-k.ready(ctx, k.HostAddress):
		k.Address = k.HostAddress
	}

	cancel()
	close(ready)

	select {
	case err := <-containerExit:
		return err
	case <-sigCh:
		return k.Stop()
	}
}

func (k *Kafka) ready(ctx context.Context, addr string) <-chan struct{} {
	readyCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
			if err == nil {
				conn.Close()
				close(readyCh)
				return
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	return readyCh
}

func (k *Kafka) wait() <-chan error {
	exitCh := make(chan error)
	go func() {
		if _, err := k.Client.WaitContainer(k.ContainerID); err != nil {
			exitCh <- err
		}
	}()

	return exitCh
}

func (k *Kafka) streamLogs(ctx context.Context) error {
	if k.ErrorStream == nil && k.OutputStream == nil {
		return nil
	}

	logOptions := docker.LogsOptions{
		Context:      ctx,
		Container:    k.ContainerID,
		ErrorStream:  k.ErrorStream,
		OutputStream: k.OutputStream,
		Stderr:       k.ErrorStream != nil,
		Stdout:       k.OutputStream != nil,
		Follow:       true,
	}
	return k.Client.Logs(logOptions)
}

// Start starts the Kafka container using an ifrit runner
func (k *Kafka) Start() error {
	p := ifrit.Invoke(k)

	select {
	case <-p.Ready():
		return nil
	case err := <-p.Wait():
		return err
	}
}

// Stop stops and removes the Kafka container
func (k *Kafka) Stop() error {
	k.mutex.Lock()
	if k.stopped {
		k.mutex.Unlock()
		return errors.Errorf("container %s already stopped", k.ContainerID)
	}
	k.stopped = true
	k.mutex.Unlock()

	err := k.Client.StopContainer(k.ContainerID, 0)
	if err != nil {
		return err
	}

	return k.Client.RemoveContainer(
		docker.RemoveContainerOptions{
			ID:    k.ContainerID,
			Force: true,
		},
	)
}
func (k *Kafka) Remove() error {
	return k.Client.RemoveContainer(
		docker.RemoveContainerOptions{
			ID:    k.ContainerID,
			Force: true,
		},
	)
}

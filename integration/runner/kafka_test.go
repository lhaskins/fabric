/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Kafka Runner", func() {
	var (
		dockerServer *ghttp.Server

		client  *docker.Client
		network *docker.Network

		errBuffer *gbytes.Buffer
		outBuffer *gbytes.Buffer
		kafka     *runner.Kafka
		zookeeper *runner.Zookeeper

		process ifrit.Process
	)

	BeforeEach(func() {
		errBuffer = gbytes.NewBuffer()
		outBuffer = gbytes.NewBuffer()
		kafka = &runner.Kafka{
			Name:                  "kafka1",
			StartTimeout:          time.Second,
			ErrorStream:           io.MultiWriter(errBuffer, GinkgoWriter),
			OutputStream:          io.MultiWriter(outBuffer, GinkgoWriter),
			KafkaZookeeperConnect: "zookeeper0:2181",
			KafkaBrokerID:         1,
		}

		process = nil

		client, err := docker.NewClientFromEnv()
		Expect(err).NotTo(HaveOccurred())

		network, err = client.CreateNetwork(
			docker.CreateNetworkOptions{
				Name: "runner_testnet",
				Driver: "bridge",
			},
		)

		// Start a zookeeper
		zookeeper = &runner.Zookeeper{
			Name:    "zookeeper0",
			ZooMyID: 1,
		}
		err = zookeeper.Start()
		Expect(err).NotTo(HaveOccurred())
		err = client.ConnectNetwork(network.ID,
			docker.NetworkConnectionOptions{Container: "zookeeper0"})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if process != nil {
			process.Signal(syscall.SIGTERM)
		}
//		err := zookeeper.Stop()
//		Expect(err).NotTo(HaveOccurred())

//		close(waitChan)
//		dockerServer.Close()
//		kafkaServer.Close()
	})

	FIt("starts and stops a docker container with the specified image", func() {
		By("using a real docker daemon")
		kafka.Client = nil
		kafka.StartTimeout = 0
		fmt.Println("LMH: network ID:", network.ID)
		kafka.Network = docker.ContainerNetwork{
			NetworkID: network.ID,
		}
		kafka.NetworkName = network.Name
		time.Sleep(2*time.Second)

		By("starting kafka broker")
		process = ifrit.Invoke(kafka)
		Eventually(process.Ready(), runner.DefaultStartTimeout).Should(BeClosed())
		Consistently(process.Wait()).ShouldNot(Receive())

//		err := client.ConnectNetwork(network.ID,
//			docker.NetworkConnectionOptions{Container: "kafka1"})
//		Expect(err).NotTo(HaveOccurred())

		By("inspecting the container by name")
		container, err := client.InspectContainer("kafka1")
		Expect(err).NotTo(HaveOccurred())
		Expect(container.Name).To(Equal("/kafka1"))
		Expect(container.State.Status).To(Equal("running"))
		Expect(container.Config).NotTo(BeNil())
		Expect(container.Config.Image).To(Equal("hyperledger/fabric-kafka:latest"))
		Expect(container.ID).To(Equal(kafka.ContainerID))
		portBindings := container.NetworkSettings.Ports[docker.Port("9092/tcp")]
		Expect(portBindings).To(HaveLen(1))
		Expect(kafka.HostAddress).To(Equal(net.JoinHostPort(portBindings[0].HostIP, portBindings[0].HostPort)))
		Expect(kafka.ContainerAddress).To(Equal(net.JoinHostPort(container.NetworkSettings.IPAddress, "9092")))

		By("getting the container logs")
		Eventually(errBuffer).Should(gbytes.Say(`WARNING: Kafka is running in Admin Party mode.`))

		By("accessing the kafka broker")
		address := kafka.Address
		Expect(address).NotTo(BeEmpty())
		resp, err := http.Get(fmt.Sprintf("http://%s/", address))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		By("terminating the container")
		process.Signal(syscall.SIGTERM)
		Eventually(process.Wait()).Should(Receive(BeNil()))
		process = nil

		_, err = client.InspectContainer("container-name")
		Expect(err).To(MatchError("No such container: container-name"))
	})

	It("can be started and stopped with ifrit", func() {
		process = ifrit.Invoke(kafka)
		Eventually(process.Ready()).Should(BeClosed())
		Expect(dockerServer.ReceivedRequests()).To(HaveLen(5))

		process.Signal(syscall.SIGTERM)
		Eventually(process.Wait()).Should(Receive())
		Expect(dockerServer.ReceivedRequests()).To(HaveLen(7))
		process = nil
	})

	It("can be started and stopped without ifrit", func() {
		err := kafka.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(dockerServer.ReceivedRequests()).To(HaveLen(5))

		err = kafka.Stop()
		Expect(err).NotTo(HaveOccurred())
		Expect(dockerServer.ReceivedRequests()).To(HaveLen(7))
	})

	Context("when a host port is provided", func() {
		BeforeEach(func() {
//			var data = struct {
//				*docker.Config
//				HostConfig *docker.HostConfig
//			}{
//				Config: &docker.Config{
//					Image: "hyperledger/fabric-kafka:latest",
//				},
//				HostConfig: &docker.HostConfig{
//					PortBindings: map[docker.Port][]docker.PortBinding{
//						docker.Port("9092/tcp"): []docker.PortBinding{{
//							HostIP:   "127.0.0.1",
//							HostPort: "33333",
//						}},
//					},
//				},
//			}
		})

		It("exposes kafka on the specified port", func() {
			err := kafka.Start()
			Expect(err).NotTo(HaveOccurred())
			err = kafka.Stop()
			Expect(err).NotTo(HaveOccurred())

			Expect(dockerServer.ReceivedRequests()).To(HaveLen(7))
		})
	})

	Context("when the container has already been stopped", func() {
		It("returns an error", func() {
			err := kafka.Start()
			Expect(err).NotTo(HaveOccurred())

			err = kafka.Stop()
			Expect(err).NotTo(HaveOccurred())
			err = kafka.Stop()
			Expect(err).To(MatchError("container container-id already stopped"))
		})
	})

	Context("when stopping the container fails", func() {
		It("returns an error", func() {
			err := kafka.Start()
			Expect(err).NotTo(HaveOccurred())

			err = kafka.Stop()
			Expect(err).To(Equal(&docker.Error{Status: http.StatusGone}))
		})
	})

	Context("when a name isn't provided", func() {
		It("generates a unique name", func() {
			db1 := &runner.Kafka{Client: client}
			err := db1.Start()
			Expect(err).To(HaveOccurred())
			Expect(db1.Name).ShouldNot(BeEmpty())
			Expect(db1.Name).To(HaveLen(26))

			db2 := &runner.Kafka{Client: client}
			err = db2.Start()
			Expect(err).To(HaveOccurred())
			Expect(db2.Name).ShouldNot(BeEmpty())
			Expect(db2.Name).To(HaveLen(26))

			Expect(db1.Name).NotTo(Equal(db2.Name))
		})
	})
})

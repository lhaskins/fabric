/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner_test

import (
	"io"
	"os"
	"net"
	"net/http"
	"syscall"
	"io/ioutil"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Zookeeper Runner", func() {
	var (
		dockerServer *ghttp.Server
		zkServer  *ghttp.Server

		createStatus    int
		createResponse  *docker.Container
		startStatus     int
		startResponse   string
		inspectStatus   int
		inspectResponse *docker.Container
		logsStatus      int
		logsResponse    string
		stopStatus      int
		stopResponse    string
		waitStatus      int
		waitResponse    string
		deleteStatus    int
		deleteResponse  string

		waitChan chan struct{}
		client   *docker.Client

		errBuffer *gbytes.Buffer
		outBuffer *gbytes.Buffer
		zookeeper *runner.Zookeeper

		process ifrit.Process
	)

	BeforeEach(func() {
		zkServer = ghttp.NewServer()
		zkServer.Writer = GinkgoWriter
		zkServer.AppendHandlers(
			ghttp.RespondWith(http.StatusServiceUnavailable, "service unavailable"),
			ghttp.RespondWith(http.StatusServiceUnavailable, "service unavailable"),
			ghttp.RespondWith(http.StatusOK, "ready"),
		)

		zkAddr := zkServer.Addr()
		zkHost, zkPort, err := net.SplitHostPort(zkAddr)
		Expect(err).NotTo(HaveOccurred())

		waitChan = make(chan struct{}, 1)
		dockerServer = ghttp.NewServer()
		dockerServer.Writer = GinkgoWriter

		createStatus = http.StatusCreated
		createResponse = &docker.Container{
			ID: "container-id",
		}

		dockerServer.RouteToHandler("POST", "/containers/create", ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/containers/create", "name=container-name"),
			ghttp.RespondWithJSONEncodedPtr(&createStatus, &createResponse),
		))

		startStatus = http.StatusNoContent
		dockerServer.RouteToHandler("POST", "/containers/container-id/start", ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/containers/container-id/start", ""),
			ghttp.RespondWithPtr(&startStatus, &startResponse),
		))


		inspectStatus = http.StatusOK
		inspectResponse = &docker.Container{
			ID: "container-id",
			NetworkSettings: &docker.NetworkSettings{
				Ports: map[docker.Port][]docker.PortBinding{
					docker.Port("2181/tcp"): []docker.PortBinding{{
						HostIP:   zkHost,
						HostPort: zkPort,
					}},
				},
			},
		}
		dockerServer.RouteToHandler("GET", "/containers/container-id/json", ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/containers/container-id/json", ""),
			ghttp.RespondWithJSONEncodedPtr(&inspectStatus, &inspectResponse),
		))

		logsStatus = http.StatusOK
		dockerServer.RouteToHandler("GET", "/containers/container-id/logs", ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/containers/container-id/logs", "stderr=1&stdout=1&tail=all"),
			ghttp.RespondWithPtr(&logsStatus, &logsResponse),
		))

		stopStatus = http.StatusNoContent
		dockerServer.RouteToHandler("POST", "/containers/container-id/stop", ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/containers/container-id/stop", "t=0"),
			ghttp.RespondWithPtr(&stopStatus, &stopResponse),
			func(_ http.ResponseWriter, _ *http.Request) {
				defer GinkgoRecover()
				Eventually(waitChan).Should(BeSent(struct{}{}))
			},
		))

		waitStatus = http.StatusNoContent
		waitResponse = `{ StatusCode: 0 }`
		dockerServer.RouteToHandler("POST", "/containers/container-id/wait", ghttp.CombineHandlers(
			ghttp.RespondWithPtr(&waitStatus, &waitResponse),
			func(w http.ResponseWriter, r *http.Request) { <-waitChan },
		))

		deleteStatus = http.StatusNoContent
		dockerServer.RouteToHandler("DELETE", "/containers/container-id", ghttp.CombineHandlers(
			ghttp.VerifyRequest("DELETE", "/containers/container-id", "force=1"),
			ghttp.RespondWithPtr(&deleteStatus, &deleteResponse),
		))

		client, err = docker.NewClient(dockerServer.URL())
		Expect(err).NotTo(HaveOccurred())

		errBuffer = gbytes.NewBuffer()
		outBuffer = gbytes.NewBuffer()
		zookeeper = &runner.Zookeeper{
			Name:         "zookeeper0",
			StartTimeout: time.Second,
			ErrorStream:  io.MultiWriter(errBuffer, GinkgoWriter),
			OutputStream: io.MultiWriter(outBuffer, GinkgoWriter),
			Client:       client,
		}

		process = nil
	})

	AfterEach(func() {
		if process != nil {
			process.Signal(syscall.SIGTERM)
		}
		close(waitChan)
		dockerServer.Close()
		zkServer.Close()
		tempDir, _ := ioutil.TempDir("", "gexec")
		os.RemoveAll(tempDir)
	})

	It("starts with minimum", func() {
		By("using a real docker daemon")
		zk := &runner.Zookeeper{
			Name:         "zookeeper0",
			StartTimeout: 5*time.Second,
		}

		err := zk.Start()
		Expect(err).NotTo(HaveOccurred())
		err = zk.Stop()
		Expect(err).NotTo(HaveOccurred())
	})

	It("starts and stops a docker container with the specified image", func() {
		By("using a real docker daemon")
		zookeeper.Client = nil
		zookeeper.StartTimeout = 5*time.Second

		By("starting zookeeper")
		process = ifrit.Invoke(zookeeper)
		Eventually(process.Ready(), runner.DefaultStartTimeout).Should(BeClosed())
		Consistently(process.Wait(), 5*time.Second).ShouldNot(Receive())

		By("inspecting the container by name")
		container, err := zookeeper.Client.InspectContainer("zookeeper0")
		Expect(err).NotTo(HaveOccurred())

		Expect(container.Name).To(Equal("/zookeeper0"))
		Expect(container.State.Status).To(Equal("running"))
		Expect(container.Config).NotTo(BeNil())
		Expect(container.Config.Image).To(Equal("hyperledger/fabric-zookeeper:latest"))
		Expect(container.ID).To(Equal(zookeeper.ContainerID()))

		Expect(zookeeper.ContainerAddress()).To(Equal(net.JoinHostPort(container.NetworkSettings.IPAddress, "2181")))

		By("getting the container logs")
		Eventually(errBuffer, 5*time.Second).Should(gbytes.Say(`Using config: /conf/zoo.cfg`))

		By("terminating the container")
		err = zookeeper.Stop()
		Expect(err).NotTo(HaveOccurred())
	})

	It("multiples can be started and stopped", func() {
		client, err := docker.NewClientFromEnv()
		zk1 := &runner.Zookeeper{
			Name:         "zookeeper1",
			ZooMyID:      1,
			ZooServers:   "server.1=zookeeper1:2888:3888 server.2=zookeeper2:2888:3888 server.3=zookeeper3:2888:3888",
			StartTimeout: 5*time.Second,
			Client:       client,
		}
		err = zk1.Start()
		Expect(err).NotTo(HaveOccurred())

		zk2 := &runner.Zookeeper{
			Name:         "zookeeper2",
			ZooMyID:      2,
			ZooServers:   "server.1=zookeeper1:2888:3888 server.2=zookeeper2:2888:3888 server.3=zookeeper3:2888:3888",
			StartTimeout: 5*time.Second,
			Client:       client,
		}
		err = zk2.Start()
		Expect(err).NotTo(HaveOccurred())

		zk3 := &runner.Zookeeper{
			Name:         "zookeeper3",
			ZooMyID:      3,
			ZooServers:   "server.1=zookeeper1:2888:3888 server.2=zookeeper2:2888:3888 server.3=zookeeper3:2888:3888",
			StartTimeout: 5*time.Second,
			Client:       client,
		}
		err = zk3.Start()
		Expect(err).NotTo(HaveOccurred())

		container, err := zk1.Client.InspectContainer("zookeeper1")
		Expect(container.Config.Env).To(ContainElement(ContainSubstring("ZOO_MY_ID=1")))
		Expect(container.Config.Env).To(ContainElement(ContainSubstring("ZOO_SERVERS=server.1=zookeeper1:2888:3888 server.2=zookeeper2:2888:3888 server.3=zookeeper3:2888:3888")))
		container, err = zk2.Client.InspectContainer("zookeeper2")
		Expect(container.Config.Env).To(ContainElement(ContainSubstring("ZOO_MY_ID=2")))
		Expect(container.Config.Env).To(ContainElement(ContainSubstring("ZOO_SERVERS=server.1=zookeeper1:2888:3888 server.2=zookeeper2:2888:3888 server.3=zookeeper3:2888:3888")))
		container, err = zk3.Client.InspectContainer("zookeeper3")
		Expect(container.Config.Env).To(ContainElement(ContainSubstring("ZOO_MY_ID=3")))
		Expect(container.Config.Env).To(ContainElement(ContainSubstring("ZOO_SERVERS=server.1=zookeeper1:2888:3888 server.2=zookeeper2:2888:3888 server.3=zookeeper3:2888:3888")))

		err = zk3.Stop()
		Expect(err).NotTo(HaveOccurred())
		err = zk2.Stop()
		Expect(err).NotTo(HaveOccurred())
		err = zk1.Stop()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when setting up multiple zookeeper servers", func() {
		BeforeEach(func() {
			dockerServer.RouteToHandler("POST", "/containers/create", ghttp.RespondWith(http.StatusServiceUnavailable, "no soup for you"))
			dockerServer.RouteToHandler("POST", "/containers/container-id/start", ghttp.RespondWith(http.StatusServiceUnavailable, "let's go"))
		})

		It("generates different names and containers", func() {
			zk := &runner.Zookeeper{
				Client: client,
				Name: "zookeeper0"}
			err := zk.Start()
			Expect(err).To(HaveOccurred())
			Expect(zk.Name).ShouldNot(BeEmpty())
			Expect(zk.Name).To(HaveLen(10))

			zk2 := &runner.Zookeeper{
				Client: client,
				Name: "zookeeper2"}
			err = zk2.Start()
			Expect(err).To(HaveOccurred())
			Expect(zk2.Name).ShouldNot(BeEmpty())
			Expect(zk2.Name).To(HaveLen(10))

			Expect(zk.Name).NotTo(Equal(zk2.Name))
		})
	})
})

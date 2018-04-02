/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/tedsuo/ifrit"
)

var _ = Describe("EndToEnd", func() {
	var (
		testdataDir    string
		ordererProcess ifrit.Process
		peerProcess    ifrit.Process
		peer2Process   ifrit.Process
	)

	BeforeEach(func() {
		var err error

		testdataDir, err = filepath.Abs("testdata")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if peerProcess != nil {
			peerProcess.Signal(syscall.SIGTERM)
		}
		if peer2Process != nil {
			peer2Process.Signal(syscall.SIGTERM)
		}
		if ordererProcess != nil {
			ordererProcess.Signal(syscall.SIGTERM)
		}
	})

	It("does something?", func() {
		By("generating crypto")
		cryptogen := components.Cryptogen()
		cryptogen.Config = filepath.Join(testdataDir, "crypto-config.yaml")
		cryptogen.Output = filepath.Join(testDir, "crypto")
		process := ifrit.Invoke(cryptogen.Generate())
		Eventually(process.Wait()).Should(Receive(BeNil()))

		By("building the orderer block")
		configtxgen := components.ConfigTxGen()
		copyFile(filepath.Join(testdataDir, "configtx.yaml"), filepath.Join(testDir, "configtx.yaml"))
		configtxgen.ConfigDir = testDir
		configtxgen.ChannelID = "systestchannel"
		configtxgen.Profile = "TwoOrgsOrdererGenesis"
		configtxgen.Output = filepath.Join(testDir, "systestchannel.block")
		process = ifrit.Invoke(configtxgen.OutputBlock())
		Eventually(process.Wait()).Should(Receive(BeNil()))

		By("building the channel transaction file")
		configtxgen.Profile = "TwoOrgsChannel"
		configtxgen.ChannelID = "testchannel"
		configtxgen.Output = filepath.Join(testDir, "testchannel.tx")
		process = ifrit.Invoke(configtxgen.OutputCreateChannelTx())
		Eventually(process.Wait()).Should(Receive(BeNil()))

		By("building the channel transaction file for Org1 anchor peer")
		configtxgen.Profile = "TwoOrgsChannel"
		configtxgen.Output = filepath.Join(testDir, "Org1MSPanchors.tx")
		configtxgen.AsOrg = "Org1"
		process = ifrit.Invoke(configtxgen.OutputAnchorPeersUpdate())
		Eventually(process.Wait()).Should(Receive(BeNil()))

		By("building the channel transaction file for Org2 anchor peer")
		configtxgen.Profile = "TwoOrgsChannel"
		configtxgen.Output = filepath.Join(testDir, "Org2MSPanchors.tx")
		configtxgen.AsOrg = "Org2"
		process = ifrit.Invoke(configtxgen.OutputAnchorPeersUpdate())
		Eventually(process.Wait()).Should(Receive(BeNil()))

		By("starting a solo orderer")
		orderer := components.Orderer()
		copyFile(filepath.Join(testdataDir, "core.yaml"), filepath.Join(testDir, "core.yaml"))
		copyFile(filepath.Join(testdataDir, "orderer.yaml"), filepath.Join(testDir, "orderer.yaml"))
		orderer.ConfigDir = testDir
		orderer.OrdererType = "solo"
		orderer.OrdererHome = testDir
		orderer.ListenAddress = "0.0.0.0"
		orderer.ListenPort = "7050"
		orderer.LedgerLocation = testDir
		orderer.GenesisProfile = "TwoOrgsOrdererGenesis"
		orderer.GenesisMethod = "file"
		orderer.GenesisFile = filepath.Join(testDir, "systestchannel.block")
		orderer.LocalMSPId = "OrdererMSP"
		orderer.LocalMSPDir = filepath.Join(testDir, "crypto", "ordererOrganizations", "example.com", "orderers", "orderer.example.com", "msp")
		orderer.LogLevel = "debug"
		ordererProcess = ifrit.Invoke(orderer.New())
		Eventually(ordererProcess.Ready()).Should(BeClosed())
		Consistently(ordererProcess.Wait()).ShouldNot(Receive())

		By("starting a peer for Org1")
		peer := components.Peer()
		peer.ConfigDir = testDir
		peer.LocalMSPID = "Org1MSP"
		peer.PeerID = "peer0.org1.example.com"
		peer.MSPConfigPath = filepath.Join(testDir, "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		peer.PeerAddress = "0.0.0.0:7051"
		peer.PeerListenAddress = "0.0.0.0:10051"
		peer.ProfileEnabled = "true"
		peer.ProfileListenAddress = "0.0.0.0:6060"
		peer.FileSystemPath = filepath.Join(testDir, "peer1")
		peer.PeerGossipBootstrap = "0.0.0.0:10051"
		peer.PeerGossipEndpoint = "0.0.0.0:10051"
		peer.PeerGossipExternalEndpoint = "0.0.0.0:10051"
		peer.PeerGossipOrgLeader = "false"
		peer.PeerGossipUseLeaderElection = "true"
		peer.PeerEventsAddress = "0.0.0.0:10052"
		// peer.PeerChaincodeAddress = "0.0.0.0:10051"
		peer.PeerChaincodeListenAddress = "0.0.0.0:7053"
		peerProcess = ifrit.Invoke(peer.NodeStart())
		Eventually(peerProcess.Ready()).Should(BeClosed())
		Consistently(peerProcess.Wait()).ShouldNot(Receive())

		By("starting a peer for Org2")
		peer2 := components.Peer()
		peer2.ConfigDir = testDir
		peer2.LocalMSPID = "Org2MSP"
		peer2.PeerID = "peer0.org2.example.com"
		peer2.MSPConfigPath = filepath.Join(testDir, "crypto", "peerOrganizations", "org2.example.com", "users", "Admin@org2.example.com", "msp")
		peer2.PeerAddress = "0.0.0.0:8051"
		peer2.PeerListenAddress = "0.0.0.0:11051"
		peer2.ProfileEnabled = "true"
		peer2.ProfileListenAddress = "0.0.0.0:8060"
		peer2.FileSystemPath = filepath.Join(testDir, "peer2")
		peer2.PeerGossipBootstrap = "0.0.0.0:11051"
		peer2.PeerGossipEndpoint = "0.0.0.0:11051"
		peer2.PeerGossipOrgLeader = "false"
		peer2.PeerGossipUseLeaderElection = "true"
		peer2.PeerGossipExternalEndpoint = "0.0.0.0:11051"
		peer2.PeerEventsAddress = "0.0.0.0:11052"
		// peer2.PeerChaincodeAddress = "0.0.0.0:11051"
		peer2.PeerChaincodeListenAddress = "0.0.0.0:8053"
		peer2Process = ifrit.Invoke(peer2.NodeStart())
		Eventually(peer2Process.Ready()).Should(BeClosed())
		Consistently(peer2Process.Wait()).ShouldNot(Receive())

		By("create channel")
		cmd := exec.Command(components.Paths["peer"], "channel", "create", "-c", "testchannel", "-o", "127.0.0.1:7050", "-f", filepath.Join(testDir, "testchannel.tx"))
		cmd.Env = append(
			cmd.Env,
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 5*time.Second).Should(gexec.Exit(0))

		By("fetch genesis block on peer for Org1")
		cmd = exec.Command(components.Paths["peer"], "channel", "fetch", "0", "-o", "127.0.0.1:7050", "-c", "testchannel", "--logging-level", "debug", filepath.Join(testDir, "peer1", "testchannel.block"))
		cmd.Env = append(
			cmd.Env,
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 30*time.Second).Should(gexec.Exit(0))

		By("fetch genesis block on peer for Org2")
		cmd = exec.Command(components.Paths["peer"], "channel", "fetch", "0", "-o", "127.0.0.1:7050", "-c", "testchannel", "--logging-level", "debug", filepath.Join(testDir, "peer2", "testchannel.block"))
		cmd.Env = append(
			cmd.Env,
			"FABRIC_CFG_PATH="+peer2.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer2.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer2.MSPConfigPath,
			"CORE_PEER_ID="+peer2.PeerID,
			"CORE_PEER_ADDRESS="+peer2.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 30*time.Second).Should(gexec.Exit(0))

		By("join channel for Org1")
		cmd = exec.Command(components.Paths["peer"], "channel", "join", "-b", filepath.Join(testDir, "peer1", "testchannel.block"))
		cmd.Env = append(
			cmd.Env,
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 30*time.Second).Should(gexec.Exit(0))

		By("join channel for Org2")
		cmd = exec.Command(components.Paths["peer"], "channel", "join", "-b", filepath.Join(testDir, "peer2", "testchannel.block"))
		cmd.Env = append(
			cmd.Env,
			"FABRIC_CFG_PATH="+peer2.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer2.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer2.MSPConfigPath,
			"CORE_PEER_ID="+peer2.PeerID,
			"CORE_PEER_ADDRESS="+peer2.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 30*time.Second).Should(gexec.Exit(0))

		By("installs chaincode for peer1")
		cmd = exec.Command(components.Paths["peer"], "chaincode", "install", "-n", "mycc", "-v", "1.0", "--logging-level", "debug", "-p", "simple/cmd")
		cmd.Env = append(
			cmd.Env,
			"PATH="+os.Getenv("PATH"),
			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 30*time.Second).Should(gexec.Exit(0))

		By("installs chaincode for peer2")
		cmd = exec.Command(components.Paths["peer"], "chaincode", "install", "-n", "mycc", "-v", "1.0", "--logging-level", "debug", "-p", "simple/cmd")
		cmd.Env = append(
			cmd.Env,
			"PATH="+os.Getenv("PATH"),
			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
			"FABRIC_CFG_PATH="+peer2.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer2.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer2.MSPConfigPath,
			"CORE_PEER_ID="+peer2.PeerID,
			"CORE_PEER_ADDRESS="+peer2.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 30*time.Second).Should(gexec.Exit(0))

		By("instantiate chaincode")
		Consistently(peerProcess.Wait()).ShouldNot(Receive())
		Consistently(peer2Process.Wait()).ShouldNot(Receive())
		cmd = exec.Command(components.Paths["peer"], "chaincode", "instantiate", "-n", "mycc", "-v", "1.0", "-o", "127.0.0.1:7050", "-C", "testchannel", "-c", `{"Args":["init","a","100","b","200"]}`, "-P", `OR ('Org1MSP.peer','Org2MSP.peer')`, "--logging-level", "debug")
		cmd.Env = append(
			cmd.Env,
			"PATH="+os.Getenv("PATH"),
			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 2*time.Minute).Should(gexec.Exit(0))
		Consistently(peerProcess.Wait()).ShouldNot(Receive())
		Consistently(peer2Process.Wait()).ShouldNot(Receive())

		fmt.Printf("\n\n\n-----\n\n\n")
		cmd = exec.Command(components.Paths["peer"], "chaincode", "list", "--installed", "-C", "testchannel")
		cmd.Env = append(
			cmd.Env,
			"PATH="+os.Getenv("PATH"),
			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		fmt.Printf("\n\n\n-%s-\n\n", session.Out.Contents())
		fmt.Printf("\n\n\n-%s-\n\n", session.Err.Contents())
		Consistently(peerProcess.Wait()).ShouldNot(Receive())
		Consistently(peer2Process.Wait()).ShouldNot(Receive())

		fmt.Printf("\n\n\n-----\n\n\n")
		cmd = exec.Command(components.Paths["peer"], "chaincode", "list", "--instantiated", "-C", "testchannel")
		cmd.Env = append(
			cmd.Env,
			"PATH="+os.Getenv("PATH"),
			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Consistently(peerProcess.Wait(), time.Second).ShouldNot(Receive())
		Consistently(peer2Process.Wait(), time.Second).ShouldNot(Receive())

		By("query chaincode")
		// peer chaincode query -C $CHANNEL_NAME -n mycc -c '{"Args":["query","a"]}'
		cmd = exec.Command(components.Paths["peer"], "chaincode", "query", "-n", "mycc", "-v", "1.0", "-C", "testchannel", "-c", `{"Args":["query","a"]}`, "--logging-level", "debug")
		cmd.Env = append(
			cmd.Env,
			"PATH="+os.Getenv("PATH"),
			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
			"FABRIC_CFG_PATH="+peer.ConfigDir,
			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
			"CORE_PEER_ID="+peer.PeerID,
			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 5*time.Second).Should(gexec.Exit(0))

		By("invoke chaincode")
		//peer chaincode invoke -o orderer.example.com:7050 -C $CHANNEL_NAME -n mycc -c '{"Args":["invoke","a","b","10"]}'
		//		cmd = exec.Command(components.Paths["peer"], "chaincode", "invoke", "-o", "127.0.0.1:7050", "-n", "mycc", "-C", "testchannel", "-c", `{"Args":["invoke","a","b","10"]}`)
		//		cmd.Env = append(
		//			cmd.Env,
		//			"PATH="+os.Getenv("PATH"),
		//			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
		//			"FABRIC_CFG_PATH="+peer.ConfigDir,
		//			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
		//			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
		//			"CORE_PEER_ID="+peer.PeerID,
		//			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		//		)
		//		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		//		Expect(err).NotTo(HaveOccurred())
		//		Eventually(session, 5*time.Second).Should(gexec.Exit(0))

		By("update channel")
		//peer channel update -o orderer.example.com:7050 -c $CHANNEL_NAME -f ./channel-artifacts/${CORE_PEER_LOCALMSPID}anchors.tx
		//		cmd = exec.Command(components.Paths["peer"], "channel", "update", "-o", "127.0.0.1:7050", "-c", "testchannel", "-f", filepath.Join(testDir, "Org1MSPanchors.tx")
		//		cmd.Env = append(
		//			cmd.Env,
		//			"PATH="+os.Getenv("PATH"),
		//			"GOPATH="+filepath.Join(testdataDir, "chaincode"),
		//			"FABRIC_CFG_PATH="+peer.ConfigDir,
		//			"CORE_PEER_LOCALMSPID="+peer.LocalMSPID,
		//			"CORE_PEER_MSPCONFIGPATH="+peer.MSPConfigPath,
		//			"CORE_PEER_ID="+peer.PeerID,
		//			"CORE_PEER_ADDRESS="+peer.PeerListenAddress,
		//		)
		//		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		//		Expect(err).NotTo(HaveOccurred())
		//		Eventually(session, time.Minute).Should(gexec.Exit(0))

		By("query chaincode")

		By("invoke chaincode")
	})
})

func copyFile(src, dest string) {
	data, err := ioutil.ReadFile(src)
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(dest, data, 0775)
	Expect(err).NotTo(HaveOccurred())
}

func execute(r ifrit.Runner) (err error) {
	p := ifrit.Invoke(r)
	Eventually(p.Ready()).Should(BeClosed())
	Eventually(p.Wait()).Should(Receive(&err))
	return err
}

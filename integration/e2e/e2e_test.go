/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package e2e

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/integration/world"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

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

	It("executes a basic solo network with 2 orgs", func() {
		By("construct crypto")
		peerOrgs := []*localconfig.Organization{{
			Name:   "Org1",
			ID:     "Org1MSP",
			MSPDir: filepath.Join("crypto", "peerOrganizations", "org1.example.com", "peers", "peer0.org1.example.com", "msp"),
			AnchorPeers: []*localconfig.AnchorPeer{{
				Host: "0.0.0.0",
				Port: 10051,
			}},
		}, {
			Name:   "Org2",
			ID:     "Org2MSP",
			MSPDir: filepath.Join("crypto", "peerOrganizations", "org2.example.com", "peers", "peer0.org2.example.com", "msp"),
			AnchorPeers: []*localconfig.AnchorPeer{{
				Host: "0.0.0.0",
				Port: 11051,
			},
			}},
		}

		ordererOrgs := []*localconfig.Organization{{
			Name:   "OrdererOrg",
			ID:     "OrdererMSP",
			MSPDir: filepath.Join("crypto", "ordererOrganizations", "example.com", "orderers", "orderer.example.com", "msp"),
		}}

		w := world.World{
			Deployment: world.Deployment{
				Channel: "testchannel",
				Chaincode: world.Chaincode{
					Name:     "mycc",
					Version:  "1.0",
					Path:     filepath.Join("simple", "cmd"),
					GoPath:   filepath.Join(testdataDir, "chaincode"),
					ExecPath: os.Getenv("PATH"),
				},
				InitArgs: `{"Args":["init","a","100","b","200"]}`,
				Peers:    []string{"peer0.org1.example.com", "peer0.org2.example.com"},
				Orderer:  "127.0.0.1:7050",
				Policy:   `OR ('Org1MSP.member','Org2MSP.member')`,
			},
			OrdererOrgs: world.Organization{
				Name:   "Orderer",
				Domain: "example.com",
				Orderers: []world.OrdererConfig{{
					Name:        "orderer",
					BrokerCount: 0,
				}},
			},
			PeerOrgs: world.Organization{
				Peers: []world.PeerOrgConfig{{
					Name:          "Org1",
					Domain:        "org1.example.com",
					EnableNodeOUs: false,
					UserCount:     1,
					PeerCount:     1,
				}, {
					Name:          "Org2",
					Domain:        "org2.example.com",
					EnableNodeOUs: false,
					UserCount:     1,
					PeerCount:     1,
				}},
			},
			Profiles: map[string]localconfig.Profile{
				"TwoOrgsChannel": localconfig.Profile{
					Consortium: "SampleConsortium",
					Application: &localconfig.Application{
						Capabilities: map[string]bool{
							"V1_2": true,
						},
						Organizations: peerOrgs,
					},
					Capabilities: map[string]bool{
						"V1_1": true,
					},
				},
				"TwoOrgsOrdererGenesis": localconfig.Profile{
					Application: &localconfig.Application{

						Organizations: ordererOrgs,
						Capabilities: map[string]bool{
							"V1_2": true,
						},
					},
					Orderer: &localconfig.Orderer{
						BatchTimeout: 1 * time.Second,
						BatchSize: localconfig.BatchSize{
							MaxMessageCount:   1,
							AbsoluteMaxBytes:  (uint32)(98 * 1024 * 1024),
							PreferredMaxBytes: (uint32)(512 * 1024),
						},
						Organizations: ordererOrgs,
						OrdererType:   "solo",
						Addresses:     []string{"0.0.0.0:7050"},
						Capabilities: map[string]bool{
							"V1_1": true,
						},
					},
					Consortiums: map[string]*localconfig.Consortium{
						"SampleConsortium": &localconfig.Consortium{
							Organizations: peerOrgs,
						},
					},
					Capabilities: map[string]bool{
						"V1_1": true,
					},
				},
			},
		}

		err := w.BootstrapNetwork(testDir)
		//		w.Construct(testDir)
		Expect(filepath.Join(testDir, "configtx.yaml")).To(BeARegularFile())
		Expect(filepath.Join(testDir, "crypto.yaml")).To(BeARegularFile())

		//		By("generating crypto")
		//		cryptogen := components.Cryptogen()
		//		cryptogen.Config = filepath.Join(testDir, "crypto.yaml")
		//		cryptogen.Output = filepath.Join(testDir, "crypto")
		//		r := cryptogen.Generate()
		//		execute(r)
		Expect(filepath.Join(testDir, "crypto", "peerOrganizations")).To(BeADirectory())
		Expect(filepath.Join(testDir, "crypto", "ordererOrganizations")).To(BeADirectory())
		//
		//		By("building the orderer block")
		//		configtxgen := components.ConfigTxGen()
		//		configtxgen.ConfigDir = testDir
		//		configtxgen.ChannelID = "systestchannel"
		//		configtxgen.Profile = "TwoOrgsOrdererGenesis"
		//		configtxgen.Output = filepath.Join(testDir, "systestchannel.block")
		//		r = configtxgen.OutputBlock()
		//		execute(r)
		Expect(filepath.Join(testDir, "systestchannel.block")).To(BeARegularFile())
		//
		//		By("building the channel transaction file")
		//		configtxgen.Profile = "TwoOrgsChannel"
		//		configtxgen.ChannelID = w.Deployment.Channel
		//		configtxgen.Output = filepath.Join(testDir, "testchannel.tx")
		//		r = configtxgen.OutputCreateChannelTx()
		//		execute(r)
		Expect(filepath.Join(testDir, "testchannel.tx")).To(BeARegularFile())
		//
		//		By("building the channel transaction file for Org1 anchor peer")
		//		configtxgen.Profile = "TwoOrgsChannel"
		//		configtxgen.Output = filepath.Join(testDir, "Org1MSPanchors.tx")
		//		configtxgen.AsOrg = "Org1"
		//		r = configtxgen.OutputAnchorPeersUpdate()
		//		execute(r)
		Expect(filepath.Join(testDir, "Org1MSPanchors.tx")).To(BeARegularFile())
		//
		//		By("building the channel transaction file for Org2 anchor peer")
		//		configtxgen.Profile = "TwoOrgsChannel"
		//		configtxgen.Output = filepath.Join(testDir, "Org2MSPanchors.tx")
		//		configtxgen.AsOrg = "Org2"
		//		r = configtxgen.OutputAnchorPeersUpdate()
		//		execute(r)
		Expect(filepath.Join(testDir, "Org2MSPanchors.tx")).To(BeARegularFile())

		By("starting a zookeeper")
		zookeeper := components.Zookeeper(0)
		err = zookeeper.Start()
		Expect(err).NotTo(HaveOccurred())
		defer zookeeper.Stop()

		//		Eventually(outBuffer1, 30*time.Second).Should(gbytes.Say(`\QWooooo Eeeeeee Ooo Ah Ah Bing Bang Walla Walla Bing Bang\E`))

		By("starting a solo orderer")
		orderer := components.Orderer()
		copyFile(filepath.Join(testdataDir, "core.yaml"), filepath.Join(testDir, "core.yaml"))
		copyFile(filepath.Join(testdataDir, "orderer.yaml"), filepath.Join(testDir, "orderer.yaml"))
		orderer.ConfigDir = testDir
		//		orderer.OrdererType = "solo"
		//		orderer.OrdererHome = testDir
		//		orderer.ListenAddress = "0.0.0.0"
		//		orderer.ListenPort = "7050"
		orderer.LedgerLocation = testDir
		//		orderer.GenesisProfile = "TwoOrgsOrdererGenesis"
		//		orderer.GenesisMethod = "file"
		//		orderer.GenesisFile = filepath.Join(testDir, "systestchannel.block")
		//		orderer.LocalMSPId = "OrdererMSP"
		//		orderer.LocalMSPDir = filepath.Join(testDir, "crypto", "ordererOrganizations", "example.com", "orderers", "orderer.example.com", "msp")
		orderer.LogLevel = "debug"
		ordererProcess = ifrit.Invoke(orderer.New())
		Eventually(ordererProcess.Ready()).Should(BeClosed())
		Consistently(ordererProcess.Wait()).ShouldNot(Receive())

		By("starting a peer for Org1")
		peer := components.Peer()
		peer.ConfigDir = testDir
		//		peer.LocalMSPID = "Org1MSP"
		//		peer.PeerID = "peer0.org1.example.com"
		//		peer.MSPConfigPath = filepath.Join(testDir, "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		//		peer.PeerAddress = "0.0.0.0:7051"
		//		peer.PeerListenAddress = "0.0.0.0:10051"
		//		peer.ProfileEnabled = "true"
		//		peer.ProfileListenAddress = "0.0.0.0:6060"
		//		peer.FileSystemPath = filepath.Join(testDir, "peer1")
		//		peer.PeerGossipBootstrap = "0.0.0.0:10051"
		//		peer.PeerGossipEndpoint = "0.0.0.0:10051"
		//		peer.PeerGossipExternalEndpoint = "0.0.0.0:10051"
		//		peer.PeerGossipOrgLeader = "false"
		//		peer.PeerGossipUseLeaderElection = "true"
		//		peer.PeerEventsAddress = "0.0.0.0:10052"
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
		adminPeer := components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner := adminPeer.CreateChannel(w.Deployment.Channel, filepath.Join(testDir, "testchannel.tx"))
		execute(adminRunner)
		Eventually(ordererRunner.Err(), 5*time.Second).Should(gbytes.Say("Created and starting new chain testchannel"))

		By("fetch genesis block on peer for Org1")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.FetchChannel(w.Deployment.Channel, filepath.Join(testDir, "peer1", "testchannel.block"), "0")
		execute(adminRunner)
		Expect(filepath.Join(testDir, "peer1", "testchannel.block")).To(BeARegularFile())

		By("fetch genesis block on peer for Org2")
		adminPeer2 := components.Peer()
		adminPeer2.ConfigDir = peer2.ConfigDir
		adminPeer2.MSPConfigPath = filepath.Join(testDir, "peer2", "crypto", "peerOrganizations", "org2.example.com", "users", "Admin@org2.example.com", "msp")
		adminRunner2 := adminPeer2.FetchChannel(w.Deployment.Channel, filepath.Join(testDir, "peer2", "testchannel.block"), "0")
		execute(adminRunner2)
		Expect(filepath.Join(testDir, "peer2", "testchannel.block")).To(BeARegularFile())

		By("join channel for Org1")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.JoinChannel(filepath.Join(testDir, "peer1", "testchannel.block"))
		execute(adminRunner)
		Eventually(adminRunner.Err(), 5*time.Second).Should(gbytes.Say("Successfully submitted proposal to join channel"))

		By("join channel for Org2")
		adminPeer2 = components.Peer()
		adminPeer2.ConfigDir = peer2.ConfigDir
		adminPeer2.MSPConfigPath = filepath.Join(testDir, "peer2", "crypto", "peerOrganizations", "org2.example.com", "users", "Admin@org2.example.com", "msp")
		adminRunner2 = adminPeer2.JoinChannel(filepath.Join(testDir, "peer2", "testchannel.block"))
		execute(adminRunner2)
		Eventually(adminRunner2.Err(), 5*time.Second).Should(gbytes.Say("Successfully submitted proposal to join channel"))

		By("installs chaincode for peer1")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminPeer.ExecPath = w.Deployment.Chaincode.ExecPath
		adminPeer.GoPath = w.Deployment.Chaincode.GoPath
		adminRunner = adminPeer.InstallChaincode(w.Deployment.Chaincode.Name, w.Deployment.Chaincode.Version, w.Deployment.Chaincode.Path)
		execute(adminRunner)
		Eventually(peerNodeRunner.Err(), 5*time.Second).Should(gbytes.Say(`\QInstalled Chaincode [mycc] Version [1.0] to peer\E`))

		By("installs chaincode for peer2")
		adminPeer2 = components.Peer()
		adminPeer2.ConfigDir = peer2.ConfigDir
		adminPeer2.MSPConfigPath = filepath.Join(testDir, "peer2", "crypto", "peerOrganizations", "org2.example.com", "users", "Admin@org2.example.com", "msp")
		adminPeer2.ExecPath = w.Deployment.Chaincode.ExecPath
		adminPeer2.GoPath = w.Deployment.Chaincode.GoPath
		adminRunner2 = adminPeer2.InstallChaincode(w.Deployment.Chaincode.Name, w.Deployment.Chaincode.Version, w.Deployment.Chaincode.Path)
		execute(adminRunner2)
		Eventually(peer2NodeRunner.Err(), 5*time.Second).Should(gbytes.Say(`\QInstalled Chaincode [mycc] Version [1.0] to peer\E`))

		By("instantiate chaincode")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.InstantiateChaincode(w.Deployment.Chaincode.Name, w.Deployment.Chaincode.Version, w.Deployment.Orderer, w.Deployment.Channel, w.Deployment.InitArgs, w.Deployment.Policy)
		adminProcess := ifrit.Invoke(adminRunner)
		Eventually(adminProcess.Ready(), 2*time.Second).Should(BeClosed())
		Eventually(adminProcess.Wait(), 5*time.Second).ShouldNot(Receive(BeNil()))

		By("Verify chaincode installed")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.ChaincodeListInstalled()
		execute(adminRunner)
		Eventually(adminRunner.Buffer()).Should(gbytes.Say("Path: simple/cmd"))

		By("Wait for chaincode to complete instantiation")
		listInstantiated := func() bool {
			adminPeer = components.Peer()
			adminPeer.ConfigDir = peer.ConfigDir
			adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
			adminRunner = adminPeer.ChaincodeListInstantiated(w.Deployment.Channel)
			err := execute(adminRunner)
			if err != nil {
				return false
			}
			return strings.Contains(string(adminRunner.Buffer().Contents()), "Path: simple/cmd")
		}
		Eventually(listInstantiated, 30*time.Second, 500*time.Millisecond).Should(BeTrue())

		By("query chaincode")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.QueryChaincode(w.Deployment.Chaincode.Name, w.Deployment.Channel, `{"Args":["query","a"]}`)
		execute(adminRunner)
		Eventually(adminRunner.Buffer()).Should(gbytes.Say("100"))

		By("invoke chaincode")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.InvokeChaincode(w.Deployment.Chaincode.Name, w.Deployment.Channel, `{"Args":["invoke","a","b","10"]}`, w.Deployment.Orderer)
		execute(adminRunner)
		Eventually(adminRunner.Err()).Should(gbytes.Say("Chaincode invoke successful. result: status:200"))

		By("query chaincode")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.QueryChaincode(w.Deployment.Chaincode.Name, w.Deployment.Channel, `{"Args":["query","a"]}`)
		execute(adminRunner)
		Eventually(adminRunner.Buffer()).Should(gbytes.Say("90"))

		By("update channel")
		adminPeer = components.Peer()
		adminPeer.ConfigDir = peer.ConfigDir
		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = adminPeer.UpdateChannel(filepath.Join(testDir, "Org1MSPanchors.tx"), w.Deployment.Channel, w.Deployment.Orderer)
		execute(adminRunner)
		Eventually(adminRunner.Err()).Should(gbytes.Say("Successfully submitted channel update"))
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

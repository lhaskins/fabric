package world_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/hyperledger/fabric/integration/world"

	docker "github.com/fsouza/go-dockerclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		tempDir string
		w       World
		client  *docker.Client
		network *docker.Network
		err     error
	)

	BeforeEach(func() {
		tempDir, err = ioutil.TempDir("", "crypto")
		Expect(err).NotTo(HaveOccurred())
		client, err = docker.NewClientFromEnv()
	})

	AfterEach(func() {
		//		output, err := exec.Command("find", tempDir, "-type", "f").Output()
		//		Expect(err).NotTo(HaveOccurred())
		//		fmt.Printf("\n---\n%s\n---\n", output)
		os.RemoveAll(tempDir)

		for _, cont := range w.RunningContainer {
			cont.Stop()
		}
		for _, localProc := range w.RunningLocalProcess {
			localProc.Signal(syscall.SIGTERM)
		}

		if network != nil {
			client.RemoveNetwork(network.Name)
		}
	})

	It("creates the crypto config file for use with cryptogen", func() {
		pOrg := []*localconfig.Organization{{
			Name:   "Org1",
			ID:     "Org1MSP",
			MSPDir: "some dir",
			AnchorPeers: []*localconfig.AnchorPeer{{
				Host: "some host",
				Port: 1111,
			}, {
				Host: "some host",
				Port: 2222,
			}},
		}, {
			Name:   "Org2",
			ID:     "Org2MSP",
			MSPDir: "some other dir",
			AnchorPeers: []*localconfig.AnchorPeer{{
				Host: "my host",
				Port: 3333,
			}, {
				Host: "some host",
				Port: 4444,
			}},
		}}

		ordererOrgs := Organization{
			Name:    "OrdererOrg1",
			Domain:  "OrdererMSP",
			Profile: "TwoOrgsOrdererGenesis",
			Orderers: []OrdererConfig{{
				Name:                          "orderer0",
				BrokerCount:                   0,
				ZookeeperCount:                1,
				KafkaMinInsyncReplicas:        2,
				KafkaDefaultReplicationFactor: 3,
			}, {
				Name:                          "orderer1",
				BrokerCount:                   0,
				ZookeeperCount:                2,
				KafkaMinInsyncReplicas:        2,
				KafkaDefaultReplicationFactor: 3,
			}},
		}

		peerOrgs := Organization{
			Profile: "TwoOrgsChannel",
			Peers: []PeerOrgConfig{{
				Name:          pOrg[0].Name,
				Domain:        pOrg[0].ID,
				EnableNodeOUs: true,
				UserCount:     2,
				PeerCount:     2,
			}, {
				Name:          pOrg[1].Name,
				Domain:        pOrg[1].ID,
				EnableNodeOUs: true,
				UserCount:     2,
				PeerCount:     2,
			}},
		}

		oOrg := []*localconfig.Organization{{
			Name:   ordererOrgs.Name,
			ID:     ordererOrgs.Domain,
			MSPDir: "orderer dir",
		}}

		crypto := runner.Cryptogen{
			Config: filepath.Join(tempDir, "crypto.yaml"),
			Output: filepath.Join(tempDir, "crypto"),
		}

		deployment := Deployment{
			SystemChannel: "syschannel",
			Channel:       "mychannel",
			Chaincode: Chaincode{
				Name:     "mycc",
				Version:  "1.0",
				Path:     filepath.Join("simple", "cmd"),
				GoPath:   filepath.Join("..", "e2e", "testdata", "chaincode"),
				ExecPath: os.Getenv("PATH"),
			},
			InitArgs: `{"Args":["init","a","100","b","200"]}`,
			Peers:    []string{"peer0.org1.example.com", "peer0.org2.example.com"},
			Policy:   `OR ('Org1MSP.member','Org2MSP.member')`,
		}

		peerProfile := localconfig.Profile{
			Consortium: "MyConsortium",
			Application: &localconfig.Application{
				Organizations: pOrg,
				Capabilities: map[string]bool{
					"V1_2": true,
				},
			},
			Capabilities: map[string]bool{
				"V1_1": true,
			},
		}

		orderer := &localconfig.Orderer{
			BatchTimeout: 1 * time.Second,
			BatchSize: localconfig.BatchSize{
				MaxMessageCount:   1,
				AbsoluteMaxBytes:  (uint32)(98 * 1024 * 1024),
				PreferredMaxBytes: (uint32)(512 * 1024),
			},
			Kafka: localconfig.Kafka{
				Brokers: []string{},
			},
			Organizations: oOrg,
			OrdererType:   "solo",
			Addresses:     []string{"0.0.0.0:7050"},
			Capabilities:  map[string]bool{"V1_1": true},
		}

		ordererProfile := localconfig.Profile{
			Application: &localconfig.Application{
				Organizations: oOrg,
				Capabilities:  map[string]bool{"V1_2": true}},
			Orderer: orderer,
			Consortiums: map[string]*localconfig.Consortium{
				"MyConsortium": &localconfig.Consortium{Organizations: pOrg},
			},
			Capabilities: map[string]bool{"V1_1": true},
		}

		profiles := map[string]localconfig.Profile{
			peerOrgs.Profile:    peerProfile,
			ordererOrgs.Profile: ordererProfile,
		}

		w = World{
			Rootpath:    tempDir,
			Components:  components,
			Cryptogen:   crypto,
			Network:     &docker.Network{},
			Deployment:  deployment,
			OrdererOrgs: ordererOrgs,
			PeerOrgs:    peerOrgs,
			Profiles:    profiles,
		}

		w.Construct()
		Expect(filepath.Join(tempDir, "crypto.yaml")).To(BeARegularFile())

		//Verify that the contents of the files are "golden"
		golden, err := ioutil.ReadFile(filepath.Join("..", "e2e", "testdata", "crypto.yaml.golden"))
		Expect(err).NotTo(HaveOccurred())
		actual, err := ioutil.ReadFile(filepath.Join(tempDir, "crypto.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(golden).To(Equal(actual))

		Expect(filepath.Join(tempDir, "configtx.yaml")).To(BeARegularFile())
		golden, err = ioutil.ReadFile(filepath.Join("..", "e2e", "testdata", "configtx.yaml.golden"))
		Expect(err).NotTo(HaveOccurred())
		actual, err = ioutil.ReadFile(filepath.Join(tempDir, "configtx.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(golden).To(Equal(actual))
	})

	It("start network", func() {
		pOrg := []*localconfig.Organization{{
			Name:   "Org1ExampleCom",
			ID:     "org1.example.com",
			MSPDir: "crypto/peerOrganizations/org1.example.com/msp",
			AnchorPeers: []*localconfig.AnchorPeer{{
				Host: "peer0.org1.example.com",
				Port: 7051,
			}},
		}, {
			Name:   "Org2ExampleCom",
			ID:     "org2.example.com",
			MSPDir: "crypto/peerOrganizations/org2.example.com/msp",
			AnchorPeers: []*localconfig.AnchorPeer{{
				Host: "peer0.org2.example.com",
				Port: 7051,
			}},
		}}

		ordererOrgs := Organization{
			Name:    "ExampleCom",
			Domain:  "example.com",
			Profile: "TwoOrgsOrdererGenesis",
			Orderers: []OrdererConfig{{
				Name:                          "orderer0",
				BrokerCount:                   4,
				ZookeeperCount:                1,
				KafkaMinInsyncReplicas:        2,
				KafkaDefaultReplicationFactor: 3,
			}},
		}

		peerOrgs := Organization{
			Profile: "TwoOrgsChannel",
			Peers: []PeerOrgConfig{{
				Name:          pOrg[0].Name,
				Domain:        pOrg[0].ID,
				EnableNodeOUs: true,
				UserCount:     2,
				PeerCount:     2,
			}, {
				Name:          pOrg[1].Name,
				Domain:        pOrg[1].ID,
				EnableNodeOUs: true,
				UserCount:     2,
				PeerCount:     2,
			}},
		}

		oOrg := []*localconfig.Organization{{
			Name:   ordererOrgs.Name,
			ID:     ordererOrgs.Domain,
			MSPDir: "crypto/ordererOrganizations/example.com/orderers/orderer0.example.com/msp",
		}}

		crypto := runner.Cryptogen{
			Config: filepath.Join(tempDir, "crypto.yaml"),
			Output: filepath.Join(tempDir, "crypto"),
		}

		network, err = client.CreateNetwork(
			docker.CreateNetworkOptions{
				Name:   "mytestnet",
				Driver: "bridge",
			},
		)
		Expect(err).NotTo(HaveOccurred())

		deployment := Deployment{
			SystemChannel: "syschannel",
			Channel:       "mychannel",
			Chaincode: Chaincode{
				Name:     "mycc",
				Version:  "1.0",
				Path:     filepath.Join("simple", "cmd"),
				GoPath:   filepath.Join("..", "e2e", "testdata", "chaincode"),
				ExecPath: os.Getenv("PATH"),
			},
			InitArgs: `{"Args":["init","a","100","b","200"]}`,
			Peers:    []string{"peer0.org1.example.com", "peer0.org2.example.com"},
			Policy:   `OR ('Org1MSP.member','Org2MSP.member')`,
		}

		peerProfile := localconfig.Profile{
			Consortium: "MyConsortium",
			Application: &localconfig.Application{
				Organizations: pOrg,
				Capabilities: map[string]bool{
					"V1_2": true,
				},
			},
			Capabilities: map[string]bool{
				"V1_1": true,
			},
		}

		orderer := &localconfig.Orderer{
			BatchTimeout: 1 * time.Second,
			BatchSize: localconfig.BatchSize{
				MaxMessageCount:   1,
				AbsoluteMaxBytes:  (uint32)(98 * 1024 * 1024),
				PreferredMaxBytes: (uint32)(512 * 1024),
			},
			Kafka: localconfig.Kafka{
				Brokers: []string{},
			},
			Organizations: oOrg,
			OrdererType:   "solo",
			Addresses:     []string{"0.0.0.0:7050"},
			Capabilities:  map[string]bool{"V1_1": true},
		}

		ordererProfile := localconfig.Profile{
			Application: &localconfig.Application{
				Organizations: append(oOrg, pOrg...),
				Capabilities:  map[string]bool{"V1_2": true}},
			Orderer: orderer,
			Consortiums: map[string]*localconfig.Consortium{
				"MyConsortium": &localconfig.Consortium{Organizations: pOrg},
			},
			Capabilities: map[string]bool{"V1_1": true},
		}

		profiles := map[string]localconfig.Profile{
			peerOrgs.Profile:    peerProfile,
			ordererOrgs.Profile: ordererProfile,
		}

		w = World{
			Rootpath:    tempDir,
			Components:  components,
			Cryptogen:   crypto,
			Network:     network,
			Deployment:  deployment,
			OrdererOrgs: ordererOrgs,
			PeerOrgs:    peerOrgs,
			Profiles:    profiles,
		}

		err = w.BootstrapNetwork()
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(tempDir, "configtx.yaml")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "crypto.yaml")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "crypto", "peerOrganizations")).To(BeADirectory())
		Expect(filepath.Join(tempDir, "crypto", "ordererOrganizations")).To(BeADirectory())
		Expect(filepath.Join(tempDir, "syschannel.block")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "mychannel.tx")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "Org1ExampleCom_anchors.tx")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "Org2ExampleCom_anchors.tx")).To(BeARegularFile())

		copyFile(filepath.Join("testdata", "orderer.yaml"), filepath.Join(tempDir, "orderer.yaml"))
		for _, peerOrg := range w.PeerOrgs.Peers {
			for peer := 0; peer < peerOrg.PeerCount; peer++ {
				err = os.Mkdir(filepath.Join(tempDir, fmt.Sprintf("%s_%d", peerOrg.Domain, peer)), 0755)
				Expect(err).NotTo(HaveOccurred())
				copyFile(filepath.Join("testdata", fmt.Sprintf("%s_%d-core.yaml", peerOrg.Domain, peer)), filepath.Join(tempDir, fmt.Sprintf("%s_%d/core.yaml", peerOrg.Domain, peer)))
			}
		}
		w.BuildNetwork()
	})

	It("installs and instantiates chaincode", func() {})

	It("deploys and bootstraps", func() {})
})

func copyFile(src, dest string) {
	data, err := ioutil.ReadFile(src)
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(dest, data, 0774)
	Expect(err).NotTo(HaveOccurred())
}

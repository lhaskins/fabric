package world_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/hyperledger/fabric/integration/world"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		tempDir string
		w       World
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "crypto")
		Expect(err).NotTo(HaveOccurred())

		pOrgProfiles := []*localconfig.Organization{{
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
				Name:        "orderer0",
				BrokerCount: 0,
				ZookeeperCount: 1,
				KafkaMinInsyncReplicas: 2,
				KafkaDefaultReplicationFactor: 3,
			}},
		}

		peerOrgs := Organization{
			Profile: "TwoOrgsChannel",
			Peers: []PeerOrgConfig{{
				Name:          pOrgProfiles[0].Name,
				Domain:        pOrgProfiles[0].ID,
				EnableNodeOUs: true,
				UserCount:     2,
				PeerCount:     2,
			}, {
				Name:          pOrgProfiles[1].Name,
				Domain:        pOrgProfiles[1].ID,
				EnableNodeOUs: true,
				UserCount:     2,
				PeerCount:     2,
			}},
		}

		oOrgProfiles := []*localconfig.Organization{{
			Name:   ordererOrgs.Name,
			ID:     ordererOrgs.Domain,
			MSPDir: "crypto/ordererOrganizations/example.com/msp",
		}}

		crypto := runner.Cryptogen{
			Config: filepath.Join(tempDir, "crypto.yaml"),
			Output: filepath.Join(tempDir, "crypto"),
		}

		client, err = docker.NewClientFromEnv()
		network, err = client.CreateNetwork(
			docker.CreateNetworkOptions{
				Name:   "mytestnet",
				Driver: "bridge",
			},
		)

		w = World{
			Components: components,
			Cryptogen:  crypto,
			Network: network,
			Deployment: Deployment{
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
			},
			OrdererOrgs: ordererOrgs,
			PeerOrgs:    peerOrgs,
			Profiles: map[string]localconfig.Profile{
				peerOrgs.Profile: localconfig.Profile{
					Consortium: "MyConsortium",
					Application: &localconfig.Application{
						Organizations: pOrgProfiles,
						Capabilities: map[string]bool{
							"V1_2": true,
						},
					},
					Capabilities: map[string]bool{
						"V1_1": true,
					},
				},
				ordererOrgs.Profile: localconfig.Profile{
					Application: &localconfig.Application{
						Organizations: append(oOrgProfiles, pOrgProfiles...),
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
						Kafka: localconfig.Kafka{
							Brokers: []string{},
						},
						Organizations: oOrgProfiles,
						OrdererType:   "solo",
						Addresses:     []string{"0.0.0.0:7050"},
						Capabilities: map[string]bool{
							"V1_1": true,
						},
					},
					Consortiums: map[string]*localconfig.Consortium{
						"MyConsortium": &localconfig.Consortium{
							Organizations: pOrgProfiles,
						},
					},
					Capabilities: map[string]bool{
						"V1_1": true,
					},
				},
			},
		}
	})

	AfterEach(func() {
		//		output, err := exec.Command("find", tempDir, "-type", "f").Output()
		//		Expect(err).NotTo(HaveOccurred())
		//		fmt.Printf("\n---\n%s\n---\n", output)
		os.RemoveAll(tempDir)
	})

	It("creates the crypto config file for use with cryptogen", func() {
		w.Construct(tempDir)
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

	FIt("start network", func() {
		err := w.BootstrapNetwork(tempDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(tempDir, "configtx.yaml")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "crypto.yaml")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "crypto", "peerOrganizations")).To(BeADirectory())
		Expect(filepath.Join(tempDir, "crypto", "ordererOrganizations")).To(BeADirectory())
		Expect(filepath.Join(tempDir, "syschannel.block")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "mychannel.tx")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "Org1ExampleCom_anchors.tx")).To(BeARegularFile())
		Expect(filepath.Join(tempDir, "Org2ExampleCom_anchors.tx")).To(BeARegularFile())

		err := w.BuildNetwork()
		Expect(err).NotTo(HaveOccurred())
	})

	It("installs and instantiates chaincode", func() {})

	It("deploys and bootstraps", func() {})
})

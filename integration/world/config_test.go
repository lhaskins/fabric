package world_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	. "github.com/hyperledger/fabric/integration/world"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "crypto")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		//		output, err := exec.Command("find", tempDir, "-type", "f").Output()
		//		Expect(err).NotTo(HaveOccurred())
		//		fmt.Printf("\n---\n%s\n---\n", output)
		os.RemoveAll(tempDir)
	})

	Describe("Construct", func() {
		It("creates the crypto config file for use with cryptogen", func() {
			pOrgProfiles := []*localconfig.Organization{{
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

			oOrgProfiles := []*localconfig.Organization{{
				Name:   "OrdererOrg1",
				ID:     "OrdererMSP",
				MSPDir: "orderer dir",
			}}

			w := World{
				Deployment: Deployment{
					Channel: "mychannel",
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
				OrdererOrgs: Organization{
					Name:   "ExampleCom",
					Domain: "example.com",
					Orderers: []OrdererConfig{{
						Name:        "orderer0",
						BrokerCount: 0,
					}},
				},
				PeerOrgs: Organization{
					Peers: []PeerOrgConfig{{
						Name:          "Org1ExampleCom",
						Domain:        "org1.example.com",
						EnableNodeOUs: true,
						UserCount:     2,
						PeerCount:     2,
					}, {
						Name:          "Org2ExampleCom",
						Domain:        "org2.example.com",
						EnableNodeOUs: true,
						UserCount:     2,
						PeerCount:     2,
					}},
				},
				Profiles: Profiles{map[string]localconfig.Profile{
					"TwoOrgsChannel": localconfig.Profile{
						Consortium: "MyConsortium",
						Application: &localconfig.Application{
							Capabilities: map[string]bool{
								"V1_2": true,
							},
							Organizations: append(oOrgProfiles, pOrgProfiles...),
						},
						Capabilities: map[string]bool{
							"V1_1": true,
						},
					},
					"TwoOrgsOrdererGenesis": localconfig.Profile{
						Application: &localconfig.Application{
							Organizations: oOrgProfiles,
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
				}},
			}

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
	})
})

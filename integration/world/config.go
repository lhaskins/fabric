/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package world

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/template"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	yaml "gopkg.in/yaml.v2"
)

type Profile struct {
	Profiles map[string]localconfig.Profile `yaml:"Profiles"`
}

type OrdererConfig struct {
	OrganizationName              string
	ProfileName                   string
	Domain                        string
	OrdererNames                  []string
	BrokerCount                   int // 0 is solo
	ZookeeperCount                int
	KafkaMinInsyncReplicas        int
	KafkaDefaultReplicationFactor int
}

type PeerOrgConfig struct {
	OrganizationName string
	ProfileName      string
	Domain           string
	EnableNodeOUs    bool
	UserCount        int
	PeerCount        int
}

type Stopper interface {
	Stop() error
}

type World struct {
	Rootpath           string
	Components         *Components
	Network            *docker.Network
	OrdererProfileName string
	ChannelProfileName string
	OrdererOrgs        []OrdererConfig
	PeerOrgs           []PeerOrgConfig
	Profiles           map[string]localconfig.Profile
	Cryptogen          runner.Cryptogen
	Deployment         Deployment

	LocalStoppers []Stopper
	LocalProcess  []ifrit.Process
}

type Chaincode struct {
	Name     string
	Path     string
	Version  string
	GoPath   string
	ExecPath string
}

type Deployment struct {
	Chaincode     Chaincode
	SystemChannel string
	Channel       string
	InitArgs      string
	Policy        string
	Orderer       string
	Peers         []string
}

func (w *World) Construct() {
	var ordererCrypto = `
OrdererOrgs:{{range .OrdererOrgs}}
  - Name: {{.OrganizationName}}
    Domain: {{.Domain}}
    CA:
        Country: US
        Province: California
        Locality: San Francisco
    Specs:{{range .OrdererNames}}
      - Hostname: {{.}}{{end}}
{{end}}`

	var peerCrypto = `
PeerOrgs:{{range .PeerOrgs}}
  - Name: {{.OrganizationName}}
    Domain: {{.Domain}}
    EnableNodeOUs: {{.EnableNodeOUs}}
    CA:
        Country: US
        Province: California
        Locality: San Francisco
    Template:
      Count: {{.PeerCount}}
    Users:
      Count: {{.UserCount}}
{{end}}`

	// Generates the crypto config
	buf := &bytes.Buffer{}
	w.buildTemplate(buf, ordererCrypto)
	w.buildTemplate(buf, peerCrypto)
	err := ioutil.WriteFile(filepath.Join(w.Rootpath, "crypto.yaml"), buf.Bytes(), 0644)
	Expect(err).NotTo(HaveOccurred())

	// Generates the configtx config
	type profiles struct {
		Profiles map[string]localconfig.Profile `yaml:"Profiles"`
	}
	profileData, err := yaml.Marshal(&profiles{w.Profiles})
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(filepath.Join(w.Rootpath, "configtx.yaml"), profileData, 0644)
	Expect(err).NotTo(HaveOccurred())
}

func (w *World) buildTemplate(writer io.Writer, orgTemplate string) {
	tmpl, err := template.New("org").Parse(orgTemplate)
	Expect(err).NotTo(HaveOccurred())
	err = tmpl.Execute(writer, w)
	Expect(err).NotTo(HaveOccurred())
}

func (w *World) BootstrapNetwork() {
	w.Construct()

	w.Cryptogen.Path = w.Components.Paths["cryptogen"]
	r := w.Cryptogen.Generate()
	execute(r)

	configtxgen := runner.Configtxgen{
		Path:      w.Components.Paths["configtxgen"],
		ChannelID: w.Deployment.SystemChannel,
		Profile:   w.OrdererProfileName,
		ConfigDir: w.Rootpath,
		Output:    filepath.Join(w.Rootpath, fmt.Sprintf("%s_block.pb", w.Deployment.SystemChannel)),
	}
	r = configtxgen.OutputBlock()
	execute(r)

	configtxgen = runner.Configtxgen{
		Path:      w.Components.Paths["configtxgen"],
		ChannelID: w.Deployment.Channel,
		Profile:   w.ChannelProfileName,
		ConfigDir: w.Rootpath,
		Output:    filepath.Join(w.Rootpath, fmt.Sprintf("%s_tx.pb", w.Deployment.Channel)),
	}
	r = configtxgen.OutputCreateChannelTx()
	execute(r)

	for _, peer := range w.PeerOrgs {
		configtxgen = runner.Configtxgen{
			Path:      w.Components.Paths["configtxgen"],
			ChannelID: w.Deployment.Channel,
			AsOrg:     peer.OrganizationName,
			Profile:   w.ChannelProfileName,
			ConfigDir: w.Rootpath,
			Output:    filepath.Join(w.Rootpath, fmt.Sprintf("%s_anchors_update_tx.pb", peer.OrganizationName)),
		}
		r = configtxgen.OutputAnchorPeersUpdate()
		execute(r)
	}
}

func (w *World) BuildNetwork() {
	w.ordererNetwork()
	w.peerNetwork()
}

func (w *World) ordererNetwork() {
	var (
		zookeepers, kafkaBrokerList []string
		zooServers                  []string
		z                           *runner.Zookeeper
		kafkas                      []*runner.Kafka
		o                           *runner.Orderer
		err                         error
		zks                         []*runner.Zookeeper
	)

	o = w.Components.Orderer()
	o.ConfigDir = w.Rootpath
	o.LedgerLocation = filepath.Join(w.Rootpath, "ledger")
	o.LogLevel = "debug"
	for _, orderer := range w.OrdererOrgs {
		if orderer.BrokerCount != 0 {
			for id := 1; id <= orderer.ZookeeperCount; id++ {
				// Start zookeeper
				z = w.Components.Zookeeper(id, w.Network)
				zooServers = append(zooServers, fmt.Sprintf("server.%d=zookeeper%d:2888:3888", id, id))
				zookeepers = append(zookeepers, fmt.Sprintf("zookeeper%d:2181", id))
				w.LocalStoppers = append(w.LocalStoppers, z)
				zks = append(zks, z)
			}
			for _, zk := range zks {
				zk.ZooServers = strings.Join(zooServers, " ")
				err = zk.Start()
				Expect(err).NotTo(HaveOccurred())
			}

			time.Sleep(2 * time.Second)
			for id := 1; id <= orderer.BrokerCount; id++ {
				// Start Kafka Broker
				k := w.Components.Kafka(id, w.Network)
				localKafkaAddress := w.Profiles[w.OrdererProfileName].Orderer.Kafka.Brokers[id-1]
				k.HostPort, err = strconv.Atoi(strings.Split(localKafkaAddress, ":")[1])
				Expect(err).NotTo(HaveOccurred())
				k.KafkaMinInsyncReplicas = orderer.KafkaMinInsyncReplicas
				k.KafkaDefaultReplicationFactor = orderer.KafkaDefaultReplicationFactor
				k.KafkaAdvertisedListeners = localKafkaAddress
				k.KafkaZookeeperConnect = strings.Join(zookeepers, ",")
				k.LogLevel = "debug"
				err = k.Start()
				Expect(err).NotTo(HaveOccurred())

				w.LocalStoppers = append(w.LocalStoppers, k)
				kafkas = append(kafkas, k)
				kafkaBrokerList = append(kafkaBrokerList, k.HostAddress)
			}
		}

		ordererRunner := o.New()
		ordererProcess := ifrit.Invoke(ordererRunner)
		Eventually(ordererProcess.Ready()).Should(BeClosed())
		Consistently(ordererProcess.Wait()).ShouldNot(Receive())
		if orderer.BrokerCount != 0 {
			Eventually(ordererRunner.Err(), 30*time.Second).Should(gbytes.Say("Start phase completed successfully"))
		}

		w.LocalProcess = append(w.LocalProcess, ordererProcess)
	}
}

func (w *World) peerNetwork() {
	var p *runner.Peer

	for _, peerOrg := range w.PeerOrgs {
		for peer := 0; peer < peerOrg.PeerCount; peer++ {
			p = w.Components.Peer()
			p.ConfigDir = filepath.Join(w.Rootpath, fmt.Sprintf("%s_%d", peerOrg.Domain, peer))
			peerProcess := ifrit.Invoke(p.NodeStart())
			Eventually(peerProcess.Ready()).Should(BeClosed())
			Consistently(peerProcess.Wait()).ShouldNot(Receive())
			w.LocalProcess = append(w.LocalProcess, peerProcess)
		}
	}
}

func (w *World) SetupChannel() error {
	var p *runner.Peer

	p = w.Components.Peer()
	p.ConfigDir = filepath.Join(w.Rootpath, "org1.example.com_0")
	p.LogLevel = "debug"
	p.MSPConfigPath = filepath.Join(w.Rootpath, "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
	adminRunner := p.CreateChannel(w.Deployment.Channel, filepath.Join(w.Rootpath, fmt.Sprintf("%s_tx.pb", w.Deployment.Channel)), w.Deployment.Orderer)
	execute(adminRunner)

	for _, peerOrg := range w.PeerOrgs {
		for peer := 0; peer < peerOrg.PeerCount; peer++ {
			p = w.Components.Peer()
			peerDir := fmt.Sprintf("%s_%d", peerOrg.Domain, peer)
			p.LogLevel = "debug"
			p.ConfigDir = filepath.Join(w.Rootpath, peerDir)
			p.MSPConfigPath = filepath.Join(w.Rootpath, "crypto", "peerOrganizations", peerOrg.Domain, "users", fmt.Sprintf("Admin@%s", peerOrg.Domain), "msp")
			adminRunner = p.FetchChannel(w.Deployment.Channel, filepath.Join(w.Rootpath, peerDir, fmt.Sprintf("%s_block.pb", w.Deployment.Channel)), "0", w.Deployment.Orderer)
			execute(adminRunner)
			Expect(adminRunner.Err()).To(gbytes.Say("Received block: 0"))

			adminRunner = p.JoinChannel(filepath.Join(w.Rootpath, peerDir, fmt.Sprintf("%s_block.pb", w.Deployment.Channel)))
			execute(adminRunner)
			Expect(adminRunner.Err()).To(gbytes.Say("Successfully submitted proposal to join channel"))

			p.ExecPath = w.Deployment.Chaincode.ExecPath
			p.GoPath = w.Deployment.Chaincode.GoPath
			adminRunner = p.InstallChaincode(w.Deployment.Chaincode.Name, w.Deployment.Chaincode.Version, w.Deployment.Chaincode.Path)
			execute(adminRunner)
			Expect(adminRunner.Err()).To(gbytes.Say(`\QInstalled remotely response:<status:200 payload:"OK" >\E`))
		}
	}

	p = w.Components.Peer()
	p.ConfigDir = filepath.Join(w.Rootpath, "org1.example.com_0")
	p.MSPConfigPath = filepath.Join(w.Rootpath, "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
	adminRunner = p.InstantiateChaincode(w.Deployment.Chaincode.Name, w.Deployment.Chaincode.Version, w.Deployment.Orderer, w.Deployment.Channel, w.Deployment.InitArgs, w.Deployment.Policy)
	execute(adminRunner)

	listInstantiated := func() bool {
		p = w.Components.Peer()
		p.ConfigDir = filepath.Join(w.Rootpath, "org1.example.com_0")
		p.MSPConfigPath = filepath.Join(w.Rootpath, "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		adminRunner = p.ChaincodeListInstantiated(w.Deployment.Channel)
		execute(adminRunner)
		return strings.Contains(string(adminRunner.Buffer().Contents()), fmt.Sprintf("Path: %s", w.Deployment.Chaincode.Path))
	}
	Eventually(listInstantiated, 30*time.Second, 500*time.Millisecond).Should(BeTrue())

	return nil
}

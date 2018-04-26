package world

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/alecthomas/template"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	yaml "gopkg.in/yaml.v2"
)

type Profile struct {
	Profiles map[string]localconfig.Profile `yaml:"Profiles"`
}

type OrdererConfig struct {
	Name                          string
	BrokerCount                   int // 0 is solo
	ZookeeperCount                int
	KafkaMinInsyncReplicas        int
	KafkaDefaultReplicationFactor int
}

type PeerOrgConfig struct {
	Name          string
	Domain        string
	EnableNodeOUs bool
	UserCount     int
	PeerCount     int
}

type Organization struct {
	Name     string
	Domain   string
	Profile  string
	Orderers []OrdererConfig
	Peers    []PeerOrgConfig
}

type container interface {
	Stop() error
}

type World struct {
	Rootpath            string
	Components          *Components
	Network             *docker.Network
	OrdererOrgs         Organization
	PeerOrgs            Organization
	Profiles            map[string]localconfig.Profile
	Cryptogen           runner.Cryptogen
	Deployment          Deployment
	RunningContainer    []container
	RunningLocalProcess []ifrit.Process
}

type Chaincode struct {
	Name     string
	Path     string
	Version  string
	GoPath   string
	ExecPath string
}

// install the chaincodes
type Deployment struct {
	Chaincode     Chaincode
	SystemChannel string
	Channel       string
	InitArgs      string
	Policy        string
	Orderer       string
	Peers         []string
	// chaincode name to peer names map[string][]string
}

func (w *World) Construct() {
	var ordererCrypto = `
OrdererOrgs:
  - Name: {{.Name}}
    Domain: {{.Domain}}
    CA:
        Country: US
        Province: California
        Locality: San Francisco
    Specs: {{range .Orderers}}
      - Hostname: {{.Name}}{{end}}
`

	var peerCrypto = `
PeerOrgs: {{range .Peers}}
  - Name: {{.Name}}
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
	w.OrdererOrgs.buildTemplate(buf, ordererCrypto)
	w.PeerOrgs.buildTemplate(buf, peerCrypto)
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

func (o *Organization) buildTemplate(w io.Writer, orgTemplate string) {
	tmpl, err := template.New("org").Parse(orgTemplate)
	Expect(err).NotTo(HaveOccurred())
	err = tmpl.Execute(w, o)
	Expect(err).NotTo(HaveOccurred())
}

func (w *World) BootstrapNetwork() (err error) {
	w.Construct()

	w.Cryptogen.Path = w.Components.Paths["cryptogen"]
	r := w.Cryptogen.Generate()
	err = execute(r)
	if err != nil {
		return err
	}

	configtxgen := runner.ConfigTxGen{
		Path:      w.Components.Paths["configtxgen"],
		ChannelID: w.Deployment.SystemChannel,
		Profile:   w.OrdererOrgs.Profile,
		ConfigDir: w.Rootpath,
		Output:    filepath.Join(w.Rootpath, fmt.Sprintf("%s.block", w.Deployment.SystemChannel)),
	}
	r = configtxgen.OutputBlock()
	err = execute(r)
	if err != nil {
		return err
	}

	configtxgen = runner.ConfigTxGen{
		Path:      w.Components.Paths["configtxgen"],
		ChannelID: w.Deployment.Channel,
		Profile:   w.PeerOrgs.Profile,
		ConfigDir: w.Rootpath,
		Output:    filepath.Join(w.Rootpath, fmt.Sprintf("%s.tx", w.Deployment.Channel)),
	}
	r = configtxgen.OutputCreateChannelTx()
	err = execute(r)
	if err != nil {
		return err
	}

	for _, peer := range w.PeerOrgs.Peers {
		configtxgen = runner.ConfigTxGen{
			Path:      w.Components.Paths["configtxgen"],
			ChannelID: w.Deployment.Channel,
			AsOrg:     peer.Name,
			Profile:   w.PeerOrgs.Profile,
			ConfigDir: w.Rootpath,
			Output:    filepath.Join(w.Rootpath, fmt.Sprintf("%s_anchors.tx", peer.Name)),
		}
		r = configtxgen.OutputAnchorPeersUpdate()
		err = execute(r)
	}
	return err
}

func (w *World) BuildNetwork() {
	w.ordererNetwork()
	w.peerNetwork()
}

func (w *World) ordererNetwork() {
	var (
		zookeepers, kafkaBrokerList []string
		z                           *runner.Zookeeper
		kafkas                      []*runner.Kafka
		o                           *runner.Orderer
	)

	o = w.Components.Orderer()
	for orderer := range w.OrdererOrgs.Orderers {
		if w.OrdererOrgs.Orderers[orderer].BrokerCount != 0 {
			for id := 1; id <= w.OrdererOrgs.Orderers[orderer].ZookeeperCount; id++ {
				z = w.Components.Zookeeper(id, w.Network)
				zookeepers = append(zookeepers, fmt.Sprintf("zookeeper%d:2181", id))
				err := z.Start()
				Expect(err).NotTo(HaveOccurred())
				w.RunningContainer = append(w.RunningContainer, z)
			}

			for id := 1; id <= w.OrdererOrgs.Orderers[orderer].BrokerCount; id++ {
				k := w.Components.Kafka(id, w.Network)
				k.KafkaMinInsyncReplicas = w.OrdererOrgs.Orderers[orderer].KafkaMinInsyncReplicas
				k.KafkaDefaultReplicationFactor = w.OrdererOrgs.Orderers[orderer].KafkaDefaultReplicationFactor
				k.KafkaZookeeperConnect = strings.Join(zookeepers, ",")
				err := k.Start()
				Expect(err).NotTo(HaveOccurred())

				w.RunningContainer = append(w.RunningContainer, k)
				kafkas = append(kafkas, k)
				kafkaBrokerList = append(kafkaBrokerList, k.HostAddress)
			}
		}

		o.ConfigDir = w.Rootpath
		o.LedgerLocation = filepath.Join(w.Rootpath, "ledger")
		o.ConfigtxOrdererKafkaBrokers = fmt.Sprintf("[%s]", strings.Join(kafkaBrokerList, ","))
		ordererProcess := ifrit.Invoke(o.New())
		Eventually(ordererProcess.Ready()).Should(BeClosed())
		Consistently(ordererProcess.Wait()).ShouldNot(Receive())
		w.RunningLocalProcess = append(w.RunningLocalProcess, ordererProcess)
	}
}

func (w *World) peerNetwork() {
	var p *runner.Peer

	for _, peerOrg := range w.PeerOrgs.Peers {
		for peer := 0; peer < peerOrg.PeerCount; peer++ {
			p = w.Components.Peer()
			p.ConfigDir = filepath.Join(w.Rootpath, fmt.Sprintf("%s_%d", peerOrg.Domain, peer))
			peerProcess := ifrit.Invoke(p.NodeStart())
			Eventually(peerProcess.Ready()).Should(BeClosed())
			Consistently(peerProcess.Wait()).ShouldNot(Receive())
			w.RunningLocalProcess = append(w.RunningLocalProcess, peerProcess)
		}
	}
}

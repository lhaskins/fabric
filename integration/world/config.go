package world

import (
	"bytes"
	"fmt"
	"strings"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/alecthomas/template"
	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
	docker "github.com/fsouza/go-dockerclient"
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
	Name        string
	Domain      string
	Profile     string
	Orderers    []OrdererConfig
	Peers       []PeerOrgConfig
}

type World struct {
	Components  *Components
	Network     *docker.Network
	OrdererOrgs Organization
	PeerOrgs    Organization
	Profiles    map[string]localconfig.Profile
	Cryptogen   runner.Cryptogen
	Deployment  Deployment
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

func (w *World) Construct(rootpath string) {
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
	err := ioutil.WriteFile(filepath.Join(rootpath, "crypto.yaml"), buf.Bytes(), 0644)
	Expect(err).NotTo(HaveOccurred())

	// Generates the configtx config
	type profiles struct {
		Profiles map[string]localconfig.Profile `yaml:"Profiles"`
	}
	profileData, err := yaml.Marshal(&profiles{w.Profiles})
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(filepath.Join(rootpath, "configtx.yaml"), profileData, 0644)
	Expect(err).NotTo(HaveOccurred())
}

func (o *Organization) buildTemplate(w io.Writer, orgTemplate string) {
	tmpl, err := template.New("org").Parse(orgTemplate)
	Expect(err).NotTo(HaveOccurred())
	err = tmpl.Execute(w, o)
	Expect(err).NotTo(HaveOccurred())
}

func (w *World) BootstrapNetwork(rootpath string) (err error) {
	w.Construct(rootpath)

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
		ConfigDir: rootpath,
		Output:    filepath.Join(rootpath, fmt.Sprintf("%s.block", w.Deployment.SystemChannel)),
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
		ConfigDir: rootpath,
		Output:    filepath.Join(rootpath, fmt.Sprintf("%s.tx", w.Deployment.Channel)),
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
			ConfigDir: rootpath,
			Output:    filepath.Join(rootpath, fmt.Sprintf("%s_anchors.tx", peer.Name)),
		}
		r = configtxgen.OutputAnchorPeersUpdate()
		err = execute(r)
	}
	return err
}

func (w *World) BuildNetwork(rootpath string) (err error){
	var (
		z       *runner.Zookeeper
		k       *runner.Kafka
		kafkas []*runner.Kafka
//		o *runner.Orderer
//		p *runner.Peer

		zookeepers []string
		zooservers []string
	)

	o = w.Components.Orderer()
	for orderer := range(w.OrdererOrgs.Orderers) {
		o.OrdererType = "solo"
		if w.OrdererOrgs.Orderers[orderer].BrokerCount !=  0 {
			o.OrdererType = "kafka"
			for id := 1; id <= w.OrdererOrgs.Orderers[orderer].ZookeeperCount; id++ {
				z = w.Components.Zookeeper(id, w.Network)
				err = z.Start()
				zookeepers = append(zookeepers, fmt.Sprintf("zookeeper%d:2181 ", id))
				zooservers = append(zooservers, fmt.Sprintf("server.%d=zookeeper%d:2888:3888 ", id, id))
			}
			fmt.Println("Zookeeper string:", zookeepers)
			fmt.Println("Zooservers string:", zooservers)
			for id := 1; id <= w.OrdererOrgs.Orderers[orderer].BrokerCount; id++ {
				k = w.Components.Kafka(id, w.Network)
				k.KafkaMinInsyncReplicas = w.OrdererOrgs.Orderers[orderer].KafkaMinInsyncReplicas
				k.KafkaDefaultReplicationFactor = w.OrdererOrgs.Orderers[orderer].KafkaDefaultReplicationFactor
				k.KafkaZookeeperConnect = strings.Join(zookeepers, " ")
				err = k.Start()
				kafkas = append(kafkas, k)
			}
			fmt.Println("kafkas string:", kafkas)
		}

		o.ConfigDir = rootPath
		o.OrdererHome = rootpath
		o.ListenAddress = "0.0.0.0"
		o.ListenPort = "7050"
		o.LedgerLocation = rootPath
		o.GenesisProfile = w.OrdererOrgs.Profile
		o.GenesisMethod = "file"
		o.GenesisFile = filepath.Join(rootPath, fmt.Sprintf("%s.block", w.Deployment.SystemChannel))
		o.LocalMSPId = w.OrdererOrgs.Domain
//		o.LocalMSPDir = filepath.Join(rootPath, "crypto", "ordererOrganizations", "example.com", "orderers", "orderer.example.com", "msp")
		o.LocalMSPDir = w.Profiles[w.OrdererOrgs.Profile].MSPDir
		o.LogLevel = "debug"
	}
	return err
}

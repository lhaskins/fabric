package world

import (
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/alecthomas/template"
	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
)

type Profiles Profile

type Profile struct {
	Profiles map[string]localconfig.Profile `yaml:"Profiles"`
}

type OrdererConfig struct {
	Name        string
	BrokerCount int // 0 is solo
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
	Orderers []OrdererConfig
	Peers    []PeerOrgConfig
}

type World struct {
	OrdererOrgs Organization
	PeerOrgs    Organization
	Profiles    Profiles `yaml:"Profiles"`
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
	Chaincode Chaincode
	Channel   string
	InitArgs  string
	Policy    string
	Orderer   string
	Peers     []string
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
	profileData, err := yaml.Marshal(&w.Profiles)
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

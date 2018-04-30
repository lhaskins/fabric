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
		err                         error
	)

	o = w.Components.Orderer()
	for _, orderer := range w.OrdererOrgs.Orderers {
		if orderer.BrokerCount != 0 {
			for id := 1; id <= orderer.ZookeeperCount; id++ {
				z = w.Components.Zookeeper(id, w.Network)
				z.ZooServers = "server.1=zookeeper1:2888:3888"
				zookeepers = append(zookeepers, fmt.Sprintf("zookeeper%d:2181/kafka", id))
				err := z.Start()
				Expect(err).NotTo(HaveOccurred())
				w.RunningContainer = append(w.RunningContainer, z)
			}

			for id := 1; id <= orderer.BrokerCount; id++ {
				k := w.Components.Kafka(id, w.Network)
				localKafkaAddress := w.Profiles[w.OrdererOrgs.Profile].Orderer.Kafka.Brokers[id-1]
				k.HostPort, err = strconv.Atoi(strings.Split(localKafkaAddress, ":")[1])
				Expect(err).NotTo(HaveOccurred())
				k.KafkaMinInsyncReplicas = orderer.KafkaMinInsyncReplicas
				k.KafkaDefaultReplicationFactor = orderer.KafkaDefaultReplicationFactor
				k.KafkaZookeeperConnect = strings.Join(zookeepers, ",")
				k.LogLevel = "debug"
				err = k.Start()
				Expect(err).NotTo(HaveOccurred())

				w.RunningContainer = append(w.RunningContainer, k)
				kafkas = append(kafkas, k)
				kafkaBrokerList = append(kafkaBrokerList, k.HostAddress)
			}
		}

		o.ConfigDir = w.Rootpath
		o.LedgerLocation = filepath.Join(w.Rootpath, "ledger")
		o.ConfigtxOrdererKafkaBrokers = fmt.Sprintf("[%s]", strings.Join(kafkaBrokerList, ","))
		o.LogLevel = "debug"
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

func (w *World) SetupChannel() error {
	var p *runner.Peer

	p = w.Components.Peer()
	p.ConfigDir = filepath.Join(w.Rootpath, "org1.example.com_0")
	p.LogLevel = "debug"
	p.MSPConfigPath = filepath.Join(w.Rootpath, "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
	adminRunner := p.CreateChannel(w.Deployment.Channel, filepath.Join(w.Rootpath, fmt.Sprintf("%s.tx", w.Deployment.Channel)))
	execute(adminRunner)
	//	Eventually(ordererRunner.Err(), 5*time.Second).Should(gbytes.Say(fmt.Sprintf("Created and starting new chain %s", w.Deployment.Channel)))

	for _, peerOrg := range w.PeerOrgs.Peers {
		for peer := 0; peer < peerOrg.PeerCount; peer++ {
			p = w.Components.Peer()
			peerDir := fmt.Sprintf("%s_%d", peerOrg.Domain, peer)
			p.LogLevel = "debug"
			p.ConfigDir = filepath.Join(w.Rootpath, peerDir)
			p.MSPConfigPath = filepath.Join(w.Rootpath, "crypto", "peerOrganizations", peerOrg.Domain, "users", fmt.Sprintf("Admin@%s", peerOrg.Domain), "msp")
			adminRunner = p.FetchChannel(w.Deployment.Channel, filepath.Join(w.Rootpath, peerDir, fmt.Sprintf("%s.block", w.Deployment.Channel)), "0")
			execute(adminRunner)
			Eventually(adminRunner.Err(), 5*time.Second).Should(gbytes.Say("Received block: 0"))

			fmt.Println("===============Joining Channel================", peerDir)
			adminRunner = p.JoinChannel(filepath.Join(w.Rootpath, peerDir, fmt.Sprintf("%s.block", w.Deployment.Channel)))
			execute(adminRunner)
			Eventually(adminRunner.Err(), 5*time.Second).Should(gbytes.Say("Successfully submitted proposal to join channel"))

			fmt.Println("===============Installing Chaincode================", peerDir)
			p.ExecPath = w.Deployment.Chaincode.ExecPath
			p.GoPath = w.Deployment.Chaincode.GoPath
			adminRunner = p.InstallChaincode(w.Deployment.Chaincode.Name, w.Deployment.Chaincode.Version, w.Deployment.Chaincode.Path)
			execute(adminRunner)
			Eventually(adminRunner.Err(), 5*time.Second).Should(gbytes.Say(`\QInstalled remotely response:<status:200 payload:"OK" >\E`))
			//Eventually(peerRunner.Err(), 5*time.Second).Should(gbytes.Say(fmt.Sprintf(`\QInstalled Chaincode [%s] Version [1.0] to peer\E`, w.Deployment.Chaincode.Name)))
		}
	}

	//	fmt.Println("===============Instantiating Chaincode================")
	//	p = w.Components.Peer()
	//	p.ConfigDir = filepath.Join(w.Rootpath, "org1.example.com_0")
	//	p.LogLevel = "debug"
	//	p.MSPConfigPath = filepath.Join(w.Rootpath, "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
	//	adminRunner = p.InstantiateChaincode(w.Deployment.Chaincode.Name, w.Deployment.Chaincode.Version, w.Deployment.Orderer, w.Deployment.Channel, w.Deployment.InitArgs, w.Deployment.Policy)
	//	adminProcess := ifrit.Invoke(adminRunner)
	//	Eventually(adminProcess.Ready(), 2*time.Second).Should(BeClosed())
	//	Eventually(adminProcess.Wait(), 10*time.Second).ShouldNot(Receive(BeNil()))
	//
	//	listInstantiated := func() bool {
	//		adminPeer = components.Peer()
	//		adminPeer.ConfigDir = peer.ConfigDir
	//		adminPeer.MSPConfigPath = filepath.Join(testDir, "peer1", "crypto", "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
	//		adminRunner = adminPeer.ChaincodeListInstantiated(w.Deployment.Channel)
	//		err := execute(adminRunner)
	//		if err != nil {
	//			return false
	//		}
	//		return strings.Contains(string(adminRunner.Buffer().Contents()), "Path: simple/cmd")
	//	}
	return nil
}

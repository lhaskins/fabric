/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner

import (
	"fmt"
	"os/exec"

	"github.com/tedsuo/ifrit/ginkgomon"
)

type Peer struct {
	Path                        string
	ConfigDir                   string
	LocalMSPID                  string
	MSPConfigPath               string
	PeerID                      string
	PeerAddress                 string
	PeerListenAddress           string
	LedgerStateStateDatabase    string
	ProfileEnabled              string
	ProfileListenAddress        string
	FileSystemPath              string
	PeerEventsAddress           string
	PeerChaincodeAddress        string
	PeerChaincodeListenAddress  string
	PeerGossipEndpoint          string
	PeerGossipExternalEndpoint  string
	PeerGossipBootstrap         string
	PeerGossipUseLeaderElection string
	PeerGossipOrgLeader         string
	LogLevel                    string
}

func (p *Peer) setupEnvironment(cmd *exec.Cmd) {
	if p.ConfigDir != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("FABRIC_CFG_PATH=%s", p.ConfigDir))
	}
	if p.LocalMSPID != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_LOCALMSPID=%s", p.LocalMSPID))
	}
	if p.MSPConfigPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_MSPCONFIGPATH=%s", p.MSPConfigPath))
	}
	if p.PeerID != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_ID=%s", p.PeerID))
	}
	if p.PeerAddress != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_ADDRESS=%s", p.PeerAddress))
	}
	if p.PeerListenAddress != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_LISTENADDRESS=%s", p.PeerListenAddress))
	}
	if p.LedgerStateStateDatabase != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_LEDGER_STATE_STATEDATABASE=%s", p.LedgerStateStateDatabase))
	}
	if p.ProfileEnabled != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_PROFILE_ENABLED=%s", p.ProfileEnabled))
	}
	if p.ProfileListenAddress != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_PROFILE_LISTENADDRESS=%s", p.ProfileListenAddress))
	}
	if p.FileSystemPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_FILESYSTEMPATH=%s", p.FileSystemPath))
	}
	if p.PeerEventsAddress != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_EVENTS_ADDRESS=%s", p.PeerEventsAddress))
	}
	if p.PeerChaincodeAddress != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_CHAINCODEADDRESS=%s", p.PeerChaincodeAddress))
	}
	if p.PeerChaincodeListenAddress != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_CHAINCODELISTENADDRESS=%s", p.PeerChaincodeListenAddress))
	}
	if p.PeerGossipEndpoint != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_GOSSIP_ENDPOINT=%s", p.PeerGossipEndpoint))
	}
	if p.PeerGossipExternalEndpoint != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_GOSSIP_EXTERNALENDPOINT=%s", p.PeerGossipExternalEndpoint))
	}
	if p.PeerGossipBootstrap != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_GOSSIP_BOOTSTRAP=%s", p.PeerGossipBootstrap))
	}
	if p.PeerGossipOrgLeader != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_GOSSIP_ORGLEADER=%s", p.PeerGossipOrgLeader))
	}
	if p.PeerGossipUseLeaderElection != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORE_PEER_GOSSIP_USELEADERELECTION=%s", p.PeerGossipUseLeaderElection))
	}
}

func (p *Peer) NodeStart() *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "node", "start")
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Name:          "peer",
		Command:       cmd,
	})

	return r
}

func (p *Peer) ChaincodeList() *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "chaincode", "list", "--installed")
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

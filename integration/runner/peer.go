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
	Path          string
	GoPath        string
	ExecPath      string
	ConfigDir     string
	MSPConfigPath string
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
	if p.GoPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOPATH=%s", p.GoPath))
	}
	if p.ExecPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", p.ExecPath))
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

func (p *Peer) ChaincodeListInstantiated(channel string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "chaincode", "list", "--instantiated", "-C", channel)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) CreateChannel(channel string, filename string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "channel", "create", "-c", channel, "-o", "127.0.0.1:7050", "-f", filename)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) FetchChannel(channel string, filename string, block string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "channel", "fetch", block, "-c", channel, "-o", "127.0.0.1:7050", filename)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) JoinChannel(transactionFile string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "channel", "join", "-b", transactionFile)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) UpdateChannel(transactionFile string, channel string, orderer string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "channel", "update", "-c", channel, "-o", orderer, "-f", transactionFile)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) InstallChaincode(name string, version string, path string) *ginkgomon.Runner {
	//func (p *Peer) InstallChaincode(w world.World) *ginkgomon.Runner {
	//cmd := exec.Command(p.Path, "chaincode", "install", "-n", w.Deployment.Chaincode.Name, "-v", w.Deployment.Chaincode.Version, "-p", w.Deployment.Chaincode.Path)
	cmd := exec.Command(p.Path, "chaincode", "install", "-n", name, "-v", version, "-p", path)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) InstantiateChaincode(name string, version string, orderer string, channel string, args string, policy string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "chaincode", "instantiate", "-n", name, "-v", version, "-o", orderer, "-C", channel, "-c", args, "-P", policy)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) QueryChaincode(name string, channel string, args string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "chaincode", "query", "-n", name, "-C", channel, "-c", args)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

func (p *Peer) InvokeChaincode(name string, channel string, args string, orderer string) *ginkgomon.Runner {
	cmd := exec.Command(p.Path, "chaincode", "invoke", "-n", name, "-C", channel, "-c", args, "-o", orderer)
	p.setupEnvironment(cmd)

	r := ginkgomon.New(ginkgomon.Config{
		AnsiColorCode: "92m",
		Command:       cmd,
	})

	return r
}

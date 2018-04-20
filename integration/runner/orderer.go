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

type Orderer struct {
	Path           string
	ConfigDir      string
//	OrdererType    string
	LedgerLocation string
//	GenesisProfile string
//	GenesisFile    string
//	GenesisMethod  string
//	LocalMSPId     string
//	LocalMSPDir    string
//	ListenAddress  string
//	ListenPort     string
	LogLevel       string
}

func (o *Orderer) setupEnvironment(cmd *exec.Cmd) {
	if o.ConfigDir != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("FABRIC_CFG_PATH=%s", o.ConfigDir))
	}
	if o.LedgerLocation != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_FILELEDGER_LOCATION=%s", o.LedgerLocation))
	}
//	if o.GenesisProfile != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_GENESISPROFILE=%s", o.GenesisProfile))
//	}
//	if o.GenesisFile != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_GENESISFILE=%s", o.GenesisFile))
//	}
//	if o.GenesisMethod != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_GENESISMETHOD=%s", o.GenesisMethod))
//	}
	if o.LogLevel != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LOGLEVEL=%s", o.LogLevel))
	}
//	if o.LocalMSPId != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LOCALMSPID=%s", o.LocalMSPId))
//	}
//	if o.LocalMSPDir != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LOCALMSPDIR=%s", o.LocalMSPDir))
//	}
//	if o.OrdererType != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("CONFIGTX_ORDERER_ORDERERTYPE=%s", o.OrdererType))
//	}
//	if o.ListenAddress != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LISTENADDRESS=%s", o.ListenAddress))
//	}
//	if o.ListenPort != "" {
//		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LISTENPORT=%s", o.ListenPort))
//	}
}

func (o *Orderer) New() *ginkgomon.Runner {
	cmd := exec.Command(o.Path)
	o.setupEnvironment(cmd)

	return ginkgomon.New(ginkgomon.Config{
		Name:    "orderer",
		Command: cmd,
	})
}

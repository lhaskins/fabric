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
	OrdererType    string
	LedgerLocation string
	GenesisProfile string
	LocalMSPId     string
	LocalMSPDir    string
	LogLevel       string
}

func (o *Orderer) setupEnvironment(cmd *exec.Cmd) {
	if o.ConfigDir != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("FABRIC_CFG_PATH=%s", o.ConfigDir))
	}
	if o.LedgerLocation != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_FILELEDGER_LOCATION=%s", o.LedgerLocation))
	}
	if o.GenesisProfile != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_GENESISPROFILE=%s", o.GenesisProfile))
	}
	if o.LogLevel != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LOGLEVEL=%s", o.LogLevel))
	}
	if o.LocalMSPId != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LOCALMSPID=%s", o.LocalMSPId))
	}
	if o.LocalMSPDir != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ORDERER_GENERAL_LOCALMSPDIR=%s", o.LocalMSPDir))
	}
	if o.OrdererType != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CONFIGTX_ORDERER_ORDERERTYPE=%s", o.OrdererType))
	}
}

func (o *Orderer) New() *ginkgomon.Runner {
	cmd := exec.Command(o.Path)
	o.setupEnvironment(cmd)

	return ginkgomon.New(ginkgomon.Config{
		Command: cmd,
	})
}

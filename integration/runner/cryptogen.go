/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner

import (
	"os/exec"

	//. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit/ginkgomon"
)

// Cryptogen creates runners that call cryptogen functions.
type Cryptogen struct {
	// The location of the cryptogen executable
	Path string
	// The location of the config file
	Config string
	// The output directory
	Output string
}

type ConfigGen struct {
	OrdererOrg Organization
	PeerOrg    Organization
}

type OrdererConfig struct {
	Name        string
	BrokerCount int    // 0 is solo
	Profile     string // name of the profile
}

type PeerConfig struct {
	Name          string
	Domain        string
	EnableNodeOUs bool
	UserCount     int
	PeerCount     int
	MSPID         string
}

type Organization struct {
	Name     string
	Domain   string
	MSPID    string
	Orderers []OrdererConfig
	Peers    []PeerConfig
}

// Generate uses cryptogen to generate cryptographic material for fabric.
func (c *Cryptogen) Generate(extraArgs ...string) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Command: exec.Command(
			c.Path,
			append([]string{
				"generate",
				"--config", c.Config,
				"--output", c.Output,
			}, extraArgs...)...,
		),
	})
}

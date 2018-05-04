/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package world

import (
	"os"

	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type Components struct {
	Paths map[string]string
}

func (c *Components) Build() {
	if c.Paths == nil {
		c.Paths = map[string]string{}
	}
	configtxgen, err := gexec.Build("github.com/hyperledger/fabric/common/tools/configtxgen")
	Expect(err).NotTo(HaveOccurred())
	c.Paths["configtxgen"] = configtxgen

	cryptogen, err := gexec.Build("github.com/hyperledger/fabric/common/tools/cryptogen")
	Expect(err).NotTo(HaveOccurred())
	c.Paths["cryptogen"] = cryptogen
}

func (c *Components) Cleanup() {
	for _, path := range c.Paths {
		err := os.Remove(path)
		Expect(err).NotTo(HaveOccurred())
	}
}

func (c *Components) Cryptogen() *runner.Cryptogen {
	return &runner.Cryptogen{
		Path: c.Paths["cryptogen"],
	}
}

func (c *Components) ConfigTxGen() *runner.ConfigTxGen {
	return &runner.ConfigTxGen{
		Path: c.Paths["configtxgen"],
	}
}

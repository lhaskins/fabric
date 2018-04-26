/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package world

import (
	"fmt"
	"os"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
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

	orderer, err := gexec.Build("github.com/hyperledger/fabric/orderer")
	Expect(err).NotTo(HaveOccurred())
	c.Paths["orderer"] = orderer

	peer, err := gexec.Build("github.com/hyperledger/fabric/peer")
	Expect(err).NotTo(HaveOccurred())
	c.Paths["peer"] = peer
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

func (c *Components) Zookeeper(id int, network *docker.Network) *runner.Zookeeper {
	return &runner.Zookeeper{
		ZooMyID:     id,
		Name:        fmt.Sprintf("zookeeper%d", id),
		NetworkID:   network.ID,
		NetworkName: network.Name,
	}
}

func (c *Components) Kafka(id int, network *docker.Network) *runner.Kafka {
	return &runner.Kafka{
		Name:          fmt.Sprintf("kafka%d", id),
		KafkaBrokerID: id,
		NetworkID:     network.ID,
		NetworkName:   network.Name,
	}
}

func (c *Components) Orderer() *runner.Orderer {
	return &runner.Orderer{
		Path: c.Paths["orderer"],
	}
}

func (c *Components) Peer() *runner.Peer {
	return &runner.Peer{
		Path: c.Paths["peer"],
	}
}

func execute(r ifrit.Runner) (err error) {
	p := ifrit.Invoke(r)
	Eventually(p.Ready()).Should(BeClosed())
	Eventually(p.Wait()).Should(Receive(&err))
	return err
}

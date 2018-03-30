/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner

import (
	"fmt"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit/ginkgomon"
)

// ConfigTxGen creates runners that call cryptogen functions.
type ConfigTxGen struct {
	// The location of the configtxgen executable
	Path string
	// The channel ID
	ChannelID string
	// The profile used for the channel
	Profile string
	// The organization for this config channel
	AsOrg string
	// The fabric config directory
	ConfigDir string
	// The directory to write the block file
	Output string
}

func (c *ConfigTxGen) setupCommandEnv(cmd *exec.Cmd) {
	if c.ConfigDir != "" {
		configDir, err := filepath.Abs(c.ConfigDir)
		Expect(err).NotTo(HaveOccurred())
		cmd.Env = append(cmd.Env, fmt.Sprintf("FABRIC_CFG_PATH=%s", configDir))
	}
}

// OutputBlock uses configtxgen to generate genesis block for fabric.
func (c *ConfigTxGen) OutputBlock(extraArgs ...string) *ginkgomon.Runner {
	cmd := exec.Command(
		c.Path,
		append([]string{
			"-outputBlock", c.Output,
			"-profile", c.Profile,
		}, extraArgs...)...,
	)
	c.setupCommandEnv(cmd)
	return ginkgomon.New(ginkgomon.Config{
		Command: cmd,
	})
}

func (c *ConfigTxGen) OutputCreateChannelTx(extraArgs ...string) *ginkgomon.Runner {
	cmd := exec.Command(
		c.Path,
		append([]string{
			"-channelID", c.ChannelID,
			"-outputCreateChannelTx", c.Output,
			"-profile", c.Profile,
		}, extraArgs...)...,
	)
	c.setupCommandEnv(cmd)

	return ginkgomon.New(ginkgomon.Config{
		Command: cmd,
	})
}

func (c *ConfigTxGen) OutputAnchorPeersUpdate(extraArgs ...string) *ginkgomon.Runner {
	cmd := exec.Command(
		c.Path,
		append([]string{
			"-channelID", c.ChannelID,
			"-outputAnchorPeersUpdate", c.Output,
			"-profile", c.Profile,
			"-asOrg", c.AsOrg,
		}, extraArgs...)...,
	)
	c.setupCommandEnv(cmd)

	return ginkgomon.New(ginkgomon.Config{
		Command: cmd,
	})
}

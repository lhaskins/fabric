/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/hyperledger/fabric/integration/runner"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("ConfigTxGen", func() {
	var (
		configtxgen *runner.ConfigTxGen
		tempDir     string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "configtx")
		Expect(err).NotTo(HaveOccurred())

		data, err := ioutil.ReadFile(filepath.Join("testdata", "configtx.yaml"))
		Expect(err).NotTo(HaveOccurred())
		err = ioutil.WriteFile(filepath.Join(tempDir, "configtx.yaml"), data, 0775)
		Expect(err).NotTo(HaveOccurred())

		cryptogen := components.Cryptogen()
		cryptogen.Config = filepath.Join("testdata", "cryptogen-config.yaml")
		cryptogen.Output = filepath.Join(tempDir, "crypto-config")
		cryptoProcess := ifrit.Invoke(cryptogen.Generate())
		Eventually(cryptoProcess.Wait()).Should(Receive(BeNil()))
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("OutputBlock", func() {
		BeforeEach(func() {
			configtxgen = &runner.ConfigTxGen{
				Path:      components.Paths["configtxgen"],
				ChannelID: "mychannel",
				Profile:   "TwoOrgsOrdererGenesis",
				ConfigDir: tempDir,
				Output:    filepath.Join(tempDir, "mychannel.block"),
			}
		})

		It("creates an orderer block successfully", func() {
			r := configtxgen.OutputBlock()
			process := ifrit.Invoke(r)
			Eventually(process.Ready()).Should(BeClosed())
			Eventually(process.Wait()).Should(Receive(BeNil()))
			Expect(r.ExitCode()).To(Equal(0))

			Expect(filepath.Join(tempDir, "mychannel.block")).To(BeARegularFile())
		})

		Context("when configtxgen fails", func() {
			BeforeEach(func() {
				configtxgen.Profile = "mango"
			})

			It("returns an error", func() {
				r := configtxgen.OutputBlock()
				process := ifrit.Invoke(r)
				Eventually(process.Wait()).Should(Receive(HaveOccurred()))
				Expect(r.Err()).To(gbytes.Say("Could not find profile: mango"))
			})
		})

		Context("when the config directory is not provided", func() {
			BeforeEach(func() {
				configtxgen.ConfigDir = ""
			})

			It("uses the default config", func() {
				r := configtxgen.OutputBlock()
				process := ifrit.Invoke(r)
				Eventually(process.Wait()).Should(Receive(HaveOccurred()))

				Eventually(r).Should(gexec.Exit(2))
				Expect(r.Err()).To(gbytes.Say("Could not find profile: TwoOrgsOrdererGenesis"))
			})
		})
	})

	Describe("OutputCreateChannelTx", func() {
		BeforeEach(func() {
			configtxgen = &runner.ConfigTxGen{
				Path:      components.Paths["configtxgen"],
				ChannelID: "mychannel",
				Profile:   "TwoOrgsChannel",
				ConfigDir: tempDir,
				Output:    filepath.Join(tempDir, "mychannel.tx"),
			}
		})

		It("creates a channel transaction file", func() {
			r := configtxgen.OutputCreateChannelTx()
			process := ifrit.Invoke(r)
			Eventually(process.Ready()).Should(BeClosed())
			Eventually(process.Wait()).Should(Receive(BeNil()))
			Expect(r.ExitCode()).To(Equal(0))

			Expect(filepath.Join(tempDir, "mychannel.tx")).To(BeARegularFile())
		})

		Context("when configtxgen fails", func() {
			BeforeEach(func() {
				configtxgen.Profile = "banana"
			})

			It("returns an error", func() {
				r := configtxgen.OutputCreateChannelTx()
				process := ifrit.Invoke(r)
				Eventually(process.Wait()).Should(Receive(HaveOccurred()))
				Expect(r.Err()).To(gbytes.Say("Could not find profile: banana"))
			})
		})

		Context("when the config directory is not provided", func() {
			BeforeEach(func() {
				configtxgen.ConfigDir = ""
			})

			It("uses the default config", func() {
				r := configtxgen.OutputCreateChannelTx()
				process := ifrit.Invoke(r)
				Eventually(process.Wait()).Should(Receive(HaveOccurred()))

				Eventually(r).Should(gexec.Exit(2))
				Expect(r.Err()).To(gbytes.Say("Could not find profile: TwoOrgsChannel"))
			})
		})
	})

	Describe("OutputAnchorPeersUpdate", func() {
		BeforeEach(func() {
			configtxgen = &runner.ConfigTxGen{
				Path:      components.Paths["configtxgen"],
				ChannelID: "mychannel",
				Profile:   "TwoOrgsChannel",
				AsOrg:     "Org1MSP",
				ConfigDir: tempDir,
				Output:    filepath.Join(tempDir, "Org1MSPanchors.tx"),
			}
		})

		It("creates a channel configuration file for the Org1 anchor peer", func() {
			r := configtxgen.OutputAnchorPeersUpdate()
			process := ifrit.Invoke(r)
			Eventually(process.Ready()).Should(BeClosed())
			Eventually(process.Wait()).Should(Receive(BeNil()))
			Expect(r.ExitCode()).To(Equal(0))

			Expect(filepath.Join(tempDir, "Org1MSPanchors.tx")).To(BeARegularFile())
		})

		Context("when configtxgen fails", func() {
			BeforeEach(func() {
				configtxgen.Profile = "kiwi"
			})

			It("returns an error", func() {
				r := configtxgen.OutputAnchorPeersUpdate()
				process := ifrit.Invoke(r)
				Eventually(process.Wait()).Should(Receive(HaveOccurred()))
				Expect(r.Err()).To(gbytes.Say("Could not find profile: kiwi"))
			})
		})

		Context("when the config directory is not provided", func() {
			BeforeEach(func() {
				configtxgen.ConfigDir = ""
			})

			It("uses the default config", func() {
				r := configtxgen.OutputCreateChannelTx()
				process := ifrit.Invoke(r)
				Eventually(process.Wait()).Should(Receive(HaveOccurred()))

				Eventually(r).Should(gexec.Exit(2))
				Expect(r.Err()).To(gbytes.Say("Could not find profile: TwoOrgsChannel"))
			})
		})
	})
})

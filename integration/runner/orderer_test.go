/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Orderer", func() {
	var (
		orderer *runner.Orderer
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "orderer")
		Expect(err).NotTo(HaveOccurred())

		copyFile(filepath.Join("testdata", "cryptogen-config.yaml"), filepath.Join(tempDir, "cryptogen-config.yaml"))
		cryptogen := components.Cryptogen()
		cryptogen.Config = filepath.Join(tempDir, "cryptogen-config.yaml")
		cryptogen.Output = filepath.Join(tempDir, "crypto-config")

		crypto := cryptogen.Generate()
		Expect(execute(crypto)).To(Succeed())

		copyFile(filepath.Join("testdata", "configtx.yaml"), filepath.Join(tempDir, "configtx.yaml"))
		configtxgen := components.ConfigTxGen()
		configtxgen.ChannelID = "mychannel"
		configtxgen.Profile = "TwoOrgsOrdererGenesis"
		configtxgen.ConfigDir = tempDir
		configtxgen.Output = filepath.Join(tempDir, "mychannel.block")

		r := configtxgen.OutputBlock()
		err = execute(r)
		Expect(err).NotTo(HaveOccurred())

		copyFile(filepath.Join("testdata", "orderer.yaml"), filepath.Join(tempDir, "orderer.yaml"))
		orderer = components.Orderer()
		orderer.ConfigDir = tempDir
		orderer.OrdererType = "solo"
		orderer.LedgerLocation = tempDir
		orderer.GenesisProfile = "TwoOrgsOrdererGenesis"
		orderer.LocalMSPId = "OrdererMSP"
		orderer.LocalMSPDir = filepath.Join(tempDir, "crypto-config/ordererOrganizations/example.com/orderers/orderer.example.com/msp")
		orderer.LogLevel = "debug"
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	It("starts an orderer", func() {
		r := orderer.New()
		process := ifrit.Invoke(r)
		Eventually(process.Ready()).Should(BeClosed())
		Eventually(r.Err()).Should(gbytes.Say("Beginning to serve requests"))

		Consistently(process.Wait()).ShouldNot(Receive())
		process.Signal(syscall.SIGTERM)
		Eventually(process.Wait()).Should(Receive())
	})
})

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
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Peer", func() {
	var (
		tempDir   string
		cryptoDir string

		orderer        *runner.Orderer
		ordererProcess ifrit.Process
		peerProcess    ifrit.Process

		peer *runner.Peer
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "peer")
		Expect(err).NotTo(HaveOccurred())

		cryptoDir = filepath.Join(tempDir, "crypto-config")
		peer = components.Peer()

		copyFile(filepath.Join("testdata", "cryptogen-config.yaml"), filepath.Join(tempDir, "cryptogen-config.yaml"))
		cryptogen := components.Cryptogen()
		cryptogen.Config = filepath.Join(tempDir, "cryptogen-config.yaml")
		cryptogen.Output = cryptoDir

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
		orderer.LocalMSPDir = filepath.Join(cryptoDir, "ordererOrganizations/example.com/orderers/orderer.example.com/msp")
		orderer.LogLevel = "DEBUG"

		o := orderer.New()
		ordererProcess = ifrit.Invoke(o)
		Eventually(ordererProcess.Ready()).Should(BeClosed())
		Consistently(ordererProcess.Wait()).ShouldNot(Receive())

		copyFile(filepath.Join("testdata", "core.yaml"), filepath.Join(tempDir, "core.yaml"))
		peer.ConfigDir = tempDir
	})

	AfterEach(func() {
		if ordererProcess != nil {
			ordererProcess.Signal(syscall.SIGTERM)
		}
		if peerProcess != nil {
			peerProcess.Signal(syscall.SIGTERM)
		}
		os.RemoveAll(tempDir)
	})

	It("starts a peer", func() {
		peer.MSPConfigPath = filepath.Join(cryptoDir, "peerOrganizations", "org1.example.com", "peers", "peer0.org1.example.com", "msp")
		r := peer.NodeStart()
		peerProcess = ifrit.Invoke(r)
		Eventually(peerProcess.Ready()).Should(BeClosed())
		Consistently(peerProcess.Wait()).ShouldNot(Receive())

		By("Listing the installed chaincodes")
		installed := components.Peer()
		installed.ConfigDir = tempDir
		installed.MSPConfigPath = filepath.Join(cryptoDir, "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")

		list := installed.ChaincodeListInstalled()
		err := execute(list)
		Expect(err).NotTo(HaveOccurred())

		peerProcess.Signal(syscall.SIGTERM)
		Eventually(peerProcess.Wait()).Should(Receive(BeNil()))

		//		By("create channel")
		//
		//		By("join channel")
		//
		//		By("installs chaincode")
		//		installChaincode := components.Peer()
		//		installChaincode.LocalMSPID = "Org1ExampleCom"
		//		install = installChaincode.InstallChaincode()
		//
		//		By("instantiate channel")
		//
		//		By("query channel")
	})

})

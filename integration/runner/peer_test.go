/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner_test

import (
	"fmt"
	"io/ioutil"
	//"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hyperledger/fabric/integration/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Peer", func() {
	var (
		tempDir   string
		cryptoDir string

		orderer        *runner.Orderer
		ordererProcess ifrit.Process
		peerProcess    ifrit.Process
		peer           *runner.Peer
		ordererRunner  *ginkgomon.Runner
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "peer")
		Expect(err).NotTo(HaveOccurred())

		// Generate crypto info
		cryptoDir = filepath.Join(tempDir, "crypto-config")
		peer = components.Peer()

		copyFile(filepath.Join("testdata", "cryptogen-config.yaml"), filepath.Join(tempDir, "cryptogen-config.yaml"))
		cryptogen := components.Cryptogen()
		cryptogen.Config = filepath.Join(tempDir, "cryptogen-config.yaml")
		cryptogen.Output = cryptoDir

		crypto := cryptogen.Generate()
		Expect(execute(crypto)).To(Succeed())

		// Generate orderer config block
		copyFile(filepath.Join("testdata", "configtx.yaml"), filepath.Join(tempDir, "configtx.yaml"))
		configtxgen := components.ConfigTxGen()
		configtxgen.ChannelID = "mychannel"
		configtxgen.Profile = "TwoOrgsOrdererGenesis"
		configtxgen.ConfigDir = tempDir
		configtxgen.Output = filepath.Join(tempDir, "mychannel.block")
		r := configtxgen.OutputBlock()
		err = execute(r)
		Expect(err).NotTo(HaveOccurred())

		// Generate channel transaction file
		configtxgen = components.ConfigTxGen()
		configtxgen.ChannelID = "mychan"
		configtxgen.Profile = "TwoOrgsChannel"
		configtxgen.ConfigDir = tempDir
		configtxgen.Output = filepath.Join(tempDir, "mychan.tx")
		r = configtxgen.OutputCreateChannelTx()
		err = execute(r)
		Expect(err).NotTo(HaveOccurred())

		// Start the orderer
		copyFile(filepath.Join("testdata", "orderer.yaml"), filepath.Join(tempDir, "orderer.yaml"))
		orderer = components.Orderer()
		orderer.ConfigDir = tempDir
		orderer.LedgerLocation = tempDir
		orderer.LogLevel = "DEBUG"

		ordererRunner = orderer.New()
		ordererProcess = ifrit.Invoke(ordererRunner)
		Eventually(ordererProcess.Ready()).Should(BeClosed())
		Consistently(ordererProcess.Wait()).ShouldNot(Receive())

//		err = os.Mkdir(filepath.Join(tempDir, "peer"), 0755)
//		Expect(err).NotTo(HaveOccurred())
//		copyFile(filepath.Join("testdata", "core.yaml"), filepath.Join(tempDir, "peer", "core.yaml"))
//		peer.ConfigDir = filepath.Join(tempDir, "peer")

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

		//output, err := exec.Command("find", tempDir, "-type", "f").Output()
		//Expect(err).NotTo(HaveOccurred())
		//fmt.Printf("\n---\n%s\n---\n", output)
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

		By("create channel")
		createChan := components.Peer()
		createChan.ConfigDir = tempDir
		createChan.MSPConfigPath = filepath.Join(cryptoDir, "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		cRunner := createChan.CreateChannel("mychan", filepath.Join(tempDir, "mychan.tx"))
		err = execute(cRunner)
		Expect(err).NotTo(HaveOccurred())
		Eventually(ordererRunner.Err(), 5*time.Second).Should(gbytes.Say("Created and starting new chain mychan"))

		By("fetch channel")
//		err = os.Mkdir(filepath.Join(tempDir, "peer"), 0755)
		fetchChan := components.Peer()
		//fetchChan.ConfigDir = filepath.Join(tempDir, "peer")
		fetchChan.ConfigDir = tempDir
		fetchChan.MSPConfigPath = filepath.Join(cryptoDir, "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		//fRunner := fetchChan.FetchChannel("mychan", filepath.Join(tempDir, "peer", "mychan.block"), "0")
		fRunner := fetchChan.FetchChannel("mychan", filepath.Join(tempDir, "mychan.block"), "0")
		err = execute(fRunner)
		//Expect(err).NotTo(HaveOccurred())
		Eventually(fRunner.Err(), 5*time.Second).Should(gbytes.Say("Received block: 0"))

		By("join channel")
		joinChan := components.Peer()
		joinChan.ConfigDir = filepath.Join(tempDir, "peer")
		joinChan.MSPConfigPath = filepath.Join(cryptoDir, "peerOrganizations", "org1.example.com", "users", "Admin@org1.example.com", "msp")
		jRunner := joinChan.JoinChannel(filepath.Join(tempDir, "peer", "mychan.block"))
		err = execute(jRunner)
		//Expect(err).NotTo(HaveOccurred())
		Eventually(jRunner.Err(), 5*time.Second).Should(gbytes.Say("Successfully submitted proposal to join channel"))

		By("installs chaincode")

		By("instantiate channel")

		By("query channel")

		peerProcess.Signal(syscall.SIGTERM)
		Eventually(peerProcess.Wait()).Should(Receive(BeNil()))

	})

})

package e2e

import (
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"
)

var _ = Describe("EndToEnd", func() {
	BeforeEach(func() {
	})

	It("does something?", func() {
		cryptogen := components.Cryptogen()
		cryptogen.Config = filepath.Join("testdata", "crypto-config.yaml")
		cryptogen.Output = filepath.Join(testDir, "crypto")

		config := components.Cryptogen()

		process := ifrit.Invoke(cryptogen.Generate())
		Eventually(process.Wait()).Should(Receive(BeNil()))
	})
})

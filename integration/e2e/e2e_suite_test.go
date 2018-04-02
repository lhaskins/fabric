/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package e2e

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/hyperledger/fabric/integration/world"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEndToEnd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EndToEnd Suite")
}

var (
	components *world.Components

	testDir string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	components = &world.Components{}
	components.Build()

	payload, err := json.Marshal(components)
	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(payload []byte) {
	err := json.Unmarshal(payload, &components)
	Expect(err).NotTo(HaveOccurred())

	testDir, err = ioutil.TempDir("", "e2e-suite")
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	// output, err := exec.Command("find", testDir, "-type", "f").Output()
	// Expect(err).NotTo(HaveOccurred())
	// fmt.Printf("\n---\n%s\n---\n", output)
	os.RemoveAll(testDir)
}, func() {
	//fmt.Printf("\n---\n%s\n---\n", components.Paths)
	components.Cleanup()
})

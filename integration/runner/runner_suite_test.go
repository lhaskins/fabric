/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var cryptogenPath string

func TestRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runner Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	cryptogenPath, err := gexec.Build("github.com/hyperledger/fabric/common/tools/cryptogen")
	Expect(err).NotTo(HaveOccurred())

	return []byte(cryptogenPath)
}, func(data []byte) {
	cryptogenPath = string(data)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.Remove(cryptogenPath)
})

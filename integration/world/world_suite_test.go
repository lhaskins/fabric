package world_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWorld(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "World Suite")
}

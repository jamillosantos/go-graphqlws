package redis_test

import (
	"testing"

	"github.com/jamillosantos/macchiato"
	"github.com/lab259/rlog/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRedis(t *testing.T) {
	rlog.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	macchiato.RunSpecs(t, "GraphQL WS Redis Test Suite")
}

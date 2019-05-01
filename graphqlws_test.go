package graphqlws_test

import (
	"testing"

	"github.com/jamillosantos/macchiato"
	"github.com/lab259/rlog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGraphQLWS(t *testing.T) {
	rlog.SetOutput(GinkgoWriter)
	// dir, _ := os.Getwd()
	RegisterFailHandler(Fail)
	macchiato.RunSpecs(t, "GraphQL WS Test Suite")
}

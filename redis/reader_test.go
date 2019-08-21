package redis

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reader", func() {
	Describe("Contexts", func() {
		It("should check context key generation", func() {
			Expect(ConnectionContextKey.String()).To(Equal("__graphql.context.key.connection"))
			Expect(SubscriptionContextKey.String()).To(Equal("__graphql.context.key.subscription"))
		})
	})
})

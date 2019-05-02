package graphqlws_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jamillosantos/go-graphqlws"
)

var _ = Describe("Subscription", func() {
	Describe("ValidateSubscription", func() {
		It("should fail validating a subscription", func() {
			subscription := graphqlws.Subscription{}
			errs := graphqlws.ValidateSubscription(&subscription)
			Expect(errs).To(ConsistOf(graphqlws.ErrSubscriptionIDEmpty, graphqlws.ErrSubscriptionHasNoConnection, graphqlws.ErrSubscriptionHasNoQuery))
		})
	})
})

package graphqlws_test

import (
	"github.com/jamillosantos/go-graphqlws"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Subscriber", func() {
	Describe("StringTopic", func() {
		It("should return the ID of the topic", func() {
			topic := graphqlws.StringTopic("topic1")
			Expect(topic.ID()).To(Equal(topic))
		})
	})

	Describe("Subscriber", func() {
		It("should register the topics for being subscribed", func() {
			subscriber := graphqlws.NewSubscriber()
			topic1 := graphqlws.StringTopic("topic1")
			topic2 := graphqlws.StringTopic("topic2")
			topic3 := graphqlws.StringTopic("topic3")
			Expect(subscriber.Subscribe(topic1)).To(Succeed())
			Expect(subscriber.Subscribe(topic2)).To(Succeed())
			Expect(subscriber.Subscribe(topic3)).To(Succeed())
			Expect(subscriber.Topics()).To(ConsistOf(topic1, topic2, topic3))
		})

		It("should register the topics for being subscribed with the graphql.Subscriber interface", func() {
			subscriber := graphqlws.NewSubscriber()
			topic1 := graphqlws.StringTopic("topic1")
			topic2 := graphqlws.StringTopic("topic2")
			topic3 := graphqlws.StringTopic("topic3")
			Expect(subscriber.SubscriberSubscribe(topic1)).To(Succeed())
			Expect(subscriber.SubscriberSubscribe(topic2)).To(Succeed())
			Expect(subscriber.SubscriberSubscribe(topic3)).To(Succeed())
			Expect(subscriber.Topics()).To(ConsistOf(topic1, topic2, topic3))
		})

		It("should fail subscribing to an invalid topic", func() {
			subscriber := graphqlws.NewSubscriber()
			Expect(subscriber.SubscriberSubscribe(12345)).To(Equal(graphqlws.ErrInvalidTopic))
		})
	})
})

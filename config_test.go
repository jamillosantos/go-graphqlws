package graphqlws_test

import (
	"time"

	"github.com/gorilla/websocket"

	"github.com/jamillosantos/go-graphqlws"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configuration", func() {
	Describe("ConfigFactory", func() {
		It("should create a configuration", func() {
			config := graphqlws.NewConfigFactory().
				ReadLimit(12345).
				PongWait(time.Second).
				WriteTimeout(time.Minute).
				Build()
			Expect(config.ReadLimit).To(Equal(int64(12345)))
			Expect(config.PongWait).To(Equal(time.Second))
			Expect(config.WriteTimeout).To(Equal(time.Minute))
		})
	})

	Describe("HandlerConfigFactory", func() {
		It("should create a configuration", func() {
			upgrader := &websocket.Upgrader{}
			config := graphqlws.NewHandlerConfigFactory().
				Upgrader(upgrader).
				Build()
			Expect(config.Upgrader).To(Equal(upgrader))
		})
	})
})

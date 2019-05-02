package graphqlws_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/lab259/graphql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jamillosantos/go-graphqlws"
)

var _ = Describe("Upgrader", func() {
	Describe("WebSocketUpgrader", func() {
		It("should add the subprotocols", func() {
			upgrader := &websocket.Upgrader{}
			wu := graphqlws.NewUpgrader(upgrader)
			wu.AddSubprotocol("subprotocol1")
			Expect(upgrader.Subprotocols).To(ConsistOf("subprotocol1"))
		})

		It("should try upgrading a connection", func() {
			ts := httptest.NewServer(graphqlws.NewHttpHandler(
				graphqlws.NewHandlerConfigFactory().
					Schema(&graphql.Schema{}).
					Upgrader(graphqlws.NewUpgrader(&websocket.Upgrader{})).
					Build(),
				graphqlws.NewConfigFactory().
					Build(),
				func(conn *graphqlws.Conn, err error) {
					defer GinkgoRecover()

					Expect(err).ToNot(HaveOccurred())
				}),
			)
			defer ts.Close()

			client, response, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http://", "ws://", 1), http.Header{
				"Sec-Websocket-Protocol": []string{"graphql-ws"},
			})
			Expect(err).ToNot(HaveOccurred())
			defer client.Close()
			Expect(response.StatusCode).To(Equal(http.StatusSwitchingProtocols))
			Expect(client.Subprotocol()).To(Equal("graphql-ws"))
		})
	})
})

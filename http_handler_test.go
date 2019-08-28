package graphqlws_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"

	"github.com/jamillosantos/go-graphqlws"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	errForcedError = errors.New("forced error")
)

// fakeFailureUpgrader is a mock that will introduce an error at the websocket
// upgrading mechanism.
type fakeFailureUpgrader struct {
}

func (fakeFailureUpgrader) AddSubprotocol(protocol string) {}

func (fakeFailureUpgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return nil, errForcedError
}

var _ = Describe("HttpHandler", func() {
	It("should fail creating a handler without upgrader", func() {
		defer func() {
			r := recover()
			// Creating a `NewHttpHandler` without the `Upgrader` should result in a
			// panic.
			Expect(r).To(Equal(graphqlws.ErrUpgraderRequired))
		}()

		// Calls `NewHttpHandler` without a `Upgrader` defined.
		handler := graphqlws.NewHttpHandler(
			graphqlws.NewHandlerConfigFactory().Build(),
			graphqlws.NewConfigFactory().Build(),
			func(conn *graphqlws.Conn, e error) {},
		)
		Expect(handler).ToNot(BeNil())
	})

	It("should fail creating a handler without schema", func() {
		defer func() {
			r := recover()
			// Creating a `NewHttpHandler` without the `Upgrader` should result in a
			// panic.
			Expect(r).To(Equal(graphqlws.ErrSchemaRequired))
		}()

		// Calls `NewHttpHandler` without a `Schema` defined.
		handler := graphqlws.NewHttpHandler(
			graphqlws.NewHandlerConfigFactory().Upgrader(graphqlws.NewUpgrader(&websocket.Upgrader{})).Build(),
			graphqlws.NewConfigFactory().Build(),
			func(conn *graphqlws.Conn, e error) {},
		)
		Expect(handler).ToNot(BeNil())
	})

	It("should fail with an invalid sub protocol", func(done Done) {
		var wg sync.WaitGroup
		wg.Add(1)
		called := false
		ts := httptest.NewServer(graphqlws.NewHttpHandler(
			graphqlws.NewHandlerConfigFactory().
				Schema(&graphql.Schema{}).
				Upgrader(graphqlws.NewUpgrader(&websocket.Upgrader{})).
				Build(),
			graphqlws.NewConfigFactory().
				Build(),
			func(conn *graphqlws.Conn, err error) {
				defer GinkgoRecover()
				defer wg.Done()

				Expect(err).To(Equal(graphqlws.ErrClientDoesNotImplementGraphqlWS))
				called = true
			}),
		)
		defer ts.Close()

		client, response, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http://", "ws://", 1), nil)
		Expect(err).ToNot(HaveOccurred())
		defer client.Close()

		Expect(response.StatusCode).To(Equal(http.StatusSwitchingProtocols))
		// Wait the
		wg.Wait()
		Expect(called).To(BeTrue(), "the connection handler was not called")
		close(done)
	}, 1)

	It("should fail when cannot upgrade connection", func() {
		var wg sync.WaitGroup
		wg.Add(1)
		called := false
		ts := httptest.NewServer(graphqlws.NewHttpHandler(
			graphqlws.NewHandlerConfigFactory().
				Schema(&graphql.Schema{}).
				Upgrader(&fakeFailureUpgrader{}).
				Build(),
			graphqlws.NewConfigFactory().
				Build(),
			func(conn *graphqlws.Conn, err error) {
				defer GinkgoRecover()
				defer wg.Done()

				Expect(err).To(Equal(errForcedError))
				called = true
			}),
		)
		defer ts.Close()

		client, _, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http://", "ws://", 1), nil)
		Expect(err).To(Equal(websocket.ErrBadHandshake))
		Expect(client).To(BeNil())
		wg.Wait()
		Expect(called).To(BeTrue(), "the connection handler was not called")
	})
})

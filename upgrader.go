package graphqlws

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// WebSocketUpgrader is an interface that serve as a proxy to the original
// `gorilla.Upgrader`. It is needed to enable developers to customize their
// process of upgrade a HTTP connection to a WebSocket connection.
//
// Also, it is a good way to customize the `responseHeader`.
type WebSocketUpgrader interface {
	AddSubprotocol(protocol string)
	Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error)
}

type gorillaUpgrader struct {
	upgrader *websocket.Upgrader
}

// NewUpgrader implements a `WeSocketUpgrader` interface.
func NewUpgrader(upgrader *websocket.Upgrader) WebSocketUpgrader {
	return &gorillaUpgrader{
		upgrader: upgrader,
	}
}

// AddProtocol add the protocol to the `.Subprotocols` property of the
// `websocket.Upgrader`.
func (upgrader *gorillaUpgrader) AddSubprotocol(protocol string) {
	if upgrader.upgrader.Subprotocols == nil {
		upgrader.upgrader.Subprotocols = make([]string, 0, 1)
	}
	upgrader.upgrader.Subprotocols = append(upgrader.upgrader.Subprotocols, protocol)
}

// Upgrade proxies its call to the `websocket.Upgrader`.
func (upgrader *gorillaUpgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return upgrader.upgrader.Upgrade(w, r, nil)
}

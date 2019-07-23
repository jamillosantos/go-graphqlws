package graphqlws

import (
	"net/http"

	"github.com/tomasen/realip"
)

// NewHttpHandler returns a `http.Handler` ready for being used.
//
// `handler`: Is triggered when a connection is established. There, you should
// add handlers to the  conn and keep track when it is active.
//
// IMPORTANT: If `conn` is not finished. It will stay on forever.
func NewHttpHandler(handlerConfig HandlerConfig, connectionConfig Config, handler func(*Conn, error)) http.Handler {
	if handlerConfig.Upgrader == nil {
		panic(ErrUpgraderRequired)
	}
	if handlerConfig.Schema == nil {
		panic(ErrSchemaRequired)
	}
	handlerConfig.Upgrader.AddSubprotocol("graphql-ws")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := handlerConfig.Upgrader.Upgrade(w, r)
		// Bail out if the WebSocket connection could not be established
		if err != nil {
			handler(nil, err)
			return
		}

		// Close the connection early if it doesn't implement the graphql-ws protocol
		if ws.Subprotocol() != "graphql-ws" {
			ws.Close()
			handler(nil, ErrClientDoesNotImplementGraphqlWS)
			return
		}

		conn, err := NewConn(ws, handlerConfig.Schema, &connectionConfig)
		if err == nil {
			conn.Logger.Info("Connection started from: ", realip.FromRequest(r))
		}
		handler(conn, err)
	})
}

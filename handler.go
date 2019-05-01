package graphqlws

import (
	"net/http"
)

// NewHttpHandler returns a `http.Handler` ready for being used.
//
// `handler`: Is triggered when a connection is established. There, you should
// add handlers to the  conn and keep track when it is active.
//
// IMPORTANT: If `conn` is not finished. It will stay on forever.
func NewHttpHandler(handlerConfig HandlerConfig, connectionConfig Config, handler func(*Conn, error)) http.Handler {
	upgrader := handlerConfig.Upgrader
	if upgrader == nil {
		panic(ErrUpgraderRequired)
	}
	upgrader.AddSubprotocol("graphql-ws")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r)
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

		conn := NewConn(ws, handlerConfig.Schema, &connectionConfig)
		handler(conn, nil)
	})
}

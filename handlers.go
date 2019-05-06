package graphqlws

// Handler is an abstraction of a callback for a specific action in the system.
type Handler interface {
}

type RWType int

const (
	Read RWType = iota
	Write
)

// SystemRecoverHandler describes the handler that will be called when any panic
// happens while interacting with the client.
type SystemRecoverHandler interface {
	Handler
	HandlePanic(t RWType, r interface{}) error
}

// ConnectionInitHandler describes the handler that will be called when a GQL_CONNECTION_INIT
// is happens.
//
// More information abuot GQL_CONNECTION_INIT at https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_connection_init
type ConnectionInitHandler interface {
	Handler
	HandleConnectionInit(*GQLConnectionInit) error
}

// ConnectionStartHandler describes the handler that will be called when a GQL_START
// is happens.
//
// More information abuot GQL_START at https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_start
type ConnectionStartHandler interface {
	Handler
	HandleConnectionStart(*GQLStart) []error
}

// ConnectionStopHandler describes the handler that will be called when a GQL_STOP
// is happens.
//
// More information abuot GQL_STOP at https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_stop
type ConnectionStopHandler interface {
	Handler
	HandleConnectionStop(*GQLStop) error
}

// ConnectionTerminateHandler describes the handler that will be called when a GQL_CONNECTION_TERMINATE
// is happens.
//
// More information abuot GQL_CONNECTION_TERMINATE at https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_connection_terminate
type ConnectionTerminateHandler interface {
	Handler
	HandleConnectionTerminate(*GQLConnectionTerminate) error
}

// SubscriptionStartHandler describes the  handler that will be called when a
// subscription starts.
type SubscriptionStartHandler interface {
	Handler
	HandleSubscriptionStart(subscription *Subscription) error
}

// SubscriptionStopHandler describes the  handler that will be called when a
// subscription stops.
type SubscriptionStopHandler interface {
	Handler
	HandleSubscriptionStop(subscription *Subscription) error
}

// WebsocketPongHandler describes the handler that will be called when the gorilla websocket pong handler is called.
type WebsocketPongHandler interface {
	Handler
	HandleWebsocketPong(message string) error
}

// WebsocketPongHandler describes the handler that will be called when the gorilla websocket pong handler is called.
type WebsocketPingHandler interface {
	Handler
	HandleWebsocketPing() error
}

// WebsocketCloseHandler describes the handler that will be called when the gorilla websocket close handler is called.
type WebsocketCloseHandler interface {
	Handler
	HandleWebsocketClose(code int, text string) error
}

package graphqlws

import "encoding/json"

type GQLType string

// this snipped was adapted from the original GraphqlQL WS implementation (check https://github.com/functionalfoundry/graphqlws/blob/4a19181eb54fa93f0fcece2df171409a755b37d8/connections.go#L15)
const (
	gqlTypeConnectionInit      GQLType = "connection_init"
	gqlTypeConnectionAck       GQLType = "connection_ack"
	gqlTypeConnectionKeepAlive GQLType = "ka"
	gqlTypeConnectionError     GQLType = "connection_error"
	gqlTypeConnectionTerminate GQLType = "connection_terminate"
	gqlTypeStart               GQLType = "start"
	gqlTypeData                GQLType = "data"
	gqlTypeError               GQLType = "error"
	gqlTypeComplete            GQLType = "complete"
	gqlTypeStop                GQLType = "stop"
)

var gqlConnectionAck = &OperationMessage{
	Type: gqlTypeConnectionAck,
}

// /////////////////////////////////////////////////////////////////////////////////////////////////
// These structs are sent from the client during the connection handshake.
// /////////////////////////////////////////////////////////////////////////////////////////////////

// OperationMessage represents all messages sent the customer to the server.
type OperationMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    GQLType         `json:"type,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// GQLConnectionInit is sent from the client after the websocket connection is started.
//
// The server will response only with GQL_CONNECTION_ACK + GQL_CONNECTION_KEEP_ALIVE
// (if used) or GQL_CONNECTION_ERROR to this message.
//
// See Also [graphql-ws GQL_CONNECTION_INIT PROTOCOL](https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_connection_init)
type GQLConnectionInit struct {
	Payload json.RawMessage `json:"payload"`
}

// GQLObject represents the payload af a GQLStart command.
type GQLObject struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// GQLStart is sent from the client to be execute as a GraphQL command.
//
// See Also [graphql-ws GQL_START PROTOCOL](https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_start)
type GQLStart struct {
	ID      string    `json:"id"`
	Payload GQLObject `json:"payload"`
}

// GQLStop is sent from the client to stop a running GraphQL operation execution.
//
// See Also [graphql-ws GQL_START PROTOCOL](https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_stop)
type GQLStop struct {
	ID string `json:"id"`
}

// GQLConnectionTerminate is sent from the client to temrinate the connection and all
// its operations.
//
// See Also [graphql-ws GQL_CONNECTION_TERMINATE PROTOCOL](https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#gql_connection_terminate)
type GQLConnectionTerminate struct{}

// /////////////////////////////////////////////////////////////////////////////////////////////////
// These structs are sent from the server to the client.
// /////////////////////////////////////////////////////////////////////////////////////////////////

type GQLConnectionError struct {
	Payload interface{} `json:"payload"`
}

type GQLDataObject struct {
	Data   interface{} `json:"data"`
	Errors []error     `json:"errors,omitempty"`
}

type GQLData struct {
	ID      string        `json:"id"`
	Payload GQLDataObject `json:"payload"`
}

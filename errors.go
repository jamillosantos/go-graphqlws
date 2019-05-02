package graphqlws

import (
	"errors"

	"github.com/graphql-go/graphql/gqlerrors"
)

var (
	ErrConnectionClosed                = errors.New("connection already closed")
	ErrUpgraderRequired                = errors.New("upgrader required")
	ErrClientDoesNotImplementGraphqlWS = errors.New("client does not implement the `graphql-ws` subprotocol")
	ErrSubscriptionNotFound            = errors.New("subscription not found")

	// ErrReinitializationForbidden is triggered when a `gqlConnectionInit` is
	// received twice.
	ErrReinitializationForbidden = errors.New("reinitalization forbidden")

	// ErrConnectionNotFullyEstablished is triggered when a `gqlStart` is received
	// without finishing a `gqlConnectionInit`.
	ErrConnectionNotFullyEstablished = errors.New("connection not established")
)

type HandlerError struct {
	defaultPrevented   bool
	propagationStopped bool
}

func (err *HandlerError) Error() string {
	return ""
}

// PreventDefault set a flag for not executing the default implementation of an event.
func (err *HandlerError) PreventDefault() *HandlerError {
	err.defaultPrevented = true
	return err
}

// StopPropagation set a flag for not executing the subsequent handlers of an event.
func (err *HandlerError) StopPropagation() *HandlerError {
	err.propagationStopped = true
	return err
}

// ErrorsFromGraphQLErrors convert from GraphQL errors to regular errors.
func ErrorsFromGraphQLErrors(errors []gqlerrors.FormattedError) []error {
	if len(errors) == 0 {
		return nil
	}

	out := make([]error, len(errors))
	for i := range errors {
		out[i] = errors[i]
	}
	return out
}

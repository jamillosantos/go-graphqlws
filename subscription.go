package graphqlws

import (
	"encoding/json"
	"errors"

	"github.com/lab259/graphql"
	"github.com/lab259/graphql/language/ast"
)

var (
	ErrSubscriptionIDEmpty         = errors.New("subscription ID is empty")
	ErrSubscriptionHasNoConnection = errors.New("subscription is not associated with a connection")
	ErrSubscriptionHasNoQuery      = errors.New("subscription query is empty")
)

func ValidateSubscription(s *Subscription) []error {
	errs := make([]error, 0)

	if s.ID == "" {
		errs = append(errs, ErrSubscriptionIDEmpty)
	}

	if s.Connection == nil {
		errs = append(errs, ErrSubscriptionHasNoConnection)
	}

	if s.Query == "" {
		errs = append(errs, ErrSubscriptionHasNoQuery)
	}

	return errs
}

// Subscription holds all information about a GraphQL subscription
// made by a client, including a function to send data back to the
// client when there are updates to the subscription query result.
//
// From https://github.com/functionalfoundry/graphqlws
type Subscription struct {
	ID            string
	Query         string
	Variables     map[string]interface{}
	OperationName string
	Document      *ast.Document
	Fields        []string
	Schema        *graphql.Schema
	Connection    *Conn
}

func (s *Subscription) SendData(data *GQLDataObject) error {
	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}
	s.Connection.SendData(&OperationMessage{
		ID:      s.ID,
		Type:    gqlTypeData,
		Payload: dataJson,
	})
	return nil
}

// MatchesField returns true if the subscription is for data that belongs to the
// given field.
func (s *Subscription) MatchesField(field string) bool {
	if s.Document == nil || len(s.Fields) == 0 {
		return false
	}

	// The subscription matches the field if any of the queries have
	// the same name as the field
	for _, name := range s.Fields {
		if name == field {
			return true
		}
	}
	return false
}

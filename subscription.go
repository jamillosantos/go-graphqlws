package graphqlws

import (
	"errors"

	"github.com/graphql-go/graphql/language/ast"
)

var (
	ErrSubscriptionIDEmpty         = errors.New("subscription ID is empty")
	ErrSubscriptionHasNoConnection = errors.New("subscription is not associated with a connection")
	ErrSubscriptionHasNoQuery      = errors.New("subscription query is empty")
)

func ValidateSubscription(s SubscriptionInterface) []error {
	errs := make([]error, 0)

	if s.GetID() == "" {
		errs = append(errs, ErrSubscriptionIDEmpty)
	}

	if s.GetConnection() == nil {
		errs = append(errs, ErrSubscriptionHasNoConnection)
	}

	if s.GetQuery() == "" {
		errs = append(errs, ErrSubscriptionHasNoQuery)
	}

	return errs
}

type SubscriptionInterface interface {
	GetID() string
	GetQuery() string
	GetVariables() map[string]interface{}
	GetOperationName() string
	GetDocument() *ast.Document
	GetFields() []string
	GetConnection() *Conn
	SetFields(document []string)
	SetDocument(document *ast.Document)
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
	Connection    *Conn
}

func (s *Subscription) GetID() string {
	return s.ID
}

func (s *Subscription) GetQuery() string {
	return s.Query
}

func (s *Subscription) GetVariables() map[string]interface{} {
	return s.Variables
}

func (s *Subscription) GetOperationName() string {
	return s.OperationName
}

func (s *Subscription) SetDocument(value *ast.Document) {
	s.Document = value
}

func (s *Subscription) GetDocument() *ast.Document {
	return s.Document
}

func (s *Subscription) SetFields(value []string) {
	s.Fields = value
}

func (s *Subscription) GetFields() []string {
	return s.Fields
}

func (s *Subscription) GetConnection() *Conn {
	return s.Connection
}

// MatchesField returns true if the subscription is for data that
// belongs to the given field.
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

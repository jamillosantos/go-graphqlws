package graphqlws

import (
	"github.com/lab259/graphql"
)

// SubscriptionField adds the Subscribe method into a graphql.Field.
type SubscriptionField struct {
	graphql.Field
	Subscribe func(subscriber Subscriber) error
}

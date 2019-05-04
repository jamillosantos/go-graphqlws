package redis

import (
	"fmt"
	"sync"

	"github.com/gomodule/redigo/redis"
	"github.com/lab259/graphql"

	"github.com/jamillosantos/go-graphqlws"
)

type readerState int

const (
	readerStateUnknown readerState = iota
	readerStateRunning
	readerStateClosed
)

type SubscriptionHandler interface {
	graphqlws.SubscriptionStartHandler
	graphqlws.SubscriptionStopHandler
	graphqlws.WebsocketCloseHandler
}

type redisSubscriptionHandler struct {
	conn          *graphqlws.Conn
	subscriptions sync.Map
	pool          *redis.Pool
}

// NewSubscriptionHandler returns a new instance of a `SubscriptionHandler` that
// will add the logic for registering
func NewSubscriptionHandler(conn *graphqlws.Conn, pool *redis.Pool) SubscriptionHandler {
	handler := &redisSubscriptionHandler{
		conn: conn,
		pool: pool,
	}
	return handler
}

func (handler *redisSubscriptionHandler) HandleSubscriptionStart(subscription *graphqlws.Subscription) error {
	subsr := &subscriptionRedisReader{
		subscription: subscription,
		handler:      handler,
	}

	handler.conn.Logger.Debug("fields: ", subscription.Fields)

	subscriber := graphqlws.NewSubscriber()
	fieldsMap := subscription.Schema.SubscriptionType().Fields()
	for _, field := range subscription.Fields {
		subscriptionField, ok := fieldsMap[field]
		if !ok {
			return fmt.Errorf("field 'subscriptions.%s' was not found", field)
		}
		if subscriptionField.Subscribe == nil {
			return fmt.Errorf("'subscriptions.%s.Subscribe' is not defined", field)
		}

		err := subscriptionField.Subscribe(graphql.SubscribeParams{
			SubscriptionID: subscription.ID,
			Variables:      subscription.Variables,
			OperationName:  subscription.OperationName,
			Subscriber:     subscriber,
		})
		if err != nil {
			return err
		}
	}

	handler.subscriptions.Store(subscription.ID, subsr)
	go subsr.readPump(subscriber)
	return nil
}

func (handler *redisSubscriptionHandler) HandleSubscriptionStop(subscription *graphqlws.Subscription) error {
	subs, ok := handler.subscriptions.Load(subscription.ID)
	if !ok {
		return graphqlws.ErrSubscriptionNotFound
	}
	subsr := subs.(*subscriptionRedisReader) // This is safe because it is handled internally.
	subsr.close()
	return nil
}

func (handler *redisSubscriptionHandler) HandleWebsocketClose(code int, text string) error {
	handler.subscriptions.Range(func(key, value interface{}) bool {
		value.(*subscriptionRedisReader).close()
		return true
	})
	return nil
}

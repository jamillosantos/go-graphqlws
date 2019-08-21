package redis

import (
	"fmt"
	"reflect"
	"sync"

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
	dialer        Dialer
}

// NewSubscriptionHandler returns a new instance of a `SubscriptionHandler` that
// will add the logic for registering
func NewSubscriptionHandler(conn *graphqlws.Conn, dialer Dialer) SubscriptionHandler {
	handler := &redisSubscriptionHandler{
		conn:   conn,
		dialer: dialer,
	}
	return handler
}

// HandlerSubscriptionStart is called when a subscription starts. It iterates
// through the fields defined in the query, calling their `Subscribe` methods.
//
// At the end, it starts a `subscriptionRedisReader` to receive subscription
// info from Redis.
func (handler *redisSubscriptionHandler) HandleSubscriptionStart(subscription *graphqlws.Subscription) error {
	subsr := newSubscriptionRedisReader(handler, subscription)

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
			Data:           subscription,
		})
		if err != nil {
			return err
		}
	}

	handler.subscriptions.Store(subscription.ID, subsr)
	go subsr.readPump(subscriber)
	return nil
}

// HandleSubscriptionStop stops the `subscriptionRedisReader` using the
// subscription id stored in the handler.
func (handler *redisSubscriptionHandler) HandleSubscriptionStop(subscription *graphqlws.Subscription) error {
	subs, ok := handler.subscriptions.Load(subscription.ID)
	if !ok {
		return graphqlws.ErrSubscriptionNotFound
	}
	subsr := subs.(*subscriptionRedisReader) // This is safe because it is handled internally.
	subsr.close()
	return nil
}

// HandleWebsocketClose captures when the websocket is closed. Hence, closing
// the `subscriptionRedisReader` as well.
func (handler *redisSubscriptionHandler) HandleWebsocketClose(code int, text string) error {
	handler.subscriptions.Range(func(key, value interface{}) bool {
		reader, ok := value.(*subscriptionRedisReader)
		if !ok {
			handler.conn.Logger.Critical(reflect.TypeOf(value), " is not a *subscriptionRedisReader")
			return true
		}
		reader.subscription.Logger.Debug("redisSubscriptionHandler: closing reader")
		reader.close()
		return true
	})
	return nil
}

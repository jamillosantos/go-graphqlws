package redis

import (
	"sync"

	"github.com/gomodule/redigo/redis"

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
	conn.AddHandler(handler)
	return handler
}

func (handler *redisSubscriptionHandler) HandleSubscriptionStart(subscription *graphqlws.Subscription) error {
	subsr := &subscriptionRedisReader{
		subscription: subscription,
		handler:      handler,
	}
	handler.subscriptions.Store(subscription.ID, subsr)
	go subsr.readPump()
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

package redis

import (
	"context"
	"sync"

	"github.com/gomodule/redigo/redis"
	"github.com/jamillosantos/go-graphqlws"
	"github.com/graphql-go/graphql"
)

type graphqlWSContextKey string

// String wil return the key with a prefix to ensure the key is unique.
func (key graphqlWSContextKey) String() string {
	return "__graphql.context.key." + string(key)
}

var (
	// ConnectionContextKey is the key for keeping the connection reference on
	// the context.
	ConnectionContextKey graphqlWSContextKey = "connection"

	// SubscriptionContextKey is the key for keeping the subscription reference
	// on the context.
	SubscriptionContextKey graphqlWSContextKey = "subscription"
)

type subscriptionRedisReader struct {
	// topicsLen is saved to ensure the subscriptions were done correctly.
	topicsLen          int
	subscription       *graphqlws.Subscription
	handler            *redisSubscriptionHandler
	stateMutex         sync.Mutex
	_state             readerState
	pubSubConn         *redis.PubSubConn
	subscriptionErrors int
}

func newSubscriptionRedisReader(handler *redisSubscriptionHandler, subscription *graphqlws.Subscription) *subscriptionRedisReader {
	return &subscriptionRedisReader{
		subscription: subscription,
		handler:      handler,
	}
}

func (reader *subscriptionRedisReader) readPump(subscriber graphqlws.Subscriber) {
	defer func() {
		reader.subscription.Logger.Debug("exiting goroutine subscriptionRedisReader:readPump")
	}()
	// TODO Add crash protection.

	ctx := context.WithValue(
		context.WithValue(
			context.Background(),
			ConnectionContextKey,
			reader.subscription.Connection,
		),
		SubscriptionContextKey,
		reader.subscription,
	)

	reader.setState(readerStateRunning)
	for reader.getState() != readerStateClosed {
		reader.readLoop(ctx, subscriber)
	}
}

func (reader *subscriptionRedisReader) readLoop(ctx context.Context, subscriber graphqlws.Subscriber) {
	conn, err := reader.handler.dialer.Dial()
	if err != nil {
		reader.subscription.Logger.Error("error getting a redis conn: ", err)
		return
	}
	if err = conn.Err(); err != nil {
		reader.subscription.Logger.Error("the connection errored: ", err)
		return
	}
	pubSubConn := &redis.PubSubConn{Conn: conn}
	defer func() {
		pubSubConn.Close()
		reader.subscription.Logger.Trace(1000, "exit readLoop (subscriptionRedisReader)")
	}()

	topics := make([]interface{}, len(subscriber.Topics()))
	for i, topic := range subscriber.Topics() {
		topics[i] = topic
	}
	reader.topicsLen = len(topics)
	err = pubSubConn.Subscribe(topics...)
	if err != nil {
		// Increase the subscriptionErrors
		reader.subscriptionErrors++
		// If we get more than 3 subscriptionErrors in a row, we close the
		// connection.
		if reader.subscriptionErrors > 3 {
			reader.handler.conn.Close()
		}
		return
	}

	reader.subscription.Logger.Debug("subscribed to ", topics)

	// The subscription errors are reset
	reader.subscriptionErrors = 0

	reader.pubSubConn = pubSubConn
	for reader.getState() != readerStateClosed {
		err := reader.readIteration(ctx)
		if err != nil {
			reader.subscription.Logger.Error("error reading from redis: ", err)
			return
		}
	}
}

func (reader *subscriptionRedisReader) readIteration(ctx context.Context) error {
	msg := reader.pubSubConn.Receive()
	switch m := msg.(type) {
	case redis.Subscription:
		switch m.Count {
		case reader.topicsLen:
			reader.subscription.Logger.Trace(1000, "all subscriptions have been subscribed")
		case 0:
			reader.subscription.Logger.Trace(1000, "all subscriptions have been unsubscribed")
			return nil
		}
	case redis.Message:
		ctx := context.WithValue(ctx, m.Channel, m.Data)
		reader.subscription.Logger.Trace(graphqlws.TraceLevelInternalGQLMessages, "fromRedis: ", m.Channel, " ", string(m.Data))

		r := graphql.Execute(graphql.ExecuteParams{
			OperationName: reader.subscription.OperationName,
			AST:           reader.subscription.Document,
			Schema:        *reader.subscription.Connection.Schema,
			Context:       ctx,
			Args:          reader.subscription.Variables,
			Root:          nil,
		})
		err := reader.subscription.SendData(&graphqlws.GQLDataObject{
			Errors: graphqlws.ErrorsFromGraphQLErrors(r.Errors),
			Data:   r.Data,
		})
		if err != nil {
			reader.subscription.Logger.Error("the message could not be sent: ", err)
			// TODO Add some handler for handling these errors
		}
	case error:
		return m
	default:
		// This will be just ignored.
		reader.subscription.Logger.Debug("readIteration: default: ", m)
	}
	return nil
}

func (reader *subscriptionRedisReader) close() {
	if reader.getState() == readerStateClosed {
		reader.subscription.Logger.Debug("subscriptionRedisReader:close: already closed")
		return
	}
	reader.setState(readerStateClosed)
	if reader.pubSubConn != nil {
		reader.subscription.Logger.Debug("subscriptionRedisReader:close: closing pubSubConn")
		err := reader.pubSubConn.Close()
		if err != nil {
			reader.subscription.Logger.Error("subscriptionRedisReader:close: closing pubSubConn: ", err)
		}
		reader.pubSubConn = nil
	}
}

func (reader *subscriptionRedisReader) getState() readerState {
	reader.stateMutex.Lock()
	defer reader.stateMutex.Unlock()
	return reader._state
}

func (reader *subscriptionRedisReader) setState(value readerState) {
	reader.stateMutex.Lock()
	defer reader.stateMutex.Unlock()

	reader._state = value
}

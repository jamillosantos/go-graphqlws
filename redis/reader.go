package redis

import (
	"context"
	"sync"

	"github.com/gomodule/redigo/redis"
	"github.com/lab259/graphql"

	"github.com/jamillosantos/go-graphqlws"
)

type subscriptionRedisReader struct {
	subscription       *graphqlws.Subscription
	handler            *redisSubscriptionHandler
	stateMutex         sync.Mutex
	_state             readerState
	pubSubConn         *redis.PubSubConn
	subscriptionErrors int
}

func (reader *subscriptionRedisReader) readPump(subscriber graphqlws.Subscriber) {
	defer func() {
		reader.subscription.Logger.Debug("exiting goroutine subscriptionRedisReader:readPump")
	}()
	// TODO Add crash protection.

	reader.setState(readerStateRunning)
	for reader.getState() != readerStateClosed {
		reader.readLoop(subscriber)
	}
}

func (reader *subscriptionRedisReader) readLoop(subscriber graphqlws.Subscriber) {
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
	defer pubSubConn.Close()

	topics := make([]interface{}, len(subscriber.Topics()))
	for i, topic := range subscriber.Topics() {
		topics[i] = topic
	}
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
		err := reader.readIteration()
		if err != nil {
			reader.subscription.Logger.Error("error reading from redis: ", err)
			return
		}
	}
}

func (reader *subscriptionRedisReader) readIteration() error {
	defer reader.subscription.Logger.Info("defer subscriptionRedisReader:readIteration")
	msg := reader.pubSubConn.Receive()
	switch m := msg.(type) {
	case redis.Message:
		ctx := context.WithValue(context.Background(), m.Channel, m.Data)
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
		reader.subscription.Logger.Error("error reading from redis: ", m)
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

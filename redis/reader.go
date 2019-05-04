package redis

import (
	"context"

	"github.com/gomodule/redigo/redis"
	"github.com/lab259/graphql"

	"github.com/jamillosantos/go-graphqlws"
)

type subscriptionRedisReader struct {
	subscription       *graphqlws.Subscription
	handler            *redisSubscriptionHandler
	state              readerState
	pubSubConn         *redis.PubSubConn
	subscriptionErrors int
}

func (reader *subscriptionRedisReader) readPump(subscriber graphqlws.Subscriber) {
	// TODO Add crash protection.

	reader.state = readerStateRunning
	for reader.state != readerStateClosed {
		reader.readLoop(subscriber)
	}
}

func (reader *subscriptionRedisReader) readLoop(subscriber graphqlws.Subscriber) {
	conn := reader.handler.pool.Get()
	if conn.Err() != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	pubSubConn := &redis.PubSubConn{Conn: conn}

	topics := make([]interface{}, len(subscriber.Topics()))
	for i, topic := range subscriber.Topics() {
		topics[i] = topic
	}
	err := pubSubConn.Subscribe(topics...)
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

	reader.handler.conn.Logger.Debug("subscribed to ", topics)

	// The subscription errors are reset
	reader.subscriptionErrors = 0

	reader.pubSubConn = pubSubConn
	for reader.state != readerStateClosed {
		err := reader.readIteration(pubSubConn)
		if err != nil {
			reader.subscription.Connection.Logger.Error("error reading from redis: ", err)
			return
		}
	}
}

func (reader *subscriptionRedisReader) readIteration(conn *redis.PubSubConn) error {
	msg := conn.Receive()
	switch m := msg.(type) {
	case redis.Message:
		ctx := context.WithValue(context.Background(), m.Channel, m.Data)
		reader.subscription.Connection.Logger.Trace(graphqlws.TraceLevelInternalGQLMessages, "fromRedis: ", m.Channel, " ", string(m.Data))

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
			reader.subscription.Connection.Logger.Error("the message could not be sent: ", err)
			// TODO Add some handler for handling these errors
		}
	case error:
		reader.subscription.Connection.Logger.Error("error reading from redis: ", m)
		return m
	default:
		// This will be just ignored.
	}
	return nil
}

func (reader *subscriptionRedisReader) close() {
	if reader.state == readerStateClosed {
		return
	}
	reader.state = readerStateClosed
	if reader.pubSubConn != nil {
		_ = reader.pubSubConn.Close()
		reader.pubSubConn = nil
	}
}

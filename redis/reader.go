package redis

import (
	"context"

	"github.com/gomodule/redigo/redis"
	"github.com/lab259/graphql"

	"github.com/jamillosantos/go-graphqlws"
)

type subscriptionRedisReader struct {
	subscription *graphqlws.Subscription
	handler      *redisSubscriptionHandler
	state        readerState
	pubSubConn   *redis.PubSubConn
}

func (reader *subscriptionRedisReader) readPump() {
	// TODO Add crash protection.

	reader.state = readerStateRunning
	for reader.state != readerStateClosed {
		reader.readLoop()
	}
}

func (reader *subscriptionRedisReader) readLoop() {
	conn := reader.handler.pool.Get()
	if conn.Err() != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	pubSubConn := &redis.PubSubConn{Conn: conn}
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
	reader.state = readerStateClosed
	if reader.pubSubConn != nil {
		_ = reader.pubSubConn.Close()
		reader.pubSubConn = nil
	}
}

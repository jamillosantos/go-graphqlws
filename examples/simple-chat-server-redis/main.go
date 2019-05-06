package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/websocket"
	"github.com/jamillosantos/handler"
	"github.com/lab259/graphql"
	"github.com/lab259/rlog"
	"github.com/rs/cors"

	"github.com/jamillosantos/go-graphqlws"
	graphqlRedis "github.com/jamillosantos/go-graphqlws/redis"
)

type User struct {
	Name string `json:"name"`
}

type AuthToken struct {
	AuthToken string `json:"authToken"`
}

type Message struct {
	Text string `json:"text"`
	User *User  `json:"user"`
}

// connectionHandler implements the `WebsocketCloseHandler` and `ConnectionInitHandler`
// interfaces and will be binded to the `graphqlws.Conn` as new connections
// arrive.
type connectionHandler struct {
	conn          *graphqlws.Conn
	user          User
	broadcastLeft func(user *User)
	broadcastJoin func(user *User)
}

// HandleWebsocketClose will broadcast the messaage through Redis pubsub the
// message that a user left the chat.
func (handler *connectionHandler) HandleWebsocketClose(code int, text string) error {
	handler.broadcastLeft(&handler.user)
	delete(users, handler.user.Name)
	return nil
}

// HandleConnectionInit evaluates the payload sent by the `connection_init`
// package (check the graphqws protocol). Then, it validates it and broadcasts
// the message that the user joined the chat room.
func (handler *connectionHandler) HandleConnectionInit(init *graphqlws.GQLConnectionInit) error {
	var at AuthToken
	err := json.Unmarshal(init.Payload, &at)
	if err != nil {
		return err
	}
	if at.AuthToken == "" {
		return errors.New("the user name should be provided")
	}
	handler.user.Name = at.AuthToken

	users[handler.user.Name] = &handler.user

	handler.broadcastJoin(&handler.user)
	return nil
}

var users = make(map[string]*User)

func main() {
	rlog.Info("Starting example server")

	// Initializes the redis Pool
	//
	// The pool is used to publish messages.
	redisPool := &redis.Pool{
		Dial: func() (conn redis.Conn, e error) {
			return redis.Dial("tcp", "localhost:6379")
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Second*10 {
				return nil
			}
			if c.Err() != nil {
				return c.Err()
			}
			_, err := c.Do("PING")
			return err
		},
	}

	// Initializes the redis dialer
	//
	// The Dialer is used to create the Pubsub connections.
	redisDialer := graphqlRedis.NewDialer("tcp", "localhost:6379")

	// Test if the redis server is up and we can get response ...
	rlog.Info("Testing redis...")
	func() {
		redisConn, err := redisDialer.Dial()
		if err != nil {
			rlog.Critical("error dialing to the redis server: ", err)
			os.Exit(1)
		}
		defer func() {
			_ = redisConn.Close()
		}()
		_, err = redisConn.Do("PING")
		if err != nil {
			rlog.Critical("failed to initialize redis: ", err)
			os.Exit(1)
		}
	}()

	// This tests the redis servr upon the redisPool.
	rlog.Info("Testing redis...")
	func() {
		redisConn := redisPool.Get()
		defer func() {
			_ = redisConn.Close()
		}()
		_, err := redisConn.Do("PING")
		if err != nil {
			rlog.Critical("failed to initialize redis: ", err)
			os.Exit(1)
		}
	}()

	// Defines the UserType for the graphql.
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	// Defines the MessageType for the graphql.
	messageType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Message",
		Fields: graphql.Fields{
			"text": &graphql.Field{
				Type: graphql.String,
			},
			"user": &graphql.Field{
				Type: userType,
			},
		},
	})

	// Defines a non empty rootQuery (it is required by the graphql-go/graphql)
	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: graphql.Fields{
		"me": &graphql.Field{
			Type: userType,
			Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
				return &User{
					Name: "unknkown",
				}, nil
			},
		},
	}}

	// Defines the function that will publish data into Redis.
	broadcast := func(topic graphqlws.Topic, data interface{}) error {
		redisConn := redisPool.Get()
		defer func() {
			_ = redisConn.Close()
		}()

		dataJson, err := json.Marshal(data)
		if err != nil {
			return err
		}

		rlog.Info("broadcasting to ", topic, ": ", string(dataJson))

		_, err = redisConn.Do("PUBLISH", topic, dataJson)
		if err != nil {
			return err
		}

		return nil
	}

	// broadcastJoin broadcast the message that a has joined the chat room.
	broadcastJoin := func(user *User) {
		rlog.Info(user.Name, " joined")
		err := broadcast(graphqlws.StringTopic("onJoin"), user)
		if err != nil {
			rlog.Error("failed to broadcastJoin: ", err)
		}
	}

	// broadcastLeft broadcast the message that a user has left the chat room.
	broadcastLeft := func(user *User) {
		rlog.Info(user.Name, " left")
		err := broadcast(graphqlws.StringTopic("onLeft"), user)
		if err != nil {
			rlog.Error("failed to broadcastLeft: ", err)
		}
	}

	// broadcastMessage broadcast a message from a user to the chat room.
	broadcastMessage := func(message *Message) {
		err := broadcast(graphqlws.StringTopic("onMessage"), message)
		if err != nil {
			rlog.Error("failed to broadcastMessage: ", err)
		}
	}

	// Define the schema of the graphql
	schemaConfig := graphql.SchemaConfig{
		Query: graphql.NewObject(rootQuery),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "MutationRoot",
			Fields: graphql.Fields{
				// This the mutation that sends messages to the server.
				"send": &graphql.Field{
					Args: graphql.FieldConfigArgument{
						"user": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(graphql.String),
						},
						"text": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(graphql.String),
						},
					},
					Type: messageType,
					Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
						userName := p.Args["user"].(string)

						// Finds the user in the array.
						user, ok := users[userName]
						if !ok {
							return nil, fmt.Errorf("user '%s' not found", userName)
						}

						text, ok := p.Args["text"]
						if !ok {
							return nil, errors.New("you must pass the text")
						}
						m := &Message{
							Text: text.(string),
							User: user,
						}

						// Broadcast the message to all subscribed users.
						broadcastMessage(m)
						return m, nil
					},
				},
			},
		}),
		// Define all possible subscriptions
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "SubscriptionRoot",
			Fields: graphql.Fields{
				"onJoin": &graphql.Field{
					Type: userType,
					Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
						userRaw, ok := p.Context.Value("onJoin").([]byte)
						if !ok {
							return nil, nil
						}
						var user User
						err := json.Unmarshal(userRaw, &user)
						if err != nil {
							return nil, err
						}
						return &user, nil
					},
					Subscribe: func(params graphql.SubscribeParams) error {
						// Subscribe the user in the `onJoin` topic.
						return params.Subscriber.SubscriberSubscribe(graphqlws.StringTopic("onJoin"))
					},
				},
				"onLeft": &graphql.Field{
					Type: userType,
					Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
						userRaw, ok := p.Context.Value("onLeft").([]byte)
						if !ok {
							return nil, nil
						}
						var user User
						err := json.Unmarshal(userRaw, &user)
						if err != nil {
							return nil, err
						}
						return &user, nil
					},
					Subscribe: func(params graphql.SubscribeParams) error {
						// Subscribe the user in the `onLeft` topic.
						return params.Subscriber.SubscriberSubscribe(graphqlws.StringTopic("onLeft"))
					},
				},
				"onMessage": &graphql.Field{
					Type: messageType,
					Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
						messageRaw, ok := p.Context.Value("onMessage").([]byte)
						if !ok {
							return nil, nil
						}
						var message Message
						err := json.Unmarshal(messageRaw, &message)
						if err != nil {
							return nil, err
						}
						return &message, nil
					},
					Subscribe: func(params graphql.SubscribeParams) error {
						// Subscribe the user in the `onMessage` topic.
						return params.Subscriber.SubscriberSubscribe(graphqlws.StringTopic("onMessage"))
					},
				},
			},
		}),
	}

	// Creates the schema
	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		rlog.WithField("err", err).Critical("GraphQL schema is invalid")
		os.Exit(1)
	}

	// Initializes the graphql http handler.
	graphqlHandler := handler.New(&handler.Config{
		Schema:     &schema,
		Pretty:     true,
		GraphiQL:   false,
		Playground: true,
	})

	// Initializes the graphqlws http handler.
	subscriptionHandler := graphqlws.NewHttpHandler(
		graphqlws.NewHandlerConfigFactory().
			Schema(&schema).
			Upgrader(graphqlws.NewUpgrader(&websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			})).
			Build(),
		graphqlws.NewConfigFactory().Build(),
		func(conn *graphqlws.Conn, err error) {
			if err != nil {
				rlog.Error("error accepting a new subscription: ", err)
				return
			}
			redisHandler := graphqlRedis.NewSubscriptionHandler(conn, redisDialer)
			conn.AddHandler(redisHandler)
			conn.AddHandler(&connectionHandler{
				broadcastJoin: broadcastJoin,
				broadcastLeft: broadcastLeft,
			})
		},
	)

	// Go routine that will print the number of goroutines each second.
	go func() {
		for {
			rlog.WithField("goRoutines", runtime.NumGoroutine()).Debug("stats")
			time.Sleep(time.Second)
		}
	}()

	// Serve the GraphQL and GraphQL WS endpoint
	mux := http.NewServeMux()
	mux.Handle("/graphql", graphqlHandler)
	mux.Handle("/subscriptions", subscriptionHandler)
	address := ":8085"
	rlog.Info("Starting http server at", address)
	rlog.Info("    /graphql")
	rlog.Info("    /subscriptions")
	if err := http.ListenAndServe(address, cors.AllowAll().Handler(mux)); err != nil {
		rlog.WithField("err", err).Critical("Failed to start server")
	}
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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

type connectionHandler struct {
	conn          *graphqlws.Conn
	user          User
	broadcastLeft func(user *User)
	broadcastJoin func(user *User)
}

func (handler *connectionHandler) HandleWebsocketClose(code int, text string) error {
	handler.broadcastLeft(&handler.user)
	delete(users, handler.user.Name)
	return nil
}

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

	rlog.Info("Testing redis...")
	func() {
		redisConn := redisPool.Get()
		defer redisConn.Close()
		_, err := redisConn.Do("PING")
		if err != nil {
			rlog.Critical("failed to initialize redis: ", err)
			os.Exit(1)
		}
	}()

	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

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

	broadcast := func(topic graphqlws.Topic, data interface{}) error {
		redisConn := redisPool.Get()
		defer redisConn.Close()

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

	// Sends this to all subscriptions
	broadcastJoin := func(user *User) {
		rlog.Info(user.Name, " joined")
		err := broadcast(graphqlws.StringTopic("onJoin"), user)
		if err != nil {
			rlog.Error("failed to broadcastJoin: ", err)
		}
	}

	broadcastLeft := func(user *User) {
		rlog.Info(user.Name, " left")
		err := broadcast(graphqlws.StringTopic("onLeft"), user)
		if err != nil {
			rlog.Error("failed to broadcastLeft: ", err)
		}
	}

	broadcastMessage := func(message *Message) {
		err := broadcast(graphqlws.StringTopic("onMessage"), message)
		if err != nil {
			rlog.Error("failed to broadcastMessage: ", err)
		}
	}
	schemaConfig := graphql.SchemaConfig{
		Query: graphql.NewObject(rootQuery),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "MutationRoot",
			Fields: graphql.Fields{
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
						broadcastMessage(m)
						return m, nil
					},
				},
			},
		}),
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
						return params.Subscriber.SubscriberSubscribe(graphqlws.StringTopic("onMessage"))
					},
				},
			},
		}),
	}
	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		rlog.WithField("err", err).Critical("GraphQL schema is invalid")
		os.Exit(1)
	}

	graphqlHandler := handler.New(&handler.Config{
		Schema:     &schema,
		Pretty:     true,
		GraphiQL:   false,
		Playground: true,
	})

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
			redisHandler := graphqlRedis.NewSubscriptionHandler(conn, redisPool)
			conn.AddHandler(redisHandler)
			conn.AddHandler(&connectionHandler{
				broadcastJoin: broadcastJoin,
				broadcastLeft: broadcastLeft,
			})
		},
	)

	// Serve the GraphQL WS endpoint
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

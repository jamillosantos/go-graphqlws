package main

import (
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
	"github.com/lab259/rlog/v2"

	"github.com/jamillosantos/go-graphqlws"
)

type connHandler struct {
	conn *graphqlws.Conn
}

func (*connHandler) HandleSubscriptionStart(subscription *graphqlws.Subscription) error {
	panic("implement me")
}

var messageType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Message",
	Fields: graphql.Fields{
		"text": &graphql.Field{
			Type: graphql.String,
		},
	},
})

var rootQuery = graphql.ObjectConfig{Name: "RootQuery", Fields: graphql.Fields{
	"message": &graphql.Field{
		Type: messageType,
		Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
			return &Message{
				Text: "Hello world",
			}, nil
		},
	},
}}

type Message struct {
	Text string `json:"text"`
}

func main() {
	schemaConfig := graphql.SchemaConfig{
		Query: graphql.NewObject(rootQuery),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "MutationRoot",
			Fields: graphql.Fields{
				"message": &graphql.Field{
					Args: graphql.FieldConfigArgument{
						"text": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(graphql.String),
						},
					},
					Type: messageType,
					Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
						text := p.Args["text"].(string)
						return &Message{
							Text: text,
						}, nil
					},
				},
			},
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "SubscriptionRoot",
			Fields: graphql.Fields{
				"onMessage": {
					Type: messageType,
					Resolve: func(p graphql.ResolveParams) (i interface{}, e error) {
						message, ok := p.Context.Value("message").(*Message)
						if !ok {
							return nil, nil
						}
						return message, nil
					},
					Subscribe: func(subscriber graphql.SubscribeParams) error {
						return subscriber.Subscriber.SubscriberSubscribe(graphqlws.StringTopic("onMessage"))
					},
				},
			},
		}),
	}

	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		rlog.WithField("err", err).Critical("GraphQL schema is invalid")
	}

	mux := http.NewServeMux()
	mux.Handle("/subscriptions", graphqlws.NewHttpHandler(
		graphqlws.NewHandlerConfigFactory().
			Upgrader(graphqlws.NewUpgrader(&websocket.Upgrader{})).
			Schema(&schema).
			Build(),
		graphqlws.NewConfigFactory().
			PongWait(time.Second).
			Build(),
		func(conn *graphqlws.Conn, err error) {
			if err != nil {
				rlog.Error(err)
				return
			}
			conn.AddHandler(&connHandler{conn: conn,})
		},
	))

	var server http.Server
	server.Addr = ":40385"
	server.Handler = mux
	rlog.Info("Binding to ", server.Addr)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		err = server.Close()
		if err != nil {
			rlog.Error("error on closing the server: ", err)
		}
	}()

	go func() {
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			rlog.
				WithField("ConnectionCount", graphqlws.ConnectionCount).
				WithField("Alloc", m.Alloc).
				WithField("HeapAlloc (MB)", m.HeapAlloc/1024/1024).
				WithField("HeapInuse (MB)", m.HeapInuse/1024/1024).
				WithField("NumGoroutine", runtime.NumGoroutine()).
				Info("state")
			time.Sleep(time.Second)
		}
	}()
	err = server.ListenAndServe()
	if err != nil {
		rlog.Critical(err)
	}
}

# go-graphqlws [![CircleCI](https://circleci.com/gh/jamillosantos/go-graphqlws.svg?style=shield)](https://circleci.com/gh/jamillosantos/go-graphqlws)

**THIS LIBRARY IS A WORK IN PROGRESS**

**THIS DOCUMENTATION IS A DRAFT**

go-graphqlws extends the [graphql-go/graphql](https://github.com/graphql-go/graphql)
by implementing the graphqlws protocol on top of the [gorilla](https://github.com/gorilla/websocket)
websocket server implementation which uses the default `net/http` library.

:exclamation: In order to get it done, we had to fork `graphql-go/graphql` and
make a couple changes that were not merged (yet, I hope) back in the main
repository. The idea is once we get it done, this library will starting using
`graphql-go/graphql`.

This implementation use a lot of the ideas contained in [functionalfoundry/graphqlws](https://github.com/functionalfoundry/graphqlws).
But, it is architecturally different. But, some of the code was heavily based on
that.

## Before you start

### Handshake

The endpoint, let's say '/subscriptions' will trigger a `graphqlws.Handler` that
will upgrade the connection and call the connection handler method:

	func (conn *graphql.Conn, err error)

This method will inform if any error happened. If not, a connection will be
provided. At this point, when the connection is already established, you should
add all **handlers** responsible for dealing with all events happens with the
client.

Check the 1st example on the next session (Handlers).

### Handlers

Handlers are interfaces that can be implemented and added to the `graphqlws.Conn`
in order to add new behavior to it. On the following example it will adding a
message for everytime a connection is closed.

```go
// ...
type connHandler struct {}

func (handler *connHandler) HandleWebsocketClose(code int, text string) error {
	fmt.Println("the connection was closed!")
	return nil
}
// ...
mux.Handle("/subscriptions", graphqlws.NewHttpHandler(
	graphqlws.NewHandlerConfigFactory().Upgrader(graphqlws.NewUpgrader(&websocket.Upgrader{})).Schema(&schema).Build(),
	graphqlws.NewConfigFactory().Build(),
	func(conn *graphqlws.Conn, err error) { // Handshake referred on the previous session
		if err != nil {
			rlog.Error(err)
			return
		}
		conn.AddHandler(&connHandler{conn: conn})
	},
))
// ...
```

One type struct can implement one or many structs, it totally depends on what
is your need and how are you going to arrange them to get your desired
behaviour.

```go
type SystemRecoverHandler interface {
	HandlePanic(t RWType, r interface{}) error
}

type ConnectionInitHandler interface {
	HandleConnectionInit(*GQLConnectionInit) error
}

type ConnectionStartHandler interface {
	HandleConnectionStart(*GQLStart) []error
}

type ConnectionStopHandler interface {
	HandleConnectionStop(*GQLStop) error
}

type ConnectionTerminateHandler interface {
	HandleConnectionTerminate(*GQLConnectionTerminate) error
}

type SubscriptionStartHandler interface {
	HandleSubscriptionStart(subscription *Subscription) error
}

type SubscriptionStopHandler interface {
	HandleSubscriptionStop(subscription *Subscription) error
}

type WebsocketPongHandler interface {
	HandleWebsocketPong(message string) error
}

type WebsocketPingHandler interface {
	HandleWebsocketPing() error
}

type WebsocketCloseHandler interface {
	HandleWebsocketClose(code int, text string) error
}
```

A useful example is at the `connection_init` step (simple-chat-server-redis example),
where the payload is analyzed to check if the user has permission to connect:

```go
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

	broadcastJoin(&handler.user)
	return nil
}
```

This example is simple but introduces the idea. A better approach would use JWT,
for example.

Handlers are powerful enough that our Redis implementation uses them to add 
subscription capabilities.

For more information about what each handler does, please refer to the [Godoc](https://godoc.org/github.com/jamillosantos/go-graphqlws).

## Getting Started

Even a small example turned out to be too much code to be entered here. So,
instead, we recommend you to follow our `simple-chat-server-redis` example. It
is well documented and follow these steps:

1. Initialize your preferred PubSub technology;

	_Today, we only support Redis, but it can easily be extended. PRs are welcome._

2. Graphql Schema definition;

	_Inside of the graphql schema definition you should have:_ 

3. Subscription definition;

	_You should define the `Subscribe` field of the graphql._ 
	
4. Define a GraphQL handler;

5. Define a GraphQL WS handler;

6. Start the http server.

## How Subscribe and Resolve are called?

The `Subscribe` method is called anytime a new subscription is started. This
library will call `Subscribe` method of all subscription fields described in the
query sent by the client.

When a field subscribes subscribes to a channel, it will be called anytime that
subscription returns any data.

But, as we know, graphqlws subscriptions might subscribe themselves to multiple
channels at once. So, we use the channel identification to distinguish between
calls.

In the snipped below, extracted from the `simple-chat-server-redis` example, you
will see that the data, received from Redis, is extracted from the
`p.ResolveParams.Context`. If it cannot be found, the Resolve execution is
cancelled by returning `nil, nil`.

```go
// Rest of the graphql schema definition...
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
				// Here onJoin is confirmed.
				// ...
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
				// Here onLeft is confirmed.
				// ...
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
				// Here onMessage is confirmed.
				// ...
			},
			Subscribe: func(params graphql.SubscribeParams) error {
				// Subscribe the user in the `onMessage` topic.
				return params.Subscriber.SubscriberSubscribe(graphqlws.StringTopic("onMessage"))
			},
		},
	},
}),
// Rest of the graphql schema definition...
``` 

## License

MIT
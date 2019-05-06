# go-graphqlws

**THIS IS A WORK IN PROGRESS**

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
	
4. A graphql handler;

5. A graphqlws handler;

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
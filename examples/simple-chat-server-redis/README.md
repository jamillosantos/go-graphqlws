# Example

You will find the UI for this example at [lab259/graphqlws-chat-client](https://github.com/lab259/graphqlws-chat-client).

Before running the example you should:

```
docker-compose up -d
```

Now, fallback to the main source directory, the one that has the `Makefile` in it.
You will install all the dependencies by:

```
make dep-ensure
```

Finally, you should be able to run this example:

```
make example-simple-chat-server-redis
```
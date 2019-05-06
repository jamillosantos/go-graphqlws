package redis

import (
	"github.com/gomodule/redigo/redis"
)

// Dialer is responsible for creating new redis connections.
//
// As connections are being used for subscriptions, it won't be pooled.
type Dialer interface {
	Dial() (redis.Conn, error)
}

type dialer struct {
	protocol string
	address  string
	options  []redis.DialOption
}

// NewDialer dials for a new connection. It will return the default
// implementation of the `Dialer` interface.
func NewDialer(protocol, address string, options ...redis.DialOption) Dialer {
	return &dialer{
		protocol: protocol,
		address:  address,
		options:  options,
	}
}

func (d *dialer) Dial() (redis.Conn, error) {
	return redis.Dial(d.protocol, d.address, d.options...)
}

package graphqlws

import (
	"time"

	"github.com/lab259/graphql"
)

const (
	// KB represents a Kilobyte size.
	KB = int64(1024)

	// MB represents a Megabyte size.
	MB = int64(1024) * KB

	// GB represents a Gigabyte.
	GB = int64(1024) * MB
)

type Config struct {
	// ReadLimit is the maximum size of the buffer used to receive raw messages
	// from the websocket.
	ReadLimit *int64

	// PongWait is how much time we will wait without sending any message to the
	// client before sending a PONG.
	PongWait *time.Duration

	// WriteTimeout is how much time is wait for sending a message to the
	// websocket before triggering a timeout error.
	WriteTimeout *time.Duration
}

var (
	defaultReadLimit    = 5 * KB
	defaultPongWait     = time.Second * 60
	defaultWriteTimeout = time.Second * 15
)

var DefaultConfig = Config{
	ReadLimit:    &defaultReadLimit,
	PongWait:     &defaultPongWait,
	WriteTimeout: &defaultWriteTimeout,
}

type configFactory struct {
	config Config
}

func NewConfigFactory() *configFactory {
	return &configFactory{
		config: DefaultConfig,
	}
}

// ReadLimit sets the value of the `ReadLimit` property of the `Config`.
func (cf *configFactory) ReadLimit(value int64) *configFactory {
	cf.config.ReadLimit = &value
	return cf
}

// PongWait sets the value of the `PongWait` property of the `Config`.
func (cf *configFactory) PongWait(value time.Duration) *configFactory {
	cf.config.PongWait = &value
	return cf
}

// WriteTimeout sets the value of the `WriteTimeout` property of the `Config`.
func (cf *configFactory) WriteTimeout(value time.Duration) *configFactory {
	cf.config.WriteTimeout = &value
	return cf
}

// Build returns a new config with all configurations set.
func (cf *configFactory) Build() Config {
	return cf.config
}

// HandlerConfig
type HandlerConfig struct {
	Upgrader WebSocketUpgrader
	Schema   *graphql.Schema
}

type handlerConfigFactory struct {
	config HandlerConfig
}

func NewHandlerConfigFactory() *handlerConfigFactory {
	return &handlerConfigFactory{}
}

func (f *handlerConfigFactory) Schema(schema *graphql.Schema) *handlerConfigFactory {
	f.config.Schema = schema
	return f
}

func (f *handlerConfigFactory) Upgrader(upgrader WebSocketUpgrader) *handlerConfigFactory {
	f.config.Upgrader = upgrader
	return f
}

func (f *handlerConfigFactory) Build() HandlerConfig {
	return f.config
}

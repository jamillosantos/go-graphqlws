package graphqlws

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

type Config struct {
	ReadLimit    int64
	PongWait     time.Duration
	WriteTimeout time.Duration
}

type configFactory struct {
	config Config
}

func NewConfigFactory() *configFactory {
	return &configFactory{}
}

// ReadLimit sets the value of the `ReadLimit` property of the `Config`.
func (cf *configFactory) ReadLimit(value int64) *configFactory {
	cf.config.ReadLimit = value
	return cf
}

// PongWait sets the value of the `PongWait` property of the `Config`.
func (cf *configFactory) PongWait(value time.Duration) *configFactory {
	cf.config.PongWait = value
	return cf
}

// WriteTimeout sets the value of the `WriteTimeout` property of the `Config`.
func (cf *configFactory) WriteTimeout(value time.Duration) *configFactory {
	cf.config.WriteTimeout = value
	return cf
}

// Build returns a new config with all configurations set.
func (cf *configFactory) Build() Config {
	return cf.config
}

type HandlerConfig struct {
	Upgrader *websocket.Upgrader
}

type handlerConfigFactory struct {
	config HandlerConfig
}

func NewHandlerConfigFactory() *handlerConfigFactory {
	return &handlerConfigFactory{}
}

func (f *handlerConfigFactory) Upgrader(upgrader *websocket.Upgrader) *handlerConfigFactory {
	f.config.Upgrader = upgrader
	return f
}

func (f *handlerConfigFactory) Build() HandlerConfig {
	return f.config
}

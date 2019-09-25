package graphqlws

import (
	"encoding/json"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/lab259/rlog/v2"
)

// ConnState is the state of the connection.
type ConnState int

const (
	connStateUndefined ConnState = iota
	connStateInitializing
	connStateEstablished
	connStateClosed
)

var (
	// operationMessageEOF is a message that when is detected on the writePump, closes the connection.
	operationMessageEOF = &OperationMessage{}
)

var (
	// ConnectionCount is the counter of new connections, also used to identify
	// each individual connection in the log.
	ConnectionCount uint64
)

// Conn is a connection with a client.
type Conn struct {
	Logger           rlog.Logger
	Schema           *graphql.Schema
	Subscriptions    sync.Map
	stateMutex       sync.RWMutex
	_state           ConnState
	handlersMutex    sync.Mutex
	Handlers         []Handler
	conn             *websocket.Conn
	config           *Config
	outgoingMessages chan *OperationMessage
	incomingMessages chan *OperationMessage
	shutdown         chan struct{}
}

// NewConn initializes a `Conn` instance.
func NewConn(conn *websocket.Conn, schema *graphql.Schema, config *Config) (*Conn, error) {
	c := &Conn{
		Schema:           schema,
		config:           config,
		Logger:           rlog.WithField("conn", atomic.AddUint64(&ConnectionCount, 1)),
		conn:             conn,
		incomingMessages: make(chan *OperationMessage, 10),
		outgoingMessages: make(chan *OperationMessage, 10),
		shutdown:         make(chan struct{}),
		Handlers:         make([]Handler, 0, 3),
	}
	go c.readPump()
	go c.processMessages()
	return c, nil
}

// getState returns the state of the conversation. In order to ensure no race
// conditions happen, it uses a `RLock`.
func (c *Conn) getState() ConnState {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	return c._state
}

// setState is the setter of the `Conn._state`. It locks a `RWMutex` to ensure
// that no race conditions happen.
func (c *Conn) setState(value ConnState) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()

	c._state = value
}

// AddHandler adds a `Handler` to the connection.
//
// See also `Handler`
func (c *Conn) AddHandler(handler Handler) {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()

	c.Handlers = append(c.Handlers, handler)
}

// RemoveHandler removes a `Handler` from the connection.
//
// See also `Handler`
func (c *Conn) RemoveHandler(handler Handler) {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()

	hs := c.Handlers
	for i, h := range c.Handlers {
		if h == handler {
			hs = append(hs[:i], hs[i+1:])
			break
		}
	}
	c.Handlers = hs
}

// SendData enqueues a message to be sent by the writePump.
func (c *Conn) SendData(message *OperationMessage) {
	c.outgoingMessages <- message
}

// SendError sends an error to the client.
func (c *Conn) SendError(err error) error {
	if c.getState() == connStateClosed {
		return ErrConnectionClosed
	}

	errJSON, err2 := json.Marshal(err.Error())
	if err2 != nil {
		return err2
	}
	c.outgoingMessages <- &OperationMessage{
		Type:    gqlTypeError,
		Payload: errJSON,
	}
	return nil
}

func (c *Conn) sendConnectionError(err error) error {
	// Ensure that this conn is not locked.
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	if c._state == connStateClosed {
		return ErrConnectionClosed
	}

	errJSON, err2 := json.Marshal(err.Error())
	if err2 != nil {
		return err2
	}

	// Write directly to the output channel for being sent to the customer.
	c.outgoingMessages <- &OperationMessage{
		Type:    gqlTypeConnectionError,
		Payload: errJSON,
	}
	return nil
}

func (c *Conn) sendOperationErrors(id string, errs []error) error {
	// Ensure that this conn is not locked.
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	if c._state == connStateClosed {
		return ErrConnectionClosed
	}

	errJSON, err := json.Marshal(errs)
	if err != nil {
		return err
	}

	// Write directly to the output channel for being sent to the customer.
	c.outgoingMessages <- &OperationMessage{
		Type:    gqlTypeError,
		ID:      id,
		Payload: errJSON,
	}
	return nil
}

func (c *Conn) close() error {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()

	if c._state == connStateClosed {
		c.Logger.Debug("ignoring close: already closed")
		return ErrConnectionClosed
	}

	c._state = connStateClosed

	close(c.shutdown)

	// Go through the handlers and call all `WebsocketCloseHandler`s found.
	for _, handler := range c.Handlers {
		h, ok := handler.(WebsocketCloseHandler)
		if !ok { // If not a `ConnectionStartHandler` try next.
			continue
		}
		err := h.HandleWebsocketClose(0, "")
		hErr, ok := err.(*HandlerError)
		if ok {
			if hErr.propagationStopped {
				break
			}
		} else if err != nil {
			return err
		}
	}

	// Add a wait to give time for the shutdown to be processed.
	time.Sleep(time.Millisecond * 10)

	close(c.outgoingMessages)
	close(c.incomingMessages)

	return c.conn.Close()
}

func (c *Conn) pongHandler(message string) error {
	pongWait := time.Second * 60 // Default pong timeout

	if c.config.PongWait != nil {
		pongWait = *c.config.PongWait
	}

	// Set the deadline for the next read
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return err
	}

	// Go through the handlers and call all `WebsocketPongHandler`s found.
	for _, handler := range c.Handlers {
		h, ok := handler.(WebsocketPongHandler)
		if !ok { // If not a `WebsocketPongHandler` try next.
			continue
		}
		err = h.HandleWebsocketPong(message)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Conn) lockHandlers(f func() error) error {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()

	return f()
}

// closeHandler is called when the connection is closed by the peer.
func (c *Conn) closeHandler(code int, text string) error {
	c.Logger.Trace(TraceLevelConnectionEvents, "closeHandler: closing: ", c.conn.RemoteAddr())
	defer func() {
		c.Logger.Trace(TraceLevelConnectionEvents, "closeHandler: defer closing: ", c.conn.RemoteAddr())
	}()

	return c.close()
}

func (c *Conn) recover(t RWType) {
	if r := recover(); r != nil {
		// This event cannot be default prevented.

		// In this case, lock handlers will do no good.
		//
		// Broadcast the message to all handlers attached.
		for _, handler := range c.Handlers {
			// Of course, only `SystemRecoverHandler` will be called.
			h, ok := handler.(SystemRecoverHandler)
			if !ok {
				continue
			}
			err := h.HandlePanic(t, r)
			if hErr, ok := err.(*HandlerError); ok {
				if hErr.propagationStopped {
					break
				}
			}
		}

		// Logs the stack with some information.
		stack := make([]byte, 2048)
		n := runtime.Stack(stack, false)
		c.Logger.Critical("panicked: ", r)
		c.Logger.Debug(string(stack[:n]))

		if c.getState() == connStateClosed {
			return
		}

		c.incomingMessages <- operationMessageEOF
	}
}

// addSubscription appends a subscription to the connection.
func (c *Conn) addSubscription(subscription *Subscription) {
	c.Subscriptions.Store(subscription.ID, subscription)
}

// removeSubscription remove a subcription from the connection.
func (c *Conn) removeSubscription(id string) {
	c.Subscriptions.Delete(id)
}

func (c *Conn) gqlStart(start *GQLStart) {
	errs := make([]error, 0, 1)
	// Go through the handlers and call all `ConnectionStartHandler`s found.
	for _, handler := range c.Handlers {
		h, ok := handler.(ConnectionStartHandler)
		if !ok { // If not a `ConnectionStartHandler` try next.
			continue
		}
		errsIn := h.HandleConnectionStart(start)
		if len(errs) > 0 { // Keep aggregating errors
			errs = append(errs, errsIn...)
		}
	}

	// If any error has happened ...
	if len(errs) > 0 {
		c.Logger.Error("failed to HandleConnectionStart at gqlStart: ", errs)
		// ... send it to the client.
		err := c.sendOperationErrors(start.ID, errs)
		if err != nil {
			c.Logger.Error("failed to sendOperationErrors when HandleConnectionStart errors at gqlStart: ", err)
		}
		return
	}

	subscription := &Subscription{
		ID:            start.ID,
		Query:         start.Payload.Query,
		Variables:     start.Payload.Variables,
		OperationName: start.Payload.OperationName,
		Connection:    c,
		Schema:        c.Schema,
		Logger:        c.Logger.WithField("subscriptionID", start.ID),
	}

	logger := c.Logger.WithFields(rlog.Fields{
		"subscription": subscription.ID,
	})

	if errors := ValidateSubscription(subscription); len(errors) > 0 {
		logger.WithField("errors", errors).Warn("Failed to add invalid subscription")
		return // errors
	}

	// Parses the subscription query
	document, err := parser.Parse(parser.ParseParams{
		Source: subscription.Query,
	})
	if err != nil {
		logger.WithField("err", err).Warn("Failed to parse subscription query")
		return // []error{err}
	}

	// Validate the query document
	validation := graphql.ValidateDocument(c.Schema, document, nil)
	if !validation.IsValid {
		logger.WithFields(rlog.Fields{
			"errors": validation.Errors,
		}).Error("Failed to validate subscription query")
		return // ErrorsFromGraphQLErrors(validation.Errors)
	}

	// Remember the query document for later
	subscription.Document = document

	// Extract query names from the document (typically, there should only be one)
	subscription.Fields = SubscriptionFieldNamesFromDocument(document)

	c.addSubscription(subscription)

	// Go through the handlers and call all `ConnectionTerminateHandler`s found.
	for _, handler := range c.Handlers {
		h, ok := handler.(SubscriptionStartHandler)
		if !ok { // If not a `ConnectionStartHandler` try next.
			continue
		}
		err := h.HandleSubscriptionStart(subscription)
		if hErr, ok := err.(*HandlerError); ok {
			// This event cannot be default prevented.
			if hErr.propagationStopped {
				break
			}
		} else if err != nil {
			c.Logger.Error("error terminating the connection: ", err)
		}
	}
}

func (c *Conn) gqlStop(stop *GQLStop) {
	// Go through the handlers and call all `ConnectionStopHandler`s found.
	for _, handler := range c.Handlers {
		h, ok := handler.(ConnectionStopHandler)
		if !ok { // If not a `ConnectionStartHandler` try next.
			continue
		}
		err := h.HandleConnectionStop(stop)
		if err != nil {
			// TODO Call the default error handler.
		}
	}

	subs, ok := c.Subscriptions.Load(stop.ID)
	if !ok { // If the subscription does not exists.
		c.Logger.Errorf("could not stop a non existing subscription: %s", stop.ID)
		return
	}

	subscription := subs.(*Subscription) // This is internally managed. So, it should be safe to force the typcast.

	// Go through the handlers and call all `SubscriptionStopHandler`s found.
	for _, handler := range c.Handlers {
		h, ok := handler.(SubscriptionStopHandler)
		if !ok { // If not a `ConnectionStartHandler` try next.
			continue
		}
		err := h.HandleSubscriptionStop(subscription)
		if hErr, ok := err.(*HandlerError); ok {
			// This event cannot be default prevented.
			if hErr.propagationStopped {
				break
			}
		} else if err != nil {
			c.Logger.Error("error terminating the connection: ", err)
		}
	}
}

func (c *Conn) readPump() {
	defer func() {
		c.Logger.Debug("leaving readPump (Conn)")
	}()
	defer c.close()

	c.setState(connStateInitializing)

	if c.config.ReadLimit != nil {
		// Prepare for the first pong.
		// The read limit is the size of the package that will be read per once.
		// That, might be adjustable depending your needs.
		c.conn.SetReadLimit(*c.config.ReadLimit)
	}

	c.conn.SetPongHandler(c.pongHandler)
	c.conn.SetCloseHandler(c.closeHandler)

	for c.getState() != connStateClosed {
		operationMessage := new(OperationMessage)

		// Read JSON from the connection.
		err := c.conn.ReadJSON(&operationMessage)
		switch err.(type) {
		// These errors stops the connection.
		case *websocket.CloseError:
			c.Logger.Error("*websocket.CloseError: ", err)
			return
		case *net.OpError:
			c.Logger.Error("*net.OpError: ", err)
			return
		case net.Error:
			c.Logger.Error("net.Error: ", err)
			c.incomingMessages <- operationMessageEOF
			return
		default:
			if err != nil {
				c.Logger.Error("default: ", err)
				// This error just stop the iteration.
				c.incomingMessages <- operationMessageEOF
				return
			}
		}

		c.Logger.WithFieldsArr(
			"id", operationMessage.ID,
			"type", operationMessage.Type,
			"payload", string(operationMessage.Payload),
		).Trace(TraceLevelConnectionEvents, "packet arrived.")

		c.incomingMessages <- operationMessage
	}
}

var emptyBytes = []byte{}

func (c *Conn) processMessages() {
	defer c.recover(Write)
	pongWait := time.Second * 60 // Default pong timeout

	if c.config.PongWait != nil {
		pongWait = *c.config.PongWait
	}

	pingTicker := time.NewTicker((pongWait * 9) / 10)
	defer func() {
		c.Logger.Debug("leaving processMessages (Conn)")
		c.close()
		pingTicker.Stop()
	}()

	logger := c.Logger.WithField("method", "processMessages")

	// Yes, this loop is forever. If the connection is closed... the `shutdown`
	// channel will close this.
	for {
		select {
		case <-c.shutdown:
			return
		case operationMessage := <-c.incomingMessages:
			// Waits until receive a message to be sent.
			logger.Trace(TraceLevelInternalGQLMessages, "incoming message")

			if operationMessage == operationMessageEOF {
				// A EOF was sent and now it finalizes the processing of a the
				// conn. The defer should gracefully closes everything.
				return
			}

			c.processIncomeMessage(operationMessage)

		case operationMessage := <-c.outgoingMessages:
			// Waits until receive a message to be sent.
			err := c.processOutgoingMessage(operationMessage)
			if err != nil {
				c.Logger.Error("error sending a message:", err)
			}

		case <-pingTicker.C:
			// In case it takes too long to detect a message to be written, we should
			// send a PING to keep the connection open.
			c.conn.SetWriteDeadline(time.Now().Add(*c.config.WriteTimeout))
			err := c.conn.WriteMessage(websocket.PingMessage, emptyBytes)
			if err != nil {
				// If cannot write the WriteMessage, the connection
				// should be closed.
				c.Logger.Error("error sending the pingMessage:", err)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
		}
	}
}

func (c *Conn) processIncomeMessage(operationMessage *OperationMessage) {
	logger := c.Logger.WithField("messageDirection", "income")

	switch operationMessage.Type {
	case gqlTypeConnectionInit:
		logger.Trace(TraceLevelInternalGQLMessages, "gqlConnectionInit: ", string(operationMessage.Payload))

		connectionInit := GQLConnectionInit{
			Payload: operationMessage.Payload,
		}

		err := c.lockHandlers(func() error {
			// Broadcast the message to all handlers attached.
			for _, handler := range c.Handlers {
				// Of course, only `ConnectionInitHandlers` will be called.
				h, ok := handler.(ConnectionInitHandler)
				if !ok {
					continue
				}
				err := h.HandleConnectionInit(&connectionInit)
				if hErr, ok := err.(*HandlerError); ok {
					defaultPrevented := false
					if hErr.defaultPrevented {
						defaultPrevented = true
					}
					if hErr.propagationStopped {
						break
					}
					if defaultPrevented {
						logger.Error("error initialized but default prevented: ", hErr)
					} else {
						return hErr
					}
				} else if err != nil {
					logger.Error("error handling connection init: ", err)
					sendConnectionErr := c.sendConnectionError(err)
					if sendConnectionErr != nil {
						logger.Error("error sending a connection error: ", sendConnectionErr)
					}
					return err // Returning here have to be checked. It might call the close too early and let the client without the response.
				}
			}
			return nil
		})
		if err != nil {
			logger.Error("fatal error while initializing the connection: ", err)
			c.incomingMessages <- operationMessageEOF
			// If the initialization failed, we should cancel the connection
			return
		}

		// Now the handshake is done.
		c.setState(connStateEstablished)

		logger.Info("connection established")

		// Add message to be sent for the writePump
		c.outgoingMessages <- gqlConnectionAck
	case gqlTypeConnectionTerminate:
		var terminate GQLConnectionTerminate

		logger.Trace(TraceLevelInternalGQLMessages, "gqlConnectionTerminate")

		// No need to unmarshal a `GQLConnectionTerminate`. The protocol does not define anything.
		// So, why does it exists? Because future improvements might add something there. So it is
		// added to provide further extension witout making it incompatible.

		// Go through the handlers and call all `ConnectionTerminateHandler`s found.
		for _, handler := range c.Handlers {
			h, ok := handler.(ConnectionTerminateHandler)
			if !ok { // If not a `ConnectionStartHandler` try next.
				continue
			}
			err := h.HandleConnectionTerminate(&terminate)
			if hErr, ok := err.(*HandlerError); ok {
				// This event cannot be default prevented.
				if hErr.propagationStopped {
					break
				}
			} else if err != nil {
				logger.Error("error terminating the connection: ", err)
			}
		}

		c.incomingMessages <- operationMessageEOF

		return // Bye bye readPump
	case gqlTypeStart:
		if c.getState() != connStateEstablished {
			panic(ErrConnectionNotFullyEstablished)
		}

		logger.Trace(TraceLevelInternalGQLMessages, "gqlStart: ", string(operationMessage.Payload))

		start := GQLStart{
			ID: operationMessage.ID,
		}
		err := json.Unmarshal(operationMessage.Payload, &start.Payload)
		if err != nil {
			logger.Error("failed to unmarshal the payload at gqlStart: ", err)
			err = c.sendOperationErrors(start.ID, []error{err})
			if err != nil {
				logger.Error("failed to sendOperationErrors at gqlStart: ", err)
			}
			return
		}

		c.gqlStart(&start)
	case gqlTypeStop:
		logger.Trace(TraceLevelInternalGQLMessages, "gqlStop: ", string(operationMessage.Payload))

		if len(operationMessage.Payload) > 0 {
			var stop GQLStop
			err := json.Unmarshal(operationMessage.Payload, &stop)
			if err != nil {
				// TODO
				panic(err)
			}
			c.gqlStop(&stop)
			return
		}
		c.close()

	default:
		// TODO To call a default error handler or, maybe, a default message handler.
	}
}

func (c *Conn) processOutgoingMessage(operationMessage *OperationMessage) error {
	c.conn.SetWriteDeadline(time.Now().Add(*c.config.WriteTimeout))
	if operationMessage == operationMessageEOF {
		// !ok: The outgoingMessages channel was closed.
		// Or the message sent was a EOF and it means that the connection was closed.
		c.conn.WriteMessage(websocket.CloseMessage, []byte{})
		return nil
	}

	// Schedule a possible write timeout.
	// Actually writes the response to the websocket connection.
	return c.conn.WriteJSON(operationMessage)
}

// Close finishes the connection.
func (c *Conn) Close() error {
	if c.getState() == connStateClosed {
		return ErrConnectionClosed
	}
	err := c.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(*c.config.WriteTimeout))
	if err != nil {
		return err
	}
	return c.close()
}

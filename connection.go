package graphqlws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lab259/graphql"
	"github.com/lab259/graphql/language/parser"
	"github.com/lab259/rlog"
)

type ConnState string

const (
	connStateUndefined    ConnState = ""
	connStateInitializing ConnState = "initializing"
	connStateErrored      ConnState = "errored"
	connStateEstablished  ConnState = "established"
	connStateClosed       ConnState = "closed"
)

var (
	// operationMessageEOF is a message that when is detected on the writePump, closes the connection.
	operationMessageEOF = &OperationMessage{}
)

type Conn struct {
	logger           rlog.Logger
	state            ConnState
	handlers         []Handler
	conn             *websocket.Conn
	config           *Config
	outgoingMessages chan *OperationMessage
	schema           *graphql.Schema
}

// NewConn initializes a `Conn` instance.
func NewConn(conn *websocket.Conn, schema *graphql.Schema, config *Config) *Conn {
	c := &Conn{
		schema:           schema,
		config:           config,
		logger:           rlog.WithFields(nil), // TODO To add the connection ID here.
		conn:             conn,
		outgoingMessages: make(chan *OperationMessage, 10),
		handlers:         make([]Handler, 0, 3),
	}
	go c.readPump()
	go c.writePump()
	return c
}

// AddHandler adds a `Handler` to array.
func (c *Conn) AddHandler(handler Handler) {
	c.handlers = append(c.handlers, handler)
}

// SendError sends an error to the client.
func (c *Conn) SendError(err error) error {
	if c.state == connStateClosed {
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
	if c.state == connStateClosed {
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
	if c.state == connStateClosed {
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

func (c *Conn) close() {
	_ = c.conn.Close()
}

func (c *Conn) pongHandler(message string) error {
	// Set the deadline for the next read
	err := c.conn.SetReadDeadline(time.Now().Add(*c.config.PongWait))
	if err != nil {
		return err
	}

	// Go through the handlers and call all `WebsocketPongHandler`s found.
	for _, handler := range c.handlers {
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

// closeHandler
func (c *Conn) closeHandler(code int, text string) error {
	defaultPrevented := false
	// Go through the handlers and call all `WebsocketCloseHandler`s found.
	for _, handler := range c.handlers {
		h, ok := handler.(WebsocketCloseHandler)
		if !ok { // If not a `ConnectionStartHandler` try next.
			continue
		}
		err := h.HandleWebsocketClose(code, text)
		hErr, ok := err.(*HandlerError)
		if ok {
			if hErr.defaultPrevented {
				defaultPrevented = true
			}
			if hErr.propagationStopped {
				break
			}
		} else if err != nil {
			return err
		}
	}

	if defaultPrevented {
		return nil
	}
	return c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(*c.config.WriteTimeout))
}

func (c *Conn) recover(t RWType) {
	if r := recover(); r != nil {
		defaultPrevented := false

		// Broadcast the message to all handlers attached.
		for _, handler := range c.handlers {
			// Of course, only `SystemRecoverHandler` will be called.
			h, ok := handler.(SystemRecoverHandler)
			if !ok {
				continue
			}
			err := h.HandlePanic(t, r)
			if hErr, ok := err.(*HandlerError); ok {
				if hErr.defaultPrevented {
					defaultPrevented = true
				}
				if hErr.propagationStopped {
					break
				}
			} else if err != nil {
				// TODO
				return
			}
		}

		if defaultPrevented {
			return
		}

		// TODO
	}
}

func (c *Conn) gqlStart(start *GQLStart) {
	var subscription SubscriptionInterface = &Subscription{
		ID:            start.ID,
		Query:         start.Payload.Query,
		Variables:     start.Payload.Variables,
		OperationName: start.Payload.OperationName,
		Connection:    c,
	}

	logger := c.logger.WithFields(rlog.Fields{
		"subscription": subscription.GetID(),
	})

	if errors := ValidateSubscription(subscription); len(errors) > 0 {
		logger.WithField("errors", errors).Warn("Failed to add invalid subscription")
		return // errors
	}

	// Parses the subscription query
	document, err := parser.Parse(parser.ParseParams{
		Source: subscription.GetQuery(),
	})
	if err != nil {
		logger.WithField("err", err).Warn("Failed to parse subscription query")
		return // []error{err}
	}

	// Validate the query document
	validation := graphql.ValidateDocument(c.schema, document, nil)
	if !validation.IsValid {
		logger.WithFields(rlog.Fields{
			"errors": validation.Errors,
		}).Error("Failed to validate subscription query")
		return // ErrorsFromGraphQLErrors(validation.Errors)
	}

	// Remember the query document for later
	subscription.SetDocument(document)

	// Extract query names from the document (typically, there should only be one)
	subscription.SetFields(SubscriptionFieldNamesFromDocument(document))

	// TODO To uncomment this
	// return config.SubscriptionManager.AddSubscription(conn, subscription)
}

// readPumpIteration runs one read iteration.
func (c *Conn) readPumpIteration() {
	// TODO Add crash protection
	var operationMessage OperationMessage
	err := c.conn.ReadJSON(&operationMessage)
	if err != nil {
		// TODO
		panic(err)
	}

	switch operationMessage.Type {
	case gqlTypeConnectionInit:
		var connectionInit GQLConnectionInit
		// Unmarshals the income payload into a `GQLConnectionInit`
		err = json.Unmarshal(operationMessage.Payload, &connectionInit)
		if err != nil {
			// TODO
			panic(err)
		}

		defaultPrevented := false
		// Broadcast the message to all handlers attached.
		for _, handler := range c.handlers {
			// Of course, only `ConnectionInitHandlers` will be called.
			h, ok := handler.(ConnectionInitHandler)
			if !ok {
				continue
			}
			err = h.HandleConnectionInit(&connectionInit)
			if hErr, ok := err.(*HandlerError); ok {
				if hErr.defaultPrevented {
					defaultPrevented = true
				}
				if hErr.propagationStopped {
					break
				}
			} else if err != nil {
				err = c.sendConnectionError(err)
				if err != nil {
					c.logger.Error("error sending a connection error: ", err)
				}
				return // Returning here have to be checked. It might call the close too early and let the client witout the response.
			}
		}

		if defaultPrevented {
			return
		}

		// Add message to be sent for the writePump
		c.outgoingMessages <- gqlConnectionAck
	case gqlTypeConnectionTerminate:
		var terminate GQLConnectionTerminate

		// No need to unmarshal a `GQLConnectionTerminate`. The protocol does not define anything.
		// So, why does it exists? Because future improvements might add something there. So it is
		// added to provide further extension witout making it incompatible.

		// Go through the handlers and call all `ConnectionTerminateHandler`s found.
		for _, handler := range c.handlers {
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
				// TODO Add something to report this error.
			}
		}

		return // Bye bye readPump
	case gqlTypeStart:
		var start GQLStart
		err = json.Unmarshal(operationMessage.Payload, &start)
		if err != nil {
			c.logger.Error("failed to unmarshal the payload at gqlStart: ", err)
			err = c.sendOperationErrors(start.ID, []error{err})
			if err != nil {
				c.logger.Error("failed to sendOperationErrors at gqlStart: ", err)
			}
			return
		}

		errs := make([]error, 0)
		// Go through the handlers and call all `ConnectionStartHandler`s found.
		for _, handler := range c.handlers {
			h, ok := handler.(ConnectionStartHandler)
			if !ok { // If not a `ConnectionStartHandler` try next.
				return
			}
			errsIn := h.HandleConnectionStart(&start)
			if len(errs) > 0 { // Keep aggregating errors
				errs = append(errs, errsIn...)
			}
		}

		// If any error happen ...
		if len(errs) > 0 {
			c.logger.Error("failed to HandleConnectionStart at gqlStart: ", errs)
			// ... send it to the client.
			err = c.sendOperationErrors(start.ID, errs)
			if err != nil {
				c.logger.Error("failed to sendOperationErrors when HandleConnectionStart errors at gqlStart: ", err)
			}
			return
		}

		c.gqlStart(&start)
	case gqlTypeStop:
		var stop GQLStop
		err = json.Unmarshal(operationMessage.Payload, &stop)
		if err != nil {
			// TODO
			panic(err)
		}

		// Go through the handlers and call all `ConnectionStopHandler`s found.
		for _, handler := range c.handlers {
			h, ok := handler.(ConnectionStopHandler)
			if !ok { // If not a `ConnectionStartHandler` try next.
				continue
			}
			err := h.HandleConnectionStop(&stop)
			if err != nil {
				// TODO Call the default erro handler.
			}
		}
	default:
		// TODO To call a default error handler or, maybe, a default mesasge handler.
	}
}

func (c *Conn) readPump() {
	defer c.close()

	// Prepare for the first pong.
	c.conn.SetReadLimit(*c.config.ReadLimit)
	c.conn.SetPongHandler(c.pongHandler)
	c.conn.SetCloseHandler(c.closeHandler)

	for c.state != connStateClosed {
		c.conn.SetReadDeadline(time.Now().Add(*c.config.PongWait))
		c.readPumpIteration()
	}
}

func (c *Conn) writePumpIteration() {
	// TODO Add crash protection
	select {
	case operationMessage, ok := <-c.outgoingMessages:
		if !ok {
			return
		}
		if operationMessage == operationMessageEOF {
			return
		}
		err := c.conn.SetWriteDeadline(time.Now().Add(*c.config.WriteTimeout))
		if err != nil {
			panic(err)
		}
		c.conn.WriteJSON(operationMessage)
	case <-time.After((*c.config.PongWait * 9) / 10):
		err := c.conn.SetWriteDeadline(time.Now().Add(*c.config.WriteTimeout))
		if err != nil {
			panic(err)
		}
		err = c.conn.WriteMessage(websocket.PingMessage, nil)
		if err != nil {
			// TODO
		}
	}
}

func (c *Conn) writePump() {
	defer c.close()

	for c.state != connStateClosed {
		c.writePumpIteration()
	}
}

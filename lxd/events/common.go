package events

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/logger"
)

// EventHandler called when the connection receives an event from the client.
type EventHandler func(event api.Event)

// serverCommon represents an instance of a comon event server.
type serverCommon struct {
	debug   bool
	verbose bool
	lock    sync.Mutex
}

// listenerCommon describes a common event listener.
type listenerCommon struct {
	*websocket.Conn

	messageTypes []string
	ctx          context.Context
	ctxCancel    func()
	id           string
	lock         sync.Mutex
	pongsPending uint
	recvFunc     EventHandler
}

func (e *listenerCommon) heartbeat() {
	logger.Debug("Event listener server handler started", logger.Ctx{"listener": e.ID(), "local": e.Conn.LocalAddr(), "remote": e.Conn.RemoteAddr()})

	defer e.Close()

	pingInterval := time.Second * 10
	e.pongsPending = 0

	e.SetPongHandler(func(msg string) error {
		e.lock.Lock()
		e.pongsPending = 0
		e.lock.Unlock()
		return nil
	})

	// Start reader from client.
	go func() {
		defer e.Close()

		if e.recvFunc != nil {
			for {
				var event api.Event
				err := e.Conn.ReadJSON(&event)
				if err != nil {
					return // This detects if client has disconnected or sent invalid data.
				}

				// Pass received event to the handler.
				e.recvFunc(event)
			}
		} else {
			// Run a blocking reader to detect if the client has disconnected. We don't expect to get
			// anything from the remote side, so this should remain blocked until disconnected.
			_, _, _ = e.Conn.NextReader()
		}
	}()

	for {
		if e.IsClosed() {
			return
		}

		e.lock.Lock()
		if e.pongsPending > 2 {
			e.lock.Unlock()
			logger.Warn("Hearbeat for event listener handler timed out", logger.Ctx{"listener": e.ID(), "local": e.Conn.LocalAddr(), "remote": e.Conn.RemoteAddr()})
			return
		}
		err := e.WriteControl(websocket.PingMessage, []byte("keepalive"), time.Now().Add(5*time.Second))
		if err != nil {
			e.lock.Unlock()
			return
		}

		e.pongsPending++
		e.lock.Unlock()

		select {
		case <-time.After(pingInterval):
		case <-e.ctx.Done():
			return
		}
	}
}

// IsClosed returns true if the listener is closed.
func (e *listenerCommon) IsClosed() bool {
	return e.ctx.Err() != nil
}

// ID returns the listener ID.
func (e *listenerCommon) ID() string {
	return e.id
}

// Wait waits for a message on its active channel or the context is cancelled, then returns.
func (e *listenerCommon) Wait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-e.ctx.Done():
	}
}

// Close Disconnects the listener.
func (e *listenerCommon) Close() {
	e.lock.Lock()
	defer e.lock.Unlock()

	if e.IsClosed() {
		return
	}

	logger.Debug("Event listener server handler stopped", logger.Ctx{"listener": e.ID(), "local": e.Conn.LocalAddr(), "remote": e.Conn.RemoteAddr()})

	err := e.Conn.Close()
	if err != nil {
		logger.Error("Failed closing listener connection", logger.Ctx{"listener": e.ID(), "err": err})
	}
	e.ctxCancel()
}

// WriteJSON message to the connection.
func (e *listenerCommon) WriteJSON(v any) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.Conn.WriteJSON(v)
}


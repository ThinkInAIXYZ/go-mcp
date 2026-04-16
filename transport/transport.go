package transport

import (
	"context"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

/*
* Transport is an abstraction of the underlying transport layer.
* GO-MCP needs to be able to transmit JSON-RPC messages between server and client.
 */

// Message defines the basic message interface
type Message []byte

func (msg Message) String() string {
	return pkg.B2S(msg)
}

type ClientTransport interface {
	// Start initiates the transport connection
	Start() error

	// Send transmits a message
	Send(ctx context.Context, msg Message) error

	// SetReceiver sets the handler for messages from the peer
	SetReceiver(receiver clientReceiver)

	// Close terminates the transport connection
	Close() error
}

type clientReceiver interface {
	Receive(ctx context.Context, msg []byte) error
	Interrupt(err error)
}

type ClientReceiver struct {
	receive   func(ctx context.Context, msg []byte) error
	interrupt func(err error)
}

func (r *ClientReceiver) Receive(ctx context.Context, msg []byte) error {
	return r.receive(ctx, msg)
}

func (r *ClientReceiver) Interrupt(err error) {
	r.interrupt(err)
}

func NewClientReceiver(receive func(ctx context.Context, msg []byte) error, interrupt func(err error)) clientReceiver {
	r := &ClientReceiver{
		receive:   receive,
		interrupt: interrupt,
	}
	return r
}

type ServerTransport interface {
	// Run starts listening for requests, this is synchronous, and cannot return before Shutdown is called
	Run() error

	// Send transmits a message
	Send(ctx context.Context, sessionID string, msg Message) error

	// SetReceiver sets the handler for messages from the peer
	SetReceiver(serverReceiver)

	SetSessionManager(manager sessionManager)

	// Shutdown gracefully closes, the internal implementation needs to stop receiving messages first,
	// then wait for serverCtx to be canceled, while using userCtx to control timeout.
	// userCtx is used to control the timeout of the server shutdown.
	// serverCtx is used to coordinate the internal cleanup sequence:
	// 1. turn off message listen
	// 2. Wait for serverCtx to be done (indicating server shutdown is complete)
	// 3. Cancel the transport's context to stop all ongoing operations
	// 4. Wait for all in-flight sends to complete
	// 5. Close all session
	Shutdown(userCtx context.Context, serverCtx context.Context) error
}

type serverReceiver interface {
	Receive(ctx context.Context, sessionID string, msg []byte) (<-chan []byte, error)
}

type ServerReceiverF func(ctx context.Context, sessionID string, msg []byte) (<-chan []byte, error)

func (f ServerReceiverF) Receive(ctx context.Context, sessionID string, msg []byte) (<-chan []byte, error) {
	return f(ctx, sessionID, msg)
}

type sessionManager interface {
	CreateSession(context.Context) string
	OpenMessageQueueForSend(sessionID string) error
	EnqueueMessageForSend(ctx context.Context, sessionID string, message []byte) error
	DequeueMessageForSend(ctx context.Context, sessionID string) ([]byte, error)
	CloseSession(sessionID string)
	CloseAllSessions()
}

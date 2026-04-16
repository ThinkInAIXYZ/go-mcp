package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

var ErrQueueNotOpened = errors.New("queue has not been opened")

type State struct {
	lastActiveAt time.Time

	mu       sync.RWMutex
	sendChan chan []byte

	requestID int64

	serverReqID2respChan cmap.ConcurrentMap[string, chan *protocol.JSONRPCResponse]

	clientReqID2cancelFunc cmap.ConcurrentMap[string, context.CancelFunc]

	// cache client initialize request info
	clientInfo         *protocol.Implementation
	clientCapabilities *protocol.ClientCapabilities

	// subscribed resources
	subscribedResources cmap.ConcurrentMap[string, struct{}]

	receivedInitRequest *pkg.AtomicBool
	ready               *pkg.AtomicBool
	closed              *pkg.AtomicBool
}

func NewState() *State {
	return &State{
		lastActiveAt:           time.Now(),
		serverReqID2respChan:   cmap.New[chan *protocol.JSONRPCResponse](),
		clientReqID2cancelFunc: cmap.New[context.CancelFunc](),
		subscribedResources:    cmap.New[struct{}](),
		receivedInitRequest:    pkg.NewAtomicBool(),
		ready:                  pkg.NewAtomicBool(),
		closed:                 pkg.NewAtomicBool(),
	}
}

func (s *State) SetClientInfo(ClientInfo *protocol.Implementation, ClientCapabilities *protocol.ClientCapabilities) {
	s.clientInfo = ClientInfo
	s.clientCapabilities = ClientCapabilities
}

func (s *State) GetClientCapabilities() *protocol.ClientCapabilities {
	return s.clientCapabilities
}

func (s *State) SetReceivedInitRequest() {
	s.receivedInitRequest.Store(true)
}

func (s *State) GetReceivedInitRequest() bool {
	return s.receivedInitRequest.Load()
}

func (s *State) SetReady() {
	s.ready.Store(true)
}

func (s *State) GetReady() bool {
	return s.ready.Load()
}

func (s *State) IncRequestID() int64 {
	return atomic.AddInt64(&s.requestID, 1)
}

func (s *State) GetServerReqID2respChan() cmap.ConcurrentMap[string, chan *protocol.JSONRPCResponse] {
	return s.serverReqID2respChan
}

func (s *State) GetClientReqID2cancelFunc() cmap.ConcurrentMap[string, context.CancelFunc] {
	return s.clientReqID2cancelFunc
}

func (s *State) GetSubscribedResources() cmap.ConcurrentMap[string, struct{}] {
	return s.subscribedResources
}

func (s *State) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed.Store(true)

	if s.sendChan != nil {
		close(s.sendChan)
	}
}

func (s *State) updateLastActiveAt() {
	s.lastActiveAt = time.Now()
}

func (s *State) openMessageQueueForSend() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sendChan == nil {
		s.sendChan = make(chan []byte, 64)
	}
}

func (s *State) enqueueMessage(ctx context.Context, message []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed.Load() {
		return errors.New("session already closed")
	}

	if s.sendChan == nil {
		return ErrQueueNotOpened
	}

	select {
	case s.sendChan <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *State) dequeueMessage(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	if s.sendChan == nil {
		s.mu.RUnlock()
		return nil, ErrQueueNotOpened
	}
	s.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-s.sendChan:
		if !ok {
			return nil, pkg.ErrSendEOF
		}
		return msg, nil
	}
}

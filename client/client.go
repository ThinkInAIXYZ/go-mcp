package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type Option func(*Client)

func WithNotifyHandler(handler NotifyHandler) Option {
	return func(s *Client) {
		s.notifyHandler = handler
	}
}

func WithSamplingHandler(handler SamplingHandler) Option {
	return func(s *Client) {
		s.samplingHandler = handler
	}
}

func WithClientInfo(info protocol.Implementation) Option {
	return func(s *Client) {
		s.clientInfo = &info
	}
}

func WithInitTimeout(timeout time.Duration) Option {
	return func(s *Client) {
		s.initTimeout = timeout
	}
}

func WithLogger(logger pkg.Logger) Option {
	return func(s *Client) {
		s.logger = logger
	}
}

type Client struct {
	transport transport.ClientTransport

	reqID2respChan cmap.ConcurrentMap[string, chan *protocol.JSONRPCResponse]

	progressChanRW           sync.RWMutex
	progressToken2notifyChan map[string]chan<- *protocol.ProgressNotification

	samplingHandler SamplingHandler

	notifyHandler NotifyHandler

	requestID int64

	ready            *pkg.AtomicBool
	initializationMu sync.Mutex

	clientInfo         *protocol.Implementation
	clientCapabilities *protocol.ClientCapabilities

	serverCapabilities *protocol.ServerCapabilities
	serverInfo         *protocol.Implementation
	serverInstructions string

	initTimeout time.Duration

	closed chan struct{}

	logger pkg.Logger
}

func NewClient(t transport.ClientTransport, opts ...Option) (*Client, error) {
	client := &Client{
		transport:                t,
		reqID2respChan:           cmap.New[chan *protocol.JSONRPCResponse](),
		progressToken2notifyChan: make(map[string]chan<- *protocol.ProgressNotification),
		ready:                    pkg.NewAtomicBool(),
		clientInfo:               &protocol.Implementation{},
		clientCapabilities:       &protocol.ClientCapabilities{},
		initTimeout:              time.Second * 30,
		closed:                   make(chan struct{}),
		logger:                   pkg.DefaultLogger,
	}
	t.SetReceiver(transport.ClientReceiverF(client.receive))

	for _, opt := range opts {
		opt(client)
	}

	if client.notifyHandler == nil {
		h := NewBaseNotifyHandler()
		h.Logger = client.logger
		client.notifyHandler = h
	}

	if client.samplingHandler != nil {
		client.clientCapabilities.Sampling = struct{}{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.initTimeout)
	defer cancel()

	if err := client.transport.Start(); err != nil {
		return nil, fmt.Errorf("init mcp client transpor start fail: %w", err)
	}

	if _, err := client.initialization(ctx, protocol.NewInitializeRequest(*client.clientInfo, *client.clientCapabilities)); err != nil {
		return nil, err
	}

	go func() {
		defer pkg.Recover()

		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-client.closed:
				return
			case <-ticker.C:
				client.sessionDetection()
			}
		}
	}()

	return client, nil
}

func (client *Client) GetServerCapabilities() protocol.ServerCapabilities {
	return *client.serverCapabilities
}

func (client *Client) GetServerInfo() protocol.Implementation {
	return *client.serverInfo
}

func (client *Client) GetServerInstructions() string {
	return client.serverInstructions
}

func (client *Client) Close() error {
	close(client.closed)

	return client.transport.Close()
}

func (client *Client) sessionDetection() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx, protocol.NewPingRequest()); err != nil {
		client.logger.Warnf("mcp client ping server fail: %v", err)
	}
}

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

func WithClientInfo(info *protocol.Implementation) Option {
	return func(s *Client) {
		s.clientInfo = info
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

func WithAuthAndRefresher(
	accessToken, refreshToken string,
	expiry time.Time,
	refresher func(string) (string, string, time.Time, error),
) Option {
	return func(c *Client) {
		c.accessToken = accessToken
		c.refreshToken = refreshToken
		c.tokenExpiry = expiry
		c.tokenRefresher = refresher
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

	accessToken    string
	refreshToken   string
	tokenExpiry    time.Time
	tokenMutex     sync.RWMutex
	tokenRefresher func(refreshToken string) (accessToken, newRefreshToken string, expiry time.Time, err error)
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
	t.SetReceiver(transport.NewClientReceiver(client.receive, client.receiveInterrupt))

	for _, opt := range opts {
		opt(client)
	}

	if tp, ok := t.(interface{ SetTokenProvider(func() string) }); ok {
		tp.SetTokenProvider(func() string {
			token, _ := client.GetAccessToken()
			return token
		})
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

	if _, err := client.initialization(ctx, protocol.NewInitializeRequest(client.clientInfo, client.clientCapabilities)); err != nil {
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

func (c *Client) GetAccessToken() (string, bool) {
	c.tokenMutex.RLock()

	if time.Now().Before(c.tokenExpiry) {
		token := c.accessToken
		c.tokenMutex.RUnlock()
		return token, true
	}

	c.tokenMutex.RUnlock()

	if c.tokenRefresher == nil {
		return "", false
	}

	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	if time.Now().Before(c.tokenExpiry) {
		return c.accessToken, true
	}

	newAccessToken, newRefreshToken, newExpiry, err := c.tokenRefresher(c.refreshToken)
	if err != nil {
		c.logger.Errorf("Failed to refresh token: %v", err)
		return "", false
	}

	c.accessToken = newAccessToken
	c.refreshToken = newRefreshToken
	c.tokenExpiry = newExpiry

	return newAccessToken, true
}

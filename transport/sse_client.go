package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type SSEClientTransportOption func(*sseClientTransport)

func WithSSEClientOptionReceiveTimeout(timeout time.Duration) SSEClientTransportOption {
	return func(t *sseClientTransport) {
		t.receiveTimeout = timeout
	}
}

func WithSSEClientOptionHTTPClient(client *http.Client) SSEClientTransportOption {
	return func(t *sseClientTransport) {
		t.client = client
	}
}

func WithSSEClientOptionLogger(log pkg.Logger) SSEClientTransportOption {
	return func(t *sseClientTransport) {
		t.logger = log
	}
}

func WithRetryFunc(retry func(func() error)) SSEClientTransportOption {
	return func(t *sseClientTransport) {
		t.retry = retry
	}
}

type sseClientTransport struct {
	ctx    context.Context
	cancel context.CancelFunc

	serverURL *url.URL

	endpointChan    chan struct{}
	messageEndpoint *url.URL
	receiver        clientReceiver

	// options
	logger         pkg.Logger
	receiveTimeout time.Duration
	client         *http.Client

	retry func(func() error)

	sseConnectClose chan struct{}
}

func NewSSEClientTransport(serverURL string, opts ...SSEClientTransportOption) (ClientTransport, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %w", err)
	}

	t := &sseClientTransport{
		serverURL:       parsedURL,
		endpointChan:    make(chan struct{}, 1),
		messageEndpoint: nil,
		receiver:        nil,
		logger:          pkg.DefaultLogger,
		receiveTimeout:  time.Second * 30,
		client:          http.DefaultClient,
		sseConnectClose: make(chan struct{}),
		retry: func(operation func() error) {
			for {
				if e := operation(); e == nil {
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		},
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

func (t *sseClientTransport) Start() (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	t.ctx = ctx
	t.cancel = cancel

	defer func() {
		if err != nil {
			t.cancel()
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		defer pkg.Recover()
		defer close(t.sseConnectClose)

		t.retry(func() error {
			if e := t.startSSE(); e != nil {
				if errors.Is(e, context.Canceled) {
					return nil
				}
				t.logger.Errorf("startSSE: %+v", e)
				t.receiver.Interrupt(fmt.Errorf("SSE connection disconnection: %w", e))
				return e
			}
			return nil
		})
	}()

	// Wait for the endpoint to be received
	select {
	case <-t.endpointChan:
	// Endpoint received, proceed
	case err = <-errChan:
		return fmt.Errorf("error in SSE stream: %w", err)
	case <-time.After(10 * time.Second): // Add a timeout
		return fmt.Errorf("timeout waiting for endpoint")
	}

	return nil
}

func (t *sseClientTransport) startSSE() error {
	req, err := http.NewRequestWithContext(t.ctx, http.MethodGet, t.serverURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := t.client.Do(req) //nolint:bodyclose
	if err != nil {
		return fmt.Errorf("failed to connect to SSE stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, status: %s", resp.StatusCode, resp.Status)
	}

	return t.readSSE(resp.Body)
}

// readSSE continuously reads the SSE stream and processes events.
// It runs until the connection is closed or an error occurs.
func (t *sseClientTransport) readSSE(reader io.ReadCloser) error {
	defer func() {
		_ = reader.Close()
	}()

	br := bufio.NewReader(reader)
	var event, data string

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Process any pending event before exit
				if event != "" && data != "" {
					t.handleSSEEvent(event, data)
				}
			}
			select {
			case <-t.ctx.Done():
				return t.ctx.Err()
			default:
				return fmt.Errorf("SSE stream error: %w", err)
			}
		}

		// Remove only newline markers
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// Empty line means end of event
			if event != "" && data != "" {
				t.handleSSEEvent(event, data)
				event = ""
				data = ""
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

// handleSSEEvent processes SSE events based on their type.
// Handles 'endpoint' events for connection setup and 'message' events for JSON-RPC communication.
func (t *sseClientTransport) handleSSEEvent(event, data string) {
	switch event {
	case "endpoint":
		endpoint, err := t.serverURL.Parse(data)
		if err != nil {
			t.logger.Errorf("Error parsing endpoint URL: %v", err)
			return
		}
		t.logger.Debugf("Received endpoint: %s", endpoint.String())
		t.messageEndpoint = endpoint
		select {
		case t.endpointChan <- struct{}{}:
		default:
		}
	case "message":
		ctx, cancel := context.WithTimeout(t.ctx, t.receiveTimeout)
		defer cancel()
		if err := t.receiver.Receive(ctx, []byte(data)); err != nil {
			t.logger.Errorf("Error receive message: %v", err)
			return
		}
	}
}

func (t *sseClientTransport) Send(ctx context.Context, msg Message) error {
	t.logger.Debugf("Sending message: %s to %s", msg, t.messageEndpoint.String())

	var (
		err  error
		req  *http.Request
		resp *http.Response
	)

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, t.messageEndpoint.String(), bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if resp, err = t.client.Do(req); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status code: %d, status: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (t *sseClientTransport) SetReceiver(receiver clientReceiver) {
	t.receiver = receiver
}

func (t *sseClientTransport) Close() error {
	t.cancel()

	<-t.sseConnectClose

	return nil
}

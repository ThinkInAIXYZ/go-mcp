package transport

import (
	"bufio"
	"bytes"
	"context"
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

type sseClientTransport struct {
	ctx    context.Context
	cancel context.CancelFunc

	serverURL *url.URL

	endpointChan    chan struct{}
	messageEndpoint *url.URL
	receiver        ClientReceiver

	// options
	logger         pkg.Logger
	receiveTimeout time.Duration
	client         *http.Client

	sseConnectClose chan struct{}
}

func NewSSEClientTransport(serverURL string, opts ...SSEClientTransportOption) (ClientTransport, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	x := &sseClientTransport{
		ctx:             ctx,
		cancel:          cancel,
		serverURL:       parsedURL,
		endpointChan:    make(chan struct{}, 1),
		messageEndpoint: nil,
		receiver:        nil,
		logger:          pkg.DefaultLogger,
		receiveTimeout:  time.Second * 30,
		client:          http.DefaultClient,
		sseConnectClose: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(x)
	}

	return x, nil
}

func (t *sseClientTransport) Start() error {
	errChan := make(chan error, 1)
	go func() {
		defer pkg.Recover()

		req, err := http.NewRequest(http.MethodGet, t.serverURL.String(), nil)
		if err != nil {
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		resp, err := t.client.Do(req) //nolint:bodyclose
		if err != nil {
			errChan <- fmt.Errorf("failed to connect to SSE stream: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errChan <- fmt.Errorf("unexpected status code: %d, status: %s", resp.StatusCode, resp.Status)
			return
		}

		go func() {
			defer pkg.Recover()

			<-t.ctx.Done()

			if err := resp.Body.Close(); err != nil {
				t.logger.Errorf("failed to close SSE stream body: %w", err)
				return
			}
		}()

		t.readSSE(resp.Body)

		close(t.sseConnectClose)
	}()

	// Wait for the endpoint to be received
	select {
	case <-t.endpointChan:
	// Endpoint received, proceed
	case err := <-errChan:
		return fmt.Errorf("error in SSE stream: %w", err)
	case <-time.After(10 * time.Second): // Add a timeout
		return fmt.Errorf("timeout waiting for endpoint")
	}

	return nil
}

// readSSE continuously reads the SSE stream and processes events.
// It runs until the connection is closed or an error occurs.
func (t *sseClientTransport) readSSE(reader io.ReadCloser) {
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
				break
			}
			select {
			case <-t.ctx.Done():
				return
			default:
				t.logger.Errorf("SSE stream error: %v", err)
				return
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
		close(t.endpointChan)
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

func (t *sseClientTransport) SetReceiver(receiver ClientReceiver) {
	t.receiver = receiver
}

func (t *sseClientTransport) Close() error {
	t.cancel()

	<-t.sseConnectClose

	return nil
}

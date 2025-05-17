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
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

const sessionIDHeader = "Mcp-Session-Id"

// const eventIDHeader = "Last-Event-ID"

type StreamableHTTPClientTransportOption func(*streamableHTTPClientTransport)

func WithStreamableHTTPClientOptionReceiveTimeout(timeout time.Duration) StreamableHTTPClientTransportOption {
	return func(t *streamableHTTPClientTransport) {
		t.receiveTimeout = timeout
	}
}

func WithStreamableHTTPClientOptionHTTPClient(client *http.Client) StreamableHTTPClientTransportOption {
	return func(t *streamableHTTPClientTransport) {
		t.client = client
	}
}

func WithStreamableHTTPClientOptionLogger(log pkg.Logger) StreamableHTTPClientTransportOption {
	return func(t *streamableHTTPClientTransport) {
		t.logger = log
	}
}

type streamableHTTPClientTransport struct {
	ctx    context.Context
	cancel context.CancelFunc

	serverURL *url.URL
	receiver  clientReceiver
	sessionID *pkg.AtomicString

	// options
	logger         pkg.Logger
	receiveTimeout time.Duration
	client         *http.Client

	sseInFlyConnect sync.WaitGroup
}

func NewStreamableHTTPClientTransport(serverURL string, opts ...StreamableHTTPClientTransportOption) (ClientTransport, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	t := &streamableHTTPClientTransport{
		ctx:            ctx,
		cancel:         cancel,
		serverURL:      parsedURL,
		sessionID:      pkg.NewAtomicString(),
		logger:         pkg.DefaultLogger,
		receiveTimeout: time.Second * 30,
		client:         http.DefaultClient,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

func (t *streamableHTTPClientTransport) Start() error {
	// Start a GET stream for server-initiated messages
	t.sseInFlyConnect.Add(1)
	go func() {
		defer pkg.Recover()
		defer t.sseInFlyConnect.Done()

		t.startSSEStream()
	}()
	return nil
}

func (t *streamableHTTPClientTransport) Send(ctx context.Context, msg Message) error {
	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, t.serverURL.String(), bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if sessionID := t.sessionID.Load(); sessionID != "" {
		req.Header.Set(sessionIDHeader, sessionID)
	}

	resp, err := t.client.Do(req) //nolint:bodyclose
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		defer resp.Body.Close()
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if req.Header.Get(sessionIDHeader) != "" && resp.StatusCode == http.StatusNotFound {
			return pkg.ErrSessionClosed
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf("unexpected status code: %d, status: %s, body=%s", resp.StatusCode, resp.Status, body)
	}

	if resp.StatusCode == http.StatusAccepted {
		return nil // Handle immediate JSON response
	}

	// Handle session ID if provided in response
	if respSessionID := resp.Header.Get(sessionIDHeader); respSessionID != "" {
		t.sessionID.Store(respSessionID)
	}

	contentType := resp.Header.Get("Content-Type")
	// Handle different response types
	switch {
	case contentType == "text/event-stream":
		go func() {
			defer pkg.Recover()

			t.sseInFlyConnect.Add(1)
			defer t.sseInFlyConnect.Done()

			t.handleSSEStream(resp.Body)
		}()
		return nil
	case strings.HasPrefix(contentType, "application/json"):
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		if err = t.receiver.Receive(ctx, body); err != nil {
			return fmt.Errorf("failed to process response: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unexpected content type: %s", contentType)
	}
}

func (t *streamableHTTPClientTransport) startSSEStream() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			sessionID := t.sessionID.Load()
			if sessionID == "" {
				continue // Try again after 1 second, waiting for the POST request to initialize the SessionID to complete
			}

			req, err := http.NewRequestWithContext(t.ctx, http.MethodGet, t.serverURL.String(), nil)
			if err != nil {
				t.logger.Errorf("failed to create SSE request: %v", err)
				return
			}

			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set(sessionIDHeader, sessionID)

			resp, err := t.client.Do(req)
			if err != nil {
				select {
				case <-t.ctx.Done():
					return
				default:
				}
				t.logger.Errorf("failed to connect to SSE stream: %v", err)
				continue
			}

			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusMethodNotAllowed:
					t.logger.Infof("server does not support SSE streaming")
					return
				case http.StatusNotFound:
					t.logger.Infof("%+v", pkg.ErrSessionClosed)
					continue // Try again after 1 second, waiting for the POST request again to initialize the SessionID to complete
				default:
					t.logger.Infof("unexpected status code: %d, status: %s", resp.StatusCode, resp.Status)
					return
				}
			}

			t.handleSSEStream(resp.Body)
		}
	}
}

func (t *streamableHTTPClientTransport) handleSSEStream(reader io.ReadCloser) {
	defer reader.Close()

	br := bufio.NewReader(reader)
	var data string

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Process any pending event before exit
				if data != "" {
					t.processSSEEvent(data)
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

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// Empty line means end of event
			if data != "" {
				t.processSSEEvent(data)
				_, data = "", ""
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

func (t *streamableHTTPClientTransport) processSSEEvent(data string) {
	ctx, cancel := context.WithTimeout(t.ctx, t.receiveTimeout)
	defer cancel()

	if err := t.receiver.Receive(ctx, []byte(data)); err != nil {
		t.logger.Errorf("Error processing SSE event: %v", err)
	}
}

func (t *streamableHTTPClientTransport) SetReceiver(receiver clientReceiver) {
	t.receiver = receiver
}

func (t *streamableHTTPClientTransport) Close() error {
	t.cancel()

	t.sseInFlyConnect.Wait()

	if sessionID := t.sessionID.Load(); sessionID != "" {
		req, err := http.NewRequest(http.MethodDelete, t.serverURL.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set(sessionIDHeader, sessionID)
		resp, err := t.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
		defer resp.Body.Close()
	}

	return nil
}

package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"

	"github.com/stretchr/testify/assert"
)

func TestSSE(t *testing.T) {
	var (
		err    error
		svr    ServerTransport
		client ClientTransport
	)

	// Get an available port
	port, err := getAvailablePort()
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}

	serverAddr := fmt.Sprintf("127.0.0.1:%d", port)
	clientURL := fmt.Sprintf("http://%s/sse", serverAddr)

	if svr, err = NewSSEServerTransport(serverAddr); err != nil {
		t.Fatalf("NewSSEServerTransport failed: %v", err)
	}

	if client, err = NewSSEClientTransport(clientURL); err != nil {
		t.Fatalf("NewSSEClientTransport failed: %v", err)
	}

	testTransport(t, client, svr)
}

func TestSSEHandler(t *testing.T) {
	var (
		messageURL = "/message"
		port       int

		err    error
		svr    ServerTransport
		client ClientTransport
	)

	// Get an available port
	port, err = getAvailablePort()
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}

	serverAddr := fmt.Sprintf("http://127.0.0.1:%d", port)
	serverURL := fmt.Sprintf("%s/sse", serverAddr)

	svr, handler, err := NewSSEServerTransportAndHandler(fmt.Sprintf("%s%s", serverAddr, messageURL))
	if err != nil {
		t.Fatalf("NewSSEServerTransport failed: %v", err)
	}

	// Set up HTTP routes
	http.Handle("/sse", handler.HandleSSE())
	http.Handle(messageURL, handler.HandleMessage())

	errCh := make(chan error, 1)
	go func() {
		if err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Use select to handle potential errors
	select {
	case err = <-errCh:
		t.Fatalf("http.ListenAndServe() failed: %v", err)
	case <-time.After(time.Second):
		// Server started normally
	}

	if client, err = NewSSEClientTransport(serverURL); err != nil {
		t.Fatalf("NewSSEClientTransport failed: %v", err)
	}

	testTransport(t, client, svr)
}

// Test SSE client options functionality
func TestSSEClientOptions(t *testing.T) {
	customLogger := newTestLogger()
	customTimeout := 5 * time.Second
	customClient := &http.Client{Timeout: 10 * time.Second}

	client, err := NewSSEClientTransport("http://example.com/sse",
		WithSSEClientOptionReceiveTimeout(customTimeout),
		WithSSEClientOptionHTTPClient(customClient),
		WithSSEClientOptionLogger(customLogger),
	)
	assert.NoError(t, err)

	sseClient, ok := client.(*sseClientTransport)
	assert.True(t, ok)

	assert.Equal(t, customTimeout, sseClient.receiveTimeout)
	assert.Equal(t, customClient, sseClient.client)
	assert.Equal(t, customLogger, sseClient.logger)
}

// Test SSE server options functionality
func TestSSEServerOptions(t *testing.T) {
	customLogger := newTestLogger()
	customSSEPath := "/custom-sse"
	customMsgPath := "/custom-message"
	customURLPrefix := "http://test.example.com"

	server, err := NewSSEServerTransport("localhost:8080",
		WithSSEServerTransportOptionLogger(customLogger),
		WithSSEServerTransportOptionSSEPath(customSSEPath),
		WithSSEServerTransportOptionMessagePath(customMsgPath),
		WithSSEServerTransportOptionURLPrefix(customURLPrefix),
	)
	assert.NoError(t, err)

	sseServer, ok := server.(*sseServerTransport)
	assert.True(t, ok)

	assert.Equal(t, customLogger, sseServer.logger)
	assert.Equal(t, customSSEPath, sseServer.ssePath)
	assert.Equal(t, customMsgPath, sseServer.messagePath)
	assert.Equal(t, customURLPrefix, sseServer.urlPrefix)
}

// Test SSE server error handling
func TestSSEServerErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client, err := NewSSEClientTransport(server.URL)
	assert.NoError(t, err)

	err = client.Start()
	assert.Error(t, err)
}

// Test SSE server message handling errors
func TestSSEServerMessageHandling(t *testing.T) {
	svr, handler, err := NewSSEServerTransportAndHandler("http://example.com/message", WithSSEServerTransportAndHandlerOptionLogger(newTestLogger()))
	assert.NoError(t, err)

	// Test unsupported HTTP method
	req := httptest.NewRequest(http.MethodGet, "/message?sessionID=test", nil)
	rr := httptest.NewRecorder()
	handler.HandleMessage().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)

	// Test missing session ID
	req = httptest.NewRequest(http.MethodPost, "/message", nil)
	rr = httptest.NewRecorder()
	handler.HandleMessage().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Test invalid session ID
	req = httptest.NewRequest(http.MethodPost, "/message?sessionID=invalid", nil)
	rr = httptest.NewRecorder()
	handler.HandleMessage().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	svrCtx, svrCancel := context.WithCancel(context.Background())
	svrCancel()
	err = svr.Shutdown(ctx, svrCtx)
	assert.NoError(t, err)
}

// Test completeMessagePath function
func TestCompleteMessagePath(t *testing.T) {
	tests := []struct {
		name          string
		urlPrefix     string
		messagePath   string
		expectedURL   string
		expectedError bool
	}{
		{
			name:        "Valid URL",
			urlPrefix:   "http://example.com",
			messagePath: "/message",
			expectedURL: "http://example.com/message",
		},
		{
			name:        "URL with trailing slash",
			urlPrefix:   "http://example.com/",
			messagePath: "message",
			expectedURL: "http://example.com/message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := completeMessagePath(tt.urlPrefix, tt.messagePath)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, url)
			}
		})
	}
}

// Test handleSSEEvent processing various event types
func TestHandleSSEEvent(t *testing.T) {
	transport := &sseClientTransport{
		ctx:            context.Background(),
		endpointChan:   make(chan struct{}, 1),
		logger:         newTestLogger(),
		receiveTimeout: time.Second,
	}

	// Set up a receiver
	receivedMsg := make(chan []byte, 1)
	transport.SetReceiver(ClientReceiverF(func(ctx context.Context, msg []byte) error {
		receivedMsg <- msg
		return nil
	}))

	// Test endpoint event
	transport.handleSSEEvent("endpoint", "http://example.com/message?sessionID=test")
	assert.NotNil(t, transport.messageEndpoint)
	assert.Equal(t, "http://example.com/message?sessionID=test", transport.messageEndpoint.String())

	// Test message event
	transport.handleSSEEvent("message", "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\"}")
	select {
	case msg := <-receivedMsg:
		assert.Equal(t, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\"}", string(msg))
	case <-time.After(time.Second):
		t.Fatal("Message not received")
	}
}

// Test readSSE function
func TestReadSSE(t *testing.T) {
	transport := &sseClientTransport{
		ctx:            context.Background(),
		cancel:         func() {},
		endpointChan:   make(chan struct{}, 1),
		logger:         newTestLogger(),
		receiveTimeout: time.Second,
	}

	// Create test SSE data
	sseData := `
event: endpoint
data: http://example.com/message

event: message
data: {"jsonrpc":"2.0","id":1,"method":"test"}

`
	// Set up a receiver
	receivedMsg := make(chan []byte, 1)
	transport.SetReceiver(ClientReceiverF(func(ctx context.Context, msg []byte) error {
		receivedMsg <- msg
		return nil
	}))

	// Create a reader
	reader := io.NopCloser(strings.NewReader(sseData))

	// Start reading
	go transport.readSSE(reader)

	// Wait for endpoint processing
	select {
	case <-transport.endpointChan:
		assert.NotNil(t, transport.messageEndpoint)
	case <-time.After(time.Second):
		t.Fatal("Endpoint not received")
	}

	// Wait for message processing
	select {
	case msg := <-receivedMsg:
		assert.Equal(t, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\"}", string(msg))
	case <-time.After(time.Second):
		t.Fatal("Message not received")
	}
}

// Test SSE client start with unavailable url
func TestSSEClientStartError(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{
			name: "Invalid URL",
			url:  "http://invalid url with space",
		},
		{
			name: "Unavailable URL",
			url:  "http://127.0.0.1:6666/sse",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, _ := NewSSEClientTransport(c.url)
			err := client.Start()
			assert.Error(t, err)
		})
	}
}

// TestSSEClientEndpointTimeout tests the case where the SSE client times out waiting for the endpoint
func TestSSEClientEndpointTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// mock sse server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set SSE response headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Simulate a server that never sends the endpoint event
		// Only send other events to keep the connection alive
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to be an http.Flusher")
		}

		// Send some other events, but not endpoint events
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				return // Test completed, exit handler
			default:
				// Send a non-endpoint event
				_, err := w.Write([]byte("event: ping\ndata: keepalive\n\n"))
				if err != nil {
					return
				}
				flusher.Flush()
				time.Sleep(time.Second)
			}
		}

		// Wait for test completion or timeout
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second): // Set a longer timeout to ensure it doesn't block forever
			return
		}
	}))
	defer mockServer.Close()

	// Create SSE client
	client, err := NewSSEClientTransport(mockServer.URL)
	assert.NoError(t, err)

	// Start the client, should wait for the endpoint event until timeout
	startTime := time.Now()
	err = client.Start()
	duration := time.Since(startTime)

	// Cancel context to ensure server handler exits
	cancel()

	// Verify that the error is a timeout error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for endpoint")

	// Verify that the wait time is close to 10 seconds
	assert.GreaterOrEqual(t, duration.Seconds(), 9.9)
	assert.Less(t, duration.Seconds(), 11.0) // Allow some margin of error
}

func TestSSEClientTransportSendFailure(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func() (*sseClientTransport, *httptest.Server)
		expectedErr string
	}{
		{
			name: "Failure - Create request failed",
			setupMock: func() (*sseClientTransport, *httptest.Server) {
				// Set an invalid URL that will cause NewRequestWithContext to fail
				transport := &sseClientTransport{
					logger: newTestLogger(),
					messageEndpoint: &url.URL{
						Scheme: "http",
						Host:   "invalid url with spaces", // Invalid URL
					},
					client: &http.Client{},
				}
				return transport, nil
			},
			expectedErr: "failed to create request:",
		},
		{
			name: "Failure - Send request failed",
			setupMock: func() (*sseClientTransport, *httptest.Server) {
				// Set up a client that will cause Do method to fail
				customClient := &http.Client{
					// Set a very short timeout to make the request timeout
					Timeout: time.Nanosecond,
				}

				// Create a test server that will never be accessed
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(time.Second)
					w.WriteHeader(http.StatusOK)
				}))

				endpoint, _ := url.Parse(server.URL)
				transport := &sseClientTransport{
					logger:          newTestLogger(),
					messageEndpoint: endpoint,
					client:          customClient,
				}
				return transport, server
			},
			expectedErr: "failed to send message:",
		},
		{
			name: "Failure - Non-success status code",
			setupMock: func() (*sseClientTransport, *httptest.Server) {
				// Create a test server that always returns an error
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error"))
				}))

				endpoint, _ := url.Parse(server.URL)
				transport := &sseClientTransport{
					logger:          newTestLogger(),
					messageEndpoint: endpoint,
					client:          &http.Client{},
				}
				return transport, server
			},
			expectedErr: "unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock environment
			transport, server := tt.setupMock()
			if server != nil {
				defer server.Close()
			}

			// Execute test
			ctx := context.Background()
			err := transport.Send(ctx, Message([]byte(`{"test":"message"}`)))

			// Verify results
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestSSEClientTransportSendContextCanceled(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay response to give context time to be canceled
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint, _ := url.Parse(server.URL)
	transport := &sseClientTransport{
		logger:          newTestLogger(),
		messageEndpoint: endpoint,
		client:          &http.Client{},
	}

	// Create a context that will be immediately canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	// Execute test
	err := transport.Send(ctx, Message([]byte(`{"test":"message"}`)))

	// Verify results
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "context canceled"))
}

// getAvailablePort returns a port that is available for use
func getAvailablePort() (int, error) {
	addr, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to get available port: %v", err)
	}
	defer func() {
		if err = addr.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	port := addr.Addr().(*net.TCPAddr).Port
	return port, nil
}

// Test handleSSEEvent error handling
func TestHandleSSEEventErrors(t *testing.T) {
	t.Run("endpoint URL parse error", func(t *testing.T) {
		// Create a custom logger that can capture logs
		var errorLogCapture string
		mockLogger := &testLogger{
			errorCapture: &errorLogCapture,
		}

		transport := &sseClientTransport{
			ctx:          context.Background(),
			endpointChan: make(chan struct{}, 1),
			logger:       mockLogger,
		}

		// Send an invalid URL as an endpoint event
		invalidURL := "http://invalid url with spaces"
		transport.handleSSEEvent("endpoint", invalidURL)

		// Verify that messageEndpoint should be nil
		assert.Nil(t, transport.messageEndpoint)

		// Verify that the error log contains the error message
		assert.Contains(t, errorLogCapture, "Error parsing endpoint URL")

		// Verify that endpointChan is not closed
		select {
		case <-transport.endpointChan:
			t.Error("endpointChan was unexpectedly closed")
		default:
			// Correct behavior, channel should remain open
		}
	})

	t.Run("message receive error", func(t *testing.T) {
		// Create a custom logger that can capture logs
		var errorLogCapture string
		mockLogger := &testLogger{
			errorCapture: &errorLogCapture,
		}

		transport := &sseClientTransport{
			ctx:            context.Background(),
			logger:         mockLogger,
			receiveTimeout: time.Second,
		}

		// Set up a receiver that returns an error
		transport.SetReceiver(ClientReceiverF(func(ctx context.Context, msg []byte) error {
			return fmt.Errorf("simulated receive error")
		}))

		// Send a message event
		transport.handleSSEEvent("message", "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\"}")

		// Verify that the error log contains the error message
		assert.Contains(t, errorLogCapture, "Error receive message")
		assert.Contains(t, errorLogCapture, "simulated receive error")
	})
}

func TestSSEServerTransportSendFail(t *testing.T) {

	svr, _ := NewSSEServerTransport("localhost:8080")

	invalidSessionID := "invalid-session-id"
	msg := Message([]byte("this is ThinkInAI team"))

	// Call the Send method and check the returned error
	err := svr.Send(context.Background(), invalidSessionID, msg)
	assert.Error(t, err)
	assert.Equal(t, pkg.ErrLackSession, err)
}

// TestReadSSEError tests the readSSE function when encountering errors during reading
func TestReadSSEError(t *testing.T) {
	logger := newTestLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transport := &sseClientTransport{
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		receiveTimeout: time.Second,
	}

	t.Run("ConnectionClosed", func(t *testing.T) {
		pr, pw := io.Pipe()

		done := make(chan struct{})
		go func() {
			transport.readSSE(pr)
			close(done)
		}()

		pw.Write([]byte("event: test\ndata: testdata\n\n"))

		pw.Close()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("readSSE did not exit after pipe closed")
		}
	})

	t.Run("ReadError", func(t *testing.T) {
		errReader := &customErrorReader{err: errors.New("custom read error")}

		done := make(chan struct{})
		go func() {
			transport.readSSE(io.NopCloser(errReader))
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("readSSE did not exit after read error")
		}
	})
}

type customErrorReader struct {
	err error
}

func (r *customErrorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

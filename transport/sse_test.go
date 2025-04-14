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
	"sync"
	"testing"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"

	"github.com/stretchr/testify/assert"
)

// Wrapper to notify when close operation is performed
type notifyingCloser struct {
	reader    io.ReadCloser
	closeOnce sync.Once
}

func (n *notifyingCloser) Read(p []byte) (int, error) {
	return n.reader.Read(p)
}

func (n *notifyingCloser) Close() error {
	var err error
	n.closeOnce.Do(func() {
		err = n.reader.Close()
	})
	return err
}

type customErrorReader struct {
	err error
}

func (r *customErrorReader) Read(_ []byte) (n int, err error) {
	return 0, r.err
}

// mockTransport 实现http.RoundTripper接口，用于模拟HTTP传输层
type mockTransport struct {
	mockDoFunc func(*http.Request) (*http.Response, error)
}

// mockReadCloser implements io.ReadCloser interface to simulate Close() method returning an error
type mockReadCloser struct {
	io.Reader
	closeFunc func() error
}

func (m *mockReadCloser) Close() error {
	return m.closeFunc()
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.mockDoFunc(req)
}

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
		if e := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); e != nil {
			log.Fatalf("Failed to start HTTP server: %v", e)
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

// TestSSEServerReadSSEError tests the readSSE function when encountering errors during reading
func TestSSEServerReadSSEError(t *testing.T) {
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

		_, _ = pw.Write(pkg.S2B("event: test\ndata: testdata\n\n"))

		_ = pw.Close()

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

	// Test scenario 3: Context cancellation (simulating the actual implementation)
	t.Run("ContextCanceled", func(t *testing.T) {
		// Create a read-write pipe
		pr, pw := io.Pipe()

		// Create a dedicated context for this test
		ctxTest, cancelTest := context.WithCancel(context.Background())
		transportTest := &sseClientTransport{
			ctx:            ctxTest,
			cancel:         cancelTest,
			logger:         logger,
			receiveTimeout: time.Second,
		}

		// Use a custom closer, so we can be notified when the reader is closed
		closeNotifier := &notifyingCloser{reader: pr}

		// Start a goroutine to read SSE
		done := make(chan struct{})
		go func() {
			transportTest.readSSE(closeNotifier)
			close(done)
		}()

		// Write some data to ensure the goroutine is running
		_, _ = pw.Write([]byte("event: ping\ndata: ping\n\n"))

		// Start a goroutine to simulate the actual closing logic in implementation
		closeDone := make(chan struct{})
		go func() {
			<-ctxTest.Done()

			// Close the reader, which will cause ReadString in readSSE to return an error
			_ = closeNotifier.Close()
			close(closeDone)
		}()

		// Wait a bit to ensure goroutines are running
		time.Sleep(100 * time.Millisecond)

		// Cancel the context
		cancelTest()

		// Wait for the close operation to complete
		select {
		case <-closeDone:
		case <-time.After(time.Second):
			t.Fatal("context cancellation didn't trigger reader close")
		}

		// Ensure readSSE function exits properly
		select {
		case <-done:
			// Success, function exited as expected
		case <-time.After(time.Second):
			t.Fatal("readSSE did not exit after context canceled and reader closed")
		}

		// Cleanup
		_ = pw.Close()
	})
}

func TestSSEServerTransportSendFail(t *testing.T) {
	svr, _ := NewSSEServerTransport("localhost:8080")

	invalidSessionID := "invalid-session-id"
	msg := Message("this is ThinkInAI team")

	// Call the Send method and check the returned error
	err := svr.Send(context.Background(), invalidSessionID, msg)
	assert.Error(t, err)
	assert.Equal(t, pkg.ErrLackSession, err)
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
			urlPrefix:   "https://example.com",
			messagePath: "/message",
			expectedURL: "https://example.com/message",
		},
		{
			name:        "URL with trailing slash",
			urlPrefix:   "https://example.com/",
			messagePath: "message",
			expectedURL: "https://example.com/message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messagePath, err := completeMessagePath(tt.urlPrefix, tt.messagePath)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, messagePath)
			}
		})
	}
}

// Test handleSSEEvent processing various event types
func TestSSEClientHandleSSEEvent(t *testing.T) {
	cases := []struct {
		name          string
		event         string
		data          string
		want          string
		errorReceiver ClientReceiverF
		wantErr       bool
	}{
		{
			name:          "URL with sessionID",
			event:         "endpoint",
			data:          "https://example.com/message?sessionID=test",
			want:          "https://example.com/message?sessionID=test",
			errorReceiver: nil,
			wantErr:       false,
		},
		{
			name:          "URL without sessionID",
			event:         "endpoint",
			data:          "https://thinkInAI.com/mcp",
			want:          "https://thinkInAI.com/mcp",
			errorReceiver: nil,
			wantErr:       false,
		},
		{
			name:          "Invalid URL in data",
			event:         "endpoint",
			data:          "httsp://invalid url with spaces",
			want:          "Error parsing endpoint URL",
			errorReceiver: nil,
			wantErr:       true,
		},
		{
			name:          "message event",
			event:         "message",
			data:          "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\"}",
			want:          "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\"}",
			errorReceiver: nil,
			wantErr:       false,
		},
		{
			name:          "message rece error",
			event:         "message",
			data:          "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\"}",
			want:          "Error receive message",
			wantErr:       true,
			errorReceiver: clientReceiveError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var errorLogCapture string
			mockLogger := &testLogger{
				errorCapture: &errorLogCapture,
			}

			transport := &sseClientTransport{
				serverURL: func() *url.URL {
					uri, err := url.Parse("http://example.com/sse")
					if err != nil {
						panic(err)
					}
					return uri
				}(),
				ctx:            context.Background(),
				endpointChan:   make(chan struct{}, 1),
				logger:         mockLogger,
				receiveTimeout: time.Second * 30,
			}

			// Set up a receiver
			receivedMsg := make(chan []byte, 1)
			transport.SetReceiver(ClientReceiverF(func(_ context.Context, msg []byte) error {
				receivedMsg <- msg
				return nil
			}))

			if tt.errorReceiver != nil {
				transport.SetReceiver(tt.errorReceiver)
			}

			transport.handleSSEEvent(tt.event, tt.data)
			switch tt.event {
			case "endpoint":

				if tt.wantErr {
					assert.Contains(t, errorLogCapture, tt.want)
					return
				}

				assert.Equal(t, tt.want, transport.messageEndpoint.String())
			case "message":

				if tt.wantErr {
					assert.Contains(t, errorLogCapture, tt.want)
					return
				}

				select {
				case msg := <-receivedMsg:
					assert.Equal(t, tt.want, string(msg))
				case <-time.After(time.Second):
					assert.Fail(t, "timed out")
				}
			}
		})
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
	transport.SetReceiver(ClientReceiverF(func(_ context.Context, msg []byte) error {
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

// Test new sse client with invalid URL
func TestNewSSEClientWithInvalidURL(t *testing.T) {
	_, err := NewSSEClientTransport("http://invalid url with space")
	assert.Error(t, err)
}

// Test sse client connect to unavailable URL
func TestSSEClientStartError(t *testing.T) {
	cases := []struct {
		name        string
		serverURL   func() string
		errContains string
	}{
		{
			name: "Invalid URL",
			serverURL: func() string {
				return "http://127.0.0.1:16666/sse"
			},
			errContains: "failed to connect to SSE stream",
		},
		{
			name: "SSE request not 200",
			serverURL: func() string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write(pkg.S2B("Internal Server Error"))
				}))
				return server.URL
			},
			errContains: "unexpected status code",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := NewSSEClientTransport(tt.serverURL())
			err := client.Start()
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// TestSSEClientEndpointTimeout tests the case where the SSE client times out waiting for the endpoint
func TestSSEClientEndpointTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// mock sse server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	assert.Less(t, duration.Seconds(), 11.0)
}

func TestSSEClientSendFailure(t *testing.T) {
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
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write(pkg.S2B("Internal Server Error"))
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
			err := transport.Send(ctx, []byte(`{"test":"message"}`))

			// Verify results
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestSSEClientSendContextCanceled(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	err := transport.Send(ctx, []byte(`{"test":"message"}`))

	// Verify results
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "context canceled"))
}

// TestSSEClientRespBodyCloseError tests the case where resp.Body.Close() returns an error
func TestSSEClientRespBodyCloseError(t *testing.T) {
	// Create a custom logger to capture logs
	var errorLogCapture string
	mockLogger := &testLogger{
		errorCapture: &errorLogCapture,
	}

	// Create an httptest server to simulate SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set SSE response headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Send endpoint event so the client can continue execution
		_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s/message\n\n", "https://example.com")

		// Flush the response
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Keep the connection open
		<-r.Context().Done()
	}))
	defer server.Close()

	// Create a custom transport layer that returns an error when closing the Body
	transport := &mockTransport{
		mockDoFunc: func(req *http.Request) (*http.Response, error) {
			// Call the real server but replace the response body with our mock
			resp, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				return nil, err
			}

			// Save the original response body for reading
			originalBody := resp.Body

			// Create a proxy response body that can read normally but returns an error on close
			resp.Body = &mockReadCloser{
				Reader: originalBody,
				closeFunc: func() error {
					// First close the original body to avoid leaks
					_ = originalBody.Close()
					// Then return our mock error
					return errors.New("mock response body close error")
				},
			}

			return resp, nil
		},
	}

	// Create a custom HTTP client using our mock transport
	client := &http.Client{
		Transport: transport,
	}

	// Create the SSE client
	sseClient, err := NewSSEClientTransport(server.URL,
		WithSSEClientOptionHTTPClient(client),
		WithSSEClientOptionLogger(mockLogger))
	assert.NoError(t, err)

	// Start the client
	err = sseClient.Start()
	assert.NoError(t, err, "SSE client should start successfully")

	// Close the client, which will trigger ctx.Done() and eventually call resp.Body.Close()
	err = sseClient.Close()
	assert.NoError(t, err, "Closing SSE client should not return an error")

	// Give some time for the log to be recorded
	time.Sleep(100 * time.Millisecond)

	// Verify that the log contains the expected error message
	assert.Contains(t, errorLogCapture, "failed to close SSE stream body")
	assert.Contains(t, errorLogCapture, "mock response body close error")
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

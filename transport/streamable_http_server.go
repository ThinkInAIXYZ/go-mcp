package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

type StateMode string

const (
	Stateful  StateMode = "stateful"
	Stateless StateMode = "stateless"
)

type SessionIDForReturnKey struct{}

type SessionIDForReturn struct {
	SessionID string
}

type StreamableHTTPServerTransportOption func(*streamableHTTPServerTransport)

func WithStreamableHTTPServerTransportOptionLogger(logger pkg.Logger) StreamableHTTPServerTransportOption {
	return func(t *streamableHTTPServerTransport) {
		t.logger = logger
	}
}

func WithStreamableHTTPServerTransportOptionEndpoint(endpoint string) StreamableHTTPServerTransportOption {
	return func(t *streamableHTTPServerTransport) {
		t.mcpEndpoint = endpoint
	}
}

func WithStreamableHTTPServerTransportOptionStateMode(mode StateMode) StreamableHTTPServerTransportOption {
	return func(t *streamableHTTPServerTransport) {
		t.stateMode = mode
	}
}

type StreamableHTTPServerTransportAndHandlerOption func(*streamableHTTPServerTransport)

func WithStreamableHTTPServerTransportAndHandlerOptionLogger(logger pkg.Logger) StreamableHTTPServerTransportAndHandlerOption {
	return func(t *streamableHTTPServerTransport) {
		t.logger = logger
	}
}

func WithStreamableHTTPServerTransportAndHandlerOptionStateMode(mode StateMode) StreamableHTTPServerTransportAndHandlerOption {
	return func(t *streamableHTTPServerTransport) {
		t.stateMode = mode
	}
}

type streamableHTTPServerTransport struct {
	// ctx is the context that controls the lifecycle of the server
	ctx    context.Context
	cancel context.CancelFunc

	httpSvr *http.Server

	stateMode StateMode

	inFlySend sync.WaitGroup

	receiver serverReceiver

	sessionManager sessionManager

	// options
	logger      pkg.Logger
	mcpEndpoint string // The single MCP endpoint path
}

type StreamableHTTPHandler struct {
	transport *streamableHTTPServerTransport
}

// HandleMCP handles incoming MCP requests
func (h *StreamableHTTPHandler) HandleMCP() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.transport.handleMCPEndpoint(w, r)
	})
}

// NewStreamableHTTPServerTransportAndHandler returns transport without starting the HTTP server,
// and returns a Handler for users to start their own HTTP server externally
// eg:
// transport, handler, _ := NewStreamableHTTPServerTransportAndHandler()
// http.Handle("/mcp", handler.HandleMCP())
// http.ListenAndServe(":8080", nil)
func NewStreamableHTTPServerTransportAndHandler(
	opts ...StreamableHTTPServerTransportAndHandlerOption,
) (ServerTransport, *StreamableHTTPHandler, error) { //nolint:whitespace

	ctx, cancel := context.WithCancel(context.Background())

	t := &streamableHTTPServerTransport{
		ctx:       ctx,
		cancel:    cancel,
		stateMode: Stateless,
		logger:    pkg.DefaultLogger,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, &StreamableHTTPHandler{transport: t}, nil
}

func NewStreamableHTTPServerTransport(addr string, opts ...StreamableHTTPServerTransportOption) ServerTransport {
	ctx, cancel := context.WithCancel(context.Background())

	t := &streamableHTTPServerTransport{
		ctx:         ctx,
		cancel:      cancel,
		stateMode:   Stateless,
		logger:      pkg.DefaultLogger,
		mcpEndpoint: "/mcp", // Default MCP endpoint
	}

	for _, opt := range opts {
		opt(t)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(t.mcpEndpoint, t.handleMCPEndpoint)

	t.httpSvr = &http.Server{
		Addr:        addr,
		Handler:     mux,
		IdleTimeout: time.Minute,
	}

	return t
}

func (t *streamableHTTPServerTransport) Run() error {
	if t.httpSvr == nil {
		<-t.ctx.Done()
		return nil
	}

	fmt.Printf("starting mcp server at http://%s%s\n", t.httpSvr.Addr, t.mcpEndpoint)

	if err := t.httpSvr.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (t *streamableHTTPServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	t.inFlySend.Add(1)
	defer t.inFlySend.Done()

	select {
	case <-t.ctx.Done():
		return t.ctx.Err()
	default:
		return t.sessionManager.EnqueueMessageForSend(ctx, sessionID, msg)
	}
}

func (t *streamableHTTPServerTransport) SetReceiver(receiver serverReceiver) {
	t.receiver = receiver
}

func (t *streamableHTTPServerTransport) SetSessionManager(manager sessionManager) {
	t.sessionManager = manager
}

func (t *streamableHTTPServerTransport) handleMCPEndpoint(w http.ResponseWriter, r *http.Request) {
	defer pkg.RecoverWithFunc(func(_ any) {
		t.writeError(w, http.StatusInternalServerError, "Internal server error")
	})

	switch r.Method {
	case http.MethodPost:
		t.handlePost(w, r)
	case http.MethodGet:
		t.handleGet(w, r)
	case http.MethodDelete:
		t.handleDelete(w, r)
	default:
		t.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (t *streamableHTTPServerTransport) handlePost(w http.ResponseWriter, r *http.Request) {
	// Validate Accept header
	accept := r.Header.Get("Accept")
	if accept == "" {
		t.writeError(w, http.StatusBadRequest, "Missing Accept header")
		return
	}

	// Read and process the message
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	ctx := r.Context()

	// For InitializeRequest HTTP response
	if t.stateMode == Stateful {
		ctx = context.WithValue(ctx, SessionIDForReturnKey{}, &SessionIDForReturn{})
	}

	outputMsgCh, err := t.receiver.Receive(ctx, r.Header.Get(sessionIDHeader), bs)
	if err != nil {
		if errors.Is(err, pkg.ErrSessionClosed) {
			t.writeError(w, http.StatusNotFound, fmt.Sprintf("Failed to receive: %v", err))
			return
		}
		t.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to receive: %v", err))
		return
	}

	if outputMsgCh == nil { // reply response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		t.writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if protocol.IsInitializedRequest(bs) { // 判断是否是init请求
		msg := <-outputMsgCh
		if t.stateMode == Stateful {
			w.Header().Set(sessionIDHeader, ctx.Value(SessionIDForReturnKey{}).(*SessionIDForReturn).SessionID)
		}

		if _, err = fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
			t.logger.Errorf("Failed to write message: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	go func() {
		defer pkg.Recover()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if _, e := fmt.Fprintf(w, " : heartbeat\n\n"); e != nil {
				t.logger.Errorf("Failed to write heartbeat: %v", e)
				continue
			}
			flusher.Flush()
		}
	}()

	for msg := range outputMsgCh {
		if _, err = fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
			t.logger.Errorf("Failed to write message: %v", err)
			continue
		}
		flusher.Flush()
	}
}

func (t *streamableHTTPServerTransport) handleGet(w http.ResponseWriter, r *http.Request) {
	defer pkg.RecoverWithFunc(func(_ any) {
		t.writeError(w, http.StatusInternalServerError, "Internal server error")
	})

	if t.stateMode == Stateless {
		t.writeError(w, http.StatusMethodNotAllowed, "server is stateless, not support sse connection")
		return
	}

	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		t.writeError(w, http.StatusBadRequest, "Must accept text/event-stream")
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create flush-supporting writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}
	sessionID := r.Header.Get(sessionIDHeader)
	if sessionID == "" {
		t.writeError(w, http.StatusBadRequest, "Missing Session ID")
		flusher.Flush()
		return
	}
	if err := t.sessionManager.OpenMessageQueueForSend(sessionID); err != nil {
		t.writeError(w, http.StatusBadRequest, err.Error())
		flusher.Flush()
		return
	}
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		msg, err := t.sessionManager.DequeueMessageForSend(r.Context(), sessionID)
		if err != nil {
			if errors.Is(err, pkg.ErrSendEOF) {
				return
			}
			t.logger.Debugf("sse connect dequeueMessage err: %+v, sessionID=%s", err.Error(), sessionID)
			return
		}

		t.logger.Debugf("Sending message: %s", string(msg))

		if _, err = fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
			t.logger.Errorf("Failed to write message: %v", err)
			continue
		}
		flusher.Flush()
	}
}

func (t *streamableHTTPServerTransport) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.writeError(w, http.StatusBadRequest, "Missing session ID")
		return
	}

	t.sessionManager.CloseSession(sessionID)
	w.WriteHeader(http.StatusOK)
}

func (t *streamableHTTPServerTransport) writeError(w http.ResponseWriter, code int, message string) {
	if code == http.StatusMethodNotAllowed {
		t.logger.Infof("streamableHTTPServerTransport response: code: %d, message: %s", code, message)
	} else {
		t.logger.Errorf("streamableHTTPServerTransport Error: code: %d, message: %s", code, message)
	}

	resp := protocol.NewJSONRPCErrorResponse(nil, protocol.InternalError, message)
	bytes, err := json.Marshal(resp)
	if err != nil {
		t.logger.Errorf("streamableHTTPServerTransport writeError json.Marshal: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(bytes); err != nil {
		t.logger.Errorf("streamableHTTPServerTransport writeError Write: %v", err)
	}
}

func (t *streamableHTTPServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	shutdownFunc := func() {
		<-serverCtx.Done()

		t.cancel()

		t.inFlySend.Wait()

		t.sessionManager.CloseAllSessions()
	}

	if t.httpSvr == nil {
		shutdownFunc()
		return nil
	}

	t.httpSvr.RegisterOnShutdown(shutdownFunc)

	if err := t.httpSvr.Shutdown(userCtx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	return nil
}

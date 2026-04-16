package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type SSEServerTransportOption func(*sseServerTransport)

func WithSSEServerTransportOptionLogger(logger pkg.Logger) SSEServerTransportOption {
	return func(t *sseServerTransport) {
		t.logger = logger
	}
}

func WithSSEServerTransportOptionSSEPath(ssePath string) SSEServerTransportOption {
	return func(t *sseServerTransport) {
		t.ssePath = ssePath
	}
}

func WithSSEServerTransportOptionMessagePath(messagePath string) SSEServerTransportOption {
	return func(t *sseServerTransport) {
		t.messagePath = messagePath
	}
}

func WithSSEServerTransportOptionURLPrefix(urlPrefix string) SSEServerTransportOption {
	return func(t *sseServerTransport) {
		t.urlPrefix = urlPrefix
	}
}

type SSEServerTransportAndHandlerOption func(*sseServerTransport)

func WithSSEServerTransportAndHandlerOptionCopyParamKeys(paramsKey []string) SSEServerTransportAndHandlerOption {
	return func(t *sseServerTransport) {
		t.copyParamKeys = paramsKey
	}
}

func WithSSEServerTransportAndHandlerOptionLogger(logger pkg.Logger) SSEServerTransportAndHandlerOption {
	return func(t *sseServerTransport) {
		t.logger = logger
	}
}

type sseServerTransport struct {
	// ctx is the context that controls the lifecycle of the SSE server.
	// It is used to coordinate cancellation of all ongoing send operations when the server is shutting down.
	ctx context.Context
	// cancel is the function to cancel the ctx when the server needs to shut down.
	// It is called during server shutdown to gracefully terminate all connections and operations.
	cancel context.CancelFunc

	httpSvr *http.Server

	messageEndpointURL string // Auto-generated

	inFlySend sync.WaitGroup

	receiver serverReceiver

	sessionManager sessionManager

	// options
	logger        pkg.Logger
	ssePath       string
	messagePath   string
	urlPrefix     string
	copyParamKeys []string
}

type SSEHandler struct {
	transport *sseServerTransport
}

// HandleSSE handles incoming SSE connections from clients and sends messages to them.
func (h *SSEHandler) HandleSSE() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.transport.handleSSE(w, r)
	})
}

// HandleMessage processes incoming JSON-RPC messages from clients and sends responses
// back through both the SSE connection and HTTP response.
func (h *SSEHandler) HandleMessage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.transport.handleMessage(w, r)
	})
}

// NewSSEServerTransport returns transport that will start an HTTP server
func NewSSEServerTransport(addr string, opts ...SSEServerTransportOption) (ServerTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	t := &sseServerTransport{
		ctx:         ctx,
		cancel:      cancel,
		logger:      pkg.DefaultLogger,
		ssePath:     "/sse",
		messagePath: "/message",
		urlPrefix:   "",
	}
	for _, opt := range opts {
		opt(t)
	}

	t.messageEndpointURL = t.messagePath
	// Set default values for ssePath and messagePath
	if t.urlPrefix != "" {
		messageEndpointFullURL, err := completeMessagePath(t.urlPrefix, t.messagePath)
		if err != nil {
			return nil, fmt.Errorf("NewSSEServerTransport failed: completeMessagePath %v", err)
		}
		t.messageEndpointURL = messageEndpointFullURL
	}

	mux := http.NewServeMux()
	mux.HandleFunc(t.ssePath, t.handleSSE)
	mux.HandleFunc(t.messagePath, t.handleMessage)

	t.httpSvr = &http.Server{
		Addr:        addr,
		Handler:     mux,
		IdleTimeout: time.Minute,
	}

	return t, nil
}

// NewSSEServerTransportAndHandler returns transport without starting the HTTP server,
// and returns a Handler for users to start their own HTTP server externally
// eg:
// 1. relative path
// transport, handler, _ :=  NewSSEServerTransportAndHandler("/sse/message")
// http.Handle("/sse", handler.HandleSSE())
// http.Handle("/sse/message", handler.HandleMessage())
// http.ListenAndServe(":8080", nil)
// 2. full url
// transport, handler, _ :=  NewSSEServerTransportAndHandler("https://thinkingai.xyz/api/v1/sse/message")
// http.Handle("/sse", handler.HandleSSE())
// http.Handle("/sse/message", handler.HandleMessage())
// http.ListenAndServe(":8080", nil)
func NewSSEServerTransportAndHandler(messageEndpointURL string,
	opts ...SSEServerTransportAndHandlerOption,
) (ServerTransport, *SSEHandler, error) { //nolint:whitespace

	ctx, cancel := context.WithCancel(context.Background())

	t := &sseServerTransport{
		ctx:                ctx,
		cancel:             cancel,
		messageEndpointURL: messageEndpointURL,
		logger:             pkg.DefaultLogger,
	}
	for _, opt := range opts {
		opt(t)
	}

	return t, &SSEHandler{transport: t}, nil
}

func (t *sseServerTransport) Run() error {
	if t.httpSvr == nil {
		<-t.ctx.Done()
		return nil
	}

	fmt.Printf("starting mcp server at http://%s%s\n", t.httpSvr.Addr, t.ssePath)

	if err := t.httpSvr.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (t *sseServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	t.inFlySend.Add(1)
	defer t.inFlySend.Done()

	select {
	case <-t.ctx.Done():
		return t.ctx.Err()
	default:
		return t.sessionManager.EnqueueMessageForSend(ctx, sessionID, msg)
	}
}

func (t *sseServerTransport) SetReceiver(receiver serverReceiver) {
	t.receiver = receiver
}

func (t *sseServerTransport) SetSessionManager(manager sessionManager) {
	t.sessionManager = manager
}

// handleSSE handles incoming SSE connections from clients and sends messages to them.
func (t *sseServerTransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	defer pkg.RecoverWithFunc(func(_ any) {
		t.writeError(w, http.StatusInternalServerError, "Internal server error")
	})

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
	w.WriteHeader(http.StatusOK)

	// Create an SSE connection
	sessionID := t.sessionManager.CreateSession(r.Context())
	defer t.sessionManager.CloseSession(sessionID)

	uri := fmt.Sprintf("%s?sessionID=%s", t.messageEndpointURL, sessionID)

	for _, key := range t.copyParamKeys {
		uri += fmt.Sprintf("&%s=%s", key, r.URL.Query().Get(key))
	}

	// Send the initial endpoint event
	if _, err := fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", uri); err != nil {
		t.logger.Errorf("send endpoint message fail")
		return
	}
	flusher.Flush()

	if err := t.sessionManager.OpenMessageQueueForSend(sessionID); err != nil {
		t.logger.Errorf("handleSSE sessionID=%s OpenMessageQueueForSend fail: %v", sessionID, err)
		return
	}

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

		if _, err = fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg); err != nil {
			t.logger.Errorf("Failed to write message: %v", err)
			continue
		}
		flusher.Flush()
	}
}

// handleMessage processes incoming JSON-RPC messages from clients and sends responses
// back through both the SSE connection and HTTP response.
func (t *sseServerTransport) handleMessage(w http.ResponseWriter, r *http.Request) {
	defer pkg.RecoverWithFunc(func(_ any) {
		t.writeError(w, http.StatusInternalServerError, "Internal server error")
	})

	if r.Method != http.MethodPost {
		t.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		t.writeError(w, http.StatusBadRequest, "Missing session ID")
		return
	}

	// Parse message as raw JSON
	inputMsg, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	outputMsgCh, err := t.receiver.Receive(r.Context(), sessionID, inputMsg)
	if err != nil {
		t.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to receive: %v", err))
		return
	}

	t.logger.Debugf("Received message: %s", string(inputMsg))
	w.WriteHeader(http.StatusAccepted)

	if outputMsgCh == nil {
		return
	}

	go func() {
		defer pkg.Recover()

		for msg := range outputMsgCh {
			if e := t.Send(context.Background(), sessionID, msg); e != nil {
				t.logger.Errorf("Failed to send message: %v", e)
			}
		}
	}()
}

// writeError writes a JSON-RPC error response with the given error details.
func (t *sseServerTransport) writeError(w http.ResponseWriter, code int, message string) {
	t.logger.Errorf("sseServerTransport Error: code: %d, message: %s", code, message)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	if _, err := w.Write([]byte(message)); err != nil {
		t.logger.Errorf("sseServerTransport writeError: %+v", err)
	}
}

func (t *sseServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
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

func completeMessagePath(urlPrefix string, messagePath string) (string, error) {
	prefixURL, err := url.Parse(urlPrefix)
	if err != nil {
		return "", fmt.Errorf("[completeMessagePath] failed to parse URL prefix: %w", err)
	}
	joinPath(prefixURL, messagePath)
	return prefixURL.String(), nil
}

// joinPath provided path elements joined to
// any existing path and the resulting path cleaned of any ./ or ../ elements.
// Any sequences of multiple / characters will be reduced to a single /.
func joinPath(u *url.URL, elem ...string) {
	elem = append([]string{u.EscapedPath()}, elem...)
	var p string
	if !strings.HasPrefix(elem[0], "/") {
		// Return a relative path if u is relative,
		// but ensure that it contains no ../ elements.
		elem[0] = "/" + elem[0]
		p = path.Join(elem...)[1:]
	} else {
		p = path.Join(elem...)
	}
	// path.Join will remove any trailing slashes.
	// Preserve at least one.
	if strings.HasSuffix(elem[len(elem)-1], "/") && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	u.Path = p
}

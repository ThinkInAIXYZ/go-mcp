package transport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type MockServerTransportOption func(*MockServerTransport)

func WithMockServerOptionLogger(log pkg.Logger) MockServerTransportOption {
	return func(t *MockServerTransport) {
		t.logger = log
	}
}

const mockSessionID = "mock"

type MockServerTransport struct {
	receiver ServerReceiver
	in       io.ReadCloser
	out      io.Writer

	logger pkg.Logger

	mu              sync.Mutex
	cancel          context.CancelFunc
	receiveShutDone chan struct{}
}

func NewMockServerTransport(in io.ReadCloser, out io.Writer, opts ...MockServerTransportOption) ServerTransport {
	server := &MockServerTransport{
		in:     in,
		out:    out,
		logger: pkg.DefaultLogger,

		receiveShutDone: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (t *MockServerTransport) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.cancel = cancel
	t.mu.Unlock()

	t.receive(ctx)

	close(t.receiveShutDone)
	return nil
}

func (t *MockServerTransport) Send(_ context.Context, _ string, msg Message) error {
	if _, err := t.out.Write(append(msg, mcpMessageDelimiter)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func (t *MockServerTransport) SetReceiver(receiver ServerReceiver) {
	t.receiver = receiver
}

func (t *MockServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()

	if err := t.in.Close(); err != nil {
		return err
	}

	select {
	case <-t.receiveShutDone:
		return nil
	case <-serverCtx.Done():
		return nil
	case <-userCtx.Done():
		return userCtx.Err()
	}
}

func (t *MockServerTransport) receive(ctx context.Context) {
	s := bufio.NewScanner(t.in)

	for s.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			if err := t.receiver.Receive(ctx, mockSessionID, s.Bytes()); err != nil {
				t.logger.Errorf("receiver failed: %v", err)
				continue
			}
		}
	}

	if err := s.Err(); err != nil {
		if !errors.Is(err, io.ErrClosedPipe) { // This error occurs during unit tests, suppressing it here
			t.logger.Errorf("server server unexpected error reading input: %v", err)
		}
		return
	}
}

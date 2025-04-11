package transport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

const mockSessionID = "mock"

type MockServerTransport struct {
	receiver ServerReceiver
	in       io.ReadCloser
	out      io.Writer

	logger pkg.Logger

	cancel          context.CancelFunc
	receiveShutDone chan struct{}
}

func NewMockServerTransport(in io.ReadCloser, out io.Writer) ServerTransport {
	return &MockServerTransport{
		in:     in,
		out:    out,
		logger: pkg.DefaultLogger,

		receiveShutDone: make(chan struct{}),
	}
}

func (t *MockServerTransport) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	t.receive(ctx)

	close(t.receiveShutDone)
	return nil
}

func (t *MockServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	if _, err := t.out.Write(append(msg, mcpMessageDelimiter)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func (t *MockServerTransport) SetReceiver(receiver ServerReceiver) {
	t.receiver = receiver
}

func (t *MockServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	t.cancel()

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

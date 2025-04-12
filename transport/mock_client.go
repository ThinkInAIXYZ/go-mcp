package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type MockClientTransport struct {
	receiver ClientReceiver
	in       io.Reader
	out      io.Writer

	logger pkg.Logger

	cancel          context.CancelFunc
	receiveShutDone chan struct{}
}

func NewMockClientTransport(in io.Reader, out io.Writer) *MockClientTransport {
	return &MockClientTransport{
		in:              in,
		out:             out,
		logger:          pkg.DefaultLogger,
		receiveShutDone: make(chan struct{}),
	}
}

func (t *MockClientTransport) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	go func() {
		defer pkg.Recover()

		t.receive(ctx)
	}()

	return nil
}

func (t *MockClientTransport) Send(_ context.Context, msg Message) error {
	if _, err := t.out.Write(append(msg, mcpMessageDelimiter)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func (t *MockClientTransport) SetReceiver(receiver ClientReceiver) {
	t.receiver = receiver
}

func (t *MockClientTransport) Close() error {
	t.cancel()

	return nil
}

func (t *MockClientTransport) receive(ctx context.Context) {
	s := bufio.NewScanner(t.in)

	for s.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			if err := t.receiver.Receive(ctx, s.Bytes()); err != nil {
				t.logger.Errorf("receiver failed: %v", err)
				return
			}
		}
	}

	if err := s.Err(); err != nil {
		t.logger.Errorf("unexpected error reading input: %v", err)
		return
	}
}

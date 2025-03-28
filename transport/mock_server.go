package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"go-mcp/pkg"
)

const mockSessionID = "mock"

type MockServerTransport struct {
	receiver ServerReceiver
	manager  pkg.TransportSessionManager

	in  io.Reader
	out io.Writer

	logger pkg.Logger

	cancel context.CancelFunc
}

func NewMockServerTransport(in io.Reader, out io.Writer) ServerTransport {
	return &MockServerTransport{
		in:     in,
		out:    out,
		logger: pkg.DefaultLogger,
	}
}

func (t *MockServerTransport) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	go func() {
		defer pkg.Recover()

		t.receive(ctx)
	}()

	return nil
}

func (t *MockServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	if _, err := t.out.Write(append(msg, "\n"...)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func (t *MockServerTransport) SetReceiver(receiver ServerReceiver) {
	t.receiver = receiver
}

func (t *MockServerTransport) SetSessionManager(manager pkg.TransportSessionManager) {
	t.manager = manager
}

func (t *MockServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	t.cancel()
	return nil
}

func (t *MockServerTransport) receive(ctx context.Context) {
	s := bufio.NewScanner(t.in)

	for s.Scan() {
		if err := t.receiver.Receive(ctx, mockSessionID, s.Bytes()); err != nil {
			t.logger.Errorf("receiver failed: %v", err)
			return
		}
	}

	if err := s.Err(); err != nil {
		if err != io.EOF {
			t.logger.Errorf("unexpected error reading input: %v", err)
		}
		return
	}
}

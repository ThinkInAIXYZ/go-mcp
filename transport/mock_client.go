package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type mockClientTransport struct {
	receiver clientReceiver
	in       io.ReadCloser
	out      io.Writer

	logger pkg.Logger

	cancel          context.CancelFunc
	receiveShutDone chan struct{}
}

func NewMockClientTransport(in io.ReadCloser, out io.Writer) ClientTransport {
	return &mockClientTransport{
		in:              in,
		out:             out,
		logger:          pkg.DefaultLogger,
		receiveShutDone: make(chan struct{}),
	}
}

func (t *mockClientTransport) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	go func() {
		defer pkg.Recover()

		t.startReceive(ctx)

		close(t.receiveShutDone)
	}()

	return nil
}

func (t *mockClientTransport) Send(_ context.Context, msg Message) error {
	if _, err := t.out.Write(append(msg, mcpMessageDelimiter)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func (t *mockClientTransport) SetReceiver(receiver clientReceiver) {
	t.receiver = receiver
}

func (t *mockClientTransport) Close() error {
	t.cancel()

	if err := t.in.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	<-t.receiveShutDone

	return nil
}

func (t *mockClientTransport) startReceive(ctx context.Context) {
	s := bufio.NewReader(t.in)

	for {
		line, err := s.ReadBytes('\n')
		if err != nil {
			t.receiver.Interrupt(fmt.Errorf("reader read error: %w", err))

			if errors.Is(err, io.ErrClosedPipe) || // This error occurs during unit tests, suppressing it here
				errors.Is(err, io.EOF) {
				return
			}
			t.logger.Errorf("reader read error: %+v", err)
			return
		}

		line = bytes.TrimRight(line, "\n")

		select {
		case <-ctx.Done():
			return
		default:
			if err = t.receiver.Receive(ctx, line); err != nil {
				t.logger.Errorf("receiver failed: %v", err)
			}
		}
	}
}

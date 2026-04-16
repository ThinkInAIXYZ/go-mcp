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

type mockServerTransport struct {
	receiver serverReceiver
	in       io.ReadCloser
	out      io.Writer

	sessionID string

	sessionManager sessionManager

	logger pkg.Logger

	cancel          context.CancelFunc
	receiveShutDone chan struct{}
}

func NewMockServerTransport(in io.ReadCloser, out io.Writer) ServerTransport {
	return &mockServerTransport{
		in:     in,
		out:    out,
		logger: pkg.DefaultLogger,

		receiveShutDone: make(chan struct{}),
	}
}

func (t *mockServerTransport) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	t.sessionID = t.sessionManager.CreateSession(context.Background())

	t.startReceive(ctx)

	close(t.receiveShutDone)
	return nil
}

func (t *mockServerTransport) Send(_ context.Context, _ string, msg Message) error {
	if _, err := t.out.Write(append(msg, mcpMessageDelimiter)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func (t *mockServerTransport) SetReceiver(receiver serverReceiver) {
	t.receiver = receiver
}

func (t *mockServerTransport) SetSessionManager(m sessionManager) {
	t.sessionManager = m
}

func (t *mockServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	t.cancel()

	if err := t.in.Close(); err != nil {
		return err
	}

	<-t.receiveShutDone

	select {
	case <-serverCtx.Done():
		return nil
	case <-userCtx.Done():
		return userCtx.Err()
	}
}

func (t *mockServerTransport) startReceive(ctx context.Context) {
	s := bufio.NewReader(t.in)

	for {
		line, err := s.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.ErrClosedPipe) || // This error occurs during unit tests, suppressing it here
				errors.Is(err, io.EOF) {
				return
			}
			t.logger.Errorf("client receive unexpected error reading input: %v", err)
			return
		}

		line = bytes.TrimRight(line, "\n")

		select {
		case <-ctx.Done():
			return
		default:
			t.receive(ctx, line)
		}
	}
}

func (t *mockServerTransport) receive(ctx context.Context, line []byte) {
	outputMsgCh, err := t.receiver.Receive(ctx, t.sessionID, line)
	if err != nil {
		t.logger.Errorf("receiver failed: %v", err)
		return
	}

	if outputMsgCh == nil {
		return
	}

	go func() {
		defer pkg.Recover()

		for msg := range outputMsgCh {
			if e := t.Send(context.Background(), t.sessionID, msg); e != nil {
				t.logger.Errorf("Failed to send message: %v", e)
			}
		}
	}()
}

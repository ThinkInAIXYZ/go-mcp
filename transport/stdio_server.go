package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type StdioServerTransportOption func(*stdioServerTransport)

func WithStdioServerOptionLogger(log pkg.Logger) StdioServerTransportOption {
	return func(t *stdioServerTransport) {
		t.logger = log
	}
}

type stdioServerTransport struct {
	receiver serverReceiver
	reader   io.ReadCloser
	writer   io.Writer

	sessionManager sessionManager
	sessionID      string

	logger pkg.Logger

	cancel          context.CancelFunc
	receiveShutDone chan struct{}
}

func NewStdioServerTransport(opts ...StdioServerTransportOption) ServerTransport {
	t := &stdioServerTransport{
		reader: os.Stdin,
		writer: os.Stdout,
		logger: pkg.DefaultLogger,

		receiveShutDone: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *stdioServerTransport) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	t.sessionID = t.sessionManager.CreateSession(context.Background())

	t.startReceive(ctx)

	close(t.receiveShutDone)
	return nil
}

func (t *stdioServerTransport) Send(_ context.Context, _ string, msg Message) error {
	if _, err := t.writer.Write(append(msg, mcpMessageDelimiter)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func (t *stdioServerTransport) SetReceiver(receiver serverReceiver) {
	t.receiver = receiver
}

func (t *stdioServerTransport) SetSessionManager(m sessionManager) {
	t.sessionManager = m
}

func (t *stdioServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	t.cancel()

	if err := t.reader.Close(); err != nil {
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

func (t *stdioServerTransport) startReceive(ctx context.Context) {
	s := bufio.NewReader(t.reader)

	for {
		line, err := s.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.ErrClosedPipe) || // This error occurs during unit tests, suppressing it here
				errors.Is(err, io.EOF) {
				return
			}
			t.logger.Errorf("client receive unexpected error reading input: %v", err)
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

func (t *stdioServerTransport) receive(ctx context.Context, line []byte) {
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

package transport

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"

	"go-mcp/pkg"
)

const stdioSessionID = "stdio"

type stdioServerTransport struct {
	receiver ServerReceiver
	manager  pkg.TransportSessionManager
	reader   *bufio.Reader
	writer   io.Writer

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func NewStdioServerTransport() ServerTransport {
	return NewStdioServerTransportWithIO(os.Stdin, os.Stdout)
}

func NewStdioServerTransportWithIO(in io.Reader, out io.Writer) ServerTransport {
	return &stdioServerTransport{
		reader: bufio.NewReader(in),
		writer: out,

		done: make(chan struct{}),
	}
}

func (t *stdioServerTransport) Run() error {
	t.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		t.cancel = cancel

		go func() {
			defer pkg.Recover()

			t.receive(ctx)
			close(t.done)
		}()
	})

	return nil
}

func (t *stdioServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	msg = append(msg, mcpMessageDelimiter)

	_, err := t.writer.Write(msg)
	return err
}

func (t *stdioServerTransport) SetReceiver(receiver ServerReceiver) {
	t.receiver = receiver
}

func (t *stdioServerTransport) SetSessionManager(manager pkg.TransportSessionManager) {
	t.manager = manager
}

func (t *stdioServerTransport) Shutdown(ctx context.Context, serverCtx context.Context) error {
	if t.cancel == nil {
		return nil
	}

	t.cancel()

	select {
	case <-t.done:
		return nil
	case <-serverCtx.Done():
		return serverCtx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *stdioServerTransport) receive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := t.reader.ReadBytes(mcpMessageDelimiter)
			if err != nil {
				if err != io.EOF {
					// todo: handler error
				}
				return
			}

			t.receiver.Receive(ctx, stdioSessionID, line)
		}
	}
}

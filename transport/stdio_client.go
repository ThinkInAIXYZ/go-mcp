package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type StdioClientTransportOption func(*stdioClientTransport)

func WithStdioClientOptionLogger(log pkg.Logger) StdioClientTransportOption {
	return func(t *stdioClientTransport) {
		t.logger = log
	}
}

func WithStdioClientOptionEnv(env ...string) StdioClientTransportOption {
	return func(t *stdioClientTransport) {
		t.cmd.Env = append(t.cmd.Env, env...)
	}
}

const mcpMessageDelimiter = '\n'

type stdioClientTransport struct {
	cmd       *exec.Cmd
	receiver  clientReceiver
	reader    io.Reader
	writer    io.WriteCloser
	errReader io.Reader

	logger pkg.Logger

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewStdioClientTransport(command string, args []string, opts ...StdioClientTransportOption) (ClientTransport, error) {
	cmd := exec.Command(command, args...)

	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	t := &stdioClientTransport{
		cmd:       cmd,
		reader:    stdout,
		writer:    stdin,
		errReader: stderr,

		logger: pkg.DefaultLogger,
	}

	for _, opt := range opts {
		opt(t)
	}
	return t, nil
}

func (t *stdioClientTransport) Start() error {
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	innerCtx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	t.wg.Add(1)
	go func() {
		defer pkg.Recover()
		defer t.wg.Done()

		t.startReceive(innerCtx)
	}()

	t.wg.Add(1)
	go func() {
		defer pkg.Recover()
		defer t.wg.Done()

		t.startReceiveErr(innerCtx)
	}()

	return nil
}

func (t *stdioClientTransport) Send(_ context.Context, msg Message) error {
	_, err := t.writer.Write(append(msg, mcpMessageDelimiter))
	return err
}

func (t *stdioClientTransport) SetReceiver(receiver clientReceiver) {
	t.receiver = receiver
}

func (t *stdioClientTransport) Close() error {
	t.cancel()

	if err := t.writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	if err := t.cmd.Wait(); err != nil {
		return err
	}

	t.wg.Wait()

	return nil
}

func (t *stdioClientTransport) startReceive(ctx context.Context) {
	s := bufio.NewReader(t.reader)

	for {
		line, err := s.ReadBytes('\n')
		if err != nil {
			t.receiver.Interrupt(fmt.Errorf("stdout read error: %w", err))

			if errors.Is(err, io.ErrClosedPipe) || // This error occurs during unit tests, suppressing it here
				errors.Is(err, io.EOF) {
				return
			}
			t.logger.Errorf("stdout read error: %+v", err)
			return
		}

		line = bytes.TrimRight(line, "\n")
		// filter empty messages
		// filter space messages and \t messages
		if len(bytes.TrimFunc(line, func(r rune) bool { return r == ' ' || r == '\t' })) == 0 {
			t.logger.Debugf("skipping empty message")
			continue
		}

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

func (t *stdioClientTransport) startReceiveErr(ctx context.Context) {
	s := bufio.NewReader(t.errReader)

	for {
		line, err := s.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.ErrClosedPipe) || // This error occurs during unit tests, suppressing it here
				errors.Is(err, io.EOF) {
				return
			}
			t.logger.Errorf("client receive unexpected server error reading input: %v", err)
			return
		}

		line = bytes.TrimRight(line, "\n")
		// filter empty messages
		// filter space messages and \t messages
		if len(bytes.TrimFunc(line, func(r rune) bool { return r == ' ' || r == '\t' })) == 0 {
			t.logger.Debugf("skipping empty message")
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
			t.logger.Errorf("receive server error: %s", line)
		}
	}
}

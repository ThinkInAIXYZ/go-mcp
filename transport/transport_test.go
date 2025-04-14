package transport

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testLogger used for test
type testLogger struct {
	errorCapture *string
}

func newTestLogger() *testLogger {
	var errorLogCapture string
	return &testLogger{
		errorCapture: &errorLogCapture,
	}
}

func (t *testLogger) Debugf(format string, a ...any) {
	log.Printf("[testLogger Debug] "+format+"\n", a...)
}

func (t *testLogger) Infof(format string, a ...any) {
	log.Printf("[testLogger Info] "+format+"\n", a...)
}

func (t *testLogger) Warnf(format string, a ...any) {
	log.Printf("[testLogger Warn] "+format+"\n", a...)
}

func (t *testLogger) Errorf(format string, a ...any) {
	*t.errorCapture = fmt.Sprintf(format, a...)
}

// Error writer for testing
type errorWriter struct{}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return len(p), nil // Write succeeds, but close will fail
}

func (w *errorWriter) Close() error {
	return fmt.Errorf("mock writer close error")
}

// Error reader for testing
type errorReader struct{}

func (s *errorReader) Read(_ []byte) (n int, err error) {
	// not ErrClosedPipe
	return 0, fmt.Errorf("mock reader read error")
}

func (s *errorReader) Close() error {
	return nil
}

var (
	serverReceiveEmpty = func(_ context.Context, _ string, _ []byte) error { return nil }
	clientReceiveEmpty = func(_ context.Context, _ []byte) error { return nil }
	clientReceiveError = func(_ context.Context, _ []byte) error { return fmt.Errorf("receiver error") }
)

type serverReceive func(ctx context.Context, sessionID string, msg []byte) error

func (r serverReceive) Receive(ctx context.Context, sessionID string, msg []byte) error {
	return r(ctx, sessionID, msg)
}

type clientReceive func(ctx context.Context, msg []byte) error

func (r clientReceive) Receive(ctx context.Context, msg []byte) error {
	return r(ctx, msg)
}

func testTransport(t *testing.T, client ClientTransport, server ServerTransport) {
	msgWithServer := "hello"
	expectedMsgWithServerCh := make(chan string, 1)
	server.SetReceiver(serverReceive(func(_ context.Context, _ string, msg []byte) error {
		expectedMsgWithServerCh <- string(msg)
		return nil
	}))

	msgWithClient := "hello"
	expectedMsgWithClientCh := make(chan string, 1)
	client.SetReceiver(clientReceive(func(_ context.Context, msg []byte) error {
		expectedMsgWithClientCh <- string(msg)
		return nil
	}))

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run()
	}()

	// Use select to handle potential errors
	select {
	case err := <-errCh:
		t.Fatalf("server.Run() failed: %v", err)
	case <-time.After(time.Second):
		// Server started normally
	}

	defer func() {
		if _, ok := server.(*stdioServerTransport); ok { // stdioServerTransport not support shutdown
			return
		}

		userCtx, cancel := context.WithTimeout(context.Background(), time.Second*1)
		defer cancel()

		serverCtx, cancel := context.WithCancel(userCtx)
		cancel()

		if err := server.Shutdown(userCtx, serverCtx); err != nil {
			t.Fatalf("server.Shutdown() failed: %v", err)
		}
	}()

	if err := client.Start(); err != nil {
		t.Fatalf("client.Run() failed: %v", err)
	}

	defer func() {
		if err := client.Close(); err != nil {
			t.Fatalf("client.Close() failed: %v", err)
		}
	}()

	if err := client.Send(context.Background(), Message(msgWithServer)); err != nil {
		t.Fatalf("client.Send() failed: %v", err)
	}
	assert.Equal(t, <-expectedMsgWithServerCh, msgWithServer)

	sessionID := ""
	if cli, ok := client.(*sseClientTransport); ok {
		sessionID = cli.messageEndpoint.Query().Get("sessionID")
	}

	if err := server.Send(context.Background(), sessionID, Message(msgWithClient)); err != nil {
		t.Fatalf("server.Send() failed: %v", err)
	}
	assert.Equal(t, <-expectedMsgWithClientCh, msgWithClient)
}

func TestClientReceiverF(t *testing.T) {
	called := false
	expectedMsg := []byte("this is a message from ThinkInAI team!")

	receiverFunc := ClientReceiverF(func(_ context.Context, msg []byte) error {
		called = true
		assert.Equal(t, expectedMsg, msg)
		return nil
	})

	err := receiverFunc.Receive(context.Background(), expectedMsg)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestServerReceiverF(t *testing.T) {
	called := false
	expectedMsg := []byte("this is a message from ThinkInAI team!")
	expectedSessionID := "test-session"

	receiverFunc := ServerReceiverF(func(_ context.Context, sessionID string, msg []byte) error {
		called = true
		assert.Equal(t, expectedSessionID, sessionID)
		assert.Equal(t, expectedMsg, msg)
		return nil
	})

	err := receiverFunc.Receive(context.Background(), expectedSessionID, expectedMsg)
	assert.NoError(t, err)
	assert.True(t, called)
}

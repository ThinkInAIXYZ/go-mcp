package transport

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMockTransport(t *testing.T) {
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	serverTransport := NewMockServerTransport(reader2, writer1)
	clientTransport := NewMockClientTransport(reader1, writer2)

	testTransport(t, clientTransport, serverTransport)
}

// TestMockClientReceiverError tests the error handling when ClientReceiver.Receive returns an error
func TestMockClientReceiverError(t *testing.T) {
	// Create a pipe for simulating bidirectional communication
	reader, writer := io.Pipe()

	// Create MockClientTransport instance
	clientTransport := NewMockClientTransport(reader, writer)

	// Create a variable to capture error logs for verification
	var errorMsg string

	// Set a custom logger to capture error logs
	customLogger := newTestLogger()
	customLogger.errorCapture = &errorMsg
	clientTransport.logger = customLogger

	// Set a receiver that will return an error
	receiverCalled := make(chan struct{}, 1)
	expectedError := fmt.Errorf("test receiver error")

	clientTransport.SetReceiver(ClientReceiverF(func(_ context.Context, _ []byte) error {
		receiverCalled <- struct{}{}
		return expectedError // Return the expected error
	}))

	// Start the transport
	err := clientTransport.Start()
	assert.NoError(t, err)

	// Wait for transport to start
	time.Sleep(100 * time.Millisecond)

	// Send a message to the pipe, which will trigger the receive method processing
	testMessage := []byte("test message")
	_, err = writer.Write(append(testMessage, '\n'))
	assert.NoError(t, err)

	// Verify that the receiver was called
	select {
	case <-receiverCalled:
		// Receiver was called as expected
	case <-time.After(time.Second):
		t.Fatal("Receiver was not called within expected time")
	}

	// Wait for the error log to be recorded
	time.Sleep(100 * time.Millisecond)

	// Verify that the error was logged correctly
	assert.Contains(t, errorMsg, "receiver failed")
	assert.Contains(t, errorMsg, expectedError.Error())

	// Close the transport
	err = clientTransport.Close()
	assert.NoError(t, err)

	// Clean up resources
	err = writer.Close()
	assert.NoError(t, err)
	err = reader.Close()
	assert.NoError(t, err)
}

// Test receive function error handling
func TestMockClientReceiveError(_ *testing.T) {
	errReader1, errWriter1 := &errorReader{}, &errorWriter{}
	clientTransport := NewMockClientTransport(errReader1, errWriter1)

	// Set a receiver that will report an error
	clientTransport.SetReceiver(ClientReceiverF(func(_ context.Context, _ []byte) error {
		return fmt.Errorf("receiver error")
	}))

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the receive function
	go func() {
		clientTransport.receive(ctx)
		close(clientTransport.receiveShutDone)
	}()

	// Wait for processing to complete
	<-clientTransport.receiveShutDone
}

// Test TestMockServerCancelByUserCtx Shutdown when user ctx cancel
func TestMockServerCancelByUserCtx(t *testing.T) {
	reader, writer := io.Pipe()
	server := NewMockServerTransport(reader, writer)

	go func() {
		err := server.Run()
		assert.NoError(t, err)
	}()

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Create a canceled context
	userCtx, userCancel := context.WithCancel(context.Background())
	userCancel()

	serverCtx, serverCancel := context.WithCancel(context.Background())

	// Don't trigger serverCtx cancellation, let it wait for user ctx timeout
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = writer.Close()
	}()

	err := server.Shutdown(userCtx, serverCtx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	// Cleanup
	serverCancel()
}

// Test MockServerTransport when the receiver returns an error
func TestMockServerReceiveError(t *testing.T) {
	reader, writer := io.Pipe()
	server := NewMockServerTransport(reader, writer)

	// Set a receiver that will report an error when receiving data
	receiverCalled := make(chan struct{})
	server.SetReceiver(ServerReceiverF(func(_ context.Context, _ string, _ []byte) error {
		close(receiverCalled)
		return fmt.Errorf("server receiver error")
	}))

	// Start the server in a goroutine
	done := make(chan struct{})
	go func() {
		err := server.Run()
		assert.NoError(t, err)
		close(done)
	}()

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Write a message to the pipe, which will trigger the receive function
	// The message should be sent to the receiver, which will return an error
	_, err := writer.Write([]byte("test message\n"))
	assert.NoError(t, err)

	// Wait for the receiver to be called
	select {
	case <-receiverCalled:
		// Receiver was called, which is what we expect
	case <-time.After(time.Second):
		t.Fatal("Receiver was not called within expected time")
	}

	// The server should continue running despite the receiver error
	select {
	case <-done:
		t.Fatal("Server stopped unexpectedly")
	case <-time.After(100 * time.Millisecond):
		// Server is still running, which is the expected behavior
	}

	// Clean up the server
	userCtx, userCancel := context.WithTimeout(context.Background(), time.Second)
	defer userCancel()

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	// Close the writer to cause the reader to get EOF
	_ = writer.Close()

	// Shutdown should complete without errors
	err = server.Shutdown(userCtx, serverCtx)
	assert.NoError(t, err)
}

// Test MockServerTransport receive not ErrClosedPipe but else
func TestMockServerReceiveNonErrClosedPipe(t *testing.T) {
	server := NewMockServerTransport(&errorReader{}, &errorWriter{})
	server.SetReceiver(ServerReceiverF(serverReceiveEmpty))

	// start server
	go func() {
		err := server.Run()
		assert.NoError(t, err)
	}()

	time.Sleep(100 * time.Millisecond)

	userCtx, userCancel := context.WithTimeout(context.Background(), time.Second)
	defer userCancel()

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	err := server.Shutdown(userCtx, serverCtx)
	assert.NoError(t, err)
}

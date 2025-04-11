package transport

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
)

func TestMockTransport(t *testing.T) {
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	serverTransport := NewMockServerTransport(reader2, writer1)
	clientTransport := NewMockClientTransport(reader1, writer2)

	testTransport(t, clientTransport, serverTransport)
}

// Test receive function error handling
func TestMockClientReceiveError(t *testing.T) {
	
	errReader1, errWriter1 := &errorReader{}, &errorWriter{}
	clientTransport := NewMockClientTransport(errReader1, errWriter1)

	//Set a receiver that will report an error
	clientTransport.SetReceiver(ClientReceiverF(func(ctx context.Context, msg []byte) error {
		return fmt.Errorf("receiver error")
	}))

	//Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the receive function
	go func() {
		clientTransport.receive(ctx)
		close(clientTransport.receiveShutDone)
	}()

	//Wait for processing to complete
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

	// Create a cancelled context
	canceledCtx, cancelFn := context.WithCancel(context.Background())
	cancelFn()

	serverCtx, serverCancel := context.WithCancel(context.Background())

	// Don't trigger serverCtx cancellation, let it wait for user ctx timeout
	go func() {
		time.Sleep(50 * time.Millisecond)
		writer.Close()
	}()

	err := server.Shutdown(canceledCtx, serverCtx)
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
	server.SetReceiver(ServerReceiverF(func(ctx context.Context, sessionID string, msg []byte) error {
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
	userCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	// Close the writer to cause the reader to get EOF
	writer.Close()

	// Shutdown should complete without errors
	err = server.Shutdown(userCtx, serverCtx)
	assert.NoError(t, err)
}

// Test MockServerTransport receive not ErrClosedPipe but else
func TestMockServerReceiveNonErrClosedPipe(t *testing.T) {

	server := NewMockServerTransport(&errorReader{}, &errorWriter{})

	server.SetReceiver(ServerReceiverF(func(ctx context.Context, sessionID string, msg []byte) error {
		return nil
	}))

	// start server
	go func() {
		err := server.Run()
		assert.NoError(t, err)
	}()

	// 给一点时间让错误发生
	time.Sleep(100 * time.Millisecond)

	// 关闭服务器
	userCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	err := server.Shutdown(userCtx, serverCtx)
	assert.NoError(t, err)
}

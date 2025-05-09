package transport

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type mock struct {
	reader    *io.PipeReader
	readerr   *io.PipeReader
	writer    *io.PipeWriter
	writerror *io.PipeWriter
	closer    io.Closer
}

func (m *mock) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

func (m *mock) Close() error {
	if err := m.writer.Close(); err != nil {
		return err
	}
	if err := m.reader.Close(); err != nil {
		return err
	}
	if err := m.readerr.Close(); err != nil {
		return err
	}
	if err := m.closer.Close(); err != nil {
		return err
	}
	if err := m.writerror.Close(); err != nil {
		return err
	}
	return nil
}

func TestStdioTransport(t *testing.T) {
	var (
		err    error
		server *stdioServerTransport
		client *stdioClientTransport
	)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	mockServerTrPath := filepath.Join(os.TempDir(), "mock_server_tr_"+strconv.Itoa(r.Int()))
	if err = compileMockStdioServerTr(mockServerTrPath); err != nil {
		t.Fatalf("Failed to compile mock server: %v", err)
	}

	defer func(name string) {
		if err = os.Remove(name); err != nil {
			fmt.Printf("Failed to remove mock server: %v\n", err)
		}
	}(mockServerTrPath)

	clientT, err := NewStdioClientTransport(mockServerTrPath, []string{})
	if err != nil {
		t.Fatalf("NewStdioClientTransport failed: %v", err)
	}

	client = clientT.(*stdioClientTransport)
	server = NewStdioServerTransport().(*stdioServerTransport)

	// Create pipes for communication
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()
	readerror, writerror := io.Pipe()

	// Set up the communication channels
	server.reader = reader2
	server.writer = writer1
	server.writerror = writerror
	client.reader = reader1
	client.readerr = readerror
	client.writer = &mock{
		reader:    reader1,
		readerr:   readerror,
		writer:    writer2,
		writerror: writerror,
		closer:    client.writer,
	}

	expectedErrorWithClientCh := make(chan string, 1)

	go func() {
		buf := make([]byte, 1024)
		n, err := client.readerr.Read(buf)
		if err != nil && err != io.EOF {
			expectedErrorWithClientCh <- fmt.Sprintf("Failed to read from server: %v", err)
			return
		}
		expectedErrorWithClientCh <- string(buf[:n])
	}()

	writeErrorDoneCh := make(chan error, 1)
	go func() {
		errorMsg := "server error"
		_, err := server.writerror.Write([]byte(errorMsg))
		if err != nil {
			writeErrorDoneCh <- fmt.Errorf("failed to write error message: %v", err)
			return
		}
		if err := server.writerror.Close(); err != nil {
			writeErrorDoneCh <- fmt.Errorf("failed to close writerror: %v", err)
			return
		}
		writeErrorDoneCh <- nil
	}()

	select {
	case receivedError := <-expectedErrorWithClientCh:
		expectedError := "server error"
		if receivedError != expectedError {
			t.Fatalf("stderr mismatch: got %q, want %q", receivedError, expectedError)
		} else {
			fmt.Printf("Received expected error message: %q\n", receivedError)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for client to read error message")
	}

	select {
	case err := <-writeErrorDoneCh:
		if err != nil {
			t.Fatalf("writing to server writerror failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for server to write error message")
	}

	testTransport(t, client, server)
}

func compileMockStdioServerTr(outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath, "../testdata/mock_block_server.go")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compilation failed: %v\nOutput: %s", err, output)
	}

	return nil
}

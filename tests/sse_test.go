package tests

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func TestSSE(t *testing.T) {
	port, err := getAvailablePort()
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}

	transportClient, err := transport.NewSSEClientTransport(fmt.Sprintf("http://127.0.0.1:%d/sse", port))
	if err != nil {
		t.Fatalf("Failed to create transport client: %v", err)
	}

	test(t, func() error { return runSSEServer(port) }, transportClient, transport.Stateful)
}

// getAvailablePort returns a port that is available for use
func getAvailablePort() (int, error) {
	addr, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to get available port: %v", err)
	}
	defer func() {
		if err = addr.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	port := addr.Addr().(*net.TCPAddr).Port
	return port, nil
}

func runSSEServer(port int) error {
	mockServerTrPath, err := compileMockStdioServerTr()
	if err != nil {
		return err
	}
	fmt.Println(mockServerTrPath)

	defer func(name string) {
		if err := os.Remove(name); err != nil {
			fmt.Printf("failed to remove mock server: %v\n", err)
		}
	}(mockServerTrPath)

	return exec.Command(mockServerTrPath, "-transport", "sse", "-port", strconv.Itoa(port)).Run()
}

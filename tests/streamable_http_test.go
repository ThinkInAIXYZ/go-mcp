package tests

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func TestStreamableHTTPWithStateless(t *testing.T) {
	port, err := getAvailablePort()
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}

	transportClient, err := transport.NewStreamableHTTPClientTransport(fmt.Sprintf("http://127.0.0.1:%d/mcp", port))
	if err != nil {
		t.Fatalf("Failed to create transport client: %v", err)
	}

	test(t, func() error { return runStreamableHTTPServer(port, transport.Stateless) }, transportClient, transport.Stateless)
}

func TestStreamableHTTPWithStateful(t *testing.T) {
	port, err := getAvailablePort()
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}

	transportClient, err := transport.NewStreamableHTTPClientTransport(fmt.Sprintf("http://127.0.0.1:%d/mcp", port))
	if err != nil {
		t.Fatalf("Failed to create transport client: %v", err)
	}

	test(t, func() error { return runStreamableHTTPServer(port, transport.Stateful) }, transportClient, transport.Stateful)
}

func runStreamableHTTPServer(port int, stateful transport.StateMode) error {
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

	return exec.Command(mockServerTrPath, "-transport", "streamable_http", "-port", strconv.Itoa(port), "-state_mode", string(stateful)).Run()
}

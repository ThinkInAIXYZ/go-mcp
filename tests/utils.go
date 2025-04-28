package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func test(t *testing.T, runServer func() error, transportClient transport.ClientTransport, mode transport.StateMode) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- runServer()
	}()

	// Use select to handle potential errors
	select {
	case err := <-errCh:
		t.Fatalf("server.Run() failed: %v", err)
	case <-time.After(time.Second * 3):
		// Server started normally
	}

	// Create MCP client using transport
	mcpClient, err := client.NewClient(transportClient, client.WithClientInfo(protocol.Implementation{
		Name:    "Example MCP Client",
		Version: "1.0.0",
	}), client.WithSamplingHandler(&sampling{}))
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	defer func() {
		if err = mcpClient.Close(); err != nil {
			t.Fatalf("Failed to close MCP client: %v", err)
			return
		}
	}()

	// List available tools
	toolsResult, err := mcpClient.ListTools(context.Background())
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}
	bytes, _ := json.Marshal(toolsResult)
	fmt.Printf("Available tools: %s\n", bytes)

	callResult, err := mcpClient.CallTool(
		context.Background(),
		protocol.NewCallToolRequestWithRawArguments("current_time", json.RawMessage(`{"timezone": "UTC"}`)))
	if err != nil {
		t.Fatalf("Failed to call tool: %v", err)
	}
	bytes, _ = json.Marshal(callResult)
	fmt.Printf("Tool call result: %s\n", bytes)

	if mode == transport.Stateful {
		// if streamable_http transport, need wait streamable_http connection start
		time.Sleep(time.Second)

		callResult, err = mcpClient.CallTool(
			context.Background(),
			protocol.NewCallToolRequestWithRawArguments("delete_file", json.RawMessage(`{"file_name": "test_file.txt"}`)))
		if err != nil {
			t.Fatalf("Failed to call tool: %v", err)
		}
		bytes, _ = json.Marshal(callResult)
		fmt.Printf("Tool call result: %s\n", bytes)
	}
}

type sampling struct{}

func (s *sampling) CreateMessage(_ context.Context, request *protocol.CreateMessageRequest) (*protocol.CreateMessageResult, error) {
	var lastUserMessages protocol.Content
	for _, message := range request.Messages {
		if message.Role == "user" {
			lastUserMessages = message.Content
		}
	}

	if lastUserMessages.GetType() != "text" {
		return nil, fmt.Errorf("expected 'text', got %s", lastUserMessages.GetType())
	}

	return &protocol.CreateMessageResult{
		Content: &protocol.TextContent{
			Annotated: protocol.Annotated{},
			Type:      "text",
			Text:      strconv.FormatBool(true),
		},
		Role:       "assistant",
		Model:      "stub-model",
		StopReason: "endTurn",
	}, nil
}

func compileMockStdioServerTr() (string, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	mockServerTrPath := filepath.Join(os.TempDir(), "mock_server_tr_"+strconv.Itoa(r.Int()))

	cmd := exec.Command("go", "build", "-o", mockServerTrPath, "../examples/everything/main.go")

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("compilation failed: %v\nOutput: %s", err, output)
	}

	return mockServerTrPath, nil
}

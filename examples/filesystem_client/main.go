package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func main() {
	t, err := transport.NewStdioClientTransport("npx", []string{"-y", "@modelcontextprotocol/server-filesystem", "~/tmp"})
	if err != nil {
		log.Fatal(err)
	}

	cli, err := client.NewClient(t, client.WithClientInfo(protocol.Implementation{
		Name:    "test",
		Version: "1.0.0",
	}))
	if err != nil {
		log.Fatalf("Failed to new client: %v", err)
	}
	defer func() {
		if err = cli.Close(); err != nil {
			log.Fatalf("Failed to close client: %v", err)
		}
	}()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List Tools
	log.Println("Listing available tools...")
	tools, err := cli.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}
	for _, tool := range tools.Tools {
		log.Printf("- %s: %s\n", tool.Name, tool.Description)
	}

	// List allowed directories
	log.Println("Listing allowed directories...")
	listDirRequest := &protocol.CallToolRequest{
		Name: "list_allowed_directories",
	}
	result, err := cli.CallTool(ctx, listDirRequest)
	if err != nil {
		log.Fatalf("Failed to list allowed directories: %v", err)
	}
	printToolResult(result)
	log.Println()

	// List ~/tmp
	log.Println("Listing ~/tmp directory...")
	listTmpRequest := &protocol.CallToolRequest{
		Name:      "list_directory",
		Arguments: map[string]interface{}{"path": "~/tmp"},
	}
	result, err = cli.CallTool(ctx, listTmpRequest)
	if err != nil {
		log.Fatalf("Failed to list directory: %v", err)
	}
	printToolResult(result)
	log.Println()

	// Create mcp directory
	log.Println("Creating ~/tmp/mcp directory...")
	createDirRequest := &protocol.CallToolRequest{
		Name:      "create_directory",
		Arguments: map[string]interface{}{"path": "~/tmp/mcp"},
	}
	result, err = cli.CallTool(ctx, createDirRequest)
	if err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}
	printToolResult(result)
	log.Println()

	// Create hello.txt
	log.Println("Creating ~/tmp/mcp/hello.txt...")
	writeFileRequest := &protocol.CallToolRequest{
		Name: "write_file",
		Arguments: map[string]interface{}{
			"path":    "~/tmp/mcp/hello.txt",
			"content": "Hello World",
		},
	}
	result, err = cli.CallTool(ctx, writeFileRequest)
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}
	printToolResult(result)
	log.Println()

	// Verify file contents
	log.Println("Reading ~/tmp/mcp/hello.txt...")
	readFileRequest := &protocol.CallToolRequest{
		Name: "read_file",
		Arguments: map[string]interface{}{
			"path": "~/tmp/mcp/hello.txt",
		},
	}
	result, err = cli.CallTool(ctx, readFileRequest)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	printToolResult(result)

	// Get file info
	log.Println("Getting info for ~/tmp/mcp/hello.txt...")
	fileInfoRequest := &protocol.CallToolRequest{
		Name: "get_file_info",
		Arguments: map[string]interface{}{
			"path": "~/tmp/mcp/hello.txt",
		},
	}
	result, err = cli.CallTool(ctx, fileInfoRequest)
	if err != nil {
		log.Fatalf("Failed to get file info: %v", err)
	}
	printToolResult(result)
}

// Helper function to print tool results
func printToolResult(result *protocol.CallToolResult) {
	for _, content := range result.Content {
		if textContent, ok := content.(*protocol.TextContent); ok {
			log.Println(textContent.Text)
		} else {
			jsonBytes, _ := json.MarshalIndent(content, "", "  ")
			log.Println(string(jsonBytes))
		}
	}
}

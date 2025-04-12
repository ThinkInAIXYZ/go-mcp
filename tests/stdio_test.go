package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func TestStdio(t *testing.T) {
	mockServerTrPath, err := compileMockStdioServerTr()
	if err != nil {
		t.Fatal(err)
	}
	defer func(name string) {
		if err = os.Remove(name); err != nil {
			fmt.Printf("Failed to remove mock server: %v\n", err)
		}
	}(mockServerTrPath)

	fmt.Println(mockServerTrPath)
	transportClient, err := transport.NewStdioClientTransport(mockServerTrPath, []string{"-transport", "stdio"})
	if err != nil {
		t.Fatalf("Failed to create transport client: %v", err)
	}

	test(t, func() error {
		<-make(chan error)
		return nil
	}, transportClient)
}

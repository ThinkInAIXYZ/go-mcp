package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

// LoggingMiddleware returns a logging middleware
func LoggingMiddleware() server.ToolMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			start := time.Now()
			log.Printf("[Middleware] Tool call started: %s, args: %v", req.Name, req.Arguments)

			result, err := next(ctx, req)

			duration := time.Since(start)
			if err != nil {
				log.Printf("[Middleware] Tool call failed: %s, error: %v, duration: %v", req.Name, err, duration)
			} else {
				log.Printf("[Middleware] Tool call succeeded: %s, duration: %v", req.Name, duration)
			}

			return result, err
		}
	}
}

// AuthMiddleware returns an authentication middleware
func AuthMiddleware() server.ToolMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			if req.Arguments == nil {
				req.Arguments = make(map[string]interface{})
			}

			if authToken, ok := req.Arguments["auth_token"]; !ok || authToken != "valid_token" {
				return nil, fmt.Errorf("unauthorized: invalid or missing auth_token")
			}

			delete(req.Arguments, "auth_token")

			log.Printf("[Middleware] Authentication passed for tool: %s", req.Name)
			return next(ctx, req)
		}
	}
}

// MetricsMiddleware returns a metrics collection middleware
func MetricsMiddleware() server.ToolMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			log.Printf("[Middleware] Collecting metrics for tool: %s", req.Name)

			result, err := next(ctx, req)

			if err != nil {
				log.Printf("[Middleware] Metric: tool=%s, status=error", req.Name)
			} else {
				log.Printf("[Middleware] Metric: tool=%s, status=success", req.Name)
			}

			return result, err
		}
	}
}

// PanicRecoveryMiddleware returns a panic recovery middleware
func PanicRecoveryMiddleware() server.ToolMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Middleware] Recovered from panic in tool %s: %v", req.Name, r)
				}
			}()

			return next(ctx, req)
		}
	}
}

// HelloWorldHandler is a sample tool handler
func HelloWorldHandler(_ context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	name := "World"
	if req.Arguments != nil {
		if n, ok := req.Arguments["name"]; ok {
			name = fmt.Sprintf("%v", n)
		}
	}

	message := fmt.Sprintf("Hello, %s! Tool executed successfully.", name)
	return protocol.NewCallToolResult([]protocol.Content{
		&protocol.TextContent{
			Type: "text",
			Text: message,
		},
	}, false), nil
}

// CounterHandler is another sample tool handler
func CounterHandler(_ context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	count := 1
	if req.Arguments != nil {
		if c, ok := req.Arguments["count"]; ok {
			if cInt, ok := c.(float64); ok { // JSON numbers are float64
				count = int(cInt)
			}
		}
	}

	message := fmt.Sprintf("Counter value: %d", count)
	return protocol.NewCallToolResult([]protocol.Content{
		&protocol.TextContent{
			Type: "text",
			Text: message,
		},
	}, false), nil
}

func main() {
	stdio := transport.NewStdioServerTransport()
	mcpServer, err := server.NewServer(stdio)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	log.Println("Registering global middlewares...")
	mcpServer.Use(
		PanicRecoveryMiddleware(),
		LoggingMiddleware(),
		AuthMiddleware(),
		MetricsMiddleware(),
	)

	log.Println("Registering tools...")

	helloTool := &protocol.Tool{
		Name:        "hello",
		Description: "Say hello to someone",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]*protocol.Property{
				"name": {
					Type:        protocol.String,
					Description: "Name to greet",
				},
				"auth_token": {
					Type:        protocol.String,
					Description: "Authentication token (required)",
				},
			},
			Required: []string{"auth_token"},
		},
	}
	mcpServer.RegisterTool(helloTool, HelloWorldHandler)

	counterTool := &protocol.Tool{
		Name:        "counter",
		Description: "Count something",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]*protocol.Property{
				"count": {
					Type:        protocol.Number,
					Description: "Number to count",
				},
				"auth_token": {
					Type:        protocol.String,
					Description: "Authentication token (required)",
				},
			},
			Required: []string{"auth_token"},
		},
	}
	mcpServer.RegisterTool(counterTool, CounterHandler)

	log.Println("Server starting...")
	log.Println("Try these commands:")
	log.Println(`echo '{"method":"tools/call","params":{"name":"hello","arguments":{"name":"World","auth_token":"valid_token"}}}' | go run examples/middleware_example/main.go`) //nolint:lll

	if err := mcpServer.Run(); err != nil {
		log.Fatal("Server failed:", err)
	}
}

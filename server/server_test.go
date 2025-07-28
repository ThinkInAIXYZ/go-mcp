package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type currentTimeReq struct {
	Timezone string `json:"timezone" description:"current time timezone"`
}

func TestServerHandle(t *testing.T) {
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	var (
		in = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader1,
			writer: writer1,
		}

		out = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader2,
			writer: writer2,
		}

		outScan = bufio.NewScanner(out.reader)
	)

	server, err := NewServer(
		transport.NewMockServerTransport(in.reader, out.writer),
		WithServerInfo(protocol.Implementation{
			Name:    "ExampleServer",
			Version: "1.0.0",
		}))
	if err != nil {
		t.Fatalf("NewServer: %+v", err)
	}

	// add tool
	testTool, err := protocol.NewTool("test_tool", "test_tool", currentTimeReq{})
	if err != nil {
		t.Fatalf("NewTool: %+v", err)
		return
	}

	testToolCallContent := protocol.TextContent{
		Type: "text",
		Text: "pong",
	}
	server.RegisterTool(testTool, func(_ context.Context, _ *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		return &protocol.CallToolResult{
			Content: []protocol.Content{&testToolCallContent},
		}, nil
	})

	// add prompt
	testPrompt := &protocol.Prompt{
		Name:        "test_prompt",
		Description: "test_prompt_description",
		Arguments: []*protocol.PromptArgument{
			{
				Name:        "params1",
				Description: "params1's description",
				Required:    true,
			},
		},
	}
	testPromptGetResponse := &protocol.GetPromptResult{
		Description: "test_prompt_description",
	}
	server.RegisterPrompt(testPrompt, func(context.Context, *protocol.GetPromptRequest) (*protocol.GetPromptResult, error) {
		return testPromptGetResponse, nil
	})

	// add resource
	testResource := &protocol.Resource{
		URI:      "file:///test.txt",
		Name:     "test.txt",
		MimeType: "text/plain-txt",
	}
	testResourceContent := &protocol.TextResourceContents{
		URI:      testResource.URI,
		MimeType: testResource.MimeType,
		Text:     "test",
	}
	server.RegisterResource(testResource, func(context.Context, *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
		return &protocol.ReadResourceResult{
			Contents: []protocol.ResourceContents{
				testResourceContent,
			},
		}, nil
	})

	// add resource template
	testResourceTemplate := &protocol.ResourceTemplate{
		URITemplate: "file:///{path}",
		Name:        "test",
	}
	if err := server.RegisterResourceTemplate(testResourceTemplate, func(context.Context, *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
		return &protocol.ReadResourceResult{
			Contents: []protocol.ResourceContents{
				testResourceContent,
			},
		}, nil
	}); err != nil {
		t.Fatalf("RegisterResourceTemplate: %+v", err)
		return
	}

	go func() {
		if err := server.Run(); err != nil {
			t.Errorf("server start: %+v", err)
		}
	}()

	testServerInit(t, server, in.writer, outScan)

	tests := []struct {
		name             string
		method           protocol.Method
		request          protocol.ClientRequest
		expectedResponse protocol.ServerResponse
	}{
		{
			name:             "test_list_tool",
			method:           protocol.ToolsList,
			request:          protocol.ListToolsRequest{},
			expectedResponse: protocol.ListToolsResult{Tools: []*protocol.Tool{testTool}},
		},
		{
			name:   "test_call_tool",
			method: protocol.ToolsCall,
			request: protocol.CallToolRequest{
				Name: testTool.Name,
			},
			expectedResponse: protocol.CallToolResult{
				Content: []protocol.Content{
					&testToolCallContent,
				},
			},
		},
		{
			name:             "test_ping",
			method:           protocol.Ping,
			request:          protocol.PingRequest{},
			expectedResponse: protocol.PingResult{},
		},
		{
			name:    "test_list_prompt",
			method:  protocol.PromptsList,
			request: protocol.ListPromptsRequest{},
			expectedResponse: protocol.ListPromptsResult{
				Prompts: []*protocol.Prompt{testPrompt},
			},
		},
		{
			name:   "test_get_prompt",
			method: protocol.PromptsGet,
			request: protocol.GetPromptRequest{
				Name: testPrompt.Name,
			},
			expectedResponse: testPromptGetResponse,
		},
		{
			name:    "test_list_resource",
			method:  protocol.ResourcesList,
			request: protocol.ListResourcesRequest{},
			expectedResponse: protocol.ListResourcesResult{
				Resources: []*protocol.Resource{testResource},
			},
		},
		{
			name:   "test_read_resource",
			method: protocol.ResourcesRead,
			request: protocol.ReadResourceRequest{
				URI: testResource.URI,
			},
			expectedResponse: protocol.ReadResourceResult{
				Contents: []protocol.ResourceContents{testResourceContent},
			},
		},
		{
			name:    "test_list_resource_template",
			method:  protocol.ResourceListTemplates,
			request: protocol.ListResourceTemplatesRequest{},
			expectedResponse: protocol.ListResourceTemplatesResult{
				ResourceTemplates: []*protocol.ResourceTemplate{testResourceTemplate},
			},
		},
		{
			name:   "test_resource_subscribe",
			method: protocol.ResourcesSubscribe,
			request: protocol.SubscribeRequest{
				URI: testResource.URI,
			},
			expectedResponse: protocol.SubscribeResult{},
		},
		{
			name:   "test_resource_unsubscribe",
			method: protocol.ResourcesUnsubscribe,
			request: protocol.UnsubscribeRequest{
				URI: testResource.URI,
			},
			expectedResponse: protocol.UnsubscribeResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, _ := uuid.NewUUID()
			req := protocol.NewJSONRPCRequest(uuid, tt.method, tt.request)
			reqBytes, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("json Marshal: %+v", err)
			}
			if _, err = in.writer.Write(append(reqBytes, "\n"...)); err != nil {
				t.Fatalf("in Write: %+v", err)
			}

			var respBytes []byte
			if outScan.Scan() {
				respBytes = outScan.Bytes()
				if outScan.Err() != nil {
					t.Fatalf("outScan: %+v", err)
				}
			}

			var respMap map[string]interface{}
			if err = pkg.JSONUnmarshal(respBytes, &respMap); err != nil {
				t.Fatal(err)
			}

			expectedResp := protocol.NewJSONRPCSuccessResponse(uuid, tt.expectedResponse)
			expectedRespBytes, err := json.Marshal(expectedResp)
			if err != nil {
				t.Fatalf("json Marshal: %+v", err)
			}
			var expectedRespMap map[string]interface{}
			if err := pkg.JSONUnmarshal(expectedRespBytes, &expectedRespMap); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(respMap, expectedRespMap) {
				t.Fatalf("response not as expected.\ngot  = %v\nwant = %v", respMap, expectedRespMap)
			}
		})
	}
}

func TestServerNotify(t *testing.T) {
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	var (
		in = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader1,
			writer: writer1,
		}

		out = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader2,
			writer: writer2,
		}

		outScan = bufio.NewScanner(out.reader)
	)

	server, err := NewServer(
		transport.NewMockServerTransport(in.reader, out.writer),
		WithServerInfo(protocol.Implementation{
			Name:    "ExampleServer",
			Version: "1.0.0",
		}))
	if err != nil {
		t.Fatalf("NewServer: %+v", err)
	}

	// add tool
	testTool, err := protocol.NewTool("test_tool", "test_tool", currentTimeReq{})
	if err != nil {
		t.Fatalf("NewTool: %+v", err)
		return
	}

	testToolCallContent := protocol.TextContent{
		Type: "text",
		Text: "pong",
	}

	// add prompt
	testPrompt := &protocol.Prompt{
		Name:        "test_prompt",
		Description: "test_prompt_description",
		Arguments: []*protocol.PromptArgument{
			{
				Name:        "params1",
				Description: "params1's description",
				Required:    true,
			},
		},
	}
	testPromptGetResponse := &protocol.GetPromptResult{
		Description: "test_prompt_description",
	}

	// add resource
	testResource := &protocol.Resource{
		URI:      "file:///test.txt",
		Name:     "test.txt",
		MimeType: "text/plain-txt",
	}
	testResourceContent := &protocol.TextResourceContents{
		URI:      testResource.URI,
		MimeType: testResource.MimeType,
		Text:     "test",
	}

	// add resource template
	testResourceTemplate := &protocol.ResourceTemplate{
		URITemplate: "file:///{path}",
		Name:        "test",
	}

	go func() {
		if err := server.Run(); err != nil {
			t.Errorf("server start: %+v", err)
		}
	}()

	testServerInit(t, server, in.writer, outScan)

	tests := []struct {
		name           string
		method         protocol.Method
		f              func()
		expectedNotify protocol.ServerResponse
	}{
		{
			name:   "test_tools_changed_notify",
			method: protocol.NotificationToolsListChanged,
			f: func() {
				server.RegisterTool(testTool, func(context.Context, *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
					return &protocol.CallToolResult{
						Content: []protocol.Content{&testToolCallContent},
					}, nil
				})
			},
			expectedNotify: protocol.NewToolListChangedNotification(),
		},
		{
			name:   "test_prompts_changed_notify",
			method: protocol.NotificationPromptsListChanged,
			f: func() {
				server.RegisterPrompt(testPrompt, func(context.Context, *protocol.GetPromptRequest) (*protocol.GetPromptResult, error) {
					return testPromptGetResponse, nil
				})
			},
			expectedNotify: protocol.NewPromptListChangedNotification(),
		},
		{
			name:   "test_resources_changed_notify",
			method: protocol.NotificationResourcesListChanged,
			f: func() {
				server.RegisterResource(testResource, func(context.Context, *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
					return &protocol.ReadResourceResult{
						Contents: []protocol.ResourceContents{
							testResourceContent,
						},
					}, nil
				})
			},
			expectedNotify: protocol.NewResourceListChangedNotification(),
		},
		{
			name:   "test_resources_template_changed_notify",
			method: protocol.NotificationResourcesListChanged,
			f: func() {
				if err := server.RegisterResourceTemplate(testResourceTemplate, func(context.Context, *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
					return &protocol.ReadResourceResult{
						Contents: []protocol.ResourceContents{
							testResourceContent,
						},
					}, nil
				}); err != nil {
					t.Fatalf("RegisterResourceTemplate: %+v", err)
					return
				}
			},
			expectedNotify: protocol.NewResourceListChangedNotification(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := make(chan struct{})

			go func() {
				var notifyBytes []byte
				if outScan.Scan() {
					notifyBytes = outScan.Bytes()
				}

				var notifyMap map[string]interface{}
				if err := pkg.JSONUnmarshal(notifyBytes, &notifyMap); err != nil {
					t.Error(err)
					return
				}

				expectedNotify := protocol.NewJSONRPCNotification(tt.method, tt.expectedNotify)
				expectedNotifyBytes, err := json.Marshal(expectedNotify)
				if err != nil {
					t.Errorf("json Marshal: %+v", err)
					return
				}
				var expectedNotifyMap map[string]interface{}
				if err := pkg.JSONUnmarshal(expectedNotifyBytes, &expectedNotifyMap); err != nil {
					t.Error(err)
					return
				}

				if !reflect.DeepEqual(notifyMap, expectedNotifyMap) {
					t.Errorf("response not as expected.\ngot  = %v\nwant = %v", notifyMap, expectedNotifyMap)
					return
				}
				ch <- struct{}{}
			}()

			tt.f()

			<-ch
		})
	}
}

func testServerInit(t *testing.T, server *Server, in io.Writer, outScan *bufio.Scanner) {
	uuid, _ := uuid.NewUUID()
	req := protocol.NewJSONRPCRequest(uuid, protocol.Initialize, protocol.InitializeRequest{ProtocolVersion: protocol.Version})
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json Marshal: %+v", err)
	}
	if _, err = in.Write(append(reqBytes, "\n"...)); err != nil {
		t.Fatalf("in Write: %+v", err)
	}

	var respBytes []byte
	if outScan.Scan() {
		respBytes = outScan.Bytes()
		if outScan.Err() != nil {
			t.Fatalf("outScan: %+v", err)
		}
	}

	var respMap map[string]interface{}
	if err = pkg.JSONUnmarshal(respBytes, &respMap); err != nil {
		t.Fatal(err)
	}

	expectedResp := protocol.NewJSONRPCSuccessResponse(uuid, protocol.InitializeResult{
		ProtocolVersion: protocol.Version,
		Capabilities:    server.capabilities,
		ServerInfo:      server.serverInfo,
	})
	expectedRespBytes, err := json.Marshal(expectedResp)
	if err != nil {
		t.Fatalf("json Marshal: %+v", err)
	}
	var expectedRespMap map[string]interface{}
	if err = pkg.JSONUnmarshal(expectedRespBytes, &expectedRespMap); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(respMap, expectedRespMap) {
		t.Fatalf("response not as expected.\ngot  = %v\nwant = %v", respMap, expectedRespMap)
	}

	notify := protocol.NewJSONRPCNotification(protocol.NotificationInitialized, nil)
	notifyBytes, err := json.Marshal(notify)
	if err != nil {
		t.Fatalf("json Marshal: %+v", err)
	}
	if _, err := in.Write(append(notifyBytes, "\n"...)); err != nil {
		t.Fatalf("in Write: %+v", err)
	}
}

func TestServerHandleForPage(t *testing.T) {
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	var (
		in = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader1,
			writer: writer1,
		}

		out = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader2,
			writer: writer2,
		}

		outScan = bufio.NewScanner(out.reader)
	)

	server, err := NewServer(
		transport.NewMockServerTransport(in.reader, out.writer),
		WithServerInfo(protocol.Implementation{
			Name:    "ExampleServer",
			Version: "1.0.0",
		}),
		WithPagination(5),
	)
	if err != nil {
		t.Fatalf("NewServer: %+v", err)
	}
	total := 10
	// add tool
	for i := 0; i < total; i++ {
		testTool, err := protocol.NewTool(fmt.Sprintf("test_tool_%d", i), fmt.Sprintf("test_tool_%d", i), currentTimeReq{})
		if err != nil {
			t.Fatalf("NewTool: %+v", err)
			return
		}

		testToolCallContent := &protocol.TextContent{
			Type: "text",
			Text: fmt.Sprintf("pong_%d", i),
		}
		server.RegisterTool(testTool, func(_ context.Context, _ *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			return &protocol.CallToolResult{
				Content: []protocol.Content{testToolCallContent},
			}, nil
		})
	}

	// add prompt
	for i := 0; i < total; i++ {
		testPrompt := &protocol.Prompt{
			Name:        fmt.Sprintf("test_prompt_%d", i),
			Description: fmt.Sprintf("test_prompt_description_%d", i),
			Arguments: []*protocol.PromptArgument{
				{
					Name:        "params1",
					Description: "params1's description",
					Required:    true,
				},
			},
		}
		testPromptGetResponse := &protocol.GetPromptResult{
			Description: fmt.Sprintf("test_prompt_description_%d", i),
		}
		server.RegisterPrompt(testPrompt, func(context.Context, *protocol.GetPromptRequest) (*protocol.GetPromptResult, error) {
			return testPromptGetResponse, nil
		})
	}
	// add resource
	for i := 0; i < total; i++ {
		testResource := &protocol.Resource{
			URI:      fmt.Sprintf("file:///test%d.txt", i),
			Name:     fmt.Sprintf("test%d.txt", i),
			MimeType: "text/plain-txt",
		}
		testResourceContent := &protocol.TextResourceContents{
			URI:      testResource.URI,
			MimeType: testResource.MimeType,
			Text:     fmt.Sprintf("test%d", i),
		}
		server.RegisterResource(testResource, func(context.Context, *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
			return &protocol.ReadResourceResult{
				Contents: []protocol.ResourceContents{
					testResourceContent,
				},
			}, nil
		})
	}
	// add resource template
	for i := 0; i < total; i++ {
		testResourceTemplate := &protocol.ResourceTemplate{
			URITemplate: fmt.Sprintf("file:///{path}/%d", i),
			Name:        fmt.Sprintf("test_%d", i),
		}
		testResourceContent := &protocol.TextResourceContents{
			URI:      fmt.Sprintf("file:///test%d.txt", i),
			MimeType: "text/plain-txt",
			Text:     fmt.Sprintf("test%d", i),
		}
		if err := server.RegisterResourceTemplate(testResourceTemplate, func(context.Context, *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
			return &protocol.ReadResourceResult{
				Contents: []protocol.ResourceContents{
					testResourceContent,
				},
			}, nil
		}); err != nil {
			t.Fatalf("RegisterResourceTemplate: %+v", err)
			return
		}
	}
	go func() {
		if err := server.Run(); err != nil {
			t.Errorf("server start: %+v", err)
		}
	}()

	testServerInit(t, server, in.writer, outScan)

	tests := []struct {
		name             string
		method           protocol.Method
		request          protocol.ClientRequest
		expectedResponse protocol.ServerResponse
	}{
		{
			name:             "test_list_tool",
			method:           protocol.ToolsList,
			request:          protocol.ListToolsRequest{},
			expectedResponse: protocol.ListToolsResult{NextCursor: ""},
		},
		{
			name:    "test_list_prompt",
			method:  protocol.PromptsList,
			request: protocol.ListPromptsRequest{},
			expectedResponse: protocol.ListPromptsResult{
				NextCursor: "",
			},
		},
		{
			name:    "test_list_resource",
			method:  protocol.ResourcesList,
			request: protocol.ListResourcesRequest{},
			expectedResponse: protocol.ListResourcesResult{
				NextCursor: "",
			},
		},
		{
			name:    "test_list_resource_template",
			method:  protocol.ResourceListTemplates,
			request: protocol.ListResourceTemplatesRequest{},
			expectedResponse: protocol.ListResourceTemplatesResult{
				NextCursor: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, _ := uuid.NewUUID()
			totalResq := 0
			req := protocol.NewJSONRPCRequest(uuid, tt.method, tt.request)
			for i := 0; i < 3; i++ {
				reqBytes, err := json.Marshal(req)
				if err != nil {
					t.Fatalf("json Marshal: %+v", err)
				}
				if _, err = in.writer.Write(append(reqBytes, "\n"...)); err != nil {
					t.Fatalf("in Write: %+v", err)
				}

				var respBytes []byte
				if outScan.Scan() {
					respBytes = outScan.Bytes()
					if outScan.Err() != nil {
						t.Fatalf("outScan: %+v", err)
					}
				}
				respStruct := struct {
					Jsonrpc string      `json:"jsonrpc"`
					ID      interface{} `json:"id"`
					Result  struct {
						NextCursor        string        `json:"nextCursor"`
						Tools             []interface{} `json:"tools"`
						Prompts           []interface{} `json:"prompts"`
						Resources         []interface{} `json:"resources"`
						ResourceTemplates []interface{} `json:"resourceTemplates"`
					} `json:"result"`
				}{}
				if err = pkg.JSONUnmarshal(respBytes, &respStruct); err != nil {
					t.Fatal(err)
				}

				if respStruct.Result.NextCursor == "" {
					break
				}
				switch tt.method {
				case protocol.ToolsList:
					tt.request = protocol.ListToolsRequest{Cursor: protocol.Cursor(respStruct.Result.NextCursor)}
				case protocol.PromptsList:
					tt.request = protocol.ListPromptsRequest{Cursor: protocol.Cursor(respStruct.Result.NextCursor)}
				case protocol.ResourceListTemplates:
					tt.request = protocol.ListResourceTemplatesRequest{Cursor: protocol.Cursor(respStruct.Result.NextCursor)}
				case protocol.ResourcesList:
					tt.request = protocol.ListResourcesRequest{Cursor: protocol.Cursor(respStruct.Result.NextCursor)}
				}
				req = protocol.NewJSONRPCRequest(uuid, tt.method, tt.request)
				totalResq = totalResq + len(respStruct.Result.Tools) + len(respStruct.Result.Prompts) +
					len(respStruct.Result.Resources) + len(respStruct.Result.ResourceTemplates)
			}
			if total != totalResq {
				t.Fatalf("totalResq: %d, total: %d", totalResq, total)
			}
			// t.Logf("totalResq: %d,total:%d", totalResq, total)
		})
	}
}

type testLimiter struct {
	name               string
	rate               pkg.Rate
	numRequests        int
	requestInterval    time.Duration // Interval between requests
	expectedErrorCount int
	description        string
}

func TestServerRateLimiters(t *testing.T) {
	tests := []testLimiter{
		{
			name: "rapid_requests_exceed_burst",
			rate: pkg.Rate{
				Limit: 5.0,
				Burst: 10,
			},
			numRequests:        15,
			requestInterval:    0, // No delay between requests
			expectedErrorCount: 5,
			description:        "Sending requests rapidly should exceed burst limit and trigger rate limiting",
		},
		{
			name: "slow_requests_under_limit",
			rate: pkg.Rate{
				Limit: 5.0,
				Burst: 5,
			},
			numRequests:        10,
			requestInterval:    210 * time.Millisecond, // ~4.7 req/s, under the 5.0 limit
			expectedErrorCount: 0,
			description:        "Sending requests under the rate limit should not trigger rate limiting",
		},
		{
			name: "mixed_rate_pattern",
			rate: pkg.Rate{
				Limit: 10.0,
				Burst: 5,
			},
			numRequests:        20,
			requestInterval:    50 * time.Millisecond, // 20 req/s, above the 10.0 limit
			expectedErrorCount: 5,
			description:        "Sending requests at a rate higher than limit should trigger rate limiting after burst is consumed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServerRateLimiter(t, tt)
		})
	}
}

func testServerRateLimiter(t *testing.T, tt testLimiter) {
	// Set up pipes for communication
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	var (
		in = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader1,
			writer: writer1,
		}

		out = struct {
			reader io.ReadCloser
			writer io.WriteCloser
		}{
			reader: reader2,
			writer: writer2,
		}

		outScan = bufio.NewScanner(out.reader)
	)

	// Create server with rate limiter
	server, err := NewServer(
		transport.NewMockServerTransport(in.reader, out.writer),
		WithServerInfo(protocol.Implementation{
			Name:    "ExampleServer",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		t.Fatalf("NewServer: %+v", err)
	}

	// Register test tool
	testTool, err := protocol.NewTool("test_tool", "test_tool", currentTimeReq{})
	if err != nil {
		t.Fatalf("NewTool: %+v", err)
		return
	}
	testToolCallContent := protocol.TextContent{
		Type: "text",
		Text: "pong",
	}

	// Add minimal processing delay to simulate real-world scenario
	server.RegisterTool(testTool, func(_ context.Context, _ *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		time.Sleep(5 * time.Millisecond) // Small processing delay
		return &protocol.CallToolResult{
			Content: []protocol.Content{&testToolCallContent},
		}, nil
	}, RateLimitMiddleware(pkg.NewTokenBucketLimiter(tt.rate)))

	// Start server
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.Run(); err != nil {
			serverErrCh <- err
		}
	}()

	// Initialize server
	testServerInit(t, server, in.writer, outScan)

	// Test rate limiting by sending multiple requests
	errorCount := 0
	successCount := 0

	for i := 0; i < tt.numRequests; i++ {
		uuid, _ := uuid.NewUUID()
		req := protocol.NewJSONRPCRequest(uuid, protocol.ToolsCall, protocol.CallToolRequest{
			Name: testTool.Name,
		})
		reqBytes, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("json Marshal: %+v", err)
		}

		if _, err = in.writer.Write(append(reqBytes, "\n"...)); err != nil {
			t.Fatalf("in Write: %+v", err)
		}

		var respBytes []byte
		if outScan.Scan() {
			respBytes = outScan.Bytes()
			if outScan.Err() != nil {
				t.Fatalf("outScan: %+v", err)
			}
		}

		var resp map[string]interface{}
		if err = pkg.JSONUnmarshal(respBytes, &resp); err != nil {
			t.Fatal(err)
		}

		// Check if response contains error
		if errObj, exists := resp["error"]; exists {
			errorObj, ok := errObj.(map[string]interface{})
			if ok {
				// Check if it's a rate limit error
				if code, codeExists := errorObj["code"].(float64); codeExists && code == float64(-32603) {
					errorCount++
				}
			}
		} else {
			successCount++
		}

		// Apply interval between requests if specified
		if tt.requestInterval > 0 && i < tt.numRequests-1 {
			time.Sleep(tt.requestInterval)
		}
	}

	// duration := time.Since(startTime)

	// Verify that we got the expected number of rate limit errors
	if errorCount != tt.expectedErrorCount {
		t.Errorf("Expected %d rate limit errors, got %d", tt.expectedErrorCount, errorCount)
	}

	// Verify that successful + errors = total requests
	if successCount+errorCount != tt.numRequests {
		t.Errorf("Request count mismatch: got %d successes + %d errors = %d, expected total %d",
			successCount, errorCount, successCount+errorCount, tt.numRequests)
	}

	// Cleanup
	in.writer.Close()
	out.reader.Close()

	// Check if server encountered errors
	select {
	case err := <-serverErrCh:
		t.Fatalf("Server error: %v", err)
	default:
		// No error, continue
	}
}

// Middleware tests
func TestMiddleware(t *testing.T) {
	mockTransport := transport.NewMockServerTransport(io.NopCloser(bytes.NewReader(nil)), io.Discard)

	s, err := NewServer(mockTransport)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if s == nil {
		t.Fatal("Server is nil")
	}

	testMiddleware := func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			return next(ctx, req)
		}
	}

	s.Use(testMiddleware)
}

// TestMiddlewareEarlyReturn tests middleware that can return early
func TestMiddlewareEarlyReturn(t *testing.T) {
	mockTransport := transport.NewMockServerTransport(io.NopCloser(bytes.NewReader(nil)), io.Discard)

	s, err := NewServer(mockTransport)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if s == nil {
		t.Fatal("Server is nil")
	}

	authMiddleware := func(_ ToolHandlerFunc) ToolHandlerFunc {
		return func(_ context.Context, _ *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			return nil, context.Canceled
		}
	}

	s.Use(authMiddleware)
}

// TestNoMiddleware tests that tools work without middleware
func TestNoMiddleware(t *testing.T) {
	mockTransport := transport.NewMockServerTransport(io.NopCloser(bytes.NewReader(nil)), io.Discard)

	s, err := NewServer(mockTransport)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if s == nil {
		t.Fatal("Server is nil")
	}

	testHandler := func(_ context.Context, _ *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		return protocol.NewCallToolResult([]protocol.Content{
			&protocol.TextContent{Type: "text", Text: "success"},
		}, false), nil
	}

	testTool := &protocol.Tool{
		Name:        "test",
		Description: "Test tool",
		InputSchema: protocol.InputSchema{
			Type:       "object",
			Properties: map[string]*protocol.Property{},
		},
	}
	s.RegisterTool(testTool, testHandler)
}

// TestNewToolBackwardCompatibility tests that NewTool works with backward compatibility (input schema only)
func TestNewToolBackwardCompatibility(t *testing.T) {
	tool, err := protocol.NewTool("test_tool", "Test tool description", currentTimeReq{})
	if err != nil {
		t.Fatalf("NewTool failed: %v", err)
	}
	if tool.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", tool.Name)
	}

	if tool.Description != "Test tool description" {
		t.Errorf("Expected description 'Test tool description', got '%s'", tool.Description)
	}

	if tool.OutputSchema.Type != "" {
		t.Errorf("Expected no output schema, got %v", tool.OutputSchema)
	}
}

// TestNewToolWithOutputSchema tests that NewTool works correctly with both input and output schemas
func TestNewToolWithOutputSchema(t *testing.T) {
	type TestInput struct {
		Name string `json:"name" description:"The name" required:"true"`
	}

	type TestOutput struct {
		Result string `json:"result" description:"The result"`
		Count  int    `json:"count" description:"The count"`
	}

	tool, err := protocol.NewTool("test_with_output", "Test tool with output schema", TestInput{}, TestOutput{})
	if err != nil {
		t.Fatalf("NewTool with output schema failed: %v", err)
	}

	if tool.OutputSchema.Type == "" {
		t.Fatalf("Expected output schema, got empty")
	}

	if tool.OutputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %v", tool.OutputSchema.Type)
	}

	if len(tool.OutputSchema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(tool.OutputSchema.Properties))
	}
}

// TestNewToolErrorHandling tests error handling for invalid input and output schemas
func TestNewToolErrorHandling(t *testing.T) {
	_, err := protocol.NewTool("invalid", "test", func() {})
	if err == nil {
		t.Fatal("Expected error for invalid input schema, got nil")
	}

	_, err = protocol.NewTool("invalid", "test", currentTimeReq{}, func() {})
	if err == nil {
		t.Fatal("Expected error for invalid output schema, got nil")
	}
}

// TestCallToolResultStructuredContent tests that CallToolResult properly handles structured content alongside regular content
func TestCallToolResultStructuredContent(t *testing.T) {
	structuredData := map[string]interface{}{
		"temperature": 25.5,
		"condition":   "sunny",
	}

	result := &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: "Weather: 25.5°C, sunny",
			},
		},
		StructuredContent: structuredData,
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal CallToolResult: %v", err)
	}

	var unmarshaledResult protocol.CallToolResult
	if err := json.Unmarshal(jsonData, &unmarshaledResult); err != nil {
		t.Fatalf("Failed to unmarshal CallToolResult: %v", err)
	}
	if unmarshaledResult.StructuredContent == nil {
		t.Fatal("Expected structured content to be preserved")
	}

	if len(unmarshaledResult.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(unmarshaledResult.Content))
	}
}

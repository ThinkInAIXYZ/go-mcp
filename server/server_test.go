package server

import (
	"bufio"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/bytedance/sonic"
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
	server.RegisterTool(testTool, func(_ *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		return &protocol.CallToolResult{
			Content: []protocol.Content{testToolCallContent},
		}, nil
	})

	// add prompt
	testPrompt := &protocol.Prompt{
		Name:        "test_prompt",
		Description: "test_prompt_description",
		Arguments: []protocol.PromptArgument{
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
	server.RegisterPrompt(testPrompt, func(*protocol.GetPromptRequest) (*protocol.GetPromptResult, error) {
		return testPromptGetResponse, nil
	})

	// add resource
	testResource := &protocol.Resource{
		URI:      "file:///test.txt",
		Name:     "test.txt",
		MimeType: "text/plain-txt",
	}
	testResourceContent := protocol.TextResourceContents{
		URI:      testResource.URI,
		MimeType: testResource.MimeType,
		Text:     "test",
	}
	server.RegisterResource(testResource, func(*protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
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
	if err := server.RegisterResourceTemplate(testResourceTemplate, func(*protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
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
					testToolCallContent,
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
				Prompts: []protocol.Prompt{*testPrompt},
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
				Resources: []protocol.Resource{*testResource},
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
				ResourceTemplates: []protocol.ResourceTemplate{*testResourceTemplate},
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
			reqBytes, err := sonic.Marshal(req)
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
		Arguments: []protocol.PromptArgument{
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
	testResourceContent := protocol.TextResourceContents{
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
				server.RegisterTool(testTool, func(_ *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
					return &protocol.CallToolResult{
						Content: []protocol.Content{testToolCallContent},
					}, nil
				})
			},
			expectedNotify: protocol.NewToolListChangedNotification(),
		},
		{
			name:   "test_prompts_changed_notify",
			method: protocol.NotificationPromptsListChanged,
			f: func() {
				server.RegisterPrompt(testPrompt, func(*protocol.GetPromptRequest) (*protocol.GetPromptResult, error) {
					return testPromptGetResponse, nil
				})
			},
			expectedNotify: protocol.NewPromptListChangedNotification(),
		},
		{
			name:   "test_resources_changed_notify",
			method: protocol.NotificationResourcesListChanged,
			f: func() {
				server.RegisterResource(testResource, func(*protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
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
				if err := server.RegisterResourceTemplate(testResourceTemplate, func(*protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
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
	reqBytes, err := sonic.Marshal(req)
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
		Capabilities:    *server.capabilities,
		ServerInfo:      *server.serverInfo,
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
	notifyBytes, err := sonic.Marshal(notify)
	if err != nil {
		t.Fatalf("json Marshal: %+v", err)
	}
	if _, err := in.Write(append(notifyBytes, "\n"...)); err != nil {
		t.Fatalf("in Write: %+v", err)
	}
}

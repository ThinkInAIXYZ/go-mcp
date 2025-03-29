package server

import (
	"bufio"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"go-mcp/pkg"
	"go-mcp/protocol"
	"go-mcp/transport"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

func TestServer(t *testing.T) {
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	var (
		in io.ReadWriter = struct {
			io.Reader
			io.Writer
		}{
			Reader: reader1,
			Writer: writer1,
		}

		out io.ReadWriter = struct {
			io.Reader
			io.Writer
		}{
			Reader: reader2,
			Writer: writer2,
		}

		outScan = bufio.NewScanner(out)
	)

	server, err := NewServer(
		transport.NewMockServerTransport(in, out),
		WithInfo(protocol.Implementation{
			Name:    "ExampleServer",
			Version: "1.0.0",
		}))

	// TODO: add mock session id auto
	server.sessionID2session.Store("mock", newSession())
	if err != nil {
		t.Fatalf("NewServer: %+v", err)
	}

	// add tool
	testTool := &protocol.Tool{
		Name:        "test_tool",
		Description: "test_tool",
		InputSchema: map[string]interface{}{
			"a": "int",
		},
	}
	testToolCallContent := protocol.TextContent{
		Type: "text",
		Text: "pong",
	}
	server.AddTool(testTool, func(ctr protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		return &protocol.CallToolResult{
			Content: []protocol.Content{testToolCallContent},
		}, nil
	})

	// add prompt
	testPrompt := protocol.Prompt{
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
	testPromtGetResponse := &protocol.GetPromptResult{
		Description: "test_prompt_description",
	}
	server.AddPrompt(testPrompt, func(protocol.GetPromptRequest) (*protocol.GetPromptResult, error) {
		return testPromtGetResponse, nil
	})

	// add resource
	testResource := protocol.Resource{
		URI:      "file:///test.txt",
		Name:     "test.txt",
		MimeType: "text/plain-txt",
	}
	testResourceContent := protocol.TextResourceContents{
		URI:      testResource.URI,
		MimeType: testResource.MimeType,
		Text:     "test",
	}
	server.AddResource(testResource, func(protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
		return &protocol.ReadResourceResult{
			Contents: []protocol.ResourceContents{
				testResourceContent,
			},
		}, nil
	})

	// add resource template
	testReourceTemplate := protocol.ResourceTemplate{
		URITemplate: "file:///{path}",
		Name:        "test",
	}
	server.AddResourceTemplate(testReourceTemplate)

	go func() {
		if err := server.Start(); err != nil {
			t.Errorf("server start: %+v", err)
		}
	}()

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
			name:    "test_initialize",
			method:  protocol.Initialize,
			request: protocol.InitializeRequest{},
			expectedResponse: protocol.InitializeResult{
				ProtocolVersion: protocol.PROTOCOL_VERSION,
				Capabilities:    server.capabilities,
				ServerInfo:      server.serverInfo,
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
				Prompts: []protocol.Prompt{testPrompt},
			},
		},
		{
			name:   "test_get_prompt",
			method: protocol.PromptsGet,
			request: protocol.GetPromptRequest{
				Name: testPrompt.Name,
			},
			expectedResponse: testPromtGetResponse,
		},
		{
			name:    "test_list_resource",
			method:  protocol.ResourcesList,
			request: protocol.ListResourcesRequest{},
			expectedResponse: protocol.ListResourcesResult{
				Resources: []protocol.Resource{testResource},
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
				ResourceTemplates: []protocol.ResourceTemplate{testReourceTemplate},
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
			if _, err := in.Write(append(reqBytes, "\n"...)); err != nil {
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
			if err := pkg.JsonUnmarshal(respBytes, &respMap); err != nil {
				t.Error(err)
			}

			expectedResp := protocol.NewJSONRPCSuccessResponse(uuid, tt.expectedResponse)
			expectedRespBytes, err := json.Marshal(expectedResp)
			if err != nil {
				t.Fatalf("json Marshal: %+v", err)
			}
			var expectedRespMap map[string]interface{}
			if err := pkg.JsonUnmarshal(expectedRespBytes, &expectedRespMap); err != nil {
				t.Error(err)
			}

			if !reflect.DeepEqual(respMap, expectedRespMap) {
				t.Fatalf("response not as expected.\ngot  = %v\nwant = %v", respMap, expectedRespMap)
			}
		})
	}
}

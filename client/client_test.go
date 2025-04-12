package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func TestClientCall(t *testing.T) {
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

	client := testClientInit(t, in, out, outScan)

	tests := []struct {
		name             string
		f                func(client *Client, request protocol.ClientRequest) (protocol.ServerResponse, error)
		request          protocol.ClientRequest
		expectedResponse protocol.ServerResponse
	}{
		{
			name: "test_ping",
			f: func(client *Client, request protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.Ping(context.Background(), request.(*protocol.PingRequest))
			},
			request:          protocol.NewPingRequest(),
			expectedResponse: protocol.NewPingResult(),
		},
		{
			name: "test_list_prompts",
			f: func(client *Client, _ protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.ListPrompts(context.Background())
			},
			request:          protocol.NewListPromptsRequest(),
			expectedResponse: protocol.NewListPromptsResult([]protocol.Prompt{{Name: "prompt1"}, {Name: "prompt2"}}, ""),
		},
		{
			name: "test_get_prompt",
			f: func(client *Client, request protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.GetPrompt(context.Background(), request.(*protocol.GetPromptRequest))
			},
			request: protocol.NewGetPromptRequest("prompt1", map[string]string{}),
			expectedResponse: protocol.NewGetPromptResult([]protocol.PromptMessage{
				{
					Role:    protocol.RoleUser,
					Content: protocol.TextContent{Type: "text", Text: "prompt content"},
				},
			}, "test description"),
		},
		{
			name: "test_list_resources",
			f: func(client *Client, _ protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.ListResources(context.Background())
			},
			request:          protocol.NewListResourcesRequest(),
			expectedResponse: protocol.NewListResourcesResult([]protocol.Resource{{Name: "resource1"}, {Name: "resource2"}}, ""),
		},
		{
			name: "test_read_resource",
			f: func(client *Client, request protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.ReadResource(context.Background(), request.(*protocol.ReadResourceRequest))
			},
			request: protocol.NewReadResourceRequest("resource1"),
			expectedResponse: protocol.NewReadResourceResult([]protocol.ResourceContents{
				protocol.TextResourceContents{URI: "resource1", Text: "resource content", MimeType: "text/plain"},
			}),
		},
		{
			name: "test_list_resource_templates",
			f: func(client *Client, _ protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.ListResourceTemplates(context.Background())
			},
			request:          protocol.NewListResourceTemplatesRequest(),
			expectedResponse: protocol.NewListResourceTemplatesResult([]protocol.ResourceTemplate{{Name: "template1"}, {Name: "template2"}}, ""),
		},
		{
			name: "test_subscribe_resource_change",
			f: func(client *Client, request protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.SubscribeResourceChange(context.Background(), request.(*protocol.SubscribeRequest))
			},
			request:          protocol.NewSubscribeRequest("resource1"),
			expectedResponse: &protocol.SubscribeResult{},
		},
		{
			name: "test_unsubscribe_resource_change",
			f: func(client *Client, request protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.UnSubscribeResourceChange(context.Background(), request.(*protocol.UnsubscribeRequest))
			},
			request:          protocol.NewUnsubscribeRequest("subscription_id"),
			expectedResponse: &protocol.UnsubscribeResult{},
		},
		{
			name: "test_list_tool",
			f: func(client *Client, _ protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.ListTools(context.Background())
			},
			request: protocol.NewListToolsRequest(),
			expectedResponse: protocol.NewListToolsResult([]*protocol.Tool{{
				Name:        "test_tool",
				Description: "test_tool",
				InputSchema: protocol.InputSchema{
					Type: protocol.Object,
					Properties: map[string]*protocol.Property{
						"timezone": {
							Type:        "string",
							Description: "current time timezone",
						},
					},
					Required: []string{"timezone"},
				},
			}}, ""),
		},
		{
			name: "test_call_tool",
			f: func(client *Client, request protocol.ClientRequest) (protocol.ServerResponse, error) {
				return client.CallTool(context.Background(), request.(*protocol.CallToolRequest))
			},
			request: protocol.NewCallToolRequest("test_tool", map[string]interface{}{
				"a": 1,
			}),
			expectedResponse: protocol.NewCallToolResult([]protocol.Content{protocol.TextContent{Type: "text", Text: "success"}}, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			go func() {
				var reqBytes []byte
				if outScan.Scan() {
					reqBytes = outScan.Bytes()
				}
				if err := outScan.Err(); err != nil {
					t.Errorf("outScan: %+v", err)
					return
				}

				jsonrpcReq := &protocol.JSONRPCRequest{}
				if err := pkg.JSONUnmarshal(reqBytes, &jsonrpcReq); err != nil {
					t.Errorf("Json Unmarshal: %+v", err)
					return
				}

				request := make(map[string]interface{})
				if err := pkg.JSONUnmarshal(jsonrpcReq.RawParams, &request); err != nil {
					t.Errorf("Json Unmarshal: %+v", err)
					return
				}

				expectedReqBytes, err := json.Marshal(tt.request)
				if err != nil {
					t.Errorf("json Marshal: %+v", err)
					return
				}
				var expectedReqMap map[string]interface{}
				if err = pkg.JSONUnmarshal(expectedReqBytes, &expectedReqMap); err != nil {
					t.Errorf("json Unmarshal: %+v", err)
					return
				}

				if !reflect.DeepEqual(request, expectedReqMap) {
					t.Errorf("response not as expected.\ngot  = %v\nwant = %v", request, expectedReqMap)
					return
				}

				respBytes, err := sonic.Marshal(protocol.NewJSONRPCSuccessResponse(jsonrpcReq.ID, tt.expectedResponse))
				if err != nil {
					t.Errorf("Json Marshal: %+v", err)
					return
				}
				if _, err := in.Write(append(respBytes, "\n"...)); err != nil {
					t.Errorf("in Write: %+v", err)
					return
				}
			}()

			response, err := tt.f(client, tt.request)
			if err != nil {
				t.Fatalf("func exectue: %+v", err)
			}

			if !reflect.DeepEqual(response, tt.expectedResponse) {
				t.Fatalf("response not as expected.\ngot  = %+v\nwant = %+v", response, tt.expectedResponse)
			}
		})
	}
}

func testClientInit(t *testing.T, in io.ReadWriter, out io.ReadWriter, outScan *bufio.Scanner) *Client {
	req := protocol.InitializeRequest{
		ClientInfo: protocol.Implementation{
			Name:    "test_client",
			Version: "0.1",
		},
		Capabilities:    protocol.ClientCapabilities{},
		ProtocolVersion: protocol.Version,
	}

	ch := make(chan struct{})

	go func() {
		var reqBytes []byte
		if outScan.Scan() { // Read initialization request
			reqBytes = outScan.Bytes()
		}
		if err := outScan.Err(); err != nil {
			t.Errorf("outScan: %+v", err)
			return
		}

		jsonrpcReq := &protocol.JSONRPCRequest{}
		if err := pkg.JSONUnmarshal(reqBytes, &jsonrpcReq); err != nil {
			t.Errorf("Json Unmarshal: %+v", err)
			return
		}

		request := make(map[string]interface{})
		if err := pkg.JSONUnmarshal(jsonrpcReq.RawParams, &request); err != nil {
			t.Errorf("Json Unmarshal: %+v", err)
			return
		}

		expectedReqBytes, err := json.Marshal(req)
		if err != nil {
			t.Errorf("json Marshal: %+v", err)
			return
		}
		var expectedReqMap map[string]interface{}
		if err = pkg.JSONUnmarshal(expectedReqBytes, &expectedReqMap); err != nil {
			t.Errorf("json Unmarshal: %+v", err)
			return
		}

		if !reflect.DeepEqual(request, expectedReqMap) {
			t.Errorf("response not as expected.\ngot  = %v\nwant = %v", request, expectedReqMap)
			return
		}

		resp := &protocol.InitializeResult{
			ServerInfo: protocol.Implementation{
				Name:    "test_server",
				Version: "0.1",
			},
			Capabilities: protocol.ServerCapabilities{
				Prompts: &protocol.PromptsCapability{
					ListChanged: true,
				},
				Resources: &protocol.ResourcesCapability{
					ListChanged: true,
					Subscribe:   true,
				},
				Tools: &protocol.ToolsCapability{
					ListChanged: true,
				},
			},
			ProtocolVersion: protocol.Version,
		}

		respBytes, err := sonic.Marshal(protocol.NewJSONRPCSuccessResponse(jsonrpcReq.ID, resp))
		if err != nil {
			t.Errorf("Json Marshal: %+v", err)
			return
		}
		if _, err := in.Write(append(respBytes, "\n"...)); err != nil {
			t.Errorf("in Write: %+v", err)
			return
		}

		if outScan.Scan() { // Read initialization notification
			notifyBytes := outScan.Bytes()
			fmt.Println("initialization notify: " + string(notifyBytes))
		}
		if err := outScan.Err(); err != nil {
			t.Errorf("outScan: %+v", err)
			return
		}
		ch <- struct{}{}
	}()

	client, err := NewClient(transport.NewMockClientTransport(in, out), WithClientInfo(req.ClientInfo))
	if err != nil {
		t.Fatalf("NewServer: %+v", err)
	}
	<-ch
	return client
}

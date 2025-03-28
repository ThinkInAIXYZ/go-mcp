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

	server, err := NewServer(transport.NewMockServerTransport(in, out))
	if err != nil {
		t.Fatalf("NewServer: %+v", err)
	}
	testTool := &protocol.Tool{
		Name:        "test_tool",
		Description: "test_tool",
		InputSchema: map[string]interface{}{
			"a": "int",
		},
	}
	server.AddTool(testTool)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid := pkg.GenerateUUID()
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

package protocol

import (
	"encoding/json"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

const jsonrpcVersion = "2.0"

// Standard JSON-RPC error codes
const (
	ParseError     = -32700 // Invalid JSON
	InvalidRequest = -32600 // The JSON sent is not a valid Request object
	MethodNotFound = -32601 // The method does not exist / is not available
	InvalidParams  = -32602 // Invalid method parameter(s)
	InternalError  = -32603 // Internal JSON-RPC error

	// 可以定义自己的错误代码，范围在-32000 以上。
	ConnectionError = -32400
)

type RequestID interface{} // 字符串/数值

type JSONRPCRequest struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        RequestID       `json:"id"`
	Method    Method          `json:"method"`
	Params    interface{}     `json:"params,omitempty"`
	RawParams json.RawMessage `json:"-"`
}

func (r *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	type alias JSONRPCRequest
	temp := &struct {
		Params json.RawMessage `json:"params,omitempty"`
		*alias
	}{
		alias: (*alias)(r),
	}

	if err := pkg.JSONUnmarshal(data, temp); err != nil {
		return err
	}

	r.RawParams = temp.Params

	if len(r.RawParams) != 0 {
		if err := pkg.JSONUnmarshal(r.RawParams, &r.Params); err != nil {
			return err
		}
	}

	return nil
}

// IsValid checks if the request is valid according to JSON-RPC 2.0 spec
func (r *JSONRPCRequest) IsValid() bool {
	return r.JSONRPC == jsonrpcVersion && r.Method != "" && r.ID != nil
}

// JSONRPCResponse represents a response to a request.
type JSONRPCResponse struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        RequestID       `json:"id"`
	Result    interface{}     `json:"result,omitempty"`
	RawResult json.RawMessage `json:"-"`
	Error     *responseErr    `json:"error,omitempty"`
}

type responseErr struct {
	// The error type that occurred.
	Code int `json:"code"`
	// A short description of the error. The message SHOULD be limited
	// to a concise single sentence.
	Message string `json:"message"`
	// Additional information about the error. The value of this member
	// is defined by the sender (e.g. detailed error information, nested errors etc.).
	Data interface{} `json:"data,omitempty"`
}

func (r *JSONRPCResponse) UnmarshalJSON(data []byte) error {
	type alias JSONRPCResponse
	temp := &struct {
		Result json.RawMessage `json:"result,omitempty"`
		*alias
	}{
		alias: (*alias)(r),
	}

	if err := pkg.JSONUnmarshal(data, temp); err != nil {
		return err
	}

	r.RawResult = temp.Result

	if len(r.RawResult) != 0 {
		if err := pkg.JSONUnmarshal(r.RawResult, &r.Result); err != nil {
			return err
		}
	}

	return nil
}

type JSONRPCNotification struct {
	JSONRPC   string          `json:"jsonrpc"`
	Method    Method          `json:"method"`
	Params    interface{}     `json:"params,omitempty"`
	RawParams json.RawMessage `json:"-"`
}

func (r *JSONRPCNotification) UnmarshalJSON(data []byte) error {
	type alias JSONRPCNotification
	temp := &struct {
		Params json.RawMessage `json:"params,omitempty"`
		*alias
	}{
		alias: (*alias)(r),
	}

	if err := pkg.JSONUnmarshal(data, temp); err != nil {
		return err
	}

	r.RawParams = temp.Params

	if len(r.RawParams) != 0 {
		if err := pkg.JSONUnmarshal(r.RawParams, &r.Params); err != nil {
			return err
		}
	}

	return nil
}

// NewJSONRPCRequest creates a new JSON-RPC request
func NewJSONRPCRequest(id RequestID, method Method, params interface{}) *JSONRPCRequest {
	return &JSONRPCRequest{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewJSONRPCSuccessResponse creates a new JSON-RPC response
func NewJSONRPCSuccessResponse(id RequestID, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Result:  result,
	}
}

// NewJSONRPCErrorResponse NewError creates a new JSON-RPC error response
func NewJSONRPCErrorResponse(id RequestID, code int, message string) *JSONRPCResponse {
	err := &JSONRPCResponse{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &responseErr{
			Code:    code,
			Message: message,
		},
	}
	return err
}

// NewJSONRPCNotification creates a new JSON-RPC notification
func NewJSONRPCNotification(method Method, params interface{}) *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  params,
	}
}

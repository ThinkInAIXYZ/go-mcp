package protocol

import (
	"encoding/json"
)

// RequestId represents a uniquely identifying ID for a request in JSON-RPC
// RequestId 表示 JSON-RPC 中请求的唯一标识 ID
type RequestId interface{} // Can be string or integer (string 或整数类型)

// JsonrpcRequest represents a request that expects a response
// JsonrpcRequest 表示一个期望响应的请求
type JsonrpcRequest struct {
	// JSON-RPC version (JSON-RPC 版本)
	Jsonrpc string `json:"jsonrpc"`
	// Communication ID, can be string or integer (通信ID，可以是字符串或整数)
	ID interface{} `json:"id,omitempty"`
	// Method to be called (调用的方法)
	Method string `json:"method"`
	// Parameters for the method (方法的参数)
	Params json.RawMessage `json:"params"`
}

// IsNotification checks if this request is a notification (which doesn't require a response)
// IsNotification 检查此请求是否为通知（不需要响应）
func (x *JsonrpcRequest) IsNotification() bool {
	return x.ID == nil
}

// GetID returns the request ID or nil if it's a notification
// GetID 返回请求ID，如果是通知则返回nil
func (x *JsonrpcRequest) GetID() interface{} {
	return x.ID
}

// JsonrpcResponse represents a successful (non-error) response to a request
// JsonrpcResponse 表示对请求的成功（非错误）响应
type JsonrpcResponse struct {
	// JSON-RPC version (JSON-RPC 版本)
	Jsonrpc string `json:"jsonrpc"`
	// Response ID matching the request ID (响应ID，与请求ID相匹配)
	ID interface{} `json:"id"`
	// Result of the method call (方法调用的结果)
	Result json.RawMessage `json:"result"`
	// Error information, if any (错误信息，如果有的话)
	Error *JsonrpcError `json:"error,omitempty"`
}

// NewJsonrpcResponse creates a new JSON-RPC response
// NewJsonrpcResponse 创建一个新的 JSON-RPC 响应
func NewJsonrpcResponse(id interface{}, result json.RawMessage, error *JsonrpcError) *JsonrpcResponse {
	return &JsonrpcResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  result,
		Error:   error,
	}
}

// JsonrpcError represents an error response to a JSON-RPC request
// JsonrpcError 表示对 JSON-RPC 请求的错误响应
type JsonrpcError struct {
	// Error code (错误代码)
	Code int64 `json:"code"`
	// Short description of the error (错误的简短描述)
	Message string `json:"message"`
	// Additional information about the error (关于错误的附加信息)
	Data interface{} `json:"data,omitempty"`
}

// JsonrpcNotification represents a notification which does not expect a response
// JsonrpcNotification 表示不需要响应的通知
type JsonrpcNotification struct {
	// JSON-RPC version (JSON-RPC 版本)
	Jsonrpc string `json:"jsonrpc"`
	// Method to be called (调用的方法)
	Method string `json:"method"`
	// Parameters for the method (方法的参数)
	Params json.RawMessage `json:"params,omitempty"`
}

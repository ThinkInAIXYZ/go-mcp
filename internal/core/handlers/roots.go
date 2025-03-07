package handlers

import (
	"context"
	"encoding/json"
	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
)

// ListRootsHandler 处理roots/list请求
type ListRootsHandler struct {
}

// NewListRootsHandler 创建一个ListRootsHandler实例
func NewListRootsHandler() *ListRootsHandler {
	return &ListRootsHandler{}
}

// Handle 处理roots/list请求
func (x *ListRootsHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.ListRootsRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 示例根目录列表，在实际应用中应该从客户端配置获取
	roots := []protocol.Root{
		{
			URI:  "file:///home/user/documents",
			Name: "Documents",
		},
		{
			URI:  "file:///home/user/projects",
			Name: "Projects",
		},
	}

	result := protocol.ListRootsResult{
		Roots: roots,
	}

	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *ListRootsHandler) Method() protocol.McpMethod {
	return protocol.MethodListRoots
}

// RootsListChangedHandler 处理roots/list_changed通知
type RootsListChangedHandler struct {
}

// NewRootsListChangedHandler 创建一个RootsListChangedHandler实例
func NewRootsListChangedHandler() *RootsListChangedHandler {
	return &RootsListChangedHandler{}
}

// Handle 处理roots/list_changed通知
func (x *RootsListChangedHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	// 这是一个通知处理程序，客户端不需要响应
	// 在这里可以处理根目录列表变更的逻辑，例如刷新缓存等
	return nil, nil
}

// Method 返回此处理程序对应的MCP方法
func (x *RootsListChangedHandler) Method() protocol.McpMethod {
	return protocol.NotificationRootsListChanged
}

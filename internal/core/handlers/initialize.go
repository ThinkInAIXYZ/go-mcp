package handlers

import (
	"context"
	"encoding/json"

	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
)

// InitializeHandler 处理initialize请求
type InitializeHandler struct {
}

// NewInitializeHandler 创建一个InitializeHandler实例
func NewInitializeHandler() *InitializeHandler {
	return &InitializeHandler{}
}

// Handle 处理initialize请求
func (x *InitializeHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.InitializeRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 构建初始化响应
	response := protocol.InitializeResult{
		ProtocolVersion: "2024-11-05", // 使用MCP协议版本
		Capabilities: protocol.ServerCapabilities{
			Logging: &protocol.ServerLogging{},
			Prompts: &protocol.ServerPrompts{
				ListChanged: true,
			},
			Resources: &protocol.ServerResources{
				Subscribe:   true,
				ListChanged: true,
			},
			Tools: &protocol.ServerTools{
				ListChanged: true,
			},
		},
		ServerInfo: protocol.Implementation{
			Name:    "MCP-Layout-Server",
			Version: "0.1.0",
		},
		Instructions: "这是一个MCP协议示例服务器，提供了基本的工具、资源和提示功能。",
	}

	return json.Marshal(response)
}

// Method 返回此处理程序对应的MCP方法
func (x *InitializeHandler) Method() protocol.McpMethod {
	return protocol.MethodInitialize
}

package handlers

import (
	"context"
	"encoding/json"
	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
)

// CreateMessageHandler 处理sampling/createMessage请求
type CreateMessageHandler struct {
}

// NewCreateMessageHandler 创建一个CreateMessageHandler实例
func NewCreateMessageHandler() *CreateMessageHandler {
	return &CreateMessageHandler{}
}

// Handle 处理sampling/createMessage请求
func (x *CreateMessageHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.CreateMessageRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 在实际应用中，这里应该调用LLM API进行采样
	// 这里只是返回一个模拟响应

	// 创建一个文本内容作为响应
	textContent := protocol.TextContent{
		Type: "text",
		Text: "这是来自LLM模型的模拟响应。在实际应用中，这里应该调用实际的LLM API并返回其响应。",
	}

	result := protocol.CreateMessageResult{
		Model:      "mock-model",
		Role:       protocol.RoleAssistant,
		Content:    textContent,
		StopReason: string(protocol.StopReasonEndTurn),
	}

	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *CreateMessageHandler) Method() protocol.McpMethod {
	return protocol.MethodCreateMessage
}

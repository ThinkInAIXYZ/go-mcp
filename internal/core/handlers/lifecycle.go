package handlers

import (
	"context"
	"encoding/json"
	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
)

// InitializedHandler 处理initialized通知
type InitializedHandler struct {
}

// NewInitializedHandler 创建一个InitializedHandler实例
func NewInitializedHandler() *InitializedHandler {
	return &InitializedHandler{}
}

// Handle 处理initialized通知
func (x *InitializedHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	// initialized是一个通知，不需要返回结果
	// 在这里可以进行一些初始化工作
	return nil, nil
}

// Method 返回此处理程序对应的MCP方法
func (x *InitializedHandler) Method() protocol.McpMethod {
	return protocol.NotificationInitialized
}

// ShutdownHandler 处理shutdown请求
type ShutdownHandler struct {
}

// NewShutdownHandler 创建一个ShutdownHandler实例
func NewShutdownHandler() *ShutdownHandler {
	return &ShutdownHandler{}
}

// Handle 处理shutdown请求
func (x *ShutdownHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.ShutdownRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 这里应该进行实际的关闭前清理工作
	// 在本示例中，我们只是返回一个成功响应

	result := protocol.ShutdownResult{}
	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *ShutdownHandler) Method() protocol.McpMethod {
	return "shutdown" // 注意：在constants.go中没有定义这个方法，根据实际需要可以添加
}

// CancelHandler 处理cancel请求
type CancelHandler struct {
}

// NewCancelHandler 创建一个CancelHandler实例
func NewCancelHandler() *CancelHandler {
	return &CancelHandler{}
}

// Handle 处理cancel请求
func (x *CancelHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.CancelRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 这里应该实现取消指定ID请求的逻辑
	// 在本示例中，我们只是返回一个成功响应

	result := protocol.CancelResult{}
	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *CancelHandler) Method() protocol.McpMethod {
	return "$/cancelRequest" // 注意：在constants.go中没有定义这个方法，根据实际需要可以添加
}

// ProgressNotificationHandler 发送进度通知
type ProgressNotificationHandler struct {
	gateway interface {
		SendNotification(method string, params interface{}) error
	}
}

// NewProgressNotificationHandler 创建一个ProgressNotificationHandler实例
func NewProgressNotificationHandler(gateway interface {
	SendNotification(method string, params interface{}) error
}) *ProgressNotificationHandler {
	return &ProgressNotificationHandler{
		gateway: gateway,
	}
}

// SendProgress 发送进度通知
func (x *ProgressNotificationHandler) SendProgress(token protocol.ProgressToken, value int, message string) error {
	params := protocol.ProgressParams{
		Token:   token,
		Value:   value,
		Message: message,
	}

	return x.gateway.SendNotification(protocol.NotificationProgress, params)
}

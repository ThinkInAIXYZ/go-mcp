package handlers

import (
	"context"
	"encoding/json"
	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
	"log"
)

// SetLevelHandler 处理logging/setLevel请求
type SetLevelHandler struct {
	// 当前日志级别
	currentLevel protocol.LoggingLevel
}

// NewSetLevelHandler 创建一个SetLevelHandler实例
func NewSetLevelHandler() *SetLevelHandler {
	return &SetLevelHandler{
		currentLevel: protocol.LoggingLevelInfo, // 默认日志级别为Info
	}
}

// Handle 处理logging/setLevel请求
func (x *SetLevelHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.SetLevelRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 设置新的日志级别
	x.currentLevel = req.Level
	log.Printf("日志级别已设置为: %s\n", x.currentLevel)

	result := protocol.SetLevelResult{}
	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *SetLevelHandler) Method() protocol.McpMethod {
	return protocol.MethodSetLevel
}

// GetCurrentLevel 获取当前日志级别
func (x *SetLevelHandler) GetCurrentLevel() protocol.LoggingLevel {
	return x.currentLevel
}

// LoggingMessageSender 用于发送日志消息
type LoggingMessageSender struct {
	gateway interface {
		SendNotification(method string, params interface{}) error
	}
	levelHandler *SetLevelHandler
}

// NewLoggingMessageSender 创建一个LoggingMessageSender实例
func NewLoggingMessageSender(
	gateway interface {
		SendNotification(method string, params interface{}) error
	},
	levelHandler *SetLevelHandler,
) *LoggingMessageSender {
	return &LoggingMessageSender{
		gateway:      gateway,
		levelHandler: levelHandler,
	}
}

// LogLevelMap 日志级别映射表，用于比较日志级别
var LogLevelMap = map[protocol.LoggingLevel]int{
	protocol.LoggingLevelDebug:     0,
	protocol.LoggingLevelInfo:      1,
	protocol.LoggingLevelNotice:    2,
	protocol.LoggingLevelWarning:   3,
	protocol.LoggingLevelError:     4,
	protocol.LoggingLevelCritical:  5,
	protocol.LoggingLevelAlert:     6,
	protocol.LoggingLevelEmergency: 7,
}

// SendLogMessage 发送日志消息，如果消息级别低于当前设置的级别则不发送
func (x *LoggingMessageSender) SendLogMessage(level protocol.LoggingLevel, logger string, data interface{}) error {
	// 检查消息级别是否应该被发送
	currentLevel := x.levelHandler.GetCurrentLevel()

	if LogLevelMap[level] < LogLevelMap[currentLevel] {
		// 消息级别低于当前设置的级别，不发送
		return nil
	}

	notification := protocol.LoggingMessageNotification{
		Level:  level,
		Logger: logger,
		Data:   data,
	}

	return x.gateway.SendNotification(protocol.NotificationLoggingMessage, notification)
}

// 便捷方法用于发送不同级别的日志
func (x *LoggingMessageSender) Debug(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelDebug, logger, data)
}

func (x *LoggingMessageSender) Info(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelInfo, logger, data)
}

func (x *LoggingMessageSender) Notice(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelNotice, logger, data)
}

func (x *LoggingMessageSender) Warning(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelWarning, logger, data)
}

func (x *LoggingMessageSender) Error(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelError, logger, data)
}

func (x *LoggingMessageSender) Critical(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelCritical, logger, data)
}

func (x *LoggingMessageSender) Alert(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelAlert, logger, data)
}

func (x *LoggingMessageSender) Emergency(logger string, data interface{}) error {
	return x.SendLogMessage(protocol.LoggingLevelEmergency, logger, data)
}

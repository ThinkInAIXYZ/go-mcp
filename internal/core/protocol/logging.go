package protocol

// LoggingLevel defines the severity of a log message (RFC-5424 syslog message severities)
// LoggingLevel 定义了日志消息的严重性（符合 RFC-5424 系统日志消息严重性标准）
type LoggingLevel string

// Logging levels based on RFC-5424 syslog severity levels
// 基于 RFC-5424 系统日志严重性级别的日志级别
const (
	// Debug-level messages (lowest severity)
	// 调试级别消息（最低严重性）
	LoggingLevelDebug LoggingLevel = "debug"
	// Informational messages
	// 信息性消息
	LoggingLevelInfo LoggingLevel = "info"
	// Normal but significant condition
	// 正常但重要的情况
	LoggingLevelNotice LoggingLevel = "notice"
	// Warning conditions
	// 警告情况
	LoggingLevelWarning LoggingLevel = "warning"
	// Error conditions
	// 错误情况
	LoggingLevelError LoggingLevel = "error"
	// Critical conditions
	// 严重情况
	LoggingLevelCritical LoggingLevel = "critical"
	// Action must be taken immediately
	// 必须立即采取行动
	LoggingLevelAlert LoggingLevel = "alert"
	// System is unusable (highest severity)
	// 系统无法使用（最高严重性）
	LoggingLevelEmergency LoggingLevel = "emergency"
)

// SetLevelRequest is sent from client to server to enable or adjust logging
// SetLevelRequest 是从客户端发送到服务器以启用或调整日志记录
type SetLevelRequest struct {
	// The level of logging that the client wants to receive from the server
	// 客户端希望从服务器接收的日志级别
	Level LoggingLevel `json:"level"`
}

// SetLevelResult is an empty result confirming the logging level change
// SetLevelResult 是一个空的结果，确认日志级别变更
type SetLevelResult struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// LoggingMessageNotification is sent from server to client to report log messages
// LoggingMessageNotification 是从服务器发送到客户端的日志消息通知
type LoggingMessageNotification struct {
	// The severity of this log message
	// 此日志消息的严重性
	Level LoggingLevel `json:"level"`
	// An optional name of the logger issuing this message
	// 发出此消息的可选日志记录器名称
	Logger string `json:"logger,omitempty"`
	// The data to be logged, such as a string message or an object
	// 要记录的数据，如字符串消息或对象
	Data interface{} `json:"data"`
}

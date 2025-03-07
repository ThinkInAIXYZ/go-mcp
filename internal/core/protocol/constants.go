package protocol

// Protocol version for the Model Context Protocol
// 模型上下文协议版本
const ProtocolVersion = "0.5.4"

// JSON-RPC version used by the protocol
// 协议使用的 JSON-RPC 版本
const JsonRpcVersion = "2.0"

// Error codes (as defined in JSON-RPC 2.0) for protocol errors
// 协议错误的错误代码（按照 JSON-RPC 2.0 定义）
const (
	// Standard JSON-RPC error codes
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603

	// MCP specific error codes start at -32000
	ErrorCodeRequestCancelled         = -32000
	ErrorCodeContentModified          = -32001
	ErrorCodeRequestFailed            = -32002
	ErrorCodeServerNotInitialized     = -32003
	ErrorCodeResourceNotFound         = -32004
	ErrorCodeToolNotFound             = -32005
	ErrorCodePromptNotFound           = -32006
	ErrorCodeResourceReadError        = -32007
	ErrorCodeToolExecutionError       = -32008
	ErrorCodePromptExecutionError     = -32009
	ErrorCodeResourceSubscribeError   = -32010
	ErrorCodeResourceUnsubscribeError = -32011
)

// McpMethod is a type representing MCP protocol method names
// McpMethod 是表示 MCP 协议方法名称的类型
type McpMethod string

// Method names for protocol requests
const (
	// Lifecycle methods
	MethodInitialize McpMethod = "initialize"          // Request for initialization
	MethodPing       McpMethod = "ping"                // Ping request to check connection
	MethodComplete   McpMethod = "completion/complete" // Request for completion

	// Tool methods
	MethodListTools McpMethod = "tools/list"
	MethodCallTool  McpMethod = "tools/call"

	// Resource methods
	MethodListResources         McpMethod = "resources/list"
	MethodReadResource          McpMethod = "resources/read"
	MethodSubscribe             McpMethod = "resources/subscribe"
	MethodUnsubscribe           McpMethod = "resources/unsubscribe"
	MethodListResourceTemplates McpMethod = "resources/templates/list"

	// Prompt methods
	MethodListPrompts McpMethod = "prompts/list"
	MethodGetPrompt   McpMethod = "prompts/get"

	// Roots methods
	MethodListRoots McpMethod = "roots/list"

	// Sampling methods
	MethodCreateMessage McpMethod = "sampling/createMessage"

	// Logging methods
	MethodSetLevel McpMethod = "logging/setLevel"
)

// Notification method names
const (
	// Lifecycle notifications
	NotificationInitialized = "notifications/initialized" // Notification after initialization
	NotificationCancelled   = "notifications/cancelled"   // Notification for cancellation
	NotificationProgress    = "notifications/progress"    // Notification for progress updates

	// List changed notifications
	NotificationToolsListChanged     = "notifications/tools/list_changed"
	NotificationResourcesListChanged = "notifications/resources/list_changed"
	NotificationPromptsListChanged   = "notifications/prompts/list_changed"
	NotificationRootsListChanged     = "notifications/roots/list_changed"

	// Content update notifications
	NotificationResourcesUpdated = "notifications/resources/updated"

	// Logging notifications
	NotificationLoggingMessage = "notifications/message" // Logging message notification
)

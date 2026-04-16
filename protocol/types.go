package protocol

const Version = "2025-03-26"

var SupportedVersion = map[string]struct{}{
	"2024-11-05": {},
	"2025-03-26": {},
}

// Method represents the JSON-RPC method name
type Method string

const (
	// Core methods
	Ping                    Method = "ping"
	Initialize              Method = "initialize"
	NotificationInitialized Method = "notifications/initialized"

	// Root related methods
	RootsList                    Method = "roots/list"
	NotificationRootsListChanged Method = "notifications/roots/list_changed"

	// Resource related methods
	ResourcesList                    Method = "resources/list"
	ResourceListTemplates            Method = "resources/templates/list"
	ResourcesRead                    Method = "resources/read"
	ResourcesSubscribe               Method = "resources/subscribe"
	ResourcesUnsubscribe             Method = "resources/unsubscribe"
	NotificationResourcesListChanged Method = "notifications/resources/list_changed"
	NotificationResourcesUpdated     Method = "notifications/resources/updated"

	// Tool related methods
	ToolsList                    Method = "tools/list"
	ToolsCall                    Method = "tools/call"
	NotificationToolsListChanged Method = "notifications/tools/list_changed"

	// Prompt related methods
	PromptsList                    Method = "prompts/list"
	PromptsGet                     Method = "prompts/get"
	NotificationPromptsListChanged Method = "notifications/prompts/list_changed"

	// Sampling related methods
	SamplingCreateMessage Method = "sampling/createMessage"

	// Logging related methods
	LoggingSetLevel        Method = "logging/setLevel"
	NotificationLogMessage Method = "notifications/message"

	// Completion related methods
	CompletionComplete Method = "completion/complete"

	// progress related methods
	NotificationProgress  Method = "notifications/progress"
	NotificationCancelled Method = "notifications/cancelled" // nolint:misspell
)

// Role represents the sender or recipient of messages and data in a conversation
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ClientRequest interface{}

var (
	_ ClientRequest = &InitializeRequest{}
	_ ClientRequest = &PingRequest{}
	_ ClientRequest = &ListPromptsRequest{}
	_ ClientRequest = &GetPromptRequest{}
	_ ClientRequest = &ListResourcesRequest{}
	_ ClientRequest = &ReadResourceRequest{}
	_ ClientRequest = &ListResourceTemplatesRequest{}
	_ ClientRequest = &SubscribeRequest{}
	_ ClientRequest = &UnsubscribeRequest{}
	_ ClientRequest = &ListToolsRequest{}
	_ ClientRequest = &CallToolRequest{}
	_ ClientRequest = &CompleteRequest{}
	_ ClientRequest = &SetLoggingLevelRequest{}
)

type ClientResponse interface{}

var (
	_ ClientResponse = &PingResult{}
	_ ClientResponse = &ListToolsResult{}
	_ ClientResponse = &CreateMessageResult{}
)

type ClientNotify interface{}

var (
	_ ClientNotify = &InitializedNotification{}
	_ ClientNotify = &CancelledNotification{}
	_ ClientNotify = &ProgressNotification{}
	_ ClientNotify = &RootsListChangedNotification{}
)

type ServerRequest interface{}

var (
	_ ServerRequest = &PingRequest{}
	_ ServerRequest = &ListRootsRequest{}
	_ ServerRequest = &CreateMessageRequest{}
)

type ServerResponse interface{}

var (
	_ ServerResponse = &InitializeResult{}
	_ ServerResponse = &PingResult{}
	_ ServerResponse = &ListPromptsResult{}
	_ ServerResponse = &GetPromptResult{}
	_ ServerResponse = &ListResourcesResult{}
	_ ServerResponse = &ReadResourceResult{}
	_ ServerResponse = &ListResourceTemplatesResult{}
	_ ServerResponse = &SubscribeResult{}
	_ ServerResponse = &UnsubscribeResult{}
	_ ServerResponse = &ListToolsResult{}
	_ ServerResponse = &CallToolResult{}
	_ ServerResponse = &CompleteResult{}
	_ ServerResponse = &SetLoggingLevelResult{}
)

type ServerNotify interface{}

var (
	_ ServerNotify = &CancelledNotification{}
	_ ServerNotify = &ProgressNotification{}
	_ ServerNotify = &ToolListChangedNotification{}
	_ ServerNotify = &PromptListChangedNotification{}
	_ ServerNotify = &ResourceListChangedNotification{}
	_ ServerNotify = &ResourceUpdatedNotification{}
	_ ServerNotify = &LogMessageNotification{}
)

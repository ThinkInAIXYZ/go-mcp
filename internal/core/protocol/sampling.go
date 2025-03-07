package protocol

// ModelHint provides hints to use for model selection
// ModelHint 提供了用于模型选择的提示
type ModelHint struct {
	// A hint for a model name (can be a substring like "claude-3-5-sonnet")
	// 模型名称的提示（可以是子字符串，如 "claude-3-5-sonnet"）
	Name string `json:"name,omitempty"`
}

// ModelPreferences defines the server's preferences for model selection
// ModelPreferences 定义了服务器对模型选择的偏好
type ModelPreferences struct {
	// Optional hints to use for model selection (evaluated in order)
	// 用于模型选择的可选提示（按顺序评估）
	Hints []ModelHint `json:"hints,omitempty"`
	// How much to prioritize cost when selecting a model (0 to 1)
	// 选择模型时对成本的优先程度（0至1）
	CostPriority float64 `json:"costPriority,omitempty"`
	// How much to prioritize sampling speed/latency when selecting a model (0 to 1)
	// 选择模型时对采样速度/延迟的优先程度（0至1）
	SpeedPriority float64 `json:"speedPriority,omitempty"`
	// How much to prioritize intelligence/capabilities when selecting a model (0 to 1)
	// 选择模型时对智能/能力的优先程度（0至1）
	IntelligencePriority float64 `json:"intelligencePriority,omitempty"`
}

// ContextInclusion defines what MCP context to include in sampling
// ContextInclusion 定义了要在采样中包含哪些 MCP 上下文
type ContextInclusion string

// Context inclusion options for sampling
// 采样的上下文包含选项
const (
	// Do not include context from any server
	// 不包含来自任何服务器的上下文
	ContextInclusionNone ContextInclusion = "none"
	// Only include context from this server
	// 仅包含来自此服务器的上下文
	ContextInclusionThisServer ContextInclusion = "thisServer"
	// Include context from all connected servers
	// 包含来自所有连接的服务器的上下文
	ContextInclusionAllServers ContextInclusion = "allServers"
)

// CreateMessageRequest is sent from server to sample an LLM via the client
// CreateMessageRequest 是从服务器发送到客户端的模型采样请求
type CreateMessageRequest struct {
	// The conversation history to send to the LLM
	// 要发送到语言模型的对话历史
	Messages []SamplingMessage `json:"messages"`
	// The server's preferences for which model to select (optional)
	// 服务器对要选择的模型的偏好设置（可选）
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	// An optional system prompt the server wants to use for sampling
	// 服务器想要用于采样的可选系统提示
	SystemPrompt string `json:"systemPrompt,omitempty"`
	// A request to include context from one or more MCP servers
	// 请求包含来自一个或多个 MCP 服务器的上下文
	IncludeContext ContextInclusion `json:"includeContext,omitempty"`
	// Temperature setting for sampling (higher = more random)
	// 采样的温度设置（越高 = 越随机）
	Temperature float64 `json:"temperature,omitempty"`
	// The maximum number of tokens to sample
	// 要采样的最大令牌数
	MaxTokens int `json:"maxTokens"`
	// Sequences that will stop generation if encountered
	// 遇到时将停止生成的序列
	StopSequences []string `json:"stopSequences,omitempty"`
	// Optional metadata to pass through to the LLM provider
	// 要传递给语言模型提供商的可选元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// StopReason indicates why sampling stopped
// StopReason 指示为什么采样停止
type StopReason string

// Possible reasons why generation might stop
// 生成可能停止的原因
const (
	// Generation naturally completed a turn
	// 生成自然完成了一个回合
	StopReasonEndTurn StopReason = "endTurn"
	// Generation encountered a stop sequence
	// 生成遇到了停止序列
	StopReasonStopSequence StopReason = "stopSequence"
	// Generation reached the maximum token limit
	// 生成达到了最大令牌限制
	StopReasonMaxTokens StopReason = "maxTokens"
)

// Role represents the sender or recipient of messages in a conversation
// Role 表示对话中消息的发送者或接收者
type Role string

// Possible roles in a conversation
// 对话中可能的角色
const (
	// User role for messages from the user
	// 用户角色的消息
	RoleUser Role = "user"
	// Assistant role for messages from the AI assistant
	// AI助手角色的消息
	RoleAssistant Role = "assistant"
)

// SamplingMessage describes a message issued to or received from an LLM API
// SamplingMessage 描述发送给语言模型 API 或从语言模型 API 接收的消息
type SamplingMessage struct {
	// The role of the message sender
	// 消息发送者的角色
	Role Role `json:"role"`
	// The content of the message (text or image)
	// 消息的内容（文本或图像）
	Content interface{} `json:"content"`
}

// TextContent represents text provided to or from an LLM
// TextContent 表示提供给语言模型或从语言模型提供的文本
type TextContent struct {
	// The type of content (always "text")
	// 内容类型（始终为 "text"）
	Type string `json:"type"`
	// The text content of the message
	// 消息的文本内容
	Text string `json:"text"`
	// Optional annotations for the content
	// 内容的可选注释
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ImageContent represents an image provided to or from an LLM
// ImageContent 表示提供给语言模型或从语言模型提供的图像
type ImageContent struct {
	// The type of content (always "image")
	// 内容类型（始终为 "image"）
	Type string `json:"type"`
	// The base64-encoded image data
	// 经过base64编码的图像数据
	Data string `json:"data"`
	// The MIME type of the image (e.g., "image/jpeg")
	// 图像的MIME类型（如 "image/jpeg"）
	MimeType string `json:"mimeType"`
	// Optional annotations for the content
	// 内容的可选注释
	Annotations *Annotations `json:"annotations,omitempty"`
}

// Annotations provides optional metadata for content items
// Annotations 为内容项提供可选的元数据
type Annotations struct {
	// Describes who the intended customer of this object or data is
	// 描述这个对象或数据的预期客户是谁
	Audience []Role `json:"audience,omitempty"`
	// Describes how important this data is (0-1)
	// 描述这个数据有多重要（0-1）
	Priority float64 `json:"priority,omitempty"`
}

// CreateMessageResult is the client's response to a sampling request
// CreateMessageResult 是客户端对采样请求的响应
type CreateMessageResult struct {
	// The name of the model that generated the message
	// 生成消息的模型名称
	Model string `json:"model"`
	// The role of the generated message (usually "assistant")
	// 生成消息的角色（通常是 "assistant"）
	Role Role `json:"role"`
	// The content of the generated message
	// 生成消息的内容
	Content interface{} `json:"content"` // TextContent or ImageContent
	// The reason why sampling stopped, if known
	// 采样停止的原因（如果已知）
	StopReason string `json:"stopReason,omitempty"`
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

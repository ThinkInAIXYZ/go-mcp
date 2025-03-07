package protocol

// PromptArgument describes an argument that a prompt can accept
// PromptArgument 描述了提示可以接受的参数
type PromptArgument struct {
	// The name of the argument
	// 参数的名称
	Name string `json:"name"`
	// A human-readable description of the argument
	// 参数的可读描述
	Description string `json:"description,omitempty"`
	// Whether this argument must be provided
	// 是否必须提供此参数
	Required bool `json:"required,omitempty"`
}

// Prompt defines a prompt or prompt template that the server offers
// Prompt 定义了服务器提供的提示或提示模板
type Prompt struct {
	// The name of the prompt or prompt template
	// 提示或提示模板的名称
	Name string `json:"name"`
	// An optional description of what this prompt provides
	// 这个提示提供什么的可选描述
	Description string `json:"description,omitempty"`
	// A list of arguments to use for templating the prompt
	// 用于提示模板化的参数列表
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// ListPromptsRequest is sent from client to request a list of prompts from the server
// ListPromptsRequest 是从客户端发送到服务器的请求提示列表的请求
type ListPromptsRequest struct {
	// An opaque token for pagination
	// 用于分页的不透明令牌
	Cursor string `json:"cursor,omitempty"`
}

// ListPromptsResult is the server's response to a prompts/list request
// ListPromptsResult 是服务器对 prompts/list 请求的响应
type ListPromptsResult struct {
	// Array of available prompts
	// 可用提示数组
	Prompts []Prompt `json:"prompts"`
	// Pagination token for fetching the next page of results
	// 获取下一页结果的分页令牌
	NextCursor string `json:"nextCursor,omitempty"`
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// GetPromptRequest is sent from client to get a prompt from the server
// GetPromptRequest 是从客户端发送的获取服务器提示的请求
type GetPromptRequest struct {
	// The name of the prompt or prompt template
	// 提示或提示模板的名称
	Name string `json:"name"`
	// Arguments to use for templating the prompt
	// 用于提示模板化的参数
	Arguments map[string]string `json:"arguments,omitempty"`
}

// We're going to use the Role type from sampling.go

// PromptMessage describes a message returned as part of a prompt
// PromptMessage 描述了作为提示一部分返回的消息
type PromptMessage struct {
	// The role of the message sender
	// 消息发送者的角色
	Role Role `json:"role"`
	// The content of the message (text, image, or resource)
	// 消息的内容（文本、图像或资源）
	Content interface{} `json:"content"` // TextContent, ImageContent, or EmbeddedResource
}

// GetPromptResult is the server's response to a prompts/get request
// GetPromptResult 是服务器对 prompts/get 请求的响应
type GetPromptResult struct {
	// An optional description for the prompt
	// 提示的可选描述
	Description string `json:"description,omitempty"`
	// The messages that make up the prompt
	// 构成提示的消息
	Messages []PromptMessage `json:"messages"`
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// PromptListChangedNotification is sent when the prompt list has changed
// PromptListChangedNotification 是当提示列表变化时发送的通知
type PromptListChangedNotification struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

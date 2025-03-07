package protocol

// Tool defines a tool the client can call
// Tool 定义了客户端可以调用的工具
type Tool struct {
	// The name of the tool
	// 工具的名称
	Name string `json:"name"`
	// A human-readable description of the tool
	// 可读的工具描述
	Description string `json:"description,omitempty"`
	// A JSON Schema object defining the expected parameters for the tool
	// 定义工具参数的 JSON Schema 对象
	InputSchema *InputSchema `json:"inputSchema"`
}

// InputSchema defines the parameter schema for tool calls
// InputSchema 定义了工具调用的参数模式
type InputSchema struct {
	// The type of the input (always "object")
	// 输入类型（始终为 "object"）
	Type string `json:"type"`
	// Properties of the input object
	// 输入对象的属性
	Properties map[string]interface{} `json:"properties,omitempty"`
	// Required properties
	// 必需的属性
	Required []string `json:"required,omitempty"`
}

// ListToolsRequest is sent from client to request a list of tools the server has
// ListToolsRequest 是从客户端发送的查询服务器拥有工具列表的请求
type ListToolsRequest struct {
	// An opaque token for pagination
	// 用于分页的不透明令牌
	Cursor string `json:"cursor,omitempty"`
}

// ListToolsResult is the server's response with available tools
// ListToolsResult 是包含可用工具的服务器响应
type ListToolsResult struct {
	// Array of available tools
	// 可用工具数组
	Tools []Tool `json:"tools"`
	// Pagination token for fetching the next page of results
	// 获取下一页结果的分页令牌
	NextCursor string `json:"nextCursor,omitempty"`
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// CallToolRequest is sent from client to invoke a tool provided by the server
// CallToolRequest 是从客户端发送的调用服务器提供的工具的请求
type CallToolRequest struct {
	// The name of the tool to call
	// 要调用的工具名称
	Name string `json:"name"`
	// Arguments for the tool
	// 工具的参数
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// We'll replace all the content structs with the ones from sampling since they're the same

// EmbeddedResource represents the contents of a resource embedded into a prompt or tool call result
// EmbeddedResource 表示嵌入到提示或工具调用结果中的资源内容
type EmbeddedResource struct {
	// The type of content (always "resource")
	// 内容类型（始终为 "resource"）
	Type string `json:"type"`
	// The resource content
	// 资源内容
	Resource interface{} `json:"resource"` // TextResourceContents or BlobResourceContents
	// Optional annotations for the content
	// 内容的可选注释
	Annotations *Annotations `json:"annotations,omitempty"`
}

// CallToolResult is the server's response to a tool call
// CallToolResult 是服务器对工具调用的响应
type CallToolResult struct {
	// Whether the tool call ended in an error
	// 工具调用是否以错误结束
	IsError bool `json:"isError,omitempty"`
	// Content items returned by the tool (can include text, images, or embedded resources)
	// 工具返回的内容项（可以包括文本、图像或嵌入的资源）
	Content []interface{} `json:"content"` // TextContent, ImageContent, or EmbeddedResource
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// ToolListChangedNotification is sent from server to client when the tool list changes
// ToolListChangedNotification 是当工具列表变化时从服务器发送到客户端的通知
type ToolListChangedNotification struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

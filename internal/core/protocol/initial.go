package protocol

// InitializeRequest is sent from the client to the server when it first connects
// InitializeRequest 是客户端在首次连接时发送给服务器的
type InitializeRequest struct {
	// The latest version of the Model Context Protocol that the client supports
	// 客户端支持的最新版本的模型上下文协议
	ProtocolVersion string `json:"protocolVersion"`
	// Client capability declaration
	// 客户端能力声明
	Capabilities ClientCapabilities `json:"capabilities"`
	// Client implementation information
	// 客户端实现信息
	ClientInfo Implementation `json:"clientInfo"`
}

// InitializeResult is sent from the server after receiving an initialize request
// InitializeResult 是服务器收到初始化请求后发送的响应
type InitializeResult struct {
	// The version of the Model Context Protocol that the server wants to use
	// 服务器想要使用的模型上下文协议版本
	ProtocolVersion string `json:"protocolVersion"`
	// Server capability declaration
	// 服务器能力声明
	Capabilities ServerCapabilities `json:"capabilities"`
	// Server implementation information
	// 服务器实现信息
	ServerInfo Implementation `json:"serverInfo"`
	// Instructions describing how to use the server and its features
	// 描述如何使用服务器及其功能的说明
	Instructions string `json:"instructions,omitempty"`
}

// ClientRoots defines roots capabilities of a client
// ClientRoots 定义了客户端的根目录能力
// The Model Context Protocol (MCP) provides a standardized way for clients to
// expose filesystem "roots" to servers.
// Roots define the boundaries of where servers can operate within the filesystem,
// allowing them to understand which directories and files they have access to.
// Servers can request the list of roots from supporting clients and receive notifications
// when that list changes.
// ref: https://spec.modelcontextprotocol.io/specification/2024-11-05/client/roots/
type ClientRoots struct {
	// Whether the client supports notifications for changes to the roots list
	// 客户端是否支持根目录列表更改的通知
	ListChanged bool `json:"listChanged"`
}

// ClientSampling defines LLM sampling capabilities of a client
// ClientSampling 定义了客户端的语言模型采样能力
// The Model Context Protocol (MCP) provides a standardized way for servers to request
// LLM sampling ("completions" or "generations") from language models via clients.
// This flow allows clients to maintain control over model access, selection, and permissions
// while enabling servers to leverage AI capabilities—with no server API keys necessary.
// Servers can request text or image-based interactions and optionally include context
// from MCP servers in their prompts.
type ClientSampling struct {
	// No properties required by the specification, it's a capability marker
	// 规范不要求属性，它是一个能力标记
}

// Implementation describes the name and version of an MCP implementation
// Implementation 描述了MCP实现的名称和版本
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities defines capabilities a client may support
// ClientCapabilities 定义了客户端可能支持的功能
// ref: https://spec.modelcontextprotocol.io/specification/2024-11-05/client/
type ClientCapabilities struct {
	// Root directory capability, allows servers to request root filesystem
	// 根目录能力，允许服务器请求根文件系统
	Roots *ClientRoots `json:"roots,omitempty"`
	// Used to interact with large language models through the client
	// 用于通过客户端与大型语言模型交互
	Sampling *ClientSampling `json:"sampling,omitempty"`
	// Experimental capabilities that the client supports
	// 客户端支持的实验性功能
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// ServerLogging defines logging capabilities of a server
// ServerLogging 定义了服务器的日志记录能力
type ServerLogging struct {
	// No properties required by the specification, it's a capability marker
	// 规范不要求属性，它是一个能力标记
}

// ServerPrompts defines prompt capabilities of a server
// ServerPrompts 定义了服务器的提示能力
type ServerPrompts struct {
	// Whether this server supports notifications for changes to the prompt list
	// 此服务器是否支持提示列表更改的通知
	ListChanged bool `json:"listChanged"`
}

// ServerResources defines resource capabilities of a server
// ServerResources 定义了服务器的资源能力
type ServerResources struct {
	// Whether this server supports subscribing to resource updates
	// 此服务器是否支持订阅资源更新
	Subscribe bool `json:"subscribe"`
	// Whether this server supports notifications for changes to the resource list
	// 此服务器是否支持资源列表更改的通知
	ListChanged bool `json:"listChanged"`
}

// ServerTools defines tool capabilities of a server
// ServerTools 定义了服务器的工具能力
type ServerTools struct {
	// Whether this server supports notifications for changes to the tool list
	// 此服务器是否支持工具列表更改的通知
	ListChanged bool `json:"listChanged"`
}

// InitializedNotification is sent from the client to the server after initialization has finished
// InitializedNotification 是客户端在初始化完成后发送给服务器的通知
type InitializedNotification struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// ServerCapabilities defines capabilities that a server may support
// ServerCapabilities 定义了服务器可能支持的功能
// ref: https://spec.modelcontextprotocol.io/specification/2024-11-05/server/
// Prompts: Pre-defined templates or instructions that guide language model interactions
// Resources: Structured data or content that provides additional context to the model
// Tools: Executable functions that allow models to perform actions or retrieve information
// | Primitive | Control | Description | Example |
// |-----------|---------|-------------|---------|
// | Prompts | User-controlled | Interactive templates invoked by user choice | Slash commands, menu options |
// | Resources | Application-controlled | Contextual data attached and managed by the client | File contents, git history |
// | Tools | Model-controlled | Functions exposed to the LLM to take actions | API POST requests, file writing |
type ServerCapabilities struct {
	// Logging capabilities
	// 日志记录能力
	Logging *ServerLogging `json:"logging,omitempty"`
	// Prompt template capabilities
	// 提示模板能力
	Prompts *ServerPrompts `json:"prompts,omitempty"`
	// Resource access capabilities
	// 资源访问能力
	Resources *ServerResources `json:"resources,omitempty"`
	// Tool invocation capabilities
	// 工具调用能力
	Tools *ServerTools `json:"tools,omitempty"`
	// Experimental capabilities that the server supports
	// 服务器支持的实验性功能
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

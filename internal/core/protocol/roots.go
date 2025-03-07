package protocol

// Root represents a root directory or file that the server can operate on
// Root 表示服务器可以操作的根目录或文件
type Root struct {
	// The URI identifying the root (must start with file:// for now)
	// 标识根的URI（目前必须以file://开头）
	URI string `json:"uri"`
	// An optional name for the root (for display purposes)
	// 根的可选名称（用于显示目的）
	Name string `json:"name,omitempty"`
}

// ListRootsRequest is sent from the server to request a list of root URIs from the client
// ListRootsRequest 是从服务器发送到客户端的请求，请求根URI列表
type ListRootsRequest struct {
	// Reserved by MCP for additional metadata including progress tracking
	// 保留给MCP用于附加元数据，包括进度跟踪
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// ListRootsResult is the client's response containing a list of roots
// ListRootsResult 是客户端的响应，包含根目录列表
type ListRootsResult struct {
	// Array of Root objects representing available roots
	// 表示可用根的Root对象数组
	Roots []Root `json:"roots"`
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// RPC methods to add or remove roots are not part of the official MCP spec
// The client should use notifications/roots/list_changed to inform the server of changes

// RootsListChangedNotification is sent from client to server when the list of roots has changed
// RootsListChangedNotification 是当根列表发生变化时从客户端发送到服务器的通知
type RootsListChangedNotification struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

package protocol

// Resource represents a known resource that the server is capable of reading
// Resource 表示服务器能够读取的已知资源
type Resource struct {
	// The URI of this resource
	// 这个资源的URI
	URI string `json:"uri"`
	// A human-readable name for this resource
	// 这个资源的可读名称
	Name string `json:"name"`
	// A description of what this resource represents
	// 这个资源表示什么的描述
	Description string `json:"description,omitempty"`
	// The MIME type of this resource, if known
	// 这个资源的MIME类型（如果已知）
	MimeType string `json:"mimeType,omitempty"`
	// The size of the resource in bytes, if known
	// 资源的大小（字节），如果已知
	Size int64 `json:"size,omitempty"`
	// Optional annotations for this resource
	// 这个资源的可选注释
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ResourceTemplate is a template description for resources available on the server
// ResourceTemplate 是服务器上可用资源的模板描述
type ResourceTemplate struct {
	// A URI template that can be used to construct resource URIs
	// 可用于构造资源URI的URI模板
	URITemplate string `json:"uriTemplate"`
	// A human-readable name for the type of resource this template refers to
	// 此模板指向的资源类型的可读名称
	Name string `json:"name"`
	// A description of what this template is for
	// 这个模板的用途描述
	Description string `json:"description,omitempty"`
	// The MIME type for all resources that match this template, if applicable
	// 与此模板匹配的所有资源的MIME类型（如果适用）
	MimeType string `json:"mimeType,omitempty"`
	// Optional annotations for this resource template
	// 这个资源模板的可选注释
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ListResourcesRequest is sent from client to request a list of resources the server has
// ListResourcesRequest 是从客户端发送的请求服务器拥有的资源列表
type ListResourcesRequest struct {
	// An opaque token for pagination
	// 用于分页的不透明令牌
	Cursor string `json:"cursor,omitempty"`
}

// ListResourcesResult is the server's response with available resources
// ListResourcesResult 是包含可用资源的服务器响应
type ListResourcesResult struct {
	// Array of available resources
	// 可用资源数组
	Resources []Resource `json:"resources"`
	// Pagination token for fetching the next page of results
	// 获取下一页结果的分页令牌
	NextCursor string `json:"nextCursor,omitempty"`
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// ReadResourceRequest is sent from client to server to read a specific resource URI
// ReadResourceRequest 是从客户端发送到服务器读取特定资源URI的请求
type ReadResourceRequest struct {
	// The URI of the resource to read
	// 要读取的资源URI
	URI string `json:"uri"`
}

// TextResourceContents represents the text contents of a resource
// TextResourceContents 表示资源的文本内容
type TextResourceContents struct {
	// The URI of this resource
	// 这个资源的URI
	URI string `json:"uri"`
	// The MIME type of this resource, if known
	// 这个资源的MIME类型，如果已知
	MimeType string `json:"mimeType,omitempty"`
	// The text of the item
	// 项目的文本
	Text string `json:"text"`
}

// BlobResourceContents represents the binary contents of a resource
// BlobResourceContents 表示资源的二进制内容
type BlobResourceContents struct {
	// The URI of this resource
	// 这个资源的URI
	URI string `json:"uri"`
	// The MIME type of this resource, if known
	// 这个资源的MIME类型，如果已知
	MimeType string `json:"mimeType,omitempty"`
	// A base64-encoded string representing the binary data of the item
	// 表示项目的二进制数据的base64编码字符串
	Blob string `json:"blob"`
}

// ReadResourceResult is the server's response containing resource contents
// ReadResourceResult 是包含资源内容的服务器响应
type ReadResourceResult struct {
	// Array of resource contents
	// 资源内容数组
	Contents []interface{} `json:"contents"` // TextResourceContents or BlobResourceContents
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// SubscribeRequest is sent to request notifications when a resource is updated
// SubscribeRequest 是发送的请求，当资源更新时收到通知
type SubscribeRequest struct {
	// The URI of the resource to subscribe to
	// 要订阅的资源URI
	URI string `json:"uri"`
}

// SubscribeResult confirms the subscription
// SubscribeResult 确认订阅
type SubscribeResult struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// UnsubscribeRequest is sent to cancel subscription notifications
// UnsubscribeRequest 是发送的用于取消订阅通知的请求
type UnsubscribeRequest struct {
	// The URI of the resource to unsubscribe from
	// 要取消订阅的资源URI
	URI string `json:"uri"`
}

// UnsubscribeResult confirms the unsubscription
// UnsubscribeResult 确认取消订阅
type UnsubscribeResult struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// ResourceUpdatedNotification is sent to notify about a resource update
// ResourceUpdatedNotification 是发送的用于通知资源更新的消息
type ResourceUpdatedNotification struct {
	// The URI of the resource that has been updated
	// 已更新的资源URI
	URI string `json:"uri"`
}

// ResourceListChangedNotification is sent when the resource list changes
// ResourceListChangedNotification 是当资源列表变化时发送的通知
type ResourceListChangedNotification struct {
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// ListResourceTemplatesRequest is sent to request a list of resource templates
// ListResourceTemplatesRequest 是发送的请求资源模板列表的请求
type ListResourceTemplatesRequest struct {
	// An opaque token for pagination
	// 用于分页的不透明令牌
	Cursor string `json:"cursor,omitempty"`
}

// ListResourceTemplatesResult contains resource templates from the server
// ListResourceTemplatesResult 包含服务器的资源模板
type ListResourceTemplatesResult struct {
	// Array of available resource templates
	// 可用资源模板数组
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	// Pagination token for fetching the next page of results
	// 获取下一页结果的分页令牌
	NextCursor string `json:"nextCursor,omitempty"`
	// Reserved by MCP for additional metadata
	// 保留给MCP用于附加元数据
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

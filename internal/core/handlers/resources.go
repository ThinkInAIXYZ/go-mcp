package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
	"strings"
)

// ListResourcesHandler 处理resources/list请求
type ListResourcesHandler struct {
}

// NewListResourcesHandler 创建一个ListResourcesHandler实例
func NewListResourcesHandler() *ListResourcesHandler {
	return &ListResourcesHandler{}
}

// Handle 处理resources/list请求
func (x *ListResourcesHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.ListResourcesRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 示例资源列表，在实际应用中应该从服务或数据库获取
	resources := []protocol.Resource{
		{
			URI:         "file:///example/document.txt",
			Name:        "示例文档",
			Description: "一个简单的文本文档示例",
			MimeType:    "text/plain",
		},
		{
			URI:         "file:///example/image.png",
			Name:        "示例图片",
			Description: "一个简单的图片示例",
			MimeType:    "image/png",
		},
		{
			URI:         "db://users/schema",
			Name:        "用户表结构",
			Description: "用户数据库表结构",
			MimeType:    "text/plain",
		},
	}

	result := protocol.ListResourcesResult{
		Resources: resources,
	}

	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *ListResourcesHandler) Method() protocol.McpMethod {
	return protocol.MethodListResources
}

// ReadResourceHandler 处理resources/read请求
type ReadResourceHandler struct {
}

// NewReadResourceHandler 创建一个ReadResourceHandler实例
func NewReadResourceHandler() *ReadResourceHandler {
	return &ReadResourceHandler{}
}

// Handle 处理resources/read请求
func (x *ReadResourceHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.ReadResourceRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	var contents []interface{}

	// 根据URI读取相应的资源内容
	if strings.HasPrefix(req.URI, "file:///example/document.txt") {
		contents = append(contents, protocol.TextResourceContents{
			URI:      req.URI,
			MimeType: "text/plain",
			Text:     "这是一个示例文档的内容。\n它包含多行文本。\n用于演示MCP资源功能。",
		})
	} else if strings.HasPrefix(req.URI, "file:///example/image.png") {
		// 通常这里应该读取并base64编码图片，这里只是示例
		contents = append(contents, protocol.BlobResourceContents{
			URI:      req.URI,
			MimeType: "image/png",
			Blob:     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==", // 1x1 transparent PNG
		})
	} else if strings.HasPrefix(req.URI, "db://users/schema") {
		contents = append(contents, protocol.TextResourceContents{
			URI:      req.URI,
			MimeType: "text/plain",
			Text:     "CREATE TABLE users (\n  id INTEGER PRIMARY KEY,\n  username TEXT NOT NULL,\n  email TEXT NOT NULL,\n  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP\n);",
		})
	} else {
		return nil, fmt.Errorf("resource not found: %s", req.URI)
	}

	result := protocol.ReadResourceResult{
		Contents: contents,
	}

	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *ReadResourceHandler) Method() protocol.McpMethod {
	return protocol.MethodReadResource
}

// ListResourceTemplatesHandler 处理resources/templates/list请求
type ListResourceTemplatesHandler struct {
}

// NewListResourceTemplatesHandler 创建一个ListResourceTemplatesHandler实例
func NewListResourceTemplatesHandler() *ListResourceTemplatesHandler {
	return &ListResourceTemplatesHandler{}
}

// Handle 处理resources/templates/list请求
func (x *ListResourceTemplatesHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.ListResourceTemplatesRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 示例资源模板列表，在实际应用中应该从服务或数据库获取
	templates := []protocol.ResourceTemplate{
		{
			URITemplate: "file:///example/{filename}",
			Name:        "示例文件",
			Description: "访问示例文件目录中的文件",
		},
		{
			URITemplate: "db://{table}/schema",
			Name:        "数据库表结构",
			Description: "访问数据库表结构",
			MimeType:    "text/plain",
		},
	}

	result := protocol.ListResourceTemplatesResult{
		ResourceTemplates: templates,
	}

	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *ListResourceTemplatesHandler) Method() protocol.McpMethod {
	return protocol.MethodListResourceTemplates
}

// SubscribeHandler 处理resources/subscribe请求
type SubscribeHandler struct {
}

// NewSubscribeHandler 创建一个SubscribeHandler实例
func NewSubscribeHandler() *SubscribeHandler {
	return &SubscribeHandler{}
}

// Handle 处理resources/subscribe请求
func (x *SubscribeHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.SubscribeRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 这里应该实现实际的订阅逻辑，例如将URI添加到订阅列表中
	// 在本示例中，我们只是返回一个成功响应

	result := protocol.SubscribeResult{}
	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *SubscribeHandler) Method() protocol.McpMethod {
	return protocol.MethodSubscribe
}

// UnsubscribeHandler 处理resources/unsubscribe请求
type UnsubscribeHandler struct {
}

// NewUnsubscribeHandler 创建一个UnsubscribeHandler实例
func NewUnsubscribeHandler() *UnsubscribeHandler {
	return &UnsubscribeHandler{}
}

// Handle 处理resources/unsubscribe请求
func (x *UnsubscribeHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.UnsubscribeRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 这里应该实现实际的取消订阅逻辑，例如从订阅列表中移除URI
	// 在本示例中，我们只是返回一个成功响应

	result := protocol.UnsubscribeResult{}
	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *UnsubscribeHandler) Method() protocol.McpMethod {
	return protocol.MethodUnsubscribe
}

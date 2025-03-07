package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
)

// ListPromptsHandler 处理prompts/list请求
type ListPromptsHandler struct {
}

// NewListPromptsHandler 创建一个ListPromptsHandler实例
func NewListPromptsHandler() *ListPromptsHandler {
	return &ListPromptsHandler{}
}

// Handle 处理prompts/list请求
func (x *ListPromptsHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.ListPromptsRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 示例提示列表，在实际应用中应该从服务或数据库获取
	prompts := []protocol.Prompt{
		{
			Name:        "greeting",
			Description: "根据名字生成问候语",
			Arguments: []protocol.PromptArgument{
				{
					Name:        "name",
					Description: "要问候的人的名字",
					Required:    true,
				},
				{
					Name:        "formal",
					Description: "是否使用正式措辞",
					Required:    false,
				},
			},
		},
		{
			Name:        "codeReview",
			Description: "对代码进行专业评审",
			Arguments: []protocol.PromptArgument{
				{
					Name:        "language",
					Description: "编程语言",
					Required:    true,
				},
				{
					Name:        "code",
					Description: "要评审的代码",
					Required:    true,
				},
				{
					Name:        "focus",
					Description: "评审重点 (性能, 安全, 风格等)",
					Required:    false,
				},
			},
		},
	}

	result := protocol.ListPromptsResult{
		Prompts: prompts,
	}

	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *ListPromptsHandler) Method() protocol.McpMethod {
	return protocol.MethodListPrompts
}

// GetPromptHandler 处理prompts/get请求
type GetPromptHandler struct {
}

// NewGetPromptHandler 创建一个GetPromptHandler实例
func NewGetPromptHandler() *GetPromptHandler {
	return &GetPromptHandler{}
}

// Handle 处理prompts/get请求
func (x *GetPromptHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.GetPromptRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	var description string
	var messages []protocol.PromptMessage

	// 根据提示名称生成相应的内容
	switch req.Name {
	case "greeting":
		description, messages = x.handleGreeting(req.Arguments)
	case "codeReview":
		description, messages = x.handleCodeReview(req.Arguments)
	default:
		return nil, fmt.Errorf("未知提示：%s", req.Name)
	}

	result := protocol.GetPromptResult{
		Description: description,
		Messages:    messages,
	}

	return json.Marshal(result)
}

// handleGreeting 处理greeting提示
func (x *GetPromptHandler) handleGreeting(args map[string]string) (string, []protocol.PromptMessage) {
	name, ok := args["name"]
	if !ok {
		name = "there"
	}

	formal := args["formal"] == "true"

	var greeting string
	if formal {
		greeting = "致敬"
	} else {
		greeting = "你好"
	}

	description := fmt.Sprintf("对%s的%s问候", name, map[bool]string{true: "正式", false: "非正式"}[formal])

	messages := []protocol.PromptMessage{
		{
			Role: protocol.RoleUser,
			Content: protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%s，%s！请问我能帮您做些什么？", greeting, name),
			},
		},
	}

	return description, messages
}

// handleCodeReview 处理codeReview提示
func (x *GetPromptHandler) handleCodeReview(args map[string]string) (string, []protocol.PromptMessage) {
	language, ok := args["language"]
	if !ok {
		language = "未知语言"
	}

	code, ok := args["code"]
	if !ok {
		code = "// 无代码提供"
	}

	focus := args["focus"]
	if focus == "" {
		focus = "一般性评审"
	}

	description := fmt.Sprintf("对%s代码进行%s评审", language, focus)

	messages := []protocol.PromptMessage{
		{
			Role: protocol.RoleUser,
			Content: protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("请对以下%s代码进行专业评审，重点关注%s方面：\n\n```%s\n%s\n```",
					language, focus, language, code),
			},
		},
	}

	return description, messages
}

// Method 返回此处理程序对应的MCP方法
func (x *GetPromptHandler) Method() protocol.McpMethod {
	return protocol.MethodGetPrompt
}

package handlers

import (
	"context"
	"encoding/json"
	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
)

// ListToolsHandler 处理tools/list请求
type ListToolsHandler struct {
}

// NewListToolsHandler 创建一个ListToolsHandler实例
func NewListToolsHandler() *ListToolsHandler {
	return &ListToolsHandler{}
}

// Handle 处理tools/list请求
func (x *ListToolsHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.ListToolsRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	// 示例工具列表，在实际应用中应该从服务或数据库获取
	tools := []protocol.Tool{
		{
			Name:        "calculator",
			Description: "执行简单的数学运算",
			InputSchema: &protocol.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"operation": map[string]string{
						"type":        "string",
						"description": "要执行的操作，例如 add, subtract, multiply, divide",
					},
					"a": map[string]string{
						"type":        "number",
						"description": "第一个操作数",
					},
					"b": map[string]string{
						"type":        "number",
						"description": "第二个操作数",
					},
				},
				Required: []string{"operation", "a", "b"},
			},
		},
		{
			Name:        "weather",
			Description: "获取指定城市的天气信息",
			InputSchema: &protocol.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"city": map[string]string{
						"type":        "string",
						"description": "城市名称",
					},
				},
				Required: []string{"city"},
			},
		},
	}

	result := protocol.ListToolsResult{
		Tools: tools,
	}

	return json.Marshal(result)
}

// Method 返回此处理程序对应的MCP方法
func (x *ListToolsHandler) Method() protocol.McpMethod {
	return protocol.MethodListTools
}

// CallToolHandler 处理tools/call请求
type CallToolHandler struct {
}

// NewCallToolHandler 创建一个CallToolHandler实例
func NewCallToolHandler() *CallToolHandler {
	return &CallToolHandler{}
}

// Handle 处理tools/call请求
func (x *CallToolHandler) Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error) {
	var req protocol.CallToolRequest
	err := json.Unmarshal(message, &req)
	if err != nil {
		return nil, err
	}

	var content []interface{}
	isError := false

	// 根据工具名称执行相应的操作
	switch req.Name {
	case "calculator":
		content, isError = x.handleCalculator(req.Arguments)
	case "weather":
		content, isError = x.handleWeather(req.Arguments)
	default:
		content = []interface{}{
			protocol.TextContent{
				Type: "text",
				Text: "未知工具：" + req.Name,
			},
		}
		isError = true
	}

	result := protocol.CallToolResult{
		IsError: isError,
		Content: content,
	}

	return json.Marshal(result)
}

// handleCalculator 处理计算器工具调用
func (x *CallToolHandler) handleCalculator(args map[string]interface{}) ([]interface{}, bool) {
	operation, ok := args["operation"].(string)
	if !ok {
		return []interface{}{
			protocol.TextContent{
				Type: "text",
				Text: "缺少operation参数或类型不正确",
			},
		}, true
	}

	a, okA := args["a"].(float64)
	b, okB := args["b"].(float64)
	if !okA || !okB {
		return []interface{}{
			protocol.TextContent{
				Type: "text",
				Text: "缺少a或b参数，或类型不正确",
			},
		}, true
	}

	var result float64
	var op string

	switch operation {
	case "add":
		result = a + b
		op = "+"
	case "subtract":
		result = a - b
		op = "-"
	case "multiply":
		result = a * b
		op = "*"
	case "divide":
		if b == 0 {
			return []interface{}{
				protocol.TextContent{
					Type: "text",
					Text: "除数不能为零",
				},
			}, true
		}
		result = a / b
		op = "/"
	default:
		return []interface{}{
			protocol.TextContent{
				Type: "text",
				Text: "不支持的操作：" + operation,
			},
		}, true
	}

	return []interface{}{
		protocol.TextContent{
			Type: "text",
			Text: "计算结果：" + jsonNumber(a) + " " + op + " " + jsonNumber(b) + " = " + jsonNumber(result),
		},
	}, false
}

// handleWeather 处理天气工具调用
func (x *CallToolHandler) handleWeather(args map[string]interface{}) ([]interface{}, bool) {
	city, ok := args["city"].(string)
	if !ok {
		return []interface{}{
			protocol.TextContent{
				Type: "text",
				Text: "缺少city参数或类型不正确",
			},
		}, true
	}

	// 这里应该调用实际的天气API，这里只是模拟
	weatherInfo := map[string]string{
		"北京": "晴天，温度28°C，东南风3级",
		"上海": "多云，温度25°C，东风2级",
		"广州": "雨天，温度30°C，无风",
	}

	info, exists := weatherInfo[city]
	if !exists {
		return []interface{}{
			protocol.TextContent{
				Type: "text",
				Text: "没有找到城市 " + city + " 的天气信息",
			},
		}, true
	}

	return []interface{}{
		protocol.TextContent{
			Type: "text",
			Text: city + "的天气：" + info,
		},
	}, false
}

// Method 返回此处理程序对应的MCP方法
func (x *CallToolHandler) Method() protocol.McpMethod {
	return protocol.MethodCallTool
}

// jsonNumber 将数字转换为字符串，避免精度问题
func jsonNumber(num float64) string {
	bytes, _ := json.Marshal(num)
	return string(bytes)
}

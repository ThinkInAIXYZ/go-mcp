package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/wangle201210/text2sql"
	"github.com/wangle201210/text2sql/eino"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

func getText2sqlTool() *protocol.Tool {
	return &protocol.Tool{
		Name:        "text2sql",
		Description: "Natural language to database query",
		InputSchema: protocol.InputSchema{
			Type: protocol.Object,
			Properties: map[string]*protocol.Property{
				"question": {
					Type:        "string",
					Description: "Questions to query data",
				},
			},
			Required: []string{"question"},
		},
	}
}

func text2sqlHandler(request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	link := os.Getenv("link")
	if link == "" {
		return nil, fmt.Errorf("link is empty")
	}
	question, ok := request.Arguments["question"].(string)
	if !ok {
		return nil, errors.New("question must be a string")
	}
	cfg := &text2sql.Config{
		DbLink:    link,
		ShouldRun: true,
		Times:     3,
		Try:       3,
	}
	newEino, err := eino.NewEino(&openai.ChatModelConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		Model:   os.Getenv("OPENAI_MODEL_NAME"),
	})
	if err != nil {
		return nil, fmt.Errorf("eino err: %+v", err)
	}
	ts := text2sql.NewText2sql(cfg, newEino)
	sql, result, err := ts.Pretty(question)
	if err != nil {
		return nil, fmt.Errorf("text2sql err: %+v", err)
	}
	res := fmt.Sprintf("sql: %s, result: %s", sql, result)
	return &protocol.CallToolResult{
		Content: []protocol.Content{
			protocol.TextContent{
				Type: "text",
				Text: res,
			},
		},
	}, nil
}

package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

// ListPromptsRequest represents a request to list available prompts
type ListPromptsRequest struct {
	Cursor Cursor `json:"cursor,omitempty"`
}

// ListPromptsResult represents the response to a list prompts request
type ListPromptsResult struct {
	Prompts    []*Prompt `json:"prompts"`
	NextCursor Cursor    `json:"nextCursor,omitempty"`
}

// Prompt related types
type Prompt struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Arguments   []*PromptArgument `json:"arguments,omitempty"`
}

func (p *Prompt) GetName() string {
	return p.Name
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// GetPromptRequest represents a request to get a specific prompt
type GetPromptRequest struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// GetPromptResult represents the response to a get prompt request
type GetPromptResult struct {
	Messages    []*PromptMessage `json:"messages"`
	Description string           `json:"description,omitempty"`
}

type PromptMessage struct {
	Role    Role    `json:"role"`
	Content Content `json:"content"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for PromptMessage
func (m *PromptMessage) UnmarshalJSON(data []byte) error {
	type Alias PromptMessage
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := pkg.JSONUnmarshal(data, &aux); err != nil {
		return err
	}

	// Try to unmarshal content as TextContent first
	var textContent *TextContent
	if err := pkg.JSONUnmarshal(aux.Content, &textContent); err == nil {
		m.Content = textContent
		return nil
	}

	// Try to unmarshal content as ImageContent
	var imageContent *ImageContent
	if err := pkg.JSONUnmarshal(aux.Content, &imageContent); err == nil {
		m.Content = imageContent
		return nil
	}

	// Try to unmarshal content as AudioContent
	var audioContent *AudioContent
	if err := pkg.JSONUnmarshal(aux.Content, &audioContent); err == nil {
		m.Content = audioContent
		return nil
	}

	// Try to unmarshal content as embeddedResource
	var embeddedResource *EmbeddedResource
	if err := pkg.JSONUnmarshal(aux.Content, &embeddedResource); err == nil {
		m.Content = embeddedResource
		return nil
	}

	return fmt.Errorf("unknown content type")
}

// PromptListChangedNotification represents a notification that the prompt list has changed
type PromptListChangedNotification struct {
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// NewListPromptsRequest creates a new list prompts request
func NewListPromptsRequest() *ListPromptsRequest {
	return &ListPromptsRequest{}
}

// NewListPromptsResult creates a new list prompts response
func NewListPromptsResult(prompts []*Prompt, nextCursor Cursor) *ListPromptsResult {
	return &ListPromptsResult{
		Prompts:    prompts,
		NextCursor: nextCursor,
	}
}

// NewGetPromptRequest creates a new get prompt request
func NewGetPromptRequest(name string, arguments map[string]string) *GetPromptRequest {
	return &GetPromptRequest{
		Name:      name,
		Arguments: arguments,
	}
}

// NewGetPromptResult creates a new get prompt response
func NewGetPromptResult(messages []*PromptMessage, description string) *GetPromptResult {
	return &GetPromptResult{
		Messages:    messages,
		Description: description,
	}
}

// NewPromptListChangedNotification creates a new prompt list changed notification
func NewPromptListChangedNotification() *PromptListChangedNotification {
	return &PromptListChangedNotification{}
}

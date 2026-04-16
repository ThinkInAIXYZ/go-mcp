package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

// CreateMessageRequest represents a request to create a message through sampling
type CreateMessageRequest struct {
	Messages         []*SamplingMessage     `json:"messages"`
	MaxTokens        int                    `json:"maxTokens"`
	Temperature      float64                `json:"temperature,omitempty"`
	StopSequences    []string               `json:"stopSequences,omitempty"`
	SystemPrompt     string                 `json:"systemPrompt,omitempty"`
	ModelPreferences *ModelPreferences      `json:"modelPreferences,omitempty"`
	IncludeContext   string                 `json:"includeContext,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type SamplingMessage struct {
	Role    Role    `json:"role"`
	Content Content `json:"content"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for SamplingMessage
func (r *SamplingMessage) UnmarshalJSON(data []byte) error {
	type Alias SamplingMessage
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := pkg.JSONUnmarshal(data, &aux); err != nil {
		return err
	}

	// Try to unmarshal content as TextContent first
	var textContent *TextContent
	if err := pkg.JSONUnmarshal(aux.Content, &textContent); err == nil {
		r.Content = textContent
		return nil
	}

	// Try to unmarshal content as ImageContent
	var imageContent *ImageContent
	if err := pkg.JSONUnmarshal(aux.Content, &imageContent); err == nil {
		r.Content = imageContent
		return nil
	}

	// Try to unmarshal content as AudioContent
	var audioContent *AudioContent
	if err := pkg.JSONUnmarshal(aux.Content, &audioContent); err == nil {
		r.Content = audioContent
		return nil
	}

	return fmt.Errorf("unknown content type, content=%s", aux.Content)
}

// CreateMessageResult represents the response to a create message request
type CreateMessageResult struct {
	Content    Content `json:"content"`
	Role       Role    `json:"role"`
	Model      string  `json:"model"`
	StopReason string  `json:"stopReason,omitempty"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for CreateMessageResult
func (r *CreateMessageResult) UnmarshalJSON(data []byte) error {
	type Alias CreateMessageResult
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := pkg.JSONUnmarshal(data, &aux); err != nil {
		return err
	}

	// Try to unmarshal content as TextContent first
	var textContent *TextContent
	if err := pkg.JSONUnmarshal(aux.Content, &textContent); err == nil {
		r.Content = textContent
		return nil
	}

	// Try to unmarshal content as ImageContent
	var imageContent *ImageContent
	if err := pkg.JSONUnmarshal(aux.Content, &imageContent); err == nil {
		r.Content = imageContent
		return nil
	}

	// Try to unmarshal content as AudioContent
	var audioContent *AudioContent
	if err := pkg.JSONUnmarshal(aux.Content, &audioContent); err == nil {
		r.Content = audioContent
		return nil
	}

	return fmt.Errorf("unknown content type, content=%s", aux.Content)
}

// NewCreateMessageRequest creates a new create message request
func NewCreateMessageRequest(messages []*SamplingMessage, maxTokens int, opts ...CreateMessageOption) *CreateMessageRequest {
	req := &CreateMessageRequest{
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewCreateMessageResult creates a new create message response
func NewCreateMessageResult(content Content, role Role, model string, stopReason string) *CreateMessageResult {
	return &CreateMessageResult{
		Content:    content,
		Role:       role,
		Model:      model,
		StopReason: stopReason,
	}
}

// CreateMessageOption represents an option for creating a message
type CreateMessageOption func(*CreateMessageRequest)

// WithTemperature sets the temperature for the request
func WithTemperature(temp float64) CreateMessageOption {
	return func(r *CreateMessageRequest) {
		r.Temperature = temp
	}
}

// WithStopSequences sets the stop sequences for the request
func WithStopSequences(sequences []string) CreateMessageOption {
	return func(r *CreateMessageRequest) {
		r.StopSequences = sequences
	}
}

// WithSystemPrompt sets the system prompt for the request
func WithSystemPrompt(prompt string) CreateMessageOption {
	return func(r *CreateMessageRequest) {
		r.SystemPrompt = prompt
	}
}

// WithModelPreferences sets the model preferences for the request
func WithModelPreferences(prefs *ModelPreferences) CreateMessageOption {
	return func(r *CreateMessageRequest) {
		r.ModelPreferences = prefs
	}
}

// WithIncludeContext sets the include context option for the request
func WithIncludeContext(ctx string) CreateMessageOption {
	return func(r *CreateMessageRequest) {
		r.IncludeContext = ctx
	}
}

// WithMetadata sets the metadata for the request
func WithMetadata(metadata map[string]interface{}) CreateMessageOption {
	return func(r *CreateMessageRequest) {
		r.Metadata = metadata
	}
}

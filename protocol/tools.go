package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

// ListToolsRequest represents a request to list available tools
type ListToolsRequest struct {
	Cursor Cursor `json:"cursor,omitempty"`
}

// ListToolsResult represents the response to a list tools request
type ListToolsResult struct {
	Tools      []*Tool `json:"tools"`
	NextCursor Cursor  `json:"nextCursor,omitempty"`
}

// ToolAnnotations contains hints about the tool's behavior
type ToolAnnotations struct {
	// Title is a human-readable title for the tool, useful for UI display
	Title string `json:"title,omitempty"`

	// ReadOnlyHint indicates the tool does not modify its environment
	ReadOnlyHint *bool `json:"readOnlyHint,omitempty"`

	// DestructiveHint indicates the tool may perform destructive updates
	// (only meaningful when ReadOnlyHint is false)
	DestructiveHint *bool `json:"destructiveHint,omitempty"`

	// IdempotentHint indicates calling the tool repeatedly with the same arguments
	// has no additional effect (only meaningful when ReadOnlyHint is false)
	IdempotentHint *bool `json:"idempotentHint,omitempty"`

	// OpenWorldHint indicates the tool may interact with an "open world" of external entities
	OpenWorldHint *bool `json:"openWorldHint,omitempty"`
}

// Tool represents a tool definition that the client can call
type Tool struct {
	// Name is the unique identifier of the tool
	Name string `json:"name"`

	// Description is a human-readable description of the tool
	Description string `json:"description,omitempty"`

	// InputSchema defines the expected parameters for the tool using JSON Schema
	InputSchema InputSchema `json:"inputSchema"`

	// Annotations provides additional hints about the tool's behavior
	Annotations *ToolAnnotations `json:"annotations,omitempty"`

	RawInputSchema json.RawMessage `json:"-"`
}

/*func (t *Tool) GetName() string {
	return t.Name
}*/

func (t *Tool) GetName() string {
	return t.Name
}

func (t *Tool) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{}, 4)

	m["name"] = t.Name
	if t.Description != "" {
		m["description"] = t.Description
	}

	// Determine which schema to use
	if t.RawInputSchema != nil {
		if t.InputSchema.Type != "" {
			return nil, fmt.Errorf("inputSchema field conflict")
		}
		m["inputSchema"] = t.RawInputSchema
	} else {
		// Use the structured InputSchema
		m["inputSchema"] = t.InputSchema
	}

	// Add annotations if present
	if t.Annotations != nil {
		m["annotations"] = t.Annotations
	}

	return json.Marshal(m)
}

type InputSchemaType string

const Object InputSchemaType = "object"

// InputSchema represents a JSON Schema object defining the expected parameters for a tool
type InputSchema struct {
	Type       InputSchemaType      `json:"type"`
	Properties map[string]*Property `json:"properties,omitempty"`
	Required   []string             `json:"required,omitempty"`
}

// CallToolRequest represents a request to call a specific tool
type CallToolRequest struct {
	Meta         map[string]interface{} `json:"_meta,omitempty"`
	Name         string                 `json:"name"`
	Arguments    map[string]interface{} `json:"arguments,omitempty"`
	RawArguments json.RawMessage        `json:"-"`
}

func (r *CallToolRequest) UnmarshalJSON(data []byte) error {
	type alias CallToolRequest
	temp := &struct {
		Arguments json.RawMessage `json:"arguments,omitempty"`
		*alias
	}{
		alias: (*alias)(r),
	}

	if err := pkg.JSONUnmarshal(data, temp); err != nil {
		return err
	}

	r.RawArguments = temp.Arguments

	if len(r.RawArguments) != 0 {
		if err := pkg.JSONUnmarshal(r.RawArguments, &r.Arguments); err != nil {
			return err
		}
	}

	return nil
}

func (r *CallToolRequest) MarshalJSON() ([]byte, error) {
	type alias CallToolRequest
	temp := &struct {
		Arguments json.RawMessage `json:"arguments,omitempty"`
		*alias
	}{
		alias: (*alias)(r),
	}

	if len(r.RawArguments) > 0 {
		temp.Arguments = r.RawArguments
	} else if r.Arguments != nil {
		var err error
		temp.Arguments, err = json.Marshal(r.Arguments)
		if err != nil {
			return nil, err
		}
	}

	return json.Marshal(temp)
}

// CallToolResult represents the response to a tool call
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for CallToolResult
func (r *CallToolResult) UnmarshalJSON(data []byte) error {
	type Alias CallToolResult
	aux := &struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := pkg.JSONUnmarshal(data, &aux); err != nil {
		return err
	}

	r.Content = make([]Content, len(aux.Content))
	for i, content := range aux.Content {
		// Try to unmarshal content as TextContent first
		var textContent *TextContent
		if err := pkg.JSONUnmarshal(content, &textContent); err == nil {
			r.Content[i] = textContent
			continue
		}

		// Try to unmarshal content as ImageContent
		var imageContent *ImageContent
		if err := pkg.JSONUnmarshal(content, &imageContent); err == nil {
			r.Content[i] = imageContent
			continue
		}

		// Try to unmarshal content as AudioContent
		var audioContent *AudioContent
		if err := pkg.JSONUnmarshal(content, &audioContent); err == nil {
			r.Content[i] = audioContent
			continue
		}

		// Try to unmarshal content as embeddedResource
		var embeddedResource *EmbeddedResource
		if err := pkg.JSONUnmarshal(content, &embeddedResource); err == nil {
			r.Content[i] = embeddedResource
			return nil
		}

		return fmt.Errorf("unknown content type at index %d", i)
	}

	return nil
}

// ToolListChangedNotification represents a notification that the tool list has changed
type ToolListChangedNotification struct {
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// NewTool create a tool
func NewTool(name string, description string, inputReqStruct interface{}) (*Tool, error) {
	schema, err := generateSchemaFromReqStruct(inputReqStruct)
	if err != nil {
		return nil, err
	}

	return &Tool{
		Name:        name,
		Description: description,
		InputSchema: *schema,
	}, nil
}

func NewToolWithRawSchema(name, description string, schema json.RawMessage) *Tool {
	return &Tool{
		Name:           name,
		Description:    description,
		RawInputSchema: schema,
	}
}

// NewListToolsRequest creates a new list tools request
func NewListToolsRequest() *ListToolsRequest {
	return &ListToolsRequest{}
}

// NewListToolsResult creates a new list tools response
func NewListToolsResult(tools []*Tool, nextCursor Cursor) *ListToolsResult {
	return &ListToolsResult{
		Tools:      tools,
		NextCursor: nextCursor,
	}
}

// NewCallToolRequest creates a new call tool request
func NewCallToolRequest(name string, arguments map[string]interface{}) *CallToolRequest {
	return &CallToolRequest{
		Name:      name,
		Arguments: arguments,
	}
}

func NewCallToolRequestWithRawArguments(name string, rawArguments json.RawMessage) *CallToolRequest {
	return &CallToolRequest{
		Name:         name,
		RawArguments: rawArguments,
	}
}

// NewCallToolResult creates a new call tool response
func NewCallToolResult(content []Content, isError bool) *CallToolResult {
	return &CallToolResult{
		Content: content,
		IsError: isError,
	}
}

// NewToolListChangedNotification creates a new tool list changed notification
func NewToolListChangedNotification() *ToolListChangedNotification {
	return &ToolListChangedNotification{}
}

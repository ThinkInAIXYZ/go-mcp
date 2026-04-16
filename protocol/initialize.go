package protocol

import (
	"encoding/json"

	"github.com/tidwall/gjson"
)

// InitializeRequest represents the initialize request sent from client to server
type InitializeRequest struct {
	ClientInfo      *Implementation     `json:"clientInfo"`
	Capabilities    *ClientCapabilities `json:"capabilities"`
	ProtocolVersion string              `json:"protocolVersion"`
}

// InitializeResult represents the server's response to an initialize request
type InitializeResult struct {
	ServerInfo      *Implementation     `json:"serverInfo"`
	Capabilities    *ServerCapabilities `json:"capabilities"`
	ProtocolVersion string              `json:"protocolVersion"`
	Instructions    string              `json:"instructions,omitempty"`
}

// Implementation describes the name and version of an MCP implementation
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities capabilities
type ClientCapabilities struct {
	// Experimental map[string]interface{} `json:"experimental,omitempty"`
	// Roots        *RootsCapability       `json:"roots,omitempty"`
	Sampling interface{} `json:"sampling,omitempty"`
}

type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ServerCapabilities struct {
	// Experimental map[string]interface{} `json:"experimental,omitempty"`
	// Logging      interface{}            `json:"logging,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
	Subscribe   bool `json:"subscribe,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializedNotification represents the notification sent from client to server after initialization
type InitializedNotification struct {
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// NewInitializeRequest creates a new initialize request
func NewInitializeRequest(clientInfo *Implementation, capabilities *ClientCapabilities) *InitializeRequest {
	return &InitializeRequest{
		ClientInfo:      clientInfo,
		Capabilities:    capabilities,
		ProtocolVersion: Version,
	}
}

// NewInitializeResult creates a new initialize response
func NewInitializeResult(serverInfo *Implementation, capabilities *ServerCapabilities, version string, instructions string) *InitializeResult {
	return &InitializeResult{
		ServerInfo:      serverInfo,
		Capabilities:    capabilities,
		ProtocolVersion: version,
		Instructions:    instructions,
	}
}

// NewInitializedNotification creates a new initialized notification
func NewInitializedNotification() *InitializedNotification {
	return &InitializedNotification{}
}

func IsInitializedRequest(rawParams json.RawMessage) bool {
	return gjson.ParseBytes(rawParams).Get("method").String() == string(Initialize)
}

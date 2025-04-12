package server

import (
	"encoding/json"
	"fmt"

	"github.com/yosida95/uritemplate/v3"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

func (server *Server) handleRequestWithPing() (*protocol.PingResult, error) {
	return protocol.NewPingResult(), nil
}

func (server *Server) handleRequestWithInitialize(sessionID string, rawParams json.RawMessage) (*protocol.InitializeResult, error) {
	var request *protocol.InitializeRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	if request.ProtocolVersion != protocol.Version {
		return nil, fmt.Errorf("protocol version not supported, supported version is %v", protocol.Version)
	}

	s := newSession()
	s.clientInfo = &request.ClientInfo
	s.clientCapabilities = &request.Capabilities
	s.receiveInitRequest.Store(true)

	server.sessionID2session.Store(sessionID, s)

	return &protocol.InitializeResult{
		ServerInfo:      *server.serverInfo,
		Capabilities:    *server.capabilities,
		ProtocolVersion: protocol.Version,
		Instructions:    server.instructions,
	}, nil
}

func (server *Server) handleRequestWithListPrompts(rawParams json.RawMessage) (*protocol.ListPromptsResult, error) {
	if server.capabilities.Prompts == nil {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.ListPromptsRequest
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
			return nil, err
		}
	}

	prompts := make([]protocol.Prompt, 0)
	server.prompts.Range(func(_ string, entry *promptEntry) bool {
		prompts = append(prompts, *entry.prompt)
		return true
	})

	return &protocol.ListPromptsResult{
		Prompts: prompts,
	}, nil
}

func (server *Server) handleRequestWithGetPrompt(rawParams json.RawMessage) (*protocol.GetPromptResult, error) {
	if server.capabilities.Prompts == nil {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.GetPromptRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	entry, ok := server.prompts.Load(request.Name)
	if !ok {
		return nil, fmt.Errorf("missing prompt, promptName=%s", request.Name)
	}
	return entry.handler(request)
}

func (server *Server) handleRequestWithListResources(rawParams json.RawMessage) (*protocol.ListResourcesResult, error) {
	if server.capabilities.Resources == nil {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.ListResourcesRequest
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
			return nil, err
		}
	}

	resources := make([]protocol.Resource, 0)
	server.resources.Range(func(_ string, entry *resourceEntry) bool {
		resources = append(resources, *entry.resource)
		return true
	})

	return &protocol.ListResourcesResult{
		Resources: resources,
	}, nil
}

func (server *Server) handleRequestWithListResourceTemplates(rawParams json.RawMessage) (*protocol.ListResourceTemplatesResult, error) {
	if server.capabilities.Resources == nil {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.ListResourceTemplatesRequest
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
			return nil, err
		}
	}

	templates := make([]protocol.ResourceTemplate, 0)
	server.resourceTemplates.Range(func(_ string, entry *resourceTemplateEntry) bool {
		templates = append(templates, *entry.resourceTemplate)
		return true
	})

	return &protocol.ListResourceTemplatesResult{
		ResourceTemplates: templates,
	}, nil
}

func (server *Server) handleRequestWithReadResource(rawParams json.RawMessage) (*protocol.ReadResourceResult, error) {
	if server.capabilities.Resources == nil {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.ReadResourceRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	var handler ResourceHandlerFunc
	if entry, ok := server.resources.Load(request.URI); ok {
		handler = entry.handler
	}

	server.resourceTemplates.Range(func(_ string, entry *resourceTemplateEntry) bool {
		if !matchesTemplate(request.URI, entry.resourceTemplate.URITemplateParsed) {
			return true
		}
		handler = entry.handler
		matchedVars := entry.resourceTemplate.URITemplateParsed.Match(request.URI)
		request.Arguments = make(map[string]interface{})
		for name, value := range matchedVars {
			request.Arguments[name] = value.V
		}
		return false
	})

	if handler == nil {
		return nil, fmt.Errorf("missing resource, resourceName=%s", request.URI)
	}
	return handler(request)
}

func matchesTemplate(uri string, template *uritemplate.Template) bool {
	return template.Regexp().MatchString(uri)
}

func (server *Server) handleRequestWithSubscribeResourceChange(sessionID string, rawParams json.RawMessage) (*protocol.SubscribeResult, error) {
	if server.capabilities.Resources == nil && !server.capabilities.Resources.Subscribe {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.SubscribeRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	s, ok := server.sessionID2session.Load(sessionID)
	if !ok {
		return nil, pkg.ErrLackSession
	}
	s.subscribedResources.Set(request.URI, struct{}{})
	return protocol.NewSubscribeResult(), nil
}

func (server *Server) handleRequestWithUnSubscribeResourceChange(sessionID string, rawParams json.RawMessage) (*protocol.UnsubscribeResult, error) {
	if server.capabilities.Resources == nil && !server.capabilities.Resources.Subscribe {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.UnsubscribeRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	s, ok := server.sessionID2session.Load(sessionID)
	if !ok {
		return nil, pkg.ErrLackSession
	}
	s.subscribedResources.Remove(request.URI)
	return protocol.NewUnsubscribeResult(), nil
}

func (server *Server) handleRequestWithListTools(rawParams json.RawMessage) (*protocol.ListToolsResult, error) {
	if server.capabilities.Tools == nil {
		return nil, pkg.ErrServerNotSupport
	}

	request := &protocol.ListToolsRequest{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
			return nil, err
		}
	}

	tools := make([]*protocol.Tool, 0)
	server.tools.Range(func(_ string, entry *toolEntry) bool {
		tools = append(tools, entry.tool)
		return true
	})

	return &protocol.ListToolsResult{Tools: tools}, nil
}

func (server *Server) handleRequestWithCallTool(rawParams json.RawMessage) (*protocol.CallToolResult, error) {
	if server.capabilities.Tools == nil {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.CallToolRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	entry, ok := server.tools.Load(request.Name)
	if !ok {
		return nil, fmt.Errorf("missing tool, toolName=%s", request.Name)
	}

	return entry.handler(request)
}

func (server *Server) handleNotifyWithInitialized(sessionID string, rawParams json.RawMessage) error {
	param := &protocol.InitializedNotification{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, param); err != nil {
			return err
		}
	}

	s, ok := server.sessionID2session.Load(sessionID)
	if !ok {
		return pkg.ErrLackSession
	}

	if !s.receiveInitRequest.Load().(bool) {
		return fmt.Errorf("the server has not received the client's initialization request")
	}
	s.ready.Store(true)
	return nil
}

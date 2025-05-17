package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yosida95/uritemplate/v3"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func (server *Server) handleRequestWithPing() (*protocol.PingResult, error) {
	return protocol.NewPingResult(), nil
}

func (server *Server) handleRequestWithInitialize(ctx context.Context, sessionID string, rawParams json.RawMessage) (*protocol.InitializeResult, error) {
	var request *protocol.InitializeRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	if _, ok := protocol.SupportedVersion[request.ProtocolVersion]; !ok {
		return nil, fmt.Errorf("protocol version not supported, supported lastest version is %v", protocol.Version)
	}
	protocolVersion := request.ProtocolVersion

	if midVar, ok := ctx.Value(transport.SessionIDForReturnKey{}).(*transport.SessionIDForReturn); ok {
		sessionID = server.sessionManager.CreateSession(ctx)
		midVar.SessionID = sessionID
	}

	if sessionID != "" {
		s, ok := server.sessionManager.GetSession(sessionID)
		if !ok {
			return nil, pkg.ErrLackSession
		}
		s.SetClientInfo(&request.ClientInfo, &request.Capabilities)
		s.SetReceivedInitRequest()
	}

	return protocol.NewInitializeResult(*server.serverInfo, *server.capabilities, protocolVersion, server.instructions), nil
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
	if server.paginationLimit > 0 {
		resourcesToReturn, nextCursor, err := protocol.PaginationLimit[protocol.Prompt](prompts, request.Cursor, server.paginationLimit)
		return &protocol.ListPromptsResult{
			Prompts:    resourcesToReturn,
			NextCursor: nextCursor,
		}, err
	}
	return &protocol.ListPromptsResult{
		Prompts: prompts,
	}, nil
}

func (server *Server) handleRequestWithGetPrompt(ctx context.Context, rawParams json.RawMessage) (*protocol.GetPromptResult, error) {
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
	return entry.handler(ctx, request)
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
	if server.paginationLimit > 0 {
		resourcesToReturn, nextCursor, err := protocol.PaginationLimit[protocol.Resource](resources, request.Cursor, server.paginationLimit)
		return &protocol.ListResourcesResult{
			Resources:  resourcesToReturn,
			NextCursor: nextCursor,
		}, err
	}

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
	if server.paginationLimit > 0 {
		resourcesToReturn, nextCursor, err := protocol.PaginationLimit[protocol.ResourceTemplate](templates, request.Cursor, server.paginationLimit)
		return &protocol.ListResourceTemplatesResult{
			ResourceTemplates: resourcesToReturn,
			NextCursor:        nextCursor,
		}, err
	}
	return &protocol.ListResourceTemplatesResult{
		ResourceTemplates: templates,
	}, nil
}

func (server *Server) handleRequestWithReadResource(ctx context.Context, rawParams json.RawMessage) (*protocol.ReadResourceResult, error) {
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
	return handler(ctx, request)
}

func (server *Server) handleRequestWithSubscribeResourceChange(sessionID string, rawParams json.RawMessage) (*protocol.SubscribeResult, error) {
	if server.capabilities.Resources == nil && !server.capabilities.Resources.Subscribe {
		return nil, pkg.ErrServerNotSupport
	}

	var request *protocol.SubscribeRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	s, ok := server.sessionManager.GetSession(sessionID)
	if !ok {
		return nil, pkg.ErrLackSession
	}
	s.GetSubscribedResources().Set(request.URI, struct{}{})
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

	s, ok := server.sessionManager.GetSession(sessionID)
	if !ok {
		return nil, pkg.ErrLackSession
	}
	s.GetSubscribedResources().Remove(request.URI)
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
	if server.paginationLimit > 0 {
		resourcesToReturn, nextCursor, err := protocol.PaginationLimit[*protocol.Tool](tools, request.Cursor, server.paginationLimit)
		return &protocol.ListToolsResult{
			Tools:      resourcesToReturn,
			NextCursor: nextCursor,
		}, err
	}
	return &protocol.ListToolsResult{Tools: tools}, nil
}

func (server *Server) handleRequestWithCallTool(ctx context.Context, rawParams json.RawMessage) (*protocol.CallToolResult, error) {
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

	return entry.handler(ctx, request)
}

func (server *Server) handleNotifyWithInitialized(sessionID string, rawParams json.RawMessage) error {
	if sessionID == "" {
		return nil
	}

	param := &protocol.InitializedNotification{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, param); err != nil {
			return err
		}
	}

	s, ok := server.sessionManager.GetSession(sessionID)
	if !ok {
		return pkg.ErrLackSession
	}

	if !s.GetReceivedInitRequest() {
		return fmt.Errorf("the server has not received the client's initialization request")
	}
	s.SetReady()
	return nil
}

func (server *Server) handleNotifyWithCancelled(sessionID string, rawParams json.RawMessage) error {
	var params protocol.CancelledNotification
	if err := pkg.JSONUnmarshal(rawParams, &params); err != nil {
		return err
	}

	s, ok := server.sessionManager.GetSession(sessionID)
	if !ok {
		return pkg.ErrLackSession
	}

	cancel, ok := s.GetClientReqID2cancelFunc().Get(fmt.Sprint(params.RequestID))
	if !ok {
		return nil
	}
	cancel()

	return nil
}

func matchesTemplate(uri string, template *uritemplate.Template) bool {
	return template.Regexp().MatchString(uri)
}

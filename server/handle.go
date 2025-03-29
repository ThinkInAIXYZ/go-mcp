package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go-mcp/pkg"
	"go-mcp/protocol"
)

func (server *Server) handleRequestWithInitialize(ctx context.Context, rawParams json.RawMessage) (*protocol.InitializeResult, error) {
	var request protocol.InitializeRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	// TODO: validate client initialize request

	// cache client information to session
	sessionID, _ := getSessionIDFromCtx(ctx)
	value, ok := server.sessionID2session.Load(sessionID)
	if !ok {
		return nil, pkg.NewLackSessionError(sessionID)
	}
	session := value.(*session)
	session.clientInitializeRequest = &request

	result := protocol.InitializeResult{
		ProtocolVersion: server.protocolVersion,
		Capabilities:    server.capabilities,
		ServerInfo:      server.serverInfo,
	}
	return &result, nil
}

func (server *Server) handleRequestWithPing(ctx context.Context, rawParams json.RawMessage) (*protocol.PingResult, error) {
	return protocol.NewPingResponse(), nil
}

func (server *Server) handleRequestWithListPrompts(ctx context.Context, rawParams json.RawMessage) (*protocol.ListPromptsResult, error) {
	var request protocol.ListPromptsRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	// TODO: list prompt with cursor
	return &protocol.ListPromptsResult{
		Prompts: server.prompts,
	}, nil
}

func (server *Server) handleRequestWithGetPrompt(ctx context.Context, rawParams json.RawMessage) (*protocol.GetPromptResult, error) {
	var request protocol.GetPromptRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	// TODO: validate request's arguments
	handlerFunc, ok := server.promptHandlers[request.Name]
	if !ok {
		return nil, fmt.Errorf("missing prompt handler, promptName=%s", request.Name)
	}
	return handlerFunc(request)
}

func (server *Server) handleRequestWithListResources(ctx context.Context, rawParams json.RawMessage) (*protocol.ListResourcesResult, error) {
	var request protocol.ListResourcesRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	// TODO: list resources with cursor
	return &protocol.ListResourcesResult{
		Resources: server.resources,
	}, nil
}

func (server *Server) handleRequestWithReadResource(ctx context.Context, rawParams json.RawMessage) (*protocol.ReadResourceResult, error) {
	var request protocol.ReadResourceRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	handlerFunc, ok := server.resourceHandlers[request.URI]
	if !ok {
		return nil, fmt.Errorf("missing resource read handler, uri=%s", request.URI)
	}
	return handlerFunc(request)
}

func (server *Server) handleRequestWitListResourceTemplates(ctx context.Context, rawParams json.RawMessage) (*protocol.ListResourceTemplatesResult, error) {
	var request protocol.ListResourceTemplatesRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	// TODO: list resource template with cursor
	return &protocol.ListResourceTemplatesResult{
		ResourceTemplates: server.resourceTemplates,
	}, nil
}

func (server *Server) handleRequestWithSubscribeResourceChange(ctx context.Context, rawParams json.RawMessage) (*protocol.SubscribeResult, error) {
	var request protocol.SubscribeRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	sessionID, _ := getSessionIDFromCtx(ctx)
	value, ok := server.sessionID2session.Load(sessionID)
	if !ok {
		return nil, pkg.NewLackSessionError(sessionID)
	}
	session := value.(*session)
	session.subscribedResources.Set(request.URI, struct{}{})
	return &protocol.SubscribeResult{}, nil
}

func (server *Server) handleRequestWithUnSubscribeResourceChange(ctx context.Context, rawParams json.RawMessage) (*protocol.UnsubscribeResult, error) {
	var request protocol.UnsubscribeRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	sessionID, _ := getSessionIDFromCtx(ctx)
	value, ok := server.sessionID2session.Load(sessionID)
	if !ok {
		return nil, pkg.NewLackSessionError(sessionID)
	}
	session := value.(*session)
	session.subscribedResources.Remove(request.URI)
	return &protocol.UnsubscribeResult{}, nil
}

func (server *Server) handleRequestWithListTools(ctx context.Context, rawParams json.RawMessage) (*protocol.ListToolsResult, error) {
	request := &protocol.ListToolsRequest{}
	if err := pkg.JsonUnmarshal(rawParams, request); err != nil {
		return nil, err
	}
	// TODO: 需要处理request.Cursor的翻页操作
	return &protocol.ListToolsResult{Tools: server.tools}, nil
}

func (server *Server) handleRequestWithCallTool(ctx context.Context, rawParams json.RawMessage) (*protocol.CallToolResult, error) {
	var request protocol.CallToolRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	// TODO: validate request params
	handlerFunc, ok := server.toolHandlers[request.Name]
	if !ok {
		return nil, fmt.Errorf("missing tool call handler, toolName=%s", request.Name)
	}

	return handlerFunc(request)
}

func (server *Server) handleRequestWithCompleteRequest(ctx context.Context, rawParams json.RawMessage) (*protocol.CompleteResult, error) {
	var request protocol.CompleteRequest
	if err := pkg.JsonUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	var handerName string
	switch v := request.Ref.(type) {
	case protocol.PromptReference:
		handerName = fmt.Sprintf("%s/%s", v.Type, v.Name)
	case protocol.ResourceReference:
		handerName = fmt.Sprintf("%s/%s", v.Type, v.URI)
	default:
		return nil, errors.New("invalid complete request")
	}

	handlerFunc, ok := server.completionHandlers[handerName]
	if !ok {
		return nil, fmt.Errorf("missing complete handler, name=%s", handerName)
	}
	return handlerFunc(request)
}

func (server *Server) handleRequestWithSetLogLevel(ctx context.Context, rawParams json.RawMessage) (*protocol.SetLoggingLevelResult, error) {
	return nil, nil
}

func (server *Server) handleNotifyWithCancelled(ctx context.Context, rawParams json.RawMessage) error {
	param := &protocol.CancelledNotification{}
	if err := pkg.JsonUnmarshal(rawParams, param); err != nil {
		return err
	}
	return server.cancelledNotifyHandler(ctx, param)
}

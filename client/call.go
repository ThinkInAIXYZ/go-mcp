package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

func (client *Client) initialization(ctx context.Context, request *protocol.InitializeRequest) (*protocol.InitializeResult, error) {
	request.ProtocolVersion = protocol.Version

	response, err := client.callServer(ctx, protocol.Initialize, request)
	if err != nil {
		return nil, err
	}
	var result protocol.InitializeResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.ProtocolVersion != request.ProtocolVersion {
		return nil, fmt.Errorf("protocol version mismatch, expected %s, got %s", request.ProtocolVersion, result.ProtocolVersion)
	}

	if err := client.sendNotification4Initialized(ctx); nil != err {
		return nil, fmt.Errorf("failed to send InitializedNotification: %w", err)
	}

	client.clientInfo = &request.ClientInfo
	client.clientCapabilities = &request.Capabilities

	client.serverInfo = &result.ServerInfo
	client.serverCapabilities = &result.Capabilities
	client.serverInstructions = result.Instructions

	client.ready.Store(true)
	return &result, nil
}

func (client *Client) Ping(ctx context.Context, request *protocol.PingRequest) (*protocol.PingResult, error) {
	response, err := client.callServer(ctx, protocol.Ping, request)
	if err != nil {
		return nil, err
	}

	var result protocol.PingResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (client *Client) ListPrompts(ctx context.Context) (*protocol.ListPromptsResult, error) {
	if client.serverCapabilities.Prompts == nil {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.PromptsList, protocol.NewListPromptsRequest())
	if err != nil {
		return nil, err
	}

	var result protocol.ListPromptsResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (client *Client) GetPrompt(ctx context.Context, request *protocol.GetPromptRequest) (*protocol.GetPromptResult, error) {
	if client.serverCapabilities.Prompts == nil {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.PromptsGet, request)
	if err != nil {
		return nil, err
	}

	var result protocol.GetPromptResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (client *Client) ListResources(ctx context.Context) (*protocol.ListResourcesResult, error) {
	if client.serverCapabilities.Resources == nil {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.ResourcesList, protocol.NewListResourcesRequest())
	if err != nil {
		return nil, err
	}

	var result protocol.ListResourcesResult
	if err = pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, err
}

func (client *Client) ListResourceTemplates(ctx context.Context) (*protocol.ListResourceTemplatesResult, error) {
	if client.serverCapabilities.Resources == nil {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.ResourceListTemplates, protocol.NewListResourceTemplatesRequest())
	if err != nil {
		return nil, err
	}

	var result protocol.ListResourceTemplatesResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (client *Client) ReadResource(ctx context.Context, request *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error) {
	if client.serverCapabilities.Resources == nil {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.ResourcesRead, request)
	if err != nil {
		return nil, err
	}

	var result protocol.ReadResourceResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (client *Client) SubscribeResourceChange(ctx context.Context, request *protocol.SubscribeRequest) (*protocol.SubscribeResult, error) {
	if client.serverCapabilities.Resources == nil || !client.serverCapabilities.Resources.Subscribe {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.ResourcesSubscribe, request)
	if err != nil {
		return nil, err
	}

	var result protocol.SubscribeResult
	if len(response) > 0 {
		if err = pkg.JSONUnmarshal(response, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}
	return &result, nil
}

func (client *Client) UnSubscribeResourceChange(ctx context.Context, request *protocol.UnsubscribeRequest) (*protocol.UnsubscribeResult, error) {
	if client.serverCapabilities.Resources == nil || !client.serverCapabilities.Resources.Subscribe {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.ResourcesUnsubscribe, request)
	if err != nil {
		return nil, err
	}

	var result protocol.UnsubscribeResult
	if len(response) > 0 {
		if err = pkg.JSONUnmarshal(response, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}
	return &result, nil
}

func (client *Client) ListTools(ctx context.Context) (*protocol.ListToolsResult, error) {
	if client.serverCapabilities.Tools == nil {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.ToolsList, protocol.NewListToolsRequest())
	if err != nil {
		return nil, err
	}

	var result protocol.ListToolsResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (client *Client) CallTool(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	if client.serverCapabilities.Tools == nil {
		return nil, pkg.ErrServerNotSupport
	}

	response, err := client.callServer(ctx, protocol.ToolsCall, request)
	if err != nil {
		return nil, err
	}

	var result protocol.CallToolResult
	if err := pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (client *Client) sendNotification4Initialized(ctx context.Context) error {
	return client.sendMsgWithNotification(ctx, protocol.NotificationInitialized, protocol.NewInitializedNotification())
}

// Responsible for request and response assembly
func (client *Client) callServer(ctx context.Context, method protocol.Method, params protocol.ClientRequest) (json.RawMessage, error) {
	if !client.ready.Load().(bool) && (method != protocol.Initialize && method != protocol.Ping) {
		return nil, fmt.Errorf("client not ready")
	}

	requestID := strconv.FormatInt(atomic.AddInt64(&client.requestID, 1), 10)
	if err := client.sendMsgWithRequest(ctx, requestID, method, params); err != nil {
		return nil, fmt.Errorf("callServer: %w", err)
	}

	respChan := make(chan *protocol.JSONRPCResponse, 1)

	client.reqID2respChan.Set(requestID, respChan)
	defer client.reqID2respChan.Remove(requestID)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-respChan:
		if err := response.Error; err != nil {
			return nil, pkg.NewResponseError(err.Code, err.Message, err.Data)
		}
		return response.RawResult, nil
	}
}

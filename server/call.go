package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"go-mcp/pkg"
	"go-mcp/protocol"
)

// 请求

// 1. 请求构造
// 2. 发送请求 server.callClient(ctx)
// 3. 响应解析

func (server *Server) ping(ctx context.Context) error {
	// server.callClient(ctx)
	return nil
}

func (server *Server) listRoots(ctx context.Context) error {
	// server.callClient(ctx)
	return nil
}

func (server *Server) CreateMessagesSample(ctx context.Context) error {
	// 可以从 ctx 里取得 session id, sessionID, exist := getSessionIDFromCtx(ctx)
	// server.callClient(ctx)
	return nil
}

// 通知
// 1. 构造通知结构体
// 2. Cancelled、Progress、LoggingMessage类型的通知，可以从 ctx 里取得 session id, sessionID, exist := getSessionIDFromCtx(ctx)
// 3. 发送通知 server.sendMsgWithNotification(ctx)

func (server *Server) SendNotification4Cancelled(ctx context.Context, notify *protocol.CancelledNotification) error {
	sessionID, exist := getSessionIDFromCtx(ctx)
	if !exist {
		return pkg.NewLackSessionError(sessionID)
	}
	return server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationCancelled, notify)
}

func (server *Server) SendNotification4Progress(ctx context.Context, notify *protocol.ProgressNotification) error {
	sessionID, exist := getSessionIDFromCtx(ctx)
	if !exist {
		return pkg.NewLackSessionError(sessionID)
	}
	return server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationProgress, notify)
}

func (server *Server) SendNotification4ToolListChanges(ctx context.Context, notify *protocol.ToolListChangedNotification) error {
	sessionID, exist := getSessionIDFromCtx(ctx)
	if !exist {
		return pkg.NewLackSessionError(sessionID)
	}
	return server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationToolsListChanged, notify)
}

func (server *Server) SendNotification4PromptListChanges(ctx context.Context, notify *protocol.PromptListChangedNotification) error {
	// TODO: 获取订阅了此通知的sessionID
	sessionIDList := []string{}

	var errList error
	for _, sessionID := range sessionIDList {
		if err := server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationPromptsListChanged, notify); err != nil {
			errList = errors.Join(fmt.Errorf("sessionID=%s, err: %w", sessionID, err))
		}
	}
	return errList
}

func (server *Server) SendNotification4ResourceListChanges(ctx context.Context, notify *protocol.ResourceListChangedNotification) error {
	// TODO: 获取订阅了此通知的sessionID
	sessionIDList := []string{}

	var errList error
	for _, sessionID := range sessionIDList {
		if err := server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationResourcesListChanged, notify); err != nil {
			errList = errors.Join(fmt.Errorf("sessionID=%s, err: %w", sessionID, err))
		}
	}
	return errList
}

func (server *Server) SendNotification4ResourcesUpdated(ctx context.Context, notify *protocol.ResourceUpdatedNotification) error {
	// TODO: 获取订阅了此通知的sessionID
	sessionIDList := []string{}

	var errList error
	for _, sessionID := range sessionIDList {
		if err := server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationResourcesUpdated, notify); err != nil {
			errList = errors.Join(fmt.Errorf("sessionID=%s, err: %w", sessionID, err))
		}
	}
	return errList
}

func (server *Server) SendNotification4LoggingMessage(ctx context.Context, notify *protocol.LogMessageNotification) error {
	sessionID, exist := getSessionIDFromCtx(ctx)
	if !exist {
		return pkg.NewLackSessionError(sessionID)
	}
	return server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationLogMessage, notify)
}

func (server *Server) callAndParse(ctx context.Context, sessionID string, method protocol.Method, request protocol.ServerRequest, result protocol.ClientResponse) error {
	rawResult, err := server.callClient(ctx, sessionID, method, request)
	if err != nil {
		return err
	}

	return pkg.JsonUnmarshal(rawResult, &result)
}

// 负责request和response的拼接
func (server *Server) callClient(ctx context.Context, sessionID string, method protocol.Method, params protocol.ServerRequest) (json.RawMessage, error) {
	// 获取会话数据
	sessionData, ok := server.GetSession(sessionID)
	if !ok {
		return nil, pkg.NewLackSessionError(sessionID)
	}

	requestID := strconv.FormatInt(sessionData.requestID.Add(1), 10)

	if err := server.sendMsgWithRequest(ctx, sessionID, requestID, method, params); err != nil {
		return nil, err
	}

	respChan := make(chan *protocol.JSONRPCResponse)
	sessionData.reqID2respChan.Set(requestID, respChan)

	select {
	case <-ctx.Done():
		sessionData.reqID2respChan.Remove(requestID)
		return nil, ctx.Err()
	case response := <-respChan:
		if err := response.Error; err != nil {
			return nil, pkg.NewResponseError(err.Code, err.Message, err.Data)
		}
		return response.RawResult, nil
	}
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server/session"
)

func (server *Server) Ping(ctx context.Context, request *protocol.PingRequest) (*protocol.PingResult, error) {
	sessionID, err := GetSessionIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	response, err := server.callClient(ctx, sessionID, protocol.Ping, request)
	if err != nil {
		return nil, err
	}

	var result protocol.PingResult
	if err = pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (server *Server) Sampling(ctx context.Context, request *protocol.CreateMessageRequest) (*protocol.CreateMessageResult, error) {
	sessionID, err := GetSessionIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	s, ok := server.sessionManager.GetSession(sessionID)
	if !ok {
		return nil, pkg.ErrLackSession
	}

	if s.GetClientCapabilities() == nil || s.GetClientCapabilities().Sampling == nil {
		return nil, pkg.ErrClientNotSupport
	}

	response, err := server.callClient(ctx, sessionID, protocol.SamplingCreateMessage, request)
	if err != nil {
		return nil, err
	}

	var result protocol.CreateMessageResult
	if err = pkg.JSONUnmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

func (server *Server) SendProgressNotification(ctx context.Context, notify *protocol.ProgressNotification) error {
	progressToken, err := getProgressTokenFromCtx(ctx)
	if err != nil {
		return err
	}
	notify.ProgressToken = progressToken

	if err = server.sendMsgWithNotification(ctx, "", protocol.NotificationProgress, notify); err != nil {
		return err
	}

	return nil
}

func (server *Server) sendNotification4ToolListChanges(ctx context.Context) error {
	if server.capabilities.Tools == nil || !server.capabilities.Tools.ListChanged {
		return pkg.ErrServerNotSupport
	}

	var errList []error
	server.sessionManager.RangeSessions(func(sessionID string, _ *session.State) bool {
		if err := server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationToolsListChanged, protocol.NewToolListChangedNotification()); err != nil {
			errList = append(errList, fmt.Errorf("sessionID=%s, err: %w", sessionID, err))
		}
		return true
	})
	return pkg.JoinErrors(errList)
}

func (server *Server) sendNotification4PromptListChanges(ctx context.Context) error {
	if server.capabilities.Prompts == nil || !server.capabilities.Prompts.ListChanged {
		return pkg.ErrServerNotSupport
	}

	var errList []error
	server.sessionManager.RangeSessions(func(sessionID string, _ *session.State) bool {
		if err := server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationPromptsListChanged, protocol.NewPromptListChangedNotification()); err != nil {
			errList = append(errList, fmt.Errorf("sessionID=%s, err: %w", sessionID, err))
		}
		return true
	})
	return pkg.JoinErrors(errList)
}

func (server *Server) sendNotification4ResourceListChanges(ctx context.Context) error {
	if server.capabilities.Resources == nil || !server.capabilities.Resources.ListChanged {
		return pkg.ErrServerNotSupport
	}

	var errList []error
	server.sessionManager.RangeSessions(func(sessionID string, _ *session.State) bool {
		if err := server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationResourcesListChanged,
			protocol.NewResourceListChangedNotification()); err != nil {
			errList = append(errList, fmt.Errorf("sessionID=%s, err: %w", sessionID, err))
		}
		return true
	})
	return pkg.JoinErrors(errList)
}

func (server *Server) SendNotification4ResourcesUpdated(ctx context.Context, notify *protocol.ResourceUpdatedNotification) error {
	if server.capabilities.Resources == nil || !server.capabilities.Resources.Subscribe {
		return pkg.ErrServerNotSupport
	}

	var errList []error
	server.sessionManager.RangeSessions(func(sessionID string, s *session.State) bool {
		if _, ok := s.GetSubscribedResources().Get(notify.URI); !ok {
			return true
		}

		if err := server.sendMsgWithNotification(ctx, sessionID, protocol.NotificationResourcesUpdated, notify); err != nil {
			errList = append(errList, fmt.Errorf("sessionID=%s, err: %w", sessionID, err))
		}
		return true
	})
	return pkg.JoinErrors(errList)
}

// Responsible for request and response assembly
func (server *Server) callClient(ctx context.Context, sessionID string, method protocol.Method, params protocol.ServerRequest) (json.RawMessage, error) {
	session, ok := server.sessionManager.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("callClient: %w", pkg.ErrLackSession)
	}

	requestID := strconv.FormatInt(session.IncRequestID(), 10)
	respChan := make(chan *protocol.JSONRPCResponse, 1)
	session.GetServerReqID2respChan().Set(requestID, respChan)
	defer session.GetServerReqID2respChan().Remove(requestID)

	if err := server.sendMsgWithRequest(ctx, sessionID, requestID, method, params); err != nil {
		return nil, fmt.Errorf("callClient: %w", err)
	}

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

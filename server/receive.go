package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tidwall/gjson"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

func (server *Server) receive(ctx context.Context, sessionID string, msg []byte) (<-chan []byte, error) {
	if sessionID != "" && !server.sessionManager.IsActiveSession(sessionID) {
		if server.sessionManager.IsClosedSession(sessionID) {
			return nil, pkg.ErrSessionClosed
		}
		return nil, pkg.ErrLackSession
	}

	if !gjson.GetBytes(msg, "id").Exists() {
		notify := &protocol.JSONRPCNotification{}
		if err := pkg.JSONUnmarshal(msg, &notify); err != nil {
			return nil, err
		}
		if err := server.receiveNotify(sessionID, notify); err != nil {
			notify.RawParams = nil // simplified log
			server.logger.Errorf("receive notify:%+v error: %s", notify, err.Error())
			return nil, err
		}
		return nil, nil
	}

	// case request or response
	if !gjson.GetBytes(msg, "method").Exists() {
		resp := &protocol.JSONRPCResponse{}
		if err := pkg.JSONUnmarshal(msg, &resp); err != nil {
			return nil, err
		}

		if err := server.receiveResponse(sessionID, resp); err != nil {
			resp.RawResult = nil // simplified log
			server.logger.Errorf("receive response:%+v error: %s", resp, err.Error())
			return nil, err
		}
		return nil, nil
	}

	req := &protocol.JSONRPCRequest{}
	if err := pkg.JSONUnmarshal(msg, &req); err != nil {
		return nil, err
	}
	if !req.IsValid() {
		return nil, pkg.ErrRequestInvalid
	}

	// if sessionID != "" && req.Method != protocol.Initialize && req.Method != protocol.Ping {
	// 	if s, ok := server.sessionManager.GetSession(sessionID); !ok {
	// 		return nil, pkg.ErrLackSession
	// 	} else if !s.GetReady() {
	// 		return nil, pkg.ErrSessionHasNotInitialized
	// 	}
	// }

	server.inFlyRequest.Add(1)

	if server.inShutdown.Load() {
		server.inFlyRequest.Done()
		return nil, errors.New("server already shutdown")
	}

	ch := make(chan []byte, 5)
	go func(ctx context.Context) {
		defer pkg.Recover()
		defer server.inFlyRequest.Done()
		defer close(ch)

		if s, ok := server.sessionManager.GetSession(sessionID); ok && req.Method != protocol.Initialize {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			requestID := fmt.Sprint(req.ID)
			s.GetClientReqID2cancelFunc().Set(requestID, cancel)
			defer s.GetClientReqID2cancelFunc().Remove(requestID)
		}

		if r := gjson.GetBytes(req.RawParams, fmt.Sprintf("_meta.%s", protocol.ProgressTokenKey)); r.Exists() {
			ctx = setProgressTokenToCtx(ctx, r.Value())
		}

		ctx = setSendChanToCtx(ctx, ch)

		resp := server.receiveRequest(ctx, sessionID, req)
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		message, err := json.Marshal(resp)
		if err != nil {
			server.logger.Errorf("receive json marshal response:%+v error: %s", resp, err.Error())
			return
		}
		ch <- message
	}(pkg.NewCancelShieldContext(ctx))
	return ch, nil
}

func (server *Server) receiveRequest(ctx context.Context, sessionID string, request *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
	if sessionID != "" {
		ctx = setSessionIDToCtx(ctx, sessionID)
	}

	if request.Method != protocol.Ping {
		server.sessionManager.UpdateSessionLastActiveAt(sessionID)
	}

	var (
		result protocol.ServerResponse
		err    error
	)

	switch request.Method {
	case protocol.Ping:
		result, err = server.handleRequestWithPing()
	case protocol.Initialize:
		result, err = server.handleRequestWithInitialize(ctx, sessionID, request.RawParams)
	case protocol.PromptsList:
		result, err = server.handleRequestWithListPrompts(request.RawParams)
	case protocol.PromptsGet:
		result, err = server.handleRequestWithGetPrompt(ctx, request.RawParams)
	case protocol.ResourcesList:
		result, err = server.handleRequestWithListResources(request.RawParams)
	case protocol.ResourceListTemplates:
		result, err = server.handleRequestWithListResourceTemplates(request.RawParams)
	case protocol.ResourcesRead:
		result, err = server.handleRequestWithReadResource(ctx, request.RawParams)
	case protocol.ResourcesSubscribe:
		result, err = server.handleRequestWithSubscribeResourceChange(sessionID, request.RawParams)
	case protocol.ResourcesUnsubscribe:
		result, err = server.handleRequestWithUnSubscribeResourceChange(sessionID, request.RawParams)
	case protocol.ToolsList:
		result, err = server.handleRequestWithListTools(ctx, request.RawParams)
	case protocol.ToolsCall:
		result, err = server.handleRequestWithCallTool(ctx, request.RawParams)
	default:
		err = fmt.Errorf("%w: method=%s", pkg.ErrMethodNotSupport, request.Method)
	}

	if err != nil {
		var code int
		switch {
		case errors.Is(err, pkg.ErrMethodNotSupport):
			code = protocol.MethodNotFound
		case errors.Is(err, pkg.ErrRequestInvalid):
			code = protocol.InvalidRequest
		case errors.Is(err, pkg.ErrJSONUnmarshal):
			code = protocol.ParseError
		default:
			code = protocol.InternalError
		}
		return protocol.NewJSONRPCErrorResponse(request.ID, code, err.Error())
	}
	return protocol.NewJSONRPCSuccessResponse(request.ID, result)
}

func (server *Server) receiveNotify(sessionID string, notify *protocol.JSONRPCNotification) error {
	// if sessionID != "" {
	// 	if s, ok := server.sessionManager.GetSession(sessionID); !ok {
	// 		return pkg.ErrLackSession
	// 	} else if notify.Method != protocol.NotificationInitialized && !s.GetReady() {
	// 		return pkg.ErrSessionHasNotInitialized
	// 	}
	// }

	switch notify.Method {
	case protocol.NotificationInitialized:
		return server.handleNotifyWithInitialized(sessionID, notify.RawParams)
	case protocol.NotificationCancelled:
		return server.handleNotifyWithCancelled(sessionID, notify.RawParams)
	default:
		return fmt.Errorf("%w: method=%s", pkg.ErrMethodNotSupport, notify.Method)
	}
}

func (server *Server) receiveResponse(sessionID string, response *protocol.JSONRPCResponse) error {
	s, ok := server.sessionManager.GetSession(sessionID)
	if !ok {
		return pkg.ErrLackSession
	}

	respChan, ok := s.GetServerReqID2respChan().Get(fmt.Sprint(response.ID))
	if !ok {
		return fmt.Errorf("%w: sessionID=%+v, requestID=%+v", pkg.ErrLackResponseChan, sessionID, response.ID)
	}

	select {
	case respChan <- response:
	default:
		return fmt.Errorf("%w: sessionID=%+v, response=%+v", pkg.ErrDuplicateResponseReceived, sessionID, response)
	}
	return nil
}

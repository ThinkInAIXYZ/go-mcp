package server

import (
	"context"
	"errors"
	"fmt"
	"go-mcp/session"
	"time"

	"go-mcp/pkg"
	"go-mcp/protocol"

	"github.com/tidwall/gjson"
)

// 对来自客户端的 message(request、response、notification)进行接收处理
// 如果是 request、notification 路由到对应的handler处理，如果是 response 则传递给对应 reqID 的 chan

func (server *Server) Receive(ctx context.Context, sessionID string, msg []byte) error {
	ctx = setSessionIDToCtx(ctx, sessionID)

	// 刷新会话状态
	server.refreshSession(sessionID)

	if !gjson.GetBytes(msg, "id").Exists() {
		notify := &protocol.JSONRPCNotification{}
		if err := pkg.JsonUnmarshal(msg, &notify); err != nil {
			return err
		}
		go func() {
			defer pkg.Recover()

			if err := server.receiveNotify(ctx, sessionID, notify); err != nil {
				// TODO: 打印日志
				return
			}
		}()
		return nil
	}

	// 判断 request和response
	if !gjson.GetBytes(msg, "method").Exists() {
		resp := &protocol.JSONRPCResponse{}
		if err := pkg.JsonUnmarshal(msg, &resp); err != nil {
			return err
		}
		go func() {
			defer pkg.Recover()

			if err := server.receiveResponse(ctx, sessionID, resp); err != nil {
				// TODO: 打印日志
				return
			}
		}()
		return nil
	}

	req := &protocol.JSONRPCRequest{}
	if err := pkg.JsonUnmarshal(msg, &req); err != nil {
		return err
	}

	server.inFlyRequest.Add(1)
	if server.inShutdown.Load() {
		defer server.inFlyRequest.Done()
		return errors.New("server already shutdown")
	}

	go func() {
		defer pkg.Recover()
		defer server.inFlyRequest.Done()

		resp := server.receiveRequest(ctx, req)

		if err := server.sendMsgWithResponse(ctx, sessionID, resp); err != nil {
			// TODO: 打印日志
			return
		}
	}()

	return nil
}

func (server *Server) receiveRequest(ctx context.Context, request *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
	if !request.IsValid() {
		return protocol.NewJSONRPCErrorResponse(request.ID, protocol.INVALID_REQUEST, fmt.Sprintf("invalid request: %v", request))
	}

	var (
		result protocol.ServerResponse
		err    error
	)

	// TODO：此处需要根据 request.Method 判断服务端是否声明此能力，如果未声明则报错返回。

	switch request.Method {
	case protocol.Ping:
		result, err = server.handleRequestWithPing(ctx, request.RawParams)
	case protocol.Initialize:
		result, err = server.handleRequestWithInitialize(ctx, request.RawParams)
	case protocol.PromptsList:
		result, err = server.handleRequestWithListPrompts(ctx, request.RawParams)
	case protocol.PromptsGet:
		result, err = server.handleRequestWithGetPrompt(ctx, request.RawParams)
	case protocol.ResourcesList:
		result, err = server.handleRequestWithListResources(ctx, request.RawParams)
	case protocol.ResourceListTemplates:
		result, err = server.handleRequestWitListResourceTemplates(ctx, request.RawParams)
	case protocol.ResourcesRead:
		result, err = server.handleRequestWithReadResource(ctx, request.RawParams)
	case protocol.ResourcesSubscribe:
		result, err = server.handleRequestWithSubscribeResourceChange(ctx, request.RawParams)
	case protocol.ResourcesUnsubscribe:
		result, err = server.handleRequestWithUnSubscribeResourceChange(ctx, request.RawParams)
	case protocol.ToolsList:
		result, err = server.handleRequestWithListTools(ctx, request.RawParams)
	case protocol.ToolsCall:
		result, err = server.handleRequestWithCallTool(ctx, request.RawParams)
	case protocol.CompletionComplete:
		result, err = server.handleRequestWithCompleteRequest(ctx, request.RawParams)
	case protocol.LoggingSetLevel:
		result, err = server.handleRequestWithSetLogLevel(ctx, request.RawParams)
	default:
		err = fmt.Errorf("request method=%s not supoort", request.Method)
	}

	if err != nil {
		// TODO: 此处需要根据err的类型传入不同的错误码
		return protocol.NewJSONRPCErrorResponse(request.ID, protocol.INVALID_REQUEST, err.Error())
	}
	return protocol.NewJSONRPCSuccessResponse(request.ID, result)
}

func (server *Server) receiveNotify(ctx context.Context, sessionID string, notify *protocol.JSONRPCNotification) error {
	switch notify.Method {
	case protocol.NotificationInitialized:
		// TODO
	case protocol.NotificationCancelled:
		return server.handleNotifyWithCancelled(ctx, notify.RawParams)
	case protocol.NotificationProgress:
		// TODO
	case protocol.NotificationRootsListChanged:
		// TODO
	default:
		// TODO: return pkg.errors
		return nil
	}
	return nil
}

func (server *Server) receiveResponse(ctx context.Context, sessionID string, response *protocol.JSONRPCResponse) error {
	// 获取会话数据
	sessionData, ok := server.GetSession(sessionID)
	if !ok {
		return pkg.NewLackSessionError(sessionID)
	}

	respChan, ok := sessionData.reqID2respChan.Get(fmt.Sprint(response.ID))
	if !ok {
		return fmt.Errorf("%w: sessionID=%+v, requestID=%+v", pkg.ErrLackResponseChan, sessionID, response.ID)
	}

	select {
	case respChan <- response:
	default:
		return fmt.Errorf("response repeat: sessionID=%+v, response=%+v", sessionID, response)
	}
	return nil
}

// refreshSession 刷新会话状态
// 根据会话管理器类型选择最优方式更新会话
func (server *Server) refreshSession(sessionID string) {
	// 统一使用UpdateSession方法刷新会话状态
	server.sessionManager.UpdateSession(sessionID, func(state *session.State) bool {
		state.LastActiveAt = time.Now()
		return true
	})
}

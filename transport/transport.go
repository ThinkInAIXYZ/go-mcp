package transport

import (
	"context"
	"go-mcp/session"

	"go-mcp/pkg"
)

/*
* Transport 是对底层传输层的抽象。
* GO-MCP 需要能够在 server/client 间传递 JSON-RPC 消息。
 */

// Message 定义基础消息接口
type Message []byte

func (msg Message) String() string {
	return pkg.B2S(msg)
}

type ClientTransport interface {
	// Start 启动传输连接
	Start() error

	// Send 发送消息
	Send(ctx context.Context, msg Message) error

	// SetReceiver 设置对对端消息的处理器
	SetReceiver(receiver ClientReceiver)

	// Close 关闭传输连接
	Close(ctx context.Context) error
}

type ClientReceiver interface {
	Receive(ctx context.Context, msg []byte) error
}

type ServerTransport interface {
	// Run 开始监听请求, 这是同步的, 在 Shutdown 之前, 不能返回
	Run() error

	// Send 发送消息
	Send(ctx context.Context, sessionID string, msg Message) error

	// SetReceiver 设置对对端消息的处理器
	SetReceiver(ServerReceiver)

	// SetSessionManager 设置会话管理器
	SetSessionManager(manager session.TransportSessionManager)

	// Shutdown 优雅关闭, 内部实现时需要先停止对消息的接收，再等待 serverCtx 被 cancel，同时使用 userCtx 控制超时。
	// userCtx is used to control the timeout of the server shutdown.
	// serverCtx is used to coordinate the internal cleanup sequence:
	// 1. turn off message listen
	// 2. Wait for serverCtx to be done (indicating server shutdown is complete)
	// 3. Cancel the transport's context to stop all ongoing operations
	// 4. Wait for all in-flight sends to complete
	// 5. Close all session
	Shutdown(userCtx context.Context, serverCtx context.Context) error
}

type ServerReceiver interface {
	Receive(ctx context.Context, sessionID string, msg []byte) error
}

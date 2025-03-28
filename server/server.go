package server

import (
	"context"
	"fmt"
	"go-mcp/session"
	"sync"
	"sync/atomic"
	"time"

	"go-mcp/pkg"
	"go-mcp/protocol"
	"go-mcp/transport"

	cmap "github.com/orcaman/concurrent-map/v2"
)

// Server 是MCP协议的服务端实现，负责处理客户端请求和管理会话
type Server struct {
	// 底层传输层实现
	transport transport.ServerTransport

	// 可用工具列表
	tools []*protocol.Tool

	// 处理取消通知的回调函数
	cancelledNotifyHandler func(ctx context.Context, notifyParam *protocol.CancelledNotification) error

	// 会话管理器，处理所有会话相关操作
	sessionManager session.Manager

	// 会话管理适配器，用于暴露给transport层
	sessionManagerAdapter *sessionManagerAdapter

	// 会话清理间隔
	sessionCleanInterval time.Duration
	// 会话最大空闲时间
	sessionMaxIdleTime time.Duration
	// 会话清理定时器
	sessionCleanTimer *time.Timer
	// 停止会话清理的通道
	sessionCleanStopCh chan struct{}

	// 服务器状态
	inShutdown   atomic.Bool    // 当服务器正在关闭时为true
	inFlyRequest sync.WaitGroup // 跟踪正在处理的请求数量

	// 日志记录器
	logger pkg.Logger
}

// sessionManagerAdapter 是一个适配器，实现transport层期望的TransportSessionManager接口
type sessionManagerAdapter struct {
	server *Server
}

type SessionData struct {
	// 用于为每个请求生成唯一的请求ID
	requestID atomic.Int64

	// 请求ID到响应通道的映射
	reqID2respChan cmap.ConcurrentMap[string, chan *protocol.JSONRPCResponse]

	// 标识是否是首次连接
	first     bool
	readyChan chan struct{}
}

// NewServer 创建一个新的MCP服务器实例
func NewServer(t transport.ServerTransport, opts ...Option) (*Server, error) {
	server := &Server{
		transport:            t,
		logger:               pkg.DefaultLogger,
		sessionManager:       session.NewSessionManager(session.MemorySessionManagerType, nil), // 默认使用内存会话管理器
		sessionCleanInterval: 5 * time.Minute,                                                  // 默认5分钟清理一次过期会话
		sessionMaxIdleTime:   30 * time.Minute,                                                 // 默认30分钟无活动视为过期
		sessionCleanStopCh:   make(chan struct{}),
	}

	// 创建会话管理适配器
	server.sessionManagerAdapter = &sessionManagerAdapter{server: server}

	// 应用选项配置 - 在设置transport之前应用选项，这样可以通过选项指定会话管理器
	for _, opt := range opts {
		opt(server)
	}

	// 设置服务器作为消息接收器和会话管理器
	t.SetReceiver(server)
	t.SetSessionManager(server.sessionManagerAdapter)

	return server, nil
}

// NewServerWithSessionManager 创建一个新的MCP服务器实例，使用指定的会话管理器类型
func NewServerWithSessionManager(
	t transport.ServerTransport,
	managerType session.ManagerType,
	managerOptions *session.ManagerOptions,
	opts ...Option,
) (*Server, error) {
	// 如果没有提供会话管理器选项，使用默认选项
	if managerOptions == nil {
		managerOptions = session.DefaultSessionManagerOptions()
	}

	server := &Server{
		transport:            t,
		logger:               pkg.DefaultLogger,
		sessionManager:       session.NewSessionManager(managerType, managerOptions), // 使用指定类型的会话管理器
		sessionCleanInterval: 5 * time.Minute,                                        // 默认5分钟清理一次过期会话
		sessionMaxIdleTime:   managerOptions.DefaultMaxIdleTime,                      // 使用管理器选项中的过期时间
		sessionCleanStopCh:   make(chan struct{}),
	}

	// 创建会话管理适配器
	server.sessionManagerAdapter = &sessionManagerAdapter{server: server}

	// 应用选项配置
	for _, opt := range opts {
		opt(server)
	}

	// 设置服务器作为消息接收器和会话管理器
	t.SetReceiver(server)
	t.SetSessionManager(server.sessionManagerAdapter)

	return server, nil
}

// startSessionCleaner 启动会话清理器
func (server *Server) startSessionCleaner() {
	server.sessionCleanTimer = time.NewTimer(server.sessionCleanInterval)

	go func() {
		defer pkg.Recover()

		for {
			select {
			case <-server.sessionCleanTimer.C:
				// 清理过期会话
				before := server.sessionManager.SessionCount()
				server.sessionManager.CleanExpiredSessions(server.sessionMaxIdleTime)
				after := server.sessionManager.SessionCount()

				if before != after {
					server.logger.Infof("清理过期会话：%d -> %d", before, after)
				}

				// 重置定时器
				server.sessionCleanTimer.Reset(server.sessionCleanInterval)

			case <-server.sessionCleanStopCh:
				server.sessionCleanTimer.Stop()
				return
			}
		}
	}()
}

// Start 启动服务器，开始监听客户端请求
func (server *Server) Start() error {
	// 启动会话清理器
	server.startSessionCleaner()

	// 启动传输层
	if err := server.transport.Run(); err != nil {
		return fmt.Errorf("init mcp server transport start fail: %w", err)
	}
	return nil
}

// Option 定义服务器配置选项
type Option func(*Server)

// WithCancelNotifyHandler 设置取消通知处理器
func WithCancelNotifyHandler(handler func(ctx context.Context, notifyParam *protocol.CancelledNotification) error) Option {
	return func(s *Server) {
		s.cancelledNotifyHandler = handler
	}
}

// WithLogger 设置日志记录器
func WithLogger(logger pkg.Logger) Option {
	return func(s *Server) {
		s.logger = logger
	}
}

// WithSessionCleanInterval 设置会话清理间隔
func WithSessionCleanInterval(interval time.Duration) Option {
	return func(s *Server) {
		s.sessionCleanInterval = interval
	}
}

// WithSessionMaxIdleTime 设置会话最大空闲时间
func WithSessionMaxIdleTime(maxIdleTime time.Duration) Option {
	return func(s *Server) {
		s.sessionMaxIdleTime = maxIdleTime
	}
}

// AddTool 添加一个工具到服务器
func (server *Server) AddTool(tool *protocol.Tool) {
	server.tools = append(server.tools, tool)
}

// Shutdown 优雅关闭服务器，等待所有处理中的请求完成
func (server *Server) Shutdown(userCtx context.Context) error {
	// 标记服务器正在关闭
	server.inShutdown.Store(true)

	// 停止会话清理器
	if server.sessionCleanTimer != nil {
		server.sessionCleanTimer.Stop()
	}
	close(server.sessionCleanStopCh)

	// 如果使用的是TimeWheelSessionManager，需要调用其Shutdown方法
	if twSessionManager, ok := server.sessionManager.(*session.TimeWheelSessionManager); ok {
		twSessionManager.Shutdown()
	}

	serverCtx, cancel := context.WithCancel(userCtx)
	defer cancel()

	go func() {
		defer pkg.Recover()

		// 等待所有正在处理的请求完成
		server.inFlyRequest.Wait()

		// 关闭所有会话
		server.sessionManager.CloseAllSessions()

		// 通知服务器关闭完成
		cancel()
	}()

	return server.transport.Shutdown(userCtx, serverCtx)
}

// GetSessionState 获取会话状态（用于内部使用）
func (server *Server) GetSessionState(sessionID string) (*session.State, bool) {
	return server.sessionManager.GetSession(sessionID)
}

// GetSession 获取会话数据
func (server *Server) GetSession(sessionID string) (*SessionData, bool) {
	state, ok := server.GetSessionState(sessionID)
	if !ok {
		return nil, false
	}

	data, ok := state.Data.(*SessionData)
	if !ok {
		return nil, false
	}

	return data, true
}

// GetSessionChan 获取会话的消息通道
func (a *sessionManagerAdapter) GetSessionChan(sessionID string) (chan []byte, bool) {
	return a.server.sessionManager.GetSessionChan(sessionID)
}

// CreateSession 创建新的会话，返回会话ID和消息通道
func (a *sessionManagerAdapter) CreateSession() (string, chan []byte) {
	// 创建会话数据
	data := &SessionData{
		reqID2respChan: cmap.New[chan *protocol.JSONRPCResponse](),
		first:          true,
		readyChan:      make(chan struct{}),
	}

	// 使用会话管理器创建会话
	sessionID, state := a.server.sessionManager.CreateSession(data)
	return sessionID, state.MessageChan
}

// CloseSession 关闭会话
func (a *sessionManagerAdapter) CloseSession(sessionID string) {
	a.server.sessionManager.CloseSession(sessionID)
}

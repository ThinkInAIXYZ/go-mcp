package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server/session"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type Option func(*Server)

func WithCapabilities(capabilities protocol.ServerCapabilities) Option {
	return func(s *Server) {
		s.capabilities = &capabilities
	}
}

func WithServerInfo(serverInfo protocol.Implementation) Option {
	return func(s *Server) {
		s.serverInfo = &serverInfo
	}
}

func WithInstructions(instructions string) Option {
	return func(s *Server) {
		s.instructions = instructions
	}
}

func WithSessionMaxIdleTime(maxIdleTime time.Duration) Option {
	return func(s *Server) {
		s.sessionManager.SetMaxIdleTime(maxIdleTime)
	}
}

func WithLogger(logger pkg.Logger) Option {
	return func(s *Server) {
		s.logger = logger
	}
}

// ToolMiddleware defines the middleware type of the tool handler
// Allow ToolHandlerFunc to be wrapped like a chain call
type ToolMiddleware func(ToolHandlerFunc) ToolHandlerFunc

// RateLimitMiddleware Return a rate-limiting middleware
func RateLimitMiddleware(limiter pkg.RateLimiter) ToolMiddleware {
	return func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			if limiter != nil && !limiter.Allow(req.Name) {
				return nil, pkg.ErrRateLimitExceeded
			}
			return next(ctx, req)
		}
	}
}

func WithPagination(limit int) Option {
	return func(s *Server) {
		s.paginationLimit = limit
	}
}

func WithGenSessionIDFunc(genSessionID func(context.Context) string) Option {
	return func(s *Server) {
		s.genSessionID = genSessionID
	}
}

type Server struct {
	transport transport.ServerTransport

	tools             pkg.SyncMap[*toolEntry]
	prompts           pkg.SyncMap[*promptEntry]
	resources         pkg.SyncMap[*resourceEntry]
	resourceTemplates pkg.SyncMap[*resourceTemplateEntry]

	sessionManager *session.Manager

	inShutdown   *pkg.AtomicBool // true when server is in shutdown
	inFlyRequest sync.WaitGroup

	capabilities *protocol.ServerCapabilities
	serverInfo   *protocol.Implementation
	instructions string

	paginationLimit int

	logger pkg.Logger

	genSessionID func(ctx context.Context) string
}

func NewServer(t transport.ServerTransport, opts ...Option) (*Server, error) {
	server := &Server{
		transport: t,
		capabilities: &protocol.ServerCapabilities{
			Prompts:   &protocol.PromptsCapability{ListChanged: true},
			Resources: &protocol.ResourcesCapability{ListChanged: true, Subscribe: true},
			Tools:     &protocol.ToolsCapability{ListChanged: true},
		},
		inShutdown:   pkg.NewAtomicBool(),
		serverInfo:   &protocol.Implementation{},
		logger:       pkg.DefaultLogger,
		genSessionID: func(context.Context) string { return uuid.NewString() },
	}

	t.SetReceiver(transport.ServerReceiverF(server.receive))

	server.sessionManager = session.NewManager(server.sessionDetection, server.genSessionID)

	for _, opt := range opts {
		opt(server)
	}

	server.sessionManager.SetLogger(server.logger)

	t.SetSessionManager(server.sessionManager)

	return server, nil
}

func (server *Server) Run() error {
	go func() {
		defer pkg.Recover()

		server.sessionManager.StartHeartbeatAndCleanInvalidSessions()
	}()

	if err := server.transport.Run(); err != nil {
		return fmt.Errorf("init mcp server transpor run fail: %w", err)
	}
	return nil
}

type toolEntry struct {
	tool    *protocol.Tool
	handler ToolHandlerFunc
}

type ToolHandlerFunc func(context.Context, *protocol.CallToolRequest) (*protocol.CallToolResult, error)

func (server *Server) RegisterTool(tool *protocol.Tool, toolHandler ToolHandlerFunc, middlewares ...ToolMiddleware) {
	for i := len(middlewares) - 1; i >= 0; i-- {
		toolHandler = middlewares[i](toolHandler)
	}
	server.tools.Store(tool.Name, &toolEntry{tool: tool, handler: toolHandler})
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4ToolListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification toll list changes fail: %v", err)
			return
		}
	}
}

func (server *Server) UnregisterTool(name string) {
	server.tools.Delete(name)
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4ToolListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification toll list changes fail: %v", err)
			return
		}
	}
}

type promptEntry struct {
	prompt  *protocol.Prompt
	handler PromptHandlerFunc
}

type PromptHandlerFunc func(context.Context, *protocol.GetPromptRequest) (*protocol.GetPromptResult, error)

func (server *Server) RegisterPrompt(prompt *protocol.Prompt, promptHandler PromptHandlerFunc) {
	server.prompts.Store(prompt.Name, &promptEntry{prompt: prompt, handler: promptHandler})
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4PromptListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification prompt list changes fail: %v", err)
			return
		}
	}
}

func (server *Server) UnregisterPrompt(name string) {
	server.prompts.Delete(name)
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4PromptListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification prompt list changes fail: %v", err)
			return
		}
	}
}

type resourceEntry struct {
	resource *protocol.Resource
	handler  ResourceHandlerFunc
}

type ResourceHandlerFunc func(context.Context, *protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error)

func (server *Server) RegisterResource(resource *protocol.Resource, resourceHandler ResourceHandlerFunc) {
	server.resources.Store(resource.URI, &resourceEntry{resource: resource, handler: resourceHandler})
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4ResourceListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification resource list changes fail: %v", err)
			return
		}
	}
}

func (server *Server) UnregisterResource(uri string) {
	server.resources.Delete(uri)
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4ResourceListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification resource list changes fail: %v", err)
			return
		}
	}
}

type resourceTemplateEntry struct {
	resourceTemplate *protocol.ResourceTemplate
	handler          ResourceHandlerFunc
}

func (server *Server) RegisterResourceTemplate(resource *protocol.ResourceTemplate, resourceHandler ResourceHandlerFunc) error {
	if err := resource.ParseURITemplate(); err != nil {
		return err
	}
	server.resourceTemplates.Store(resource.URITemplate, &resourceTemplateEntry{resourceTemplate: resource, handler: resourceHandler})
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4ResourceListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification resource list changes fail: %v", err)
			return nil
		}
	}
	return nil
}

func (server *Server) UnregisterResourceTemplate(uriTemplate string) {
	server.resourceTemplates.Delete(uriTemplate)
	if !server.sessionManager.IsEmpty() {
		if err := server.sendNotification4ResourceListChanges(context.Background()); err != nil {
			server.logger.Warnf("send notification resource list changes fail: %v", err)
			return
		}
	}
}

func (server *Server) Shutdown(userCtx context.Context) error {
	server.inShutdown.Store(true)

	serverCtx, cancel := context.WithCancel(userCtx)
	defer cancel()

	go func() {
		defer pkg.Recover()

		server.inFlyRequest.Wait()
		cancel()
	}()

	server.sessionManager.StopHeartbeat()

	return server.transport.Shutdown(userCtx, serverCtx)
}

func (server *Server) sessionDetection(ctx context.Context, sessionID string) error {
	if server.inShutdown.Load() {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if _, err := server.Ping(setSessionIDToCtx(ctx, sessionID), protocol.NewPingRequest()); err != nil {
		return err
	}
	return nil
}

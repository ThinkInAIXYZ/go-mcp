package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ThinkInAIXYZ/go-mcp/auth"
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

// ToolHandlerFunc and ToolMiddleware are defined and stay in the server package.
type ToolHandlerFunc func(context.Context, *protocol.CallToolRequest) (*protocol.CallToolResult, error)
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

func WithAuth(authServer *auth.Server, toolScopes map[string][]string) Option {
	return func(s *Server) {
		if authServer == nil {
			return
		}

		middleware := auth.NewMiddleware(authServer)
		s.authMiddleware = middleware

		s.authHandler = auth.NewHandler(authServer)
		s.authServer = authServer

		if len(toolScopes) > 0 {
			// Call the generic function with the specific type from this package.
			mw := auth.ScopeBasedToolMiddleware[ToolHandlerFunc](middleware, toolScopes)
			s.Use(mw)
		} else {
			// Call the generic function with the specific type from this package.
			mw := auth.MCPToolMiddleware[ToolHandlerFunc](middleware, &auth.MiddlewareConfig{
				RequiredScopes: []string{"read"},
			})
			s.Use(mw)
		}
	}
}

type ToolFilter func(context.Context, []*protocol.Tool) []*protocol.Tool

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

	globalMiddlewares []ToolMiddleware

	toolFilters ToolFilter

	authMiddleware *auth.Middleware
	authServer     *auth.Server
	authHandler    *auth.Handler
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

	if server.authMiddleware != nil {
		t.ApplyAuthMiddleware(server.authMiddleware.HTTPMiddleware)
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

func (server *Server) SetToolFilter(filter ToolFilter) {
	server.toolFilters = filter
}

type toolEntry struct {
	tool    *protocol.Tool
	handler ToolHandlerFunc
}

func (server *Server) RegisterTool(tool *protocol.Tool, toolHandler ToolHandlerFunc, middlewares ...ToolMiddleware) {
	for i := len(middlewares) - 1; i >= 0; i-- {
		toolHandler = middlewares[i](toolHandler)
	}

	finalHandler := server.buildMiddlewareChain(toolHandler)

	server.tools.Store(tool.Name, &toolEntry{tool: tool, handler: finalHandler})
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

func (server *Server) Use(middlewares ...ToolMiddleware) {
	server.globalMiddlewares = append(server.globalMiddlewares, middlewares...)
}

func (server *Server) buildMiddlewareChain(finalHandler ToolHandlerFunc) ToolHandlerFunc {
	if len(server.globalMiddlewares) == 0 {
		return finalHandler
	}

	handler := finalHandler
	for i := len(server.globalMiddlewares) - 1; i >= 0; i-- {
		handler = server.globalMiddlewares[i](handler)
	}
	return handler
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

func (s *Server) WrapWithAuth(handler http.Handler) http.Handler {
	if s.authMiddleware == nil {
		return handler
	}
	return s.authMiddleware.HTTPMiddleware(handler)
}

func (server *Server) GetOAuthHandler() *auth.Handler {
	return server.authHandler
}

// RegisterOAuthRoutes automatically registers OAuth routes with transport
// Only effective if transport implements the HTTPRouteRegistrar interface
func (server *Server) RegisterOAuthRoutes() error {
	if server.authHandler == nil {
		return fmt.Errorf("OAuth not configured, use WithAuth option")
	}

	// 尝试获取 HTTPRouteRegistrar 接口
	registrar, ok := server.transport.(interface {
		RegisterHandler(pattern string, handler http.Handler) error
	})

	if !ok {
		return fmt.Errorf("transport does not support HTTP route registration")
	}

	// 注册 OAuth 路由
	routes := map[string]http.Handler{
		"/oauth/authorize":  http.HandlerFunc(server.authHandler.HandleAuthorization),
		"/oauth/token":      http.HandlerFunc(server.authHandler.HandleToken),
		"/oauth/introspect": http.HandlerFunc(server.authHandler.HandleIntrospection),
		"/oauth/revoke":     http.HandlerFunc(server.authHandler.HandleRevocation),
		"/register":         http.HandlerFunc(server.authHandler.GetDynamicRegistrationHandler().HandleRegister),
		"/.well-known/oauth-authorization-server": server.authServer.GetMetadataProvider(),
		"/.well-known/oauth-protected-resource":   http.HandlerFunc(server.authServer.GetMetadataProvider().ServeProtectedResourceMetadata),
		"/.well-known/jwks.json":                  server.authServer.GetJWKSProvider(),
	}

	for pattern, handler := range routes {
		if err := registrar.RegisterHandler(pattern, handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", pattern, err)
		}
	}

	return nil
}

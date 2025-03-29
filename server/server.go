package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go-mcp/pkg"
	"go-mcp/protocol"
	"go-mcp/transport"

	cmap "github.com/orcaman/concurrent-map/v2"
)

type Server struct {
	transport transport.ServerTransport

	tools              []*protocol.Tool
	toolHandlers       map[string]ToolHandlerFunc
	prompts            []protocol.Prompt
	promptHandlers     map[string]PromptHandlerFunc
	resources          []protocol.Resource
	resourceHandlers   map[string]ResourceHandlerFunc
	resourceTemplates  []protocol.ResourceTemplate
	completionHandlers map[string]CompletionHandlerFunc

	cancelledNotifyHandler func(ctx context.Context, notifyParam *protocol.CancelledNotification) error

	// TODO：需要定期清理无效session
	sessionID2session *pkg.MemorySessionStore

	inShutdown   atomic.Bool // true when server is in shutdown
	inFlyRequest sync.WaitGroup

	// The result requirements
	protocolVersion string
	capabilities    protocol.ServerCapabilities
	serverInfo      protocol.Implementation

	logger pkg.Logger
}

type session struct {
	requestID atomic.Int64

	reqID2respChan cmap.ConcurrentMap[string, chan *protocol.JSONRPCResponse]

	// cache client initialize reqeust info
	clientInitializeRequest *protocol.InitializeRequest

	// subscribed resources
	subscribedResources cmap.ConcurrentMap[string, struct{}]

	first     bool
	readyChan chan struct{}
}

func newSession() *session {
	return &session{
		reqID2respChan:      cmap.New[chan *protocol.JSONRPCResponse](),
		subscribedResources: cmap.New[struct{}](),
	}
}

func NewServer(t transport.ServerTransport, opts ...Option) (*Server, error) {
	server := &Server{
		transport:         t,
		logger:            pkg.DefaultLogger,
		sessionID2session: pkg.NewMemorySessionStore(),
		protocolVersion:   protocol.PROTOCOL_VERSION,
		capabilities: protocol.ServerCapabilities{
			Prompts: &protocol.PromptsCapability{
				ListChanged: true,
			},
			Resources: &protocol.ResourcesCapability{
				Subscribe:   true,
				ListChanged: true,
			},
			Tools: &protocol.ToolsCapability{
				ListChanged: true,
			},
		},
	}
	t.SetReceiver(server)

	for _, opt := range opts {
		opt(server)
	}

	return server, nil
}

func (server *Server) Start() error {
	if err := server.transport.Run(); err != nil {
		return fmt.Errorf("init mcp server transpor start fail: %w", err)
	}
	return nil
}

type ToolHandlerFunc func(protocol.CallToolRequest) (*protocol.CallToolResult, error)

func (server *Server) AddTool(tool *protocol.Tool, toolHandler ToolHandlerFunc) {
	server.tools = append(server.tools, tool)
	if server.toolHandlers == nil {
		server.toolHandlers = map[string]ToolHandlerFunc{}
	}
	server.toolHandlers[tool.Name] = toolHandler
}

type PromptHandlerFunc func(protocol.GetPromptRequest) (*protocol.GetPromptResult, error)

func (server *Server) AddPrompt(prompt protocol.Prompt, promptHandler PromptHandlerFunc) {
	server.prompts = append(server.prompts, prompt)
	if server.promptHandlers == nil {
		server.promptHandlers = map[string]PromptHandlerFunc{}
	}
	server.promptHandlers[prompt.Name] = promptHandler
}

type ResourceHandlerFunc func(protocol.ReadResourceRequest) (*protocol.ReadResourceResult, error)

func (server *Server) AddResource(resource protocol.Resource, resourceHandler ResourceHandlerFunc) {
	server.resources = append(server.resources, resource)
	if server.resourceHandlers == nil {
		server.resourceHandlers = map[string]ResourceHandlerFunc{}
	}
	server.resourceHandlers[resource.URI] = resourceHandler
}

func (server *Server) AddResourceTemplate(tmpl protocol.ResourceTemplate) {
	server.resourceTemplates = append(server.resourceTemplates, tmpl)
}

type CompletionHandlerFunc func(protocol.CompleteRequest) (*protocol.CompleteResult, error)

func (server *Server) AddCompletion(id string, handler CompletionHandlerFunc) {
	if server.completionHandlers == nil {
		server.completionHandlers = map[string]CompletionHandlerFunc{}
	}

	server.completionHandlers[id] = handler
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

	return server.transport.Shutdown(userCtx, serverCtx)
}

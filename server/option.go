package server

import (
	"context"
	"go-mcp/pkg"
	"go-mcp/protocol"
)

type Option func(*Server)

func WithCancelNotifyHandler(handler func(ctx context.Context, notifyParam *protocol.CancelledNotification) error) Option {
	return func(s *Server) {
		s.cancelledNotifyHandler = handler
	}
}

func WithLogger(logger pkg.Logger) Option {
	return func(s *Server) {
		s.logger = logger
	}
}

func WithProtocolVersion(protocolVersion string) Option {
	return func(s *Server) {
		s.protocolVersion = protocolVersion
	}
}

func WithCapabilities(capabilities protocol.ServerCapabilities) Option {
	return func(s *Server) {
		s.capabilities = capabilities
	}
}

func WithInfo(serverInfo protocol.Implementation) Option {
	return func(s *Server) {
		s.serverInfo = serverInfo
	}
}

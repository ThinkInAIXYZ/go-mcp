package client

import (
	"context"
	"encoding/json"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

type SamplingHandler interface {
	CreateMessage(ctx context.Context, request *protocol.CreateMessageRequest) (*protocol.CreateMessageResult, error)
}

// NotifyHandler
// When implementing a custom NotifyHandler, you can combine it with BaseNotifyHandler to implement it on demand without implementing extra methods.
type NotifyHandler interface {
	ToolsListChanged(ctx context.Context, request *protocol.ToolListChangedNotification) error
	PromptListChanged(ctx context.Context, request *protocol.PromptListChangedNotification) error
	ResourceListChanged(ctx context.Context, request *protocol.ResourceListChangedNotification) error
	ResourcesUpdated(ctx context.Context, request *protocol.ResourceUpdatedNotification) error
}

type BaseNotifyHandler struct {
	Logger pkg.Logger
}

func NewBaseNotifyHandler() *BaseNotifyHandler {
	return &BaseNotifyHandler{pkg.DefaultLogger}
}

func (handler *BaseNotifyHandler) ToolsListChanged(_ context.Context, request *protocol.ToolListChangedNotification) error {
	return handler.defaultNotifyHandler(protocol.NotificationToolsListChanged, request)
}

func (handler *BaseNotifyHandler) PromptListChanged(_ context.Context, request *protocol.PromptListChangedNotification) error {
	return handler.defaultNotifyHandler(protocol.NotificationPromptsListChanged, request)
}

func (handler *BaseNotifyHandler) ResourceListChanged(_ context.Context, request *protocol.ResourceListChangedNotification) error {
	return handler.defaultNotifyHandler(protocol.NotificationResourcesListChanged, request)
}

func (handler *BaseNotifyHandler) ResourcesUpdated(_ context.Context, request *protocol.ResourceUpdatedNotification) error {
	return handler.defaultNotifyHandler(protocol.NotificationResourcesUpdated, request)
}

func (handler *BaseNotifyHandler) defaultNotifyHandler(method protocol.Method, notify interface{}) error {
	b, err := json.Marshal(notify)
	if err != nil {
		return err
	}
	handler.Logger.Infof("receive notify: method=%s, notify=%s", method, b)
	return nil
}

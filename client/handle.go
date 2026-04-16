package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

func (client *Client) handleRequestWithPing() (*protocol.PingResult, error) {
	return protocol.NewPingResult(), nil
}

func (client *Client) handleRequestWithCreateMessagesSampling(ctx context.Context, rawParams json.RawMessage) (*protocol.CreateMessageResult, error) {
	if client.clientCapabilities.Sampling == nil {
		return nil, pkg.ErrClientNotSupport
	}

	var request *protocol.CreateMessageRequest
	if err := pkg.JSONUnmarshal(rawParams, &request); err != nil {
		return nil, err
	}

	return client.samplingHandler.CreateMessage(ctx, request)
}

func (client *Client) handleNotifyWithToolsListChanged(ctx context.Context, rawParams json.RawMessage) error {
	notify := &protocol.ToolListChangedNotification{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, notify); err != nil {
			return err
		}
	}
	return client.notifyHandler.ToolsListChanged(ctx, notify)
}

func (client *Client) handleNotifyWithPromptsListChanged(ctx context.Context, rawParams json.RawMessage) error {
	notify := &protocol.PromptListChangedNotification{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, notify); err != nil {
			return err
		}
	}
	return client.notifyHandler.PromptListChanged(ctx, notify)
}

func (client *Client) handleNotifyWithResourcesListChanged(ctx context.Context, rawParams json.RawMessage) error {
	notify := &protocol.ResourceListChangedNotification{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, notify); err != nil {
			return err
		}
	}
	return client.notifyHandler.ResourceListChanged(ctx, notify)
}

func (client *Client) handleNotifyWithResourcesUpdated(ctx context.Context, rawParams json.RawMessage) error {
	notify := &protocol.ResourceUpdatedNotification{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, notify); err != nil {
			return err
		}
	}
	return client.notifyHandler.ResourcesUpdated(ctx, notify)
}

func (client *Client) handleNotifyWithProgress(ctx context.Context, rawParams json.RawMessage) error {
	notify := &protocol.ProgressNotification{}
	if len(rawParams) > 0 {
		if err := pkg.JSONUnmarshal(rawParams, notify); err != nil {
			return err
		}
	}
	client.progressChanRW.RLock()
	defer client.progressChanRW.RUnlock()

	ch, ok := client.progressToken2notifyChan[fmt.Sprint(notify.ProgressToken)]
	if !ok {
		return fmt.Errorf("progress token not found")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*1)
	defer cancel()

	select {
	case ch <- notify:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

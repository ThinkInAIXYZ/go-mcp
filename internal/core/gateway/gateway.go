package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/ThinkInAIXYZ/go-mcp/internal/core/protocol"
	"github.com/ccheers/xpkg/generic/arrayx"
	"github.com/ccheers/xpkg/sync/errgroup"
)

type IGateway interface {
	Handle(ctx context.Context, pipe io.ReadWriteCloser) error
}

type GatewayOptions struct {
	handleNotFound NotFoundHandleFunc
}

func defaultGatewayOptions() GatewayOptions {
	return GatewayOptions{
		handleNotFound: DefaultNotFoundHandleFunc,
	}
}

type NotFoundHandleFunc func(ctx context.Context, method string, message json.RawMessage) (json.RawMessage, error)

type IHandler interface {
	Handle(ctx context.Context, message json.RawMessage) (json.RawMessage, error)
	Method() protocol.McpMethod
}

type Gateway struct {
	options GatewayOptions

	writePackCH chan *protocol.JsonrpcResponse

	handlers map[protocol.McpMethod]IHandler
}

func NewGateway(handlers []IHandler) *Gateway {
	options := defaultGatewayOptions()
	return &Gateway{
		options:     options,
		writePackCH: make(chan *protocol.JsonrpcResponse, 2048),
		handlers: arrayx.BuildMap(handlers, func(t IHandler) protocol.McpMethod {
			return t.Method()
		}),
	}
}

func (x *Gateway) Handle(ctx context.Context, reader io.Reader, writer io.Writer) error {
	eg := errgroup.WithCancel(ctx)
	eg.Go(func(ctx context.Context) error {
		return x.readLoop(ctx, reader)
	})
	eg.Go(func(ctx context.Context) error {
		return x.writeLoop(ctx, writer)
	})
	return eg.Wait()
}

func (x *Gateway) readLoop(ctx context.Context, reader io.Reader) error {
	decoder := json.NewDecoder(reader)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		err := func() error {
			var req protocol.JsonrpcRequest
			err := decoder.Decode(&req)
			if err != nil {
				return err
			}
			log.Println("req====", req)
			// handle request
			handler, ok := x.handlers[protocol.McpMethod(req.Method)]
			var respBs json.RawMessage
			if !ok {
				respBs, err = x.options.handleNotFound(ctx, req.Method, req.Params)
			} else {
				respBs, err = handler.Handle(ctx, req.Params)
			}
			if req.IsNotification() {
				return nil
			}
			if err != nil {
				x.writePackCH <- protocol.NewJsonrpcResponse(req.GetID(), nil, &protocol.JsonrpcError{
					Code:    -1,
					Message: err.Error(),
					Data:    "",
				})
				return nil
			}
			x.writePackCH <- protocol.NewJsonrpcResponse(req.GetID(), respBs, nil)
			return nil
		}()
		if err != nil {
			return err
		}
	}
}

func (x *Gateway) writeLoop(ctx context.Context, writer io.Writer) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case pack := <-x.writePackCH:
			encoder := json.NewEncoder(writer)
			err := encoder.Encode(pack)
			if err != nil {
				return err
			}
		}
	}
}

func DefaultNotFoundHandleFunc(ctx context.Context, method string, message json.RawMessage) (json.RawMessage, error) {
	log.Println("message====", string(message))
	return nil, fmt.Errorf("method(%s) not found", method)
}

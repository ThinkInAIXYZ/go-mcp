package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type currentTimeReq struct {
	Timezone string `json:"timezone" description:"current time timezone"`
}

func main() {
	messageEndpointURL := "/message"

	sseTransport, mcpHandler, err := transport.NewSSEServerTransportAndHandler(messageEndpointURL)
	if err != nil {
		log.Panicf("new sse transport and hander with error: %v", err)
	}

	mcpServer, err := server.NewServer(sseTransport,
		server.WithServerInfo(protocol.Implementation{
			Name:    "mcp-example",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		panic(err)
	}

	tool, err := protocol.NewTool("current_time", "Get current time with timezone, Asia/Shanghai is default", currentTimeReq{})
	if err != nil {
		panic(fmt.Sprintf("Failed to create tool: %v", err))
	}

	mcpServer.RegisterTool(tool, currentTime)

	router := http.NewServeMux()
	router.HandleFunc("/sse", mcpHandler.HandleSSE().ServeHTTP)
	router.HandleFunc(messageEndpointURL, mcpHandler.HandleMessage().ServeHTTP)

	// Can be replaced by using gin framework
	// router := gin.Default()
	// router.GET("/sse", func(ctx *gin.Context) {
	// 	mcpHandler.HandleSSE().ServeHTTP(ctx.Writer, ctx.Request)
	// })
	// router.POST(messageEndpointURL, func(ctx *gin.Context) {
	// 	mcpHandler.HandleMessage().ServeHTTP(ctx.Writer, ctx.Request)
	// })

	httpServer := &http.Server{
		Addr:        ":8080",
		Handler:     router,
		IdleTimeout: time.Minute,
	}

	errCh := make(chan error, 3)
	go func() {
		errCh <- mcpServer.Run()
	}()

	go func() {
		if err = httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	if err = signalWaiter(errCh); err != nil {
		panic(fmt.Sprintf("signal waiter: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	httpServer.RegisterOnShutdown(func() {
		if err = mcpServer.Shutdown(ctx); err != nil {
			panic(err)
		}
	})

	if err = httpServer.Shutdown(ctx); err != nil {
		panic(err)
	}
}

func currentTime(_ context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	req := new(currentTimeReq)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, &req); err != nil {
		return nil, err
	}

	loc, err := time.LoadLocation(req.Timezone)
	if err != nil {
		return nil, fmt.Errorf("parse timezone with error: %v", err)
	}
	text := fmt.Sprintf(`current time is %s`, time.Now().In(loc))

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: text,
			},
		},
	}, nil
}

func signalWaiter(errCh chan error) error {
	signalToNotify := []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM}
	if signal.Ignored(syscall.SIGHUP) {
		signalToNotify = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, signalToNotify...)

	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM:
			log.Printf("Received signal: %s\n", sig)
			// graceful shutdown
			return nil
		}
	case err := <-errCh:
		return err
	}

	return nil
}

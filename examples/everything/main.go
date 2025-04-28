package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
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

type deleteFileReq struct {
	FileName string `json:"file_name" description:"file name"`
}

var srv *server.Server

func main() {
	// new mcp server with stdio or sse transport
	var err error
	srv, err = server.NewServer(
		getTransport(),
		server.WithServerInfo(protocol.Implementation{
			Name:    "current-time-v2-server",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	tool1, err := protocol.NewTool("current_time", "Get current time with timezone, Asia/Shanghai is default", currentTimeReq{})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
		return
	}

	tool2, err := protocol.NewTool("delete_file", "delete file", deleteFileReq{})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
		return
	}

	// register tool and start mcp server
	srv.RegisterTool(tool1, currentTime)
	srv.RegisterTool(tool2, deleteFile)
	// srv.RegisterResource()
	// srv.RegisterPrompt()
	// srv.RegisterResourceTemplate()

	errCh := make(chan error)
	go func() {
		errCh <- srv.Run()
	}()

	if err = signalWaiter(errCh); err != nil {
		log.Fatalf("signal waiter: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
}

func getTransport() (t transport.ServerTransport) {
	var mode, port, stateMode string
	flag.StringVar(&mode, "transport", "streamable_http", "The transport to use, should be \"stdio\" or \"sse\" or \"streamable_http\"")
	flag.StringVar(&port, "port", "8080", "sse server address")
	flag.StringVar(&stateMode, "state_mode", "stateful", "streamable_http server state mode, should be \"stateless\" or \"stateful\"")
	flag.Parse()

	switch mode {
	case "stdio":
		log.Println("start current time mcp server with stdio transport")
		t = transport.NewStdioServerTransport()
	case "sse":
		addr := fmt.Sprintf("127.0.0.1:%s", port)
		log.Printf("start current time mcp server with sse transport, listen %s", addr)
		t, _ = transport.NewSSEServerTransport(addr)
	case "streamable_http":
		addr := fmt.Sprintf("127.0.0.1:%s", port)
		log.Printf("start current time mcp server with streamable_http transport, listen %s", addr)
		t = transport.NewStreamableHTTPServerTransport(addr, transport.WithStreamableHTTPServerTransportOptionStateMode(transport.StateMode(stateMode)))
	default:
		panic(fmt.Errorf("unknown mode: %s", mode))
	}

	return t
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

func deleteFile(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	req := new(deleteFileReq)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, &req); err != nil {
		return nil, err
	}

	if err := requestConfirm(ctx); err != nil {
		return nil, err
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("delete file %s successfully", req.FileName),
			},
		},
	}, nil
}

func requestConfirm(ctx context.Context) error {
	resp, err := srv.Sampling(ctx, &protocol.CreateMessageRequest{
		Messages: []protocol.SamplingMessage{
			{
				Role: "user",
				Content: &protocol.TextContent{
					Type: "text",
					Text: "您确定要删除文件「${file_name}」吗？请回复\"是\"或\"否\"。?",
				},
			},
		},
		MaxTokens:      10,
		Temperature:    0.1,
		SystemPrompt:   "你是一个帮助用户确认是否删除文件的助手。请只回复'是'或'否'。",
		IncludeContext: "none",
	})
	if err != nil {
		return err
	}

	// 判断用户返回
	if resp.Content.GetType() != "text" {
		return errors.New("type is not text")
	}
	respContent := resp.Content.(*protocol.TextContent)

	if respContent.Text != "true" {
		return errors.New("respContent.Text !=true")
	}
	return nil
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

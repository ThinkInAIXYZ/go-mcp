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

type userIDKey struct{}

func setUserIDToCtx(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func getUserIDFromCtx(ctx context.Context) (string, error) {
	userID := ctx.Value(userIDKey{})
	if userID == nil {
		return "", errors.New("no userID found")
	}
	return userID.(string), nil
}

type currentTimeReq struct {
	Timezone string `json:"timezone" description:"current time timezone"`
}

func main() {
	messageEndpointURL := "/message"

	userParamKey := "user_id"
	paramKeysOpt := transport.WithSSEServerTransportAndHandlerOptionCopyParamKeys([]string{userParamKey})
	sseTransport, mcpHandler, err := transport.NewSSEServerTransportAndHandler(messageEndpointURL, paramKeysOpt)
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

	authentication := authenticationMiddleware(map[string][]string{
		tool.Name: {"test_1"},
	})
	mcpServer.RegisterTool(tool, currentTime, authentication)

	router := http.NewServeMux()
	router.HandleFunc("/sse", mcpHandler.HandleSSE().ServeHTTP)
	router.HandleFunc(messageEndpointURL, func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get(userParamKey)
		if userID == "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			if _, e := w.Write([]byte("lack user_id")); e != nil {
				fmt.Printf("writeError: %+v", e)
			}
			return
		}

		r = r.WithContext(setUserIDToCtx(r.Context(), userID))

		mcpHandler.HandleMessage().ServeHTTP(w, r)
	})

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

func authenticationMiddleware(toolName2UserID map[string][]string) server.ToolMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			userID, err := getUserIDFromCtx(ctx)
			if err != nil {
				return nil, err
			}

			for _, id := range toolName2UserID[req.Name] {
				if userID == id {
					return next(ctx, req)
				}
			}
			return nil, fmt.Errorf("user %s not authorized", userID)
		}
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

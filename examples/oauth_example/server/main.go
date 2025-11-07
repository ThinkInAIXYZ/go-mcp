package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/auth"
	"github.com/ThinkInAIXYZ/go-mcp/auth/memory"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func main() {
	// 1. Create the OAuth server
	store := memory.NewStore()
	signingKey := []byte("my-secret-key-must-be-32-bytes!!")

	authServer, _ := auth.NewServer(
		store,
		signingKey,
		auth.DefaultServerConfig(
			"http://localhost:8080",
			"http://localhost:8080/mcp",
		),
	)

	// 2. Register a demo client
	store.CreateClient(context.Background(), &auth.Client{
		ID:           "demo-client",
		Name:         "Demo Client",
		RedirectURIs: []string{"http://localhost:9999/callback"},
		GrantTypes:   []string{"authorization_code", "refresh_token"},
		Scopes:       []string{"read", "write"},
		IsPublic:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	// 3. Create SSE transport using NewSSEServerTransportAndHandler
	sseTransport, sseHandler, _ := transport.NewSSEServerTransportAndHandler(
		"http://localhost:8080/mcp/message",
	)

	// 4. Create the MCP server (integrated with OAuth)
	mcpServer, _ := server.NewServer(sseTransport,
		server.WithAuth(authServer, map[string][]string{
			"echo": {"read"},
		}),
	)

	// 5. Register a simple test tool
	echoTool, _ := protocol.NewTool("echo", "Echo back your message", struct {
		Message string `json:"message" jsonschema:"required,description=Your message"`
	}{})

	mcpServer.RegisterTool(echoTool, func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		msg := req.Arguments["message"].(string)
		userID := auth.GetUserID(ctx)

		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Echo: %s (user: %s)", msg, userID),
				},
			},
		}, nil
	})

	// 6. Manually create HTTP routes (including auto-approval for OAuth authorization)
	mux := http.NewServeMux()

	// ✅ MCP SSE endpoints
	mux.Handle("/mcp", sseHandler.HandleSSE())
	mux.Handle("/mcp/message", sseHandler.HandleMessage())

	// ✅ OAuth endpoints (custom authorization handler - auto-approve mode)
	oauthHandler := mcpServer.GetOAuthHandler()

	// Register the auto-approval authorization endpoint
	mux.HandleFunc("/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		handleAutoApproveAuthorize(w, r, authServer)
	})

	// Other standard OAuth endpoints
	mux.HandleFunc("/oauth/token", oauthHandler.HandleToken)
	mux.HandleFunc("/oauth/introspect", oauthHandler.HandleIntrospection)
	mux.HandleFunc("/oauth/revoke", oauthHandler.HandleRevocation)

	// Metadata endpoints
	mux.Handle("/.well-known/oauth-authorization-server", authServer.GetMetadataProvider())
	mux.Handle("/.well-known/jwks.json", authServer.GetJWKSProvider())

	// 7. Start the HTTP server and MCP server
	go mcpServer.Run()

	log.Println("🚀 Server started at http://localhost:8080")
	log.Println("   MCP SSE:   http://localhost:8080/mcp")
	log.Println("   OAuth:     http://localhost:8080/oauth/authorize")
	log.Println("")
	log.Println("👉 Run client: go run client/main.go")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// ✅ Auto-approve authorization requests (for demo/testing purposes)
func handleAutoApproveAuthorize(w http.ResponseWriter, r *http.Request, authServer *auth.Server) {
	// Parse authorization request
	req := &auth.AuthorizationRequest{
		ResponseType:        r.URL.Query().Get("response_type"),
		ClientID:            r.URL.Query().Get("client_id"),
		RedirectURI:         r.URL.Query().Get("redirect_uri"),
		Scope:               r.URL.Query().Get("scope"),
		State:               r.URL.Query().Get("state"),
		CodeChallenge:       r.URL.Query().Get("code_challenge"),
		CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
		Resource:            r.URL.Query()["resource"],
	}

	log.Printf("📝 Authorization request: client=%s, scope=%s", req.ClientID, req.Scope)

	// ✅ Auto-approve: use client ID as the user ID
	// In production, you should:
	// 1. Authenticate the user (check session/cookie)
	// 2. Show a consent screen to ask for user approval
	// 3. Generate an authorization code only after user consent
	userID := req.ClientID + "-user"

	// Generate authorization code and redirect
	redirectURL, err := authServer.HandleAuthorizationRequest(r.Context(), req, userID)
	if err != nil {
		log.Printf("❌ Authorization failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("✅ Auto-approved for user: %s", userID)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

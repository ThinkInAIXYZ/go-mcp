package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func main() {
	log.Println("🚀 OAuth Client Demo")

	// Step 1: OAuth Authorization Flow
	log.Println("\n📋 Step 1: Getting OAuth token...")
	codeVerifier, codeChallenge := generatePKCE()
	state := randomString(16)

	// Start callback server
	codeChan := make(chan string, 1)
	go startCallback(codeChan)
	time.Sleep(500 * time.Millisecond)

	// Build authorization URL
	authURL := fmt.Sprintf(
		"http://localhost:8080/oauth/authorize?response_type=code&client_id=demo-client&redirect_uri=http://localhost:9999/callback&scope=read&state=%s&code_challenge=%s&code_challenge_method=S256",
		state, codeChallenge,
	)

	log.Printf("📋 Authorization URL:\n   %s\n", authURL)
	log.Println("⏳ Waiting for authorization...")

	// Simulate a browser visit
	go func() {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(authURL)
		if err != nil {
			log.Printf("❌ Authorization request failed: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("❌ Authorization failed (%d): %s", resp.StatusCode, string(body))
		}
	}()

	// Add timeout protection
	select {
	case code := <-codeChan:
		if code == "" {
			log.Fatal("❌ Received empty authorization code")
		}
		log.Printf("✅ Got authorization code: %s...\n", code[:min(10, len(code))])

		// Exchange authorization code for tokens
		accessToken, refreshToken := exchangeToken(code, codeVerifier)
		if accessToken == "" {
			log.Fatal("❌ Failed to get access token")
		}
		log.Printf("✅ Got access token: %s...\n", accessToken[:min(20, len(accessToken))])

		// Step 2: Connect to MCP with token
		log.Println("\n📋 Step 2: Connecting to MCP with token...")

		sseTransport, err := transport.NewSSEClientTransport(
			"http://localhost:8080/mcp",
			transport.WithSSEClientTransportAuthToken(func() string {
				return accessToken
			}),
		)
		if err != nil {
			log.Fatalf("❌ Failed to create transport: %v", err)
		}

		mcpClient, err := client.NewClient(sseTransport,
			client.WithAuthAndRefresher(
				accessToken,
				refreshToken,
				time.Now().Add(15*time.Minute),
				func(rt string) (string, string, time.Time, error) {
					log.Println("🔄 Refreshing token...")
					newAccess, newRefresh := refreshToken_(rt)
					return newAccess, newRefresh, time.Now().Add(15 * time.Minute), nil
				},
			),
		)
		if err != nil {
			log.Fatalf("❌ Failed to create MCP client: %v", err)
		}
		defer mcpClient.Close()

		log.Println("✅ Connected to MCP Server")

		// Step 3: Call a registered tool
		log.Println("\n📋 Step 3: Calling tool...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := mcpClient.CallTool(ctx, &protocol.CallToolRequest{
			Name: "echo",
			Arguments: map[string]interface{}{
				"message": "Hello OAuth!",
			},
		})

		if err != nil {
			log.Fatalf("❌ Tool call failed: %v", err)
		}

		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*protocol.TextContent); ok {
				log.Printf("✅ Result: %s\n", textContent.Text)
			}
		}

		log.Println("\n✅ Demo completed!")
		log.Println("   Press Ctrl+C to exit")
		select {}

	case <-time.After(10 * time.Second):
		log.Fatal("❌ Timeout waiting for authorization code")
	}
}

// ========== Helper Functions ==========

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generatePKCE creates a PKCE code verifier and its corresponding challenge.
func generatePKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

// randomString generates a random URL-safe string of length n.
func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	result := base64.RawURLEncoding.EncodeToString(b)
	if len(result) > n {
		return result[:n]
	}
	return result
}

// startCallback starts a local HTTP server to handle OAuth redirect responses.
func startCallback(codeChan chan string) {
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errorCode := r.URL.Query().Get("error")

		if errorCode != "" {
			errorDesc := r.URL.Query().Get("error_description")
			log.Printf("❌ Authorization error: %s - %s", errorCode, errorDesc)
			fmt.Fprintf(w, "❌ Authorization failed: %s", errorDesc)
			codeChan <- ""
			return
		}

		if code == "" {
			log.Println("❌ No authorization code received")
			fmt.Fprintf(w, "❌ No authorization code received")
			codeChan <- ""
			return
		}

		log.Println("✅ Callback received")
		fmt.Fprintf(w, "✅ Authorization successful! You can close this window.")
		codeChan <- code
	})

	log.Println("🌐 Callback server started at http://localhost:9999")
	if err := http.ListenAndServe(":9999", nil); err != nil {
		log.Printf("❌ Callback server error: %v", err)
	}
}

// exchangeToken exchanges the authorization code for an access and refresh token.
func exchangeToken(code, verifier string) (accessToken, refreshToken string) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", "http://localhost:9999/callback")
	data.Set("client_id", "demo-client")
	data.Set("code_verifier", verifier)

	resp, err := http.Post(
		"http://localhost:8080/oauth/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		log.Printf("❌ Token request failed: %v", err)
		return "", ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Token endpoint error (%d): %s", resp.StatusCode, string(body))
		return "", ""
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		log.Printf("❌ Failed to parse token response: %v", err)
		return "", ""
	}

	if tokenResp.Error != "" {
		log.Printf("❌ Token error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
		return "", ""
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken
}

// refreshToken_ refreshes the access token using a refresh token.
func refreshToken_(refreshToken string) (accessToken, newRefreshToken string) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", "demo-client")

	resp, err := http.Post(
		"http://localhost:8080/oauth/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		log.Printf("❌ Token refresh failed: %v", err)
		return "", ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	json.Unmarshal(body, &tokenResp)

	return tokenResp.AccessToken, tokenResp.RefreshToken
}

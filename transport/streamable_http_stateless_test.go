package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// In stateless mode there are no sessions to terminate, so a DELETE to the MCP
// endpoint must respond 405 Method Not Allowed (per the Streamable HTTP session
// management spec), rather than 400 "Missing session ID".
func TestStatelessDeleteReturnsMethodNotAllowed(t *testing.T) {
	_, handler, err := NewStreamableHTTPServerTransportAndHandler(
		WithStreamableHTTPServerTransportAndHandlerOptionStateMode(Stateless),
	)
	if err != nil {
		t.Fatalf("NewStreamableHTTPServerTransportAndHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	w := httptest.NewRecorder()
	handler.HandleMCP().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("stateless DELETE: got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

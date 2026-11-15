package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamableHTTPClientTransport_customHeaders(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			var authorization, customHeader, sessionID string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("method = %q, want %q", r.Method, method)
				}
				authorization = r.Header.Get("Authorization")
				customHeader = r.Header.Get("X-Custom-Header")
				sessionID = r.Header.Get(sessionIDHeader)
				if method == http.MethodGet {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			client, err := NewStreamableHTTPClientTransport(server.URL,
				WithStreamableHTTPClientOptionHeader(map[string]string{
					"Authorization":   "Bearer test-token",
					"X-Custom-Header": "custom-value",
					sessionIDHeader:   "configured-session",
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			transport := client.(*streamableHTTPClientTransport)
			defer transport.cancel()
			transport.sessionID.Store("active-session")

			switch method {
			case http.MethodPost:
				err = transport.Send(context.Background(), Message(`{}`))
			case http.MethodGet:
				transport.startSSEStream()
			case http.MethodDelete:
				err = transport.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			if authorization != "Bearer test-token" {
				t.Errorf("Authorization = %q, want Bearer test-token", authorization)
			}
			if customHeader != "custom-value" {
				t.Errorf("X-Custom-Header = %q, want custom-value", customHeader)
			}
			if sessionID != "active-session" {
				t.Errorf("session ID = %q, want active-session", sessionID)
			}
		})
	}
}

package server

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

// A cancellation that cannot be matched to an in-progress request (e.g. in
// stateless mode, where there is no session to look it up in) is fire-and-forget
// per the MCP cancellation spec and must be ignored, not answered with an error.
func TestStatelessCancelledNotificationIgnored(t *testing.T) {
	reader, _ := io.Pipe()
	_, writer := io.Pipe()

	s, err := NewServer(transport.NewMockServerTransport(reader, writer))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	params := json.RawMessage(`{"requestId":"123","reason":"Request timed out"}`)

	// sessionID "" => no session (stateless); must be ignored, returning nil.
	if err := s.handleNotifyWithCancelled("", params); err != nil {
		t.Fatalf("stateless cancellation notification: got err %v, want nil (ignored)", err)
	}
}

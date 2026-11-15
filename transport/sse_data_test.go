package transport

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSSEClientTransport_multilineData(t *testing.T) {
	tests := []struct {
		name   string
		stream string
		want   []string
	}{
		{
			name:   "single line",
			stream: "data: {\"id\":1}\n\n",
			want:   []string{`{"id":1}`},
		},
		{
			name:   "multiline JSON",
			stream: "data: {\n: comment\ndata: \"id\":1,\ndata:\"result\":{}\ndata: }\n\n",
			want:   []string{"{\n\"id\":1,\n\"result\":{}\n}"},
		},
		{
			name:   "empty data lines",
			stream: "data:\ndata: first\ndata:\ndata: last\ndata:\n\n",
			want:   []string{"\nfirst\n\nlast\n"},
		},
		{
			name:   "empty event",
			stream: "data:\n\n",
			want:   []string{""},
		},
		{
			name:   "preserved whitespace",
			stream: "data:  first \r\ndata:\tsecond\t\r\n\r\n",
			want:   []string{" first \n\tsecond\t"},
		},
		{
			name:   "consecutive events",
			stream: "data: first\ndata: second\n\nevent: message\ndata: third\n\n",
			want:   []string{"first\nsecond", "third"},
		},
	}
	for _, transportType := range []string{"sse", "streamable HTTP"} {
		t.Run(transportType, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var got []string
					receiver := NewClientReceiver(func(_ context.Context, msg []byte) error {
						got = append(got, string(msg))
						return nil
					}, nil)
					if transportType == "sse" {
						client, err := NewSSEClientTransport("http://localhost/sse")
						if err != nil {
							t.Fatal(err)
						}
						transport := client.(*sseClientTransport)
						transport.ctx = context.Background()
						transport.SetReceiver(receiver)
						err = transport.readSSE(io.NopCloser(strings.NewReader("event: message\n" + tt.stream)))
						if !errors.Is(err, io.EOF) {
							t.Fatalf("readSSE() = %v, want EOF", err)
						}
					} else {
						client, err := NewStreamableHTTPClientTransport("http://localhost/mcp")
						if err != nil {
							t.Fatal(err)
						}
						transport := client.(*streamableHTTPClientTransport)
						defer transport.cancel()
						transport.SetReceiver(receiver)
						transport.handleSSEStream(io.NopCloser(strings.NewReader(tt.stream)))
					}
					if !reflect.DeepEqual(got, tt.want) {
						t.Errorf("messages = %q, want %q", got, tt.want)
					}
				})
			}
		})
	}
}

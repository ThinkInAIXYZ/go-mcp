package transport

import (
	"net/url"
	"testing"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

func Test_sseClientTransport_handleSSEEvent(t1 *testing.T) {
	type fields struct {
		serverURL *url.URL
		logger    pkg.Logger
	}
	type args struct {
		event string
		data  string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "1",
			fields: fields{
				serverURL: func() *url.URL {
					uri, err := url.Parse("https://api.baidu.com/mcp")
					if err != nil {
						panic(err)
					}
					return uri
				}(),
				logger: pkg.DefaultLogger,
			},
			args: args{
				event: "endpoint",
				data:  "/sse/messages",
			},
			want: "https://api.baidu.com/sse/messages",
		},
		{
			name: "2",
			fields: fields{
				serverURL: func() *url.URL {
					uri, err := url.Parse("https://api.baidu.com/mcp")
					if err != nil {
						panic(err)
					}
					return uri
				}(),
				logger: pkg.DefaultLogger,
			},
			args: args{
				event: "endpoint",
				data:  "https://api.google.com/sse/messages",
			},
			want: "https://api.google.com/sse/messages",
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &sseClientTransport{
				serverURL:    tt.fields.serverURL,
				logger:       tt.fields.logger,
				endpointChan: make(chan struct{}),
			}
			t.handleSSEEvent(tt.args.event, tt.args.data)
			if t.messageEndpoint.String() != tt.want {
				t1.Errorf("handleSSEEvent() = %v, want %v", t.messageEndpoint.String(), tt.want)
			}
		})
	}
}

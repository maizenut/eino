package mcp

import "testing"

func TestServerSpecValidate(t *testing.T) {
	cases := []struct {
		name    string
		spec    ServerSpec
		wantErr bool
	}{
		{
			name: "stdio_ok",
			spec: ServerSpec{Name: "s1", Transport: TransportStdio, Command: "mcp-server"},
		},
		{
			name: "http_ok",
			spec: ServerSpec{Name: "s1", Transport: TransportHTTP, URL: "http://localhost:1234"},
		},
		{
			name: "sse_ok",
			spec: ServerSpec{Name: "s1", Transport: TransportSSE, URL: "http://localhost:1234"},
		},
		{
			name: "websocket_ok",
			spec: ServerSpec{Name: "s1", Transport: TransportWebSocket, URL: "ws://localhost:1234"},
		},
		{
			name: "bytedmcp_ok_url",
			spec: ServerSpec{Name: "s1", Transport: TransportBytedMCP, URL: "http://gateway/mcp"},
		},
		{
			name: "bytedmcp_ok_psm",
			spec: ServerSpec{Name: "s1", Transport: TransportBytedMCP, Metadata: map[string]any{"psm": "data.mcp.demo"}},
		},
		{
			name:    "bytedmcp_missing_url_and_psm",
			spec:    ServerSpec{Name: "s1", Transport: TransportBytedMCP},
			wantErr: true,
		},
		{
			name:    "missing_name",
			spec:    ServerSpec{Transport: TransportStdio, Command: "mcp-server"},
			wantErr: true,
		},
		{
			name:    "missing_transport",
			spec:    ServerSpec{Name: "s1"},
			wantErr: true,
		},
		{
			name:    "stdio_missing_command",
			spec:    ServerSpec{Name: "s1", Transport: TransportStdio},
			wantErr: true,
		},
		{
			name:    "http_missing_url",
			spec:    ServerSpec{Name: "s1", Transport: TransportHTTP},
			wantErr: true,
		},
		{
			name:    "sse_missing_url",
			spec:    ServerSpec{Name: "s1", Transport: TransportSSE},
			wantErr: true,
		},
		{
			name:    "websocket_missing_url",
			spec:    ServerSpec{Name: "s1", Transport: TransportWebSocket},
			wantErr: true,
		},
		{
			name:    "unknown_transport",
			spec:    ServerSpec{Name: "s1", Transport: "grpc", URL: "http://x"},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spec.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

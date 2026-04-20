package stdio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	mcppkg "github.com/cloudwego/eino/composite/mcp"
)

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }

type fakeStarter struct {
	stdinBuf  *bytes.Buffer
	stdoutBuf *bytes.Buffer
	stopped   int
}

func (s *fakeStarter) Start(ctx context.Context, spec mcppkg.ServerSpec) (io.WriteCloser, io.ReadCloser, func(context.Context) error, error) {
	_ = ctx
	_ = spec
	if s.stdinBuf == nil {
		s.stdinBuf = &bytes.Buffer{}
	}
	if s.stdoutBuf == nil {
		s.stdoutBuf = &bytes.Buffer{}
	}
	stop := func(ctx context.Context) error {
		_ = ctx
		s.stopped++
		return nil
	}
	return nopWriteCloser{Writer: s.stdinBuf}, io.NopCloser(bytes.NewReader(s.stdoutBuf.Bytes())), stop, nil
}

func TestServerSpecValidate_Stdio(t *testing.T) {
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportStdio, Command: "mcp-server"}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestServerSpecValidate_UnsupportedTransport(t *testing.T) {
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: "grpc", URL: "http://example.com"}
	if err := spec.Validate(); err == nil {
		t.Fatalf("Validate error = nil, want error")
	}
}

func TestClientConnectInfoDisconnect(t *testing.T) {
	starter := &fakeStarter{stdoutBuf: bytes.NewBufferString(jsonRPCLine(t, 1, map[string]any{
		"serverInfo": map[string]any{
			"name":    "codebase-server",
			"version": "1.0.0",
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	}))}
	client := NewClient(starter)
	spec := mcppkg.ServerSpec{
		Name:      "codebase",
		Transport: mcppkg.TransportStdio,
		Command:   "mcp-server",
		Metadata:  map[string]any{"env": "test"},
	}

	if err := client.Connect(context.Background(), spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "codebase-server" {
		t.Fatalf("info.Name = %q, want codebase-server", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Fatalf("info.Version = %q, want 1.0.0", info.Version)
	}
	if len(info.Capabilities) != 1 || info.Capabilities[0] != "tools" {
		t.Fatalf("info.Capabilities = %#v, want [tools]", info.Capabilities)
	}
	if info.Metadata["env"] != "test" {
		t.Fatalf("info.Metadata = %#v, want env=test", info.Metadata)
	}
	info.Metadata["env"] = "changed"
	info2, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info second call: %v", err)
	}
	if info2.Metadata["env"] != "test" {
		t.Fatalf("info clone failed: %#v", info2.Metadata)
	}
	if err := client.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if starter.stopped != 1 {
		t.Fatalf("starter.stopped = %d, want 1", starter.stopped)
	}
}

func TestClientCallToolWritesJSONRPC(t *testing.T) {
	starter := &fakeStarter{stdoutBuf: bytes.NewBufferString(
		jsonRPCLine(t, 1, map[string]any{
			"serverInfo": map[string]any{
				"name": "codebase",
			},
			"capabilities": map[string]any{},
		}) + jsonRPCLine(t, 1, map[string]any{"content": "ok"}),
	)}
	client := NewClient(starter)
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportStdio, Command: "mcp-server"}
	if err := client.Connect(context.Background(), spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	result, err := client.CallTool(context.Background(), "search", map[string]any{"query": "hello"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	payload := starter.stdinBuf.String()
	if payload == "" {
		t.Fatalf("written payload is empty")
	}
	lines := bytes.Split([]byte(payload), []byte("\n"))
	if len(lines) < 2 || len(lines[1]) == 0 {
		t.Fatalf("written payload = %q, want initialize and tools/call lines", payload)
	}
	var req map[string]any
	if err := json.Unmarshal(lines[1], &req); err != nil {
		t.Fatalf("Unmarshal request: %v", err)
	}
	if req["method"] != "tools/call" {
		t.Fatalf("request method = %#v, want tools/call", req["method"])
	}
	params := req["params"].(map[string]any)
	if params["name"] != "search" {
		t.Fatalf("request params name = %#v, want search", params["name"])
	}
	resultMap, ok := result.(map[string]any)
	if !ok || resultMap["content"] != "ok" {
		t.Fatalf("result = %#v, want content=ok", result)
	}
}

func TestClientListRemoteCapabilities(t *testing.T) {
	starter := &fakeStarter{stdoutBuf: bytes.NewBufferString(
		jsonRPCLine(t, 1, map[string]any{
			"serverInfo": map[string]any{
				"name": "codebase",
			},
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
				"prompts":   map[string]any{},
			},
		}) +
			jsonRPCLine(t, 2, map[string]any{
				"tools": []map[string]any{{
					"name":        "search",
					"description": "search code",
				}},
			}) +
			jsonRPCLine(t, 3, map[string]any{
				"resources": []map[string]any{{
					"uri":         "file://repo",
					"name":        "repo",
					"description": "repository",
					"mimeType":    "text/plain",
				}},
			}) +
			jsonRPCLine(t, 4, map[string]any{
				"prompts": []map[string]any{{
					"name":        "review",
					"description": "review prompt",
					"arguments": map[string]string{
						"topic": "string",
					},
				}},
			}),
	)}

	client := NewClient(starter)
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportStdio, Command: "mcp-server"}
	if err := client.Connect(context.Background(), spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "search" {
		t.Fatalf("tools = %#v, want search", tools)
	}

	resources, err := client.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(resources) != 1 || resources[0].URI != "file://repo" {
		t.Fatalf("resources = %#v, want file://repo", resources)
	}

	prompts, err := client.ListPrompts(context.Background())
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(prompts) != 1 || prompts[0].Name != "review" {
		t.Fatalf("prompts = %#v, want review", prompts)
	}
}

func jsonRPCLine(t *testing.T, id int, result any) string {
	t.Helper()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal JSON-RPC response: %v", err)
	}
	return string(append(data, '\n'))
}

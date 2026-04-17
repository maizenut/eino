package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"testing"

	mcppkg "github.com/cloudwego/eino/mcp"
)

type fakeSSEDoer struct {
	t         *testing.T
	streamR   *io.PipeReader
	streamW   *io.PipeWriter
	requests  []*nethttp.Request
	getCount  int
	postCount int
}

func newFakeSSEDoer(t *testing.T) *fakeSSEDoer {
	reader, writer := io.Pipe()
	return &fakeSSEDoer{
		t:       t,
		streamR: reader,
		streamW: writer,
	}
}

func (d *fakeSSEDoer) Do(req *nethttp.Request) (*nethttp.Response, error) {
	d.requests = append(d.requests, req)
	switch req.Method {
	case nethttp.MethodGet:
		d.getCount++
		return &nethttp.Response{
			StatusCode: 200,
			Header:     make(nethttp.Header),
			Body:       d.streamR,
		}, nil
	case nethttp.MethodPost:
		d.postCount++
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))

		var rpcReq map[string]any
		if err := json.Unmarshal(body, &rpcReq); err != nil {
			return nil, err
		}
		method, _ := rpcReq["method"].(string)
		idValue, _ := rpcReq["id"].(float64)
		id := int(idValue)

		var result any
		switch method {
		case "initialize":
			result = map[string]any{
				"serverInfo": map[string]any{
					"name":    "codebase-sse",
					"version": "1.0.0",
				},
				"capabilities": map[string]any{
					"tools":     map[string]any{},
					"resources": map[string]any{},
					"prompts":   map[string]any{},
				},
			}
		case "tools/list":
			result = map[string]any{
				"tools": []map[string]any{{
					"name":        "search",
					"description": "search code",
				}},
			}
		case "resources/list":
			result = map[string]any{
				"resources": []map[string]any{{
					"uri":         "file://repo",
					"name":        "repo",
					"description": "repository",
					"mimeType":    "text/plain",
				}},
			}
		case "prompts/list":
			result = map[string]any{
				"prompts": []map[string]any{{
					"name":        "review",
					"description": "review prompt",
					"arguments": map[string]string{
						"topic": "string",
					},
				}},
			}
		default:
			result = map[string]any{"ok": true}
		}

		go func(payload string) {
			_, _ = io.WriteString(d.streamW, payload)
		}(sseEvent(d.t, id, result))
		return &nethttp.Response{
			StatusCode: 202,
			Header:     make(nethttp.Header),
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	default:
		return nil, nil
	}
}

func TestServerSpecValidate_SSE(t *testing.T) {
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportSSE, URL: "http://example.com/sse"}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestClientConnectInfoDisconnect(t *testing.T) {
	doer := newFakeSSEDoer(t)
	client := NewClient(doer)
	spec := mcppkg.ServerSpec{
		Name:      "codebase",
		Transport: mcppkg.TransportSSE,
		URL:       "http://example.com/sse",
		Headers:   map[string]string{"X-Test": "1"},
		Metadata:  map[string]any{"env": "test"},
	}

	if err := client.Connect(context.Background(), spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "codebase-sse" {
		t.Fatalf("info.Name = %q, want codebase-sse", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Fatalf("info.Version = %q, want 1.0.0", info.Version)
	}
	if len(info.Capabilities) != 3 {
		t.Fatalf("info.Capabilities = %#v, want 3 items", info.Capabilities)
	}
	if doer.getCount != 1 {
		t.Fatalf("getCount = %d, want 1", doer.getCount)
	}
	if len(doer.requests) == 0 || doer.requests[0].Header.Get("X-Test") != "1" {
		t.Fatalf("stream headers missing: %#v", doer.requests)
	}
	if err := client.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

func TestClientListRemoteCapabilities(t *testing.T) {
	doer := newFakeSSEDoer(t)
	client := NewClient(doer)
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportSSE, URL: "http://example.com/sse"}
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
	if doer.postCount != 4 {
		t.Fatalf("postCount = %d, want 4", doer.postCount)
	}
}

func sseEvent(t *testing.T, id int, result any) string {
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
	return "data: " + string(data) + "\n\n"
}

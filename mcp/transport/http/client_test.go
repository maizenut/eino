package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"testing"

	mcppkg "github.com/cloudwego/eino/mcp"
)

type fakeDoer struct {
	requests []*nethttp.Request
	handler  func(req *nethttp.Request, body []byte) (*nethttp.Response, error)
}

func (d *fakeDoer) Do(req *nethttp.Request) (*nethttp.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	d.requests = append(d.requests, req)
	return d.handler(req, body)
}

func TestServerSpecValidate_HTTP(t *testing.T) {
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportHTTP, URL: "http://example.com/mcp"}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestClientConnectInfoDisconnect(t *testing.T) {
	doer := &fakeDoer{
		handler: func(req *nethttp.Request, body []byte) (*nethttp.Response, error) {
			var rpcReq map[string]any
			if err := json.Unmarshal(body, &rpcReq); err != nil {
				t.Fatalf("Unmarshal request: %v", err)
			}
			if req.Method != nethttp.MethodPost {
				t.Fatalf("request method = %s, want POST", req.Method)
			}
			if req.URL.String() != "http://example.com/mcp" {
				t.Fatalf("request url = %s", req.URL.String())
			}
			if req.Header.Get("X-Test") != "1" {
				t.Fatalf("request header X-Test = %q, want 1", req.Header.Get("X-Test"))
			}
			if rpcReq["method"] != "initialize" {
				t.Fatalf("request method = %#v, want initialize", rpcReq["method"])
			}
			return jsonResponse(t, 1, map[string]any{
				"serverInfo": map[string]any{
					"name":    "codebase-http",
					"version": "1.0.0",
				},
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
			}), nil
		},
	}
	client := NewClient(doer)
	spec := mcppkg.ServerSpec{
		Name:      "codebase",
		Transport: mcppkg.TransportHTTP,
		URL:       "http://example.com/mcp",
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
	if info.Name != "codebase-http" {
		t.Fatalf("info.Name = %q, want codebase-http", info.Name)
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
	if err := client.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

func TestClientListRemoteCapabilities(t *testing.T) {
	doer := &fakeDoer{
		handler: func(req *nethttp.Request, body []byte) (*nethttp.Response, error) {
			var rpcReq map[string]any
			if err := json.Unmarshal(body, &rpcReq); err != nil {
				t.Fatalf("Unmarshal request: %v", err)
			}
			method, _ := rpcReq["method"].(string)
			switch method {
			case "initialize":
				return jsonResponse(t, 1, map[string]any{
					"serverInfo": map[string]any{"name": "codebase"},
					"capabilities": map[string]any{
						"tools":     map[string]any{},
						"resources": map[string]any{},
						"prompts":   map[string]any{},
					},
				}), nil
			case "tools/list":
				return jsonResponse(t, 1, map[string]any{
					"tools": []map[string]any{{
						"name":        "search",
						"description": "search code",
					}},
				}), nil
			case "resources/list":
				return jsonResponse(t, 2, map[string]any{
					"resources": []map[string]any{{
						"uri":         "file://repo",
						"name":        "repo",
						"description": "repository",
						"mimeType":    "text/plain",
					}},
				}), nil
			case "prompts/list":
				return jsonResponse(t, 3, map[string]any{
					"prompts": []map[string]any{{
						"name":        "review",
						"description": "review prompt",
						"arguments": map[string]string{
							"topic": "string",
						},
					}},
				}), nil
			default:
				t.Fatalf("unexpected method %q", method)
				return nil, nil
			}
		},
	}

	client := NewClient(doer)
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportHTTP, URL: "http://example.com/mcp"}
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

func jsonResponse(t *testing.T, id int, result any) *nethttp.Response {
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
	return &nethttp.Response{
		StatusCode: 200,
		Header:     make(nethttp.Header),
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

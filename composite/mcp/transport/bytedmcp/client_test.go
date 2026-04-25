package bytedmcp
package bytedmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcppkg "github.com/cloudwego/eino/composite/mcp"
)

func newTestServer(t *testing.T, handler func(method string, params map[string]any) (any, error)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     int64          `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			nethttp.Error(w, err.Error(), nethttp.StatusBadRequest)
			return
		}
		// Stash latest request on the response writer's request via a header round-trip.
		// Tests inspect via captured server fields.
		w.Header().Set("X-Echo-Method", req.Method)
		result, err := handler(req.Method, req.Params)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if err != nil {
			resp["error"] = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			resp["result"] = result
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

type capturingHandler struct {
	calls atomic.Int32
	last  struct {
		method  string
		params  map[string]any
		headers nethttp.Header
	}
}

func TestClient_HappyPath(t *testing.T) {
	cap := &capturingHandler{}
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     int64          `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		cap.calls.Add(1)
		cap.last.method = req.Method
		cap.last.params = req.Params
		cap.last.headers = r.Header.Clone()
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"serverInfo":   map[string]any{"name": "demo", "version": "1.0"},
				"capabilities": map[string]any{"tools": map[string]any{}},
			}
		case "tools/list":
			result = map[string]any{
				"tools": []map[string]any{{"name": "search", "description": "find"}},
			}
		case "tools/call":
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}
		default:
			result = map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srv.Close()

	c := NewClient(srv.Client())
	ctx := context.Background()
	spec := mcppkg.ServerSpec{Name: "internal", Transport: mcppkg.TransportBytedMCP, URL: srv.URL}
	if err := c.Connect(ctx, spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	info, err := c.Info(ctx)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "demo" || info.Version != "1.0" {
		t.Fatalf("info = %+v", info)
	}
	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "search" {
		t.Fatalf("tools = %+v", tools)
	}
	if _, err := c.CallTool(ctx, "search", map[string]any{"q": "hi"}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if cap.calls.Load() < 3 {
		t.Fatalf("expected at least 3 calls, got %d", cap.calls.Load())
	}
	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if _, err := c.ListTools(ctx); err == nil {
		t.Fatalf("expected error after Disconnect")
	}
}

func TestClient_RejectsWrongTransport(t *testing.T) {
	c := NewClient(nil)
	err := c.Connect(context.Background(), mcppkg.ServerSpec{Name: "x", Transport: mcppkg.TransportHTTP, URL: "http://x"})
	if err == nil || !strings.Contains(err.Error(), "bytedmcp") {
		t.Fatalf("expected transport rejection, got %v", err)
	}
}

func TestClient_CallToolMetaInjection(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     int64          `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Method == "tools/call" {
			captured = req.Params
		}
		var result any = map[string]any{}
		if req.Method == "initialize" {
			result = map[string]any{"serverInfo": map[string]any{"name": "demo"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srv.Close()

	c := NewClient(srv.Client())
	ctx := WithIdentityToken(context.Background(), "id-token-123")
	spec := mcppkg.ServerSpec{
		Name: "internal", Transport: mcppkg.TransportBytedMCP, URL: srv.URL,
		Metadata: map[string]any{
			"identity_forward": true,
			"call_tool_trace":  true,
			"meta_headers":     map[string]string{"x-tenant": "t1"},
			"meta_params":      map[string]any{"region": "cn"},
			"base_user_extra":  map[string]string{"user_id": "u1"},
		},
	}
	if err := c.Connect(ctx, spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := c.CallTool(ctx, "search", map[string]any{"q": "hi"}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	meta, ok := captured["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected _meta in params, got %v", captured)
	}
	if v, _ := meta["traceEnabled"].(bool); !v {
		t.Fatalf("traceEnabled missing in %v", meta)
	}
	headers, _ := meta["headers"].(map[string]any)
	if headers == nil {
		// JSON round-trip would yield map[string]any; if hook produced map[string]string that's fine too
		hs, _ := meta["headers"].(map[string]string)
		if hs["x-tenant"] != "t1" {
			t.Fatalf("expected meta.headers.x-tenant=t1, got %v", meta["headers"])
		}
	} else if headers["x-tenant"] != "t1" {
		t.Fatalf("expected meta.headers.x-tenant=t1, got %v", headers)
	}
	params, _ := meta["params"].(map[string]any)
	if params == nil || params["region"] != "cn" {
		t.Fatalf("expected meta.params.region=cn, got %v", meta["params"])
	}
}

func TestClient_RequiresURLOrPSM(t *testing.T) {
	c := NewClient(nil)
	err := c.Connect(context.Background(), mcppkg.ServerSpec{Name: "x", Transport: mcppkg.TransportBytedMCP})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "url or metadata.psm") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestClient_ResolvesViaControlPlane(t *testing.T) {
	srv := newJSONRPCServer(t)
	cp := &fakeResolver{url: srv.URL}
	c := NewClient(srv.Client(), WithControlPlane(cp))
	spec := mcppkg.ServerSpec{
		Name: "internal", Transport: mcppkg.TransportBytedMCP,
		Metadata: map[string]any{"psm": "data.mcp.demo", "region": "cn", "env": "prod"},
	}
	if err := c.Connect(context.Background(), spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if cp.calls != 1 {
		t.Fatalf("resolver should be called once, got %d", cp.calls)
	}
	if cp.lastPSM != "data.mcp.demo" || cp.lastRegion != "cn" || cp.lastEnv != "prod" {
		t.Fatalf("resolver inputs unexpected: %+v", cp)
	}
}

func TestClient_Timeout(t *testing.T) {
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		var req struct {
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if req.Method == "initialize" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
			return
		}
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{}})
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	spec := mcppkg.ServerSpec{
		Name: "internal", Transport: mcppkg.TransportBytedMCP, URL: srv.URL,
		Metadata: map[string]any{"request_timeout_ms": int64(20)},
	}
	if err := c.Connect(context.Background(), spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	_, err := c.CallTool(context.Background(), "slow", nil)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
}

// helpers ---------------------------------------------------------------

func newJSONRPCServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		var req struct {
			ID     int64          `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		var result any = map[string]any{}
		if req.Method == "initialize" {
			result = map[string]any{"serverInfo": map[string]any{"name": "demo"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

type fakeResolver struct {
	url        string
	calls      int
	lastPSM    string
	lastRegion string
	lastEnv    string
}

func (f *fakeResolver) Resolve(_ context.Context, psm, region, env string) (string, error) {
	f.calls++
	f.lastPSM, f.lastRegion, f.lastEnv = psm, region, env
	if f.url == "" {
		return "", fmt.Errorf("no url")
	}
	return f.url, nil
}

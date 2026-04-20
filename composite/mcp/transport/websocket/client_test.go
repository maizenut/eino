package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	mcppkg "github.com/cloudwego/eino/composite/mcp"
)

func TestServerSpecValidate_WebSocket(t *testing.T) {
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportWebSocket, URL: "ws://example.com/ws"}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestClientConnectAndListTools(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade: %v", err)
		}
		defer conn.Close()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req map[string]any
			if err := json.Unmarshal(msg, &req); err != nil {
				return
			}
			method, _ := req["method"].(string)
			id, _ := req["id"].(float64)
			var result any
			switch method {
			case "initialize":
				result = map[string]any{
					"serverInfo": map[string]any{
						"name":    "ws-server",
						"version": "0.1.0",
					},
					"capabilities": map[string]any{
						"tools": map[string]any{},
					},
				}
			case "tools/list":
				result = map[string]any{
					"tools": []map[string]any{{
						"name":        "search",
						"description": "search code",
					}},
				}
			default:
				result = map[string]any{"ok": true}
			}
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      int(id),
				"result":  result,
			}
			data, _ := json.Marshal(resp)
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	client := NewClient(nil)
	spec := mcppkg.ServerSpec{Name: "codebase", Transport: mcppkg.TransportWebSocket, URL: wsURL}
	if err := client.Connect(context.Background(), spec); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Disconnect(context.Background())

	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "ws-server" {
		t.Fatalf("info.Name = %q, want ws-server", info.Name)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "search" {
		t.Fatalf("tools = %#v, want search", tools)
	}
}


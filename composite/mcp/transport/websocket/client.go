package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	mcppkg "github.com/cloudwego/eino/composite/mcp"
)

type request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client is a lightweight WebSocket MCP client.
//
// It assumes one JSON-RPC request yields one JSON-RPC response message on the same WebSocket.
// Server push / notifications are ignored.
type Client struct {
	dialer *websocket.Dialer

	mu         sync.Mutex
	spec       *mcppkg.ServerSpec
	serverInfo *mcppkg.ServerInfo
	conn       *websocket.Conn
	connected  bool
	nextID     atomic.Int64
}

// NewClient creates a WebSocket MCP client.
func NewClient(dialer *websocket.Dialer) *Client {
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	return &Client{dialer: dialer}
}

// Connect dials the WebSocket endpoint and performs MCP initialize.
func (c *Client) Connect(ctx context.Context, spec mcppkg.ServerSpec, opts ...mcppkg.Option) error {
	debugLogf("connect begin: server=%s transport=%s url=%s", spec.Name, spec.Transport, spec.URL)
	if err := spec.Validate(); err != nil {
		debugLogf("connect validate failed: server=%s err=%v", spec.Name, err)
		return err
	}
	if spec.Transport != mcppkg.TransportWebSocket {
		debugLogf("connect unsupported transport: server=%s transport=%s", spec.Name, spec.Transport)
		return fmt.Errorf("websocket client only supports transport %s", mcppkg.TransportWebSocket)
	}

	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		debugLogf("connect skipped: server=%s already connected", spec.Name)
		return nil
	}
	c.mu.Unlock()

	header := http.Header{}
	for k, v := range spec.Headers {
		header.Set(k, v)
	}

	// websocket.DialContext honours ctx cancellation for the dial.
	conn, _, err := c.dialer.DialContext(ctx, spec.URL, header)
	if err != nil {
		debugLogf("connect dial failed: server=%s err=%v", spec.Name, err)
		return fmt.Errorf("dial websocket %s: %w", spec.URL, err)
	}

	serverInfo, err := initializeSession(ctx, conn, spec, opts...)
	if err != nil {
		_ = conn.Close()
		debugLogf("connect initialize failed: server=%s err=%v", spec.Name, err)
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	cloned := spec
	c.spec = &cloned
	c.serverInfo = serverInfo
	c.conn = conn
	c.connected = true
	debugLogf("connect success: server=%s remote_name=%s version=%s capabilities=%v", spec.Name, serverInfo.Name, serverInfo.Version, serverInfo.Capabilities)
	return nil
}

// Disconnect closes the WebSocket connection and clears state.
func (c *Client) Disconnect(ctx context.Context) error {
	_ = ctx
	c.mu.Lock()
	conn := c.conn
	serverName := ""
	if c.spec != nil {
		serverName = c.spec.Name
	}
	c.spec = nil
	c.serverInfo = nil
	c.conn = nil
	c.connected = false
	c.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	debugLogf("disconnect success: server=%s", serverName)
	return nil
}

// Info returns the cached server metadata from initialize.
func (c *Client) Info(ctx context.Context) (*mcppkg.ServerInfo, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.spec == nil {
		return nil, fmt.Errorf("mcp websocket client is not connected")
	}
	if c.serverInfo != nil {
		return cloneServerInfo(c.serverInfo), nil
	}
	return &mcppkg.ServerInfo{Name: c.spec.Name, Metadata: cloneAnyMap(c.spec.Metadata)}, nil
}

func (c *Client) ListTools(ctx context.Context, opts ...mcppkg.Option) ([]mcppkg.ToolDescriptor, error) {
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description,omitempty"`
			Metadata    map[string]any  `json:"metadata,omitempty"`
			InputSchema json.RawMessage `json:"inputSchema,omitempty"`
		} `json:"tools"`
	}
	if err := c.call(ctx, "tools/list", nil, &result, opts...); err != nil {
		return nil, err
	}
	tools := make([]mcppkg.ToolDescriptor, 0, len(result.Tools))
	for _, item := range result.Tools {
		tools = append(tools, mcppkg.ToolDescriptor{
			Name:        item.Name,
			Description: item.Description,
			Metadata:    cloneAnyMap(item.Metadata),
			InputSchema: append(json.RawMessage(nil), item.InputSchema...),
		})
	}
	return tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]any, opts ...mcppkg.Option) (any, error) {
	var result map[string]any
	if err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &result, opts...); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListResources(ctx context.Context, opts ...mcppkg.Option) ([]mcppkg.ResourceDescriptor, error) {
	var result struct {
		Resources []struct {
			URI         string         `json:"uri"`
			Name        string         `json:"name,omitempty"`
			Description string         `json:"description,omitempty"`
			MIMEType    string         `json:"mimeType,omitempty"`
			Metadata    map[string]any `json:"metadata,omitempty"`
		} `json:"resources"`
	}
	if err := c.call(ctx, "resources/list", nil, &result, opts...); err != nil {
		return nil, err
	}
	resources := make([]mcppkg.ResourceDescriptor, 0, len(result.Resources))
	for _, item := range result.Resources {
		resources = append(resources, mcppkg.ResourceDescriptor{
			URI:         item.URI,
			Name:        item.Name,
			Description: item.Description,
			MIMEType:    item.MIMEType,
			Metadata:    cloneAnyMap(item.Metadata),
		})
	}
	return resources, nil
}

func (c *Client) ReadResource(ctx context.Context, uri string, opts ...mcppkg.Option) (any, error) {
	var result map[string]any
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &result, opts...); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListPrompts(ctx context.Context, opts ...mcppkg.Option) ([]mcppkg.PromptDescriptor, error) {
	var result struct {
		Prompts []struct {
			Name        string            `json:"name"`
			Description string            `json:"description,omitempty"`
			Arguments   map[string]string `json:"arguments,omitempty"`
			Metadata    map[string]any    `json:"metadata,omitempty"`
		} `json:"prompts"`
	}
	if err := c.call(ctx, "prompts/list", nil, &result, opts...); err != nil {
		return nil, err
	}
	prompts := make([]mcppkg.PromptDescriptor, 0, len(result.Prompts))
	for _, item := range result.Prompts {
		prompts = append(prompts, mcppkg.PromptDescriptor{
			Name:        item.Name,
			Description: item.Description,
			Arguments:   cloneStringMap(item.Arguments),
			Metadata:    cloneAnyMap(item.Metadata),
		})
	}
	return prompts, nil
}

func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]any, opts ...mcppkg.Option) (string, error) {
	var result struct {
		Prompt string `json:"prompt"`
	}
	if err := c.call(ctx, "prompts/get", map[string]any{"name": name, "arguments": args}, &result, opts...); err != nil {
		return "", err
	}
	return result.Prompt, nil
}

func (c *Client) call(ctx context.Context, method string, params map[string]any, out any, opts ...mcppkg.Option) error {
	_ = opts

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.spec == nil || c.conn == nil {
		debugLogf("call rejected: method=%s connected=%v spec_nil=%v conn_nil=%v", method, c.connected, c.spec == nil, c.conn == nil)
		return fmt.Errorf("mcp websocket client is not connected")
	}

	id := c.nextID.Add(1)
	debugLogf("call dispatch: server=%s method=%s id=%d params=%v", c.spec.Name, method, id, params)

	// Apply deadline if present to bound read/write for this call.
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
		_ = c.conn.SetReadDeadline(deadline)
	} else {
		_ = c.conn.SetWriteDeadline(time.Time{})
		_ = c.conn.SetReadDeadline(time.Time{})
	}

	reqBody := request{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal mcp request %s: %w", method, err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("write mcp request %s: %w", method, err)
	}

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			// Surface ctx timeout/cancel as-is when possible.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read mcp response %s: %w", method, err)
		}
		var resp response
		if err := json.Unmarshal(msg, &resp); err != nil {
			return fmt.Errorf("decode mcp response %s: %w", method, err)
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return fmt.Errorf("mcp %s failed: %s", method, resp.Error.Message)
		}
		if out == nil || len(resp.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("decode mcp result %s: %w", method, err)
		}
		return nil
	}
}

func initializeSession(ctx context.Context, conn *websocket.Conn, spec mcppkg.ServerSpec, opts ...mcppkg.Option) (*mcppkg.ServerInfo, error) {
	_ = opts
	var result struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version,omitempty"`
		} `json:"serverInfo"`
		Capabilities map[string]any `json:"capabilities,omitempty"`
		Metadata     map[string]any `json:"metadata,omitempty"`
	}
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "eino-mcp-websocket",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{},
	}
	tmp := &Client{dialer: websocket.DefaultDialer, spec: &spec, conn: conn, connected: true}
	// Force initialize id to 1 to match other transports.
	tmp.nextID.Store(0)
	if err := tmp.call(ctx, "initialize", params, &result); err != nil {
		return nil, err
	}
	info := &mcppkg.ServerInfo{
		Name:     spec.Name,
		Metadata: cloneAnyMap(spec.Metadata),
	}
	if result.ServerInfo.Name != "" {
		info.Name = result.ServerInfo.Name
	}
	info.Version = result.ServerInfo.Version
	info.Capabilities = flattenCapabilityKeys(result.Capabilities)
	if len(result.Metadata) > 0 {
		if info.Metadata == nil {
			info.Metadata = map[string]any{}
		}
		for k, v := range result.Metadata {
			info.Metadata[k] = v
		}
	}
	return info, nil
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func flattenCapabilityKeys(capabilities map[string]any) []string {
	if len(capabilities) == 0 {
		return nil
	}
	keys := make([]string, 0, len(capabilities))
	for key := range capabilities {
		keys = append(keys, key)
	}
	return keys
}

func cloneServerInfo(info *mcppkg.ServerInfo) *mcppkg.ServerInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	if len(info.Capabilities) > 0 {
		cloned.Capabilities = append([]string(nil), info.Capabilities...)
	}
	cloned.Metadata = cloneAnyMap(info.Metadata)
	return &cloned
}

func debugLogf(format string, args ...any) {
	log.Printf("[mcp/websocket] "+format, args...)
}

var _ mcppkg.Client = (*Client)(nil)

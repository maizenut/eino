package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	nethttp "net/http"
	"sync"
	"sync/atomic"
	"time"

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

// Doer sends HTTP requests for the MCP HTTP transport.
type Doer interface {
	Do(req *nethttp.Request) (*nethttp.Response, error)
}

// Client is a lightweight HTTP MCP client.
type Client struct {
	doer Doer

	mu         sync.Mutex
	spec       *mcppkg.ServerSpec
	serverInfo *mcppkg.ServerInfo
	connected  bool
	nextID     atomic.Int64
}

// NewClient creates an HTTP MCP client.
func NewClient(doer Doer) *Client {
	if doer == nil {
		doer = &nethttp.Client{}
	}
	return &Client{doer: doer}
}

// Connect validates the server spec and performs MCP initialize over HTTP.
func (c *Client) Connect(ctx context.Context, spec mcppkg.ServerSpec, opts ...mcppkg.Option) error {
	debugLogf("connect begin: server=%s transport=%s url=%s", spec.Name, spec.Transport, spec.URL)
	if err := spec.Validate(); err != nil {
		debugLogf("connect validate failed: server=%s err=%v", spec.Name, err)
		return err
	}
	if spec.Transport != mcppkg.TransportHTTP {
		debugLogf("connect unsupported transport: server=%s transport=%s", spec.Name, spec.Transport)
		return fmt.Errorf("http client only supports transport %s", mcppkg.TransportHTTP)
	}

	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		debugLogf("connect skipped: server=%s already connected", spec.Name)
		return nil
	}
	c.mu.Unlock()

	serverInfo, err := initializeSession(ctx, c.doer, spec, opts...)
	if err != nil {
		debugLogf("connect initialize failed: server=%s err=%v", spec.Name, err)
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	cloned := spec
	c.spec = &cloned
	c.serverInfo = serverInfo
	c.connected = true
	debugLogf("connect success: server=%s remote_name=%s version=%s capabilities=%v", spec.Name, serverInfo.Name, serverInfo.Version, serverInfo.Capabilities)
	return nil
}

// Disconnect clears in-memory connection state.
func (c *Client) Disconnect(ctx context.Context) error {
	_ = ctx
	c.mu.Lock()
	serverName := ""
	if c.spec != nil {
		serverName = c.spec.Name
	}
	c.spec = nil
	c.serverInfo = nil
	c.connected = false
	c.mu.Unlock()
	debugLogf("disconnect success: server=%s", serverName)
	return nil
}

// Info returns the cached server metadata from initialize.
func (c *Client) Info(ctx context.Context) (*mcppkg.ServerInfo, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.spec == nil {
		return nil, fmt.Errorf("mcp http client is not connected")
	}
	if c.serverInfo != nil {
		return cloneServerInfo(c.serverInfo), nil
	}
	return &mcppkg.ServerInfo{Name: c.spec.Name, Metadata: cloneAnyMap(c.spec.Metadata)}, nil
}

// ListTools fetches remote tool descriptors.
func (c *Client) ListTools(ctx context.Context, opts ...mcppkg.Option) ([]mcppkg.ToolDescriptor, error) {
	var result struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description,omitempty"`
			Metadata    map[string]any `json:"metadata,omitempty"`
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
		})
	}
	return tools, nil
}

// CallTool sends a tools/call request over HTTP.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any, opts ...mcppkg.Option) (any, error) {
	var result map[string]any
	if err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &result, opts...); err != nil {
		return nil, err
	}
	return result, nil
}

// ListResources fetches remote resource descriptors.
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

// ReadResource sends a resources/read request over HTTP.
func (c *Client) ReadResource(ctx context.Context, uri string, opts ...mcppkg.Option) (any, error) {
	var result map[string]any
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &result, opts...); err != nil {
		return nil, err
	}
	return result, nil
}

// ListPrompts fetches remote prompt descriptors.
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

// GetPrompt sends a prompts/get request over HTTP.
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
	spec := c.spec
	connected := c.connected
	c.mu.Unlock()
	if !connected || spec == nil {
		debugLogf("call rejected: method=%s connected=%v spec_nil=%v", method, connected, spec == nil)
		return fmt.Errorf("mcp http client is not connected")
	}

	id := c.nextID.Add(1)
	debugLogf("call dispatch: server=%s method=%s id=%d params=%v", spec.Name, method, id, params)
	return callHTTP(ctx, c.doer, *spec, id, method, params, out)
}

func initializeSession(ctx context.Context, doer Doer, spec mcppkg.ServerSpec, opts ...mcppkg.Option) (*mcppkg.ServerInfo, error) {
	_ = opts
	start := time.Now()

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
			"name":    "eino-mcp-http",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{},
	}
	debugLogf("initialize begin: server=%s url=%s deadline=%s headers=%v params=%v", spec.Name, spec.URL, contextDeadlineString(ctx), headerKeys(spec.Headers), params)
	if err := callHTTP(ctx, doer, spec, 1, "initialize", params, &result); err != nil {
		debugLogf("initialize failed: server=%s url=%s elapsed=%s err=%v", spec.Name, spec.URL, time.Since(start), err)
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
	debugLogf("initialize success: server=%s remote_name=%s version=%s capabilities=%v metadata=%v elapsed=%s", spec.Name, info.Name, info.Version, info.Capabilities, info.Metadata, time.Since(start))
	return info, nil
}

func callHTTP(ctx context.Context, doer Doer, spec mcppkg.ServerSpec, id int64, method string, params map[string]any, out any) error {
	start := time.Now()
	reqBody := request{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		debugLogf("marshal request failed: method=%s id=%d err=%v", method, id, err)
		return fmt.Errorf("marshal mcp request %s: %w", method, err)
	}
	debugLogf("request build: server=%s method=%s id=%d url=%s deadline=%s header_keys=%v payload_bytes=%d payload=%s", spec.Name, method, id, spec.URL, contextDeadlineString(ctx), headerKeys(spec.Headers), len(payload), string(payload))

	httpReq, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, spec.URL, bytes.NewReader(payload))
	if err != nil {
		debugLogf("request build failed: server=%s method=%s id=%d err=%v", spec.Name, method, id, err)
		return fmt.Errorf("build http request %s: %w", method, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	for key, value := range spec.Headers {
		httpReq.Header.Set(key, value)
	}

	debugLogf("request send: server=%s method=%s id=%d headers=%v", spec.Name, method, id, httpReq.Header)
	httpResp, err := doer.Do(httpReq)
	if err != nil {
		debugLogf("request failed: server=%s method=%s id=%d elapsed=%s err=%v", spec.Name, method, id, time.Since(start), err)
		return fmt.Errorf("send mcp request %s: %w", method, err)
	}
	defer httpResp.Body.Close()
	debugLogf("response headers: server=%s method=%s id=%d status=%d elapsed=%s headers=%v", spec.Name, method, id, httpResp.StatusCode, time.Since(start), httpResp.Header)

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		debugLogf("response read failed: server=%s method=%s id=%d elapsed=%s err=%v", spec.Name, method, id, time.Since(start), err)
		return fmt.Errorf("read mcp response %s: %w", method, err)
	}
	debugLogf("response read: server=%s method=%s id=%d status=%d elapsed=%s payload_bytes=%d payload=%s", spec.Name, method, id, httpResp.StatusCode, time.Since(start), len(body), string(body))
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		debugLogf("response status rejected: server=%s method=%s id=%d status=%d elapsed=%s", spec.Name, method, id, httpResp.StatusCode, time.Since(start))
		return fmt.Errorf("mcp %s http status %d: %s", method, httpResp.StatusCode, string(body))
	}

	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		debugLogf("response decode failed: server=%s method=%s id=%d elapsed=%s err=%v", spec.Name, method, id, time.Since(start), err)
		return fmt.Errorf("decode mcp response %s: %w", method, err)
	}
	if resp.Error != nil {
		debugLogf("response rpc error: server=%s method=%s id=%d elapsed=%s code=%d message=%s", spec.Name, method, id, time.Since(start), resp.Error.Code, resp.Error.Message)
		return fmt.Errorf("mcp %s failed: %s", method, resp.Error.Message)
	}
	if out == nil || len(resp.Result) == 0 {
		debugLogf("response empty result: server=%s method=%s id=%d elapsed=%s", spec.Name, method, id, time.Since(start))
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		debugLogf("result decode failed: server=%s method=%s id=%d elapsed=%s raw=%s err=%v", spec.Name, method, id, time.Since(start), string(resp.Result), err)
		return fmt.Errorf("decode mcp result %s: %w", method, err)
	}
	debugLogf("result decode success: server=%s method=%s id=%d elapsed=%s result_bytes=%d", spec.Name, method, id, time.Since(start), len(resp.Result))
	return nil
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
	log.Printf("[mcp/http] "+format, args...)
}

func contextDeadlineString(ctx context.Context) string {
	if ctx == nil {
		return "none"
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return "none"
	}
	return deadline.Format(time.RFC3339Nano)
}

func headerKeys(headers map[string]string) []string {
	if len(headers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	return keys
}

var _ mcppkg.Client = (*Client)(nil)

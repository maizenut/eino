// Package bytedmcp implements an MCP transport that speaks the ByteDance
// internal MCP gateway protocol on top of plain HTTP JSON-RPC. It mirrors the
// shape of code.byted.org/inf/bytedmcp/go but does not depend on internal
// libraries, so it can be built and tested in any environment.
package bytedmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"sync"
	"sync/atomic"
	"time"

	mcppkg "github.com/cloudwego/eino/composite/mcp"
)

// Doer sends HTTP requests for the bytedmcp transport.
type Doer interface {
	Do(req *nethttp.Request) (*nethttp.Response, error)
}

// Client implements mcppkg.Client over the ByteDance internal MCP gateway.
type Client struct {
	doer       Doer
	resolver   ControlPlaneResolver
	extraHooks []Hook

	mu         sync.Mutex
	spec       *mcppkg.ServerSpec
	serverInfo *mcppkg.ServerInfo
	gatewayURL string
	hooks      HookChain
	connected  bool
	nextID     atomic.Int64
}

// Option configures a Client.
type Option func(*Client)

// WithControlPlane installs a custom resolver. The default resolver returns
// spec.URL (or metadata.gateway_url) verbatim.
func WithControlPlane(resolver ControlPlaneResolver) Option {
	return func(c *Client) { c.resolver = resolver }
}

// WithExtraHook appends an extra hook after the built-in chain. It cannot
// reorder built-in hooks.
func WithExtraHook(h Hook) Option {
	return func(c *Client) {
		if h != nil {
			c.extraHooks = append(c.extraHooks, h)
		}
	}
}

// NewClient builds a bytedmcp client. doer defaults to http.DefaultClient when
// nil.
func NewClient(doer Doer, opts ...Option) *Client {
	if doer == nil {
		doer = &nethttp.Client{Timeout: 30 * time.Second}
	}
	c := &Client{doer: doer}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.resolver == nil {
		c.resolver = &StaticControlPlane{}
	}
	return c
}

// Connect validates the spec, resolves the gateway URL, and runs the MCP
// initialize handshake.
func (c *Client) Connect(ctx context.Context, spec mcppkg.ServerSpec, _ ...mcppkg.Option) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if spec.Transport != mcppkg.TransportBytedMCP {
		return fmt.Errorf("bytedmcp client only supports transport %s", mcppkg.TransportBytedMCP)
	}

	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	url, err := c.resolveURL(ctx, spec)
	if err != nil {
		return fmt.Errorf("bytedmcp resolve gateway: %w", err)
	}

	hooks := buildBuiltinHooks(spec)
	hooks = append(hooks, c.extraHooks...)

	c.mu.Lock()
	cloned := spec
	c.spec = &cloned
	c.gatewayURL = url
	c.hooks = hooks
	c.connected = true
	c.mu.Unlock()

	info, err := c.initialize(ctx)
	if err != nil {
		c.mu.Lock()
		c.connected = false
		c.spec = nil
		c.gatewayURL = ""
		c.hooks = nil
		c.mu.Unlock()
		return err
	}

	c.mu.Lock()
	c.serverInfo = info
	c.mu.Unlock()
	return nil
}

// Disconnect resets the in-memory session state.
func (c *Client) Disconnect(_ context.Context) error {
	c.mu.Lock()
	c.spec = nil
	c.serverInfo = nil
	c.gatewayURL = ""
	c.hooks = nil
	c.connected = false
	c.mu.Unlock()
	return nil
}

// Info returns cached server metadata.
func (c *Client) Info(_ context.Context) (*mcppkg.ServerInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected || c.spec == nil {
		return nil, fmt.Errorf("bytedmcp client is not connected")
	}
	if c.serverInfo != nil {
		return cloneServerInfo(c.serverInfo), nil
	}
	return &mcppkg.ServerInfo{Name: c.spec.Name, Metadata: cloneAnyMap(c.spec.Metadata)}, nil
}

// ListTools issues tools/list.
func (c *Client) ListTools(ctx context.Context, opts ...mcppkg.Option) ([]mcppkg.ToolDescriptor, error) {
	_ = opts
	var out struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description,omitempty"`
			Metadata    map[string]any  `json:"metadata,omitempty"`
			InputSchema json.RawMessage `json:"inputSchema,omitempty"`
		} `json:"tools"`
	}
	if err := c.call(ctx, "tools/list", nil, &out); err != nil {
		return nil, err
	}
	tools := make([]mcppkg.ToolDescriptor, 0, len(out.Tools))
	for _, t := range out.Tools {
		tools = append(tools, mcppkg.ToolDescriptor{
			Name:        t.Name,
			Description: t.Description,
			Metadata:    cloneAnyMap(t.Metadata),
			InputSchema: append(json.RawMessage(nil), t.InputSchema...),
		})
	}
	return tools, nil
}

// CallTool issues tools/call. The hook chain is responsible for injecting
// _meta fields onto the params map before serialization.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any, opts ...mcppkg.Option) (any, error) {
	_ = opts
	params := map[string]any{
		"name":      name,
		"arguments": args,
	}
	var out map[string]any
	if err := c.call(ctx, "tools/call", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListResources issues resources/list.
func (c *Client) ListResources(ctx context.Context, opts ...mcppkg.Option) ([]mcppkg.ResourceDescriptor, error) {
	_ = opts
	var out struct {
		Resources []struct {
			URI         string         `json:"uri"`
			Name        string         `json:"name,omitempty"`
			Description string         `json:"description,omitempty"`
			MIMEType    string         `json:"mimeType,omitempty"`
			Metadata    map[string]any `json:"metadata,omitempty"`
		} `json:"resources"`
	}
	if err := c.call(ctx, "resources/list", nil, &out); err != nil {
		return nil, err
	}
	res := make([]mcppkg.ResourceDescriptor, 0, len(out.Resources))
	for _, r := range out.Resources {
		res = append(res, mcppkg.ResourceDescriptor{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MIMEType:    r.MIMEType,
			Metadata:    cloneAnyMap(r.Metadata),
		})
	}
	return res, nil
}

// ReadResource issues resources/read.
func (c *Client) ReadResource(ctx context.Context, uri string, opts ...mcppkg.Option) (any, error) {
	_ = opts
	var out map[string]any
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPrompts issues prompts/list.
func (c *Client) ListPrompts(ctx context.Context, opts ...mcppkg.Option) ([]mcppkg.PromptDescriptor, error) {
	_ = opts
	var out struct {
		Prompts []struct {
			Name        string            `json:"name"`
			Description string            `json:"description,omitempty"`
			Arguments   map[string]string `json:"arguments,omitempty"`
			Metadata    map[string]any    `json:"metadata,omitempty"`
		} `json:"prompts"`
	}
	if err := c.call(ctx, "prompts/list", nil, &out); err != nil {
		return nil, err
	}
	prompts := make([]mcppkg.PromptDescriptor, 0, len(out.Prompts))
	for _, p := range out.Prompts {
		prompts = append(prompts, mcppkg.PromptDescriptor{
			Name:        p.Name,
			Description: p.Description,
			Arguments:   cloneStringMap(p.Arguments),
			Metadata:    cloneAnyMap(p.Metadata),
		})
	}
	return prompts, nil
}

// GetPrompt issues prompts/get.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]any, opts ...mcppkg.Option) (string, error) {
	_ = opts
	var out struct {
		Prompt string `json:"prompt"`
	}
	if err := c.call(ctx, "prompts/get", map[string]any{"name": name, "arguments": args}, &out); err != nil {
		return "", err
	}
	return out.Prompt, nil
}

// initialize runs the MCP initialize handshake against the gateway.
func (c *Client) initialize(ctx context.Context) (*mcppkg.ServerInfo, error) {
	var raw struct {
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
			"name":    "eino-mcp-bytedmcp",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{},
	}
	if err := c.call(ctx, "initialize", params, &raw); err != nil {
		return nil, err
	}

	c.mu.Lock()
	specName := ""
	specMeta := map[string]any(nil)
	if c.spec != nil {
		specName = c.spec.Name
		specMeta = cloneAnyMap(c.spec.Metadata)
	}
	c.mu.Unlock()

	info := &mcppkg.ServerInfo{Name: specName, Metadata: specMeta}
	if raw.ServerInfo.Name != "" {
		info.Name = raw.ServerInfo.Name
	}
	info.Version = raw.ServerInfo.Version
	for k := range raw.Capabilities {
		info.Capabilities = append(info.Capabilities, k)
	}
	if len(raw.Metadata) > 0 {
		if info.Metadata == nil {
			info.Metadata = map[string]any{}
		}
		for k, v := range raw.Metadata {
			info.Metadata[k] = v
		}
	}
	return info, nil
}

// call wraps a JSON-RPC request with the hook chain.
func (c *Client) call(ctx context.Context, method string, params map[string]any, out any) error {
	c.mu.Lock()
	if !c.connected || c.spec == nil {
		c.mu.Unlock()
		return fmt.Errorf("bytedmcp client is not connected")
	}
	spec := *c.spec
	url := c.gatewayURL
	hooks := append(HookChain(nil), c.hooks...)
	c.mu.Unlock()

	if params == nil {
		params = map[string]any{}
	}
	ctx, err := hooks.Before(ctx, method, params)
	if err != nil {
		return fmt.Errorf("bytedmcp before-call hook: %w", err)
	}

	id := c.nextID.Add(1)
	body := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("bytedmcp marshal request: %w", err)
	}

	httpReq, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("bytedmcp build http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	for k, v := range spec.Headers {
		httpReq.Header.Set(k, v)
	}
	if err := hooks.OnHTTP(ctx, httpReq); err != nil {
		return fmt.Errorf("bytedmcp http hook: %w", err)
	}

	resp, err := c.doer.Do(httpReq)
	if err != nil {
		return fmt.Errorf("bytedmcp send request %s: %w", method, err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("bytedmcp read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bytedmcp %s http status %d: %s", method, resp.StatusCode, string(respBytes))
	}
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return fmt.Errorf("bytedmcp decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("bytedmcp %s failed: %s", method, rpcResp.Error.Message)
	}
	hooks.After(ctx, method, rpcResp.Result)
	if out == nil || len(rpcResp.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, out); err != nil {
		return fmt.Errorf("bytedmcp decode result: %w", err)
	}
	return nil
}

func (c *Client) resolveURL(ctx context.Context, spec mcppkg.ServerSpec) (string, error) {
	if gw, _ := spec.Metadata["gateway_url"].(string); gw != "" {
		return gw, nil
	}
	if spec.URL != "" {
		return spec.URL, nil
	}
	psm, _ := spec.Metadata["psm"].(string)
	region, _ := spec.Metadata["region"].(string)
	env, _ := spec.Metadata["env"].(string)
	if psm == "" {
		return "", fmt.Errorf("bytedmcp spec %s has no url, gateway_url, or psm", spec.Name)
	}
	return c.resolver.Resolve(ctx, psm, region, env)
}

type jsonRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      int64              `json:"id"`
	Result  json.RawMessage    `json:"result,omitempty"`
	Error   *jsonRPCErrorBlock `json:"error,omitempty"`
}

type jsonRPCErrorBlock struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneServerInfo(info *mcppkg.ServerInfo) *mcppkg.ServerInfo {
	if info == nil {
		return nil
	}
	out := *info
	if len(info.Capabilities) > 0 {
		out.Capabilities = append([]string(nil), info.Capabilities...)
	}
	out.Metadata = cloneAnyMap(info.Metadata)
	return &out
}

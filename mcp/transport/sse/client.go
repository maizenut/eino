package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	nethttp "net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mcppkg "github.com/cloudwego/eino/mcp"
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

// Doer sends HTTP requests for the MCP SSE transport.
type Doer interface {
	Do(req *nethttp.Request) (*nethttp.Response, error)
}

// Client is a lightweight SSE MCP client.
type Client struct {
	doer Doer

	mu         sync.Mutex
	spec       *mcppkg.ServerSpec
	serverInfo *mcppkg.ServerInfo
	stream     io.ReadCloser
	reader     *bufio.Reader
	connected  bool
	nextID     atomic.Int64
}

// NewClient creates an SSE MCP client.
func NewClient(doer Doer) *Client {
	if doer == nil {
		doer = &nethttp.Client{}
	}
	return &Client{doer: doer}
}

// Connect opens the SSE stream and performs MCP initialize over the paired POST channel.
func (c *Client) Connect(ctx context.Context, spec mcppkg.ServerSpec, opts ...mcppkg.Option) error {
	debugLogf("connect begin: server=%s transport=%s url=%s", spec.Name, spec.Transport, spec.URL)
	if err := spec.Validate(); err != nil {
		debugLogf("connect validate failed: server=%s err=%v", spec.Name, err)
		return err
	}
	if spec.Transport != mcppkg.TransportSSE {
		debugLogf("connect unsupported transport: server=%s transport=%s", spec.Name, spec.Transport)
		return fmt.Errorf("sse client only supports transport %s", mcppkg.TransportSSE)
	}

	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		debugLogf("connect skipped: server=%s already connected", spec.Name)
		return nil
	}
	c.mu.Unlock()

	stream, reader, err := openStream(ctx, c.doer, spec)
	if err != nil {
		debugLogf("connect open stream failed: server=%s err=%v", spec.Name, err)
		return err
	}
	serverInfo, err := initializeSession(ctx, c.doer, reader, spec, opts...)
	if err != nil {
		_ = stream.Close()
		debugLogf("connect initialize failed: server=%s err=%v", spec.Name, err)
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	cloned := spec
	c.spec = &cloned
	c.serverInfo = serverInfo
	c.stream = stream
	c.reader = reader
	c.connected = true
	debugLogf("connect success: server=%s remote_name=%s version=%s capabilities=%v", spec.Name, serverInfo.Name, serverInfo.Version, serverInfo.Capabilities)
	return nil
}

// Disconnect closes the SSE stream and clears local state.
func (c *Client) Disconnect(ctx context.Context) error {
	_ = ctx
	c.mu.Lock()
	stream := c.stream
	serverName := ""
	if c.spec != nil {
		serverName = c.spec.Name
	}
	c.spec = nil
	c.serverInfo = nil
	c.stream = nil
	c.reader = nil
	c.connected = false
	c.mu.Unlock()

	if stream != nil {
		_ = stream.Close()
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
		return nil, fmt.Errorf("mcp sse client is not connected")
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

// CallTool sends a tools/call request over the SSE companion POST channel.
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

// ReadResource sends a resources/read request over the SSE companion POST channel.
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

// GetPrompt sends a prompts/get request over the SSE companion POST channel.
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

	if !c.connected || c.spec == nil || c.reader == nil {
		debugLogf("call rejected: method=%s connected=%v spec_nil=%v reader_nil=%v", method, c.connected, c.spec == nil, c.reader == nil)
		return fmt.Errorf("mcp sse client is not connected")
	}
	id := c.nextID.Add(1)
	debugLogf("call dispatch: server=%s method=%s id=%d params=%v", c.spec.Name, method, id, params)
	return callSSE(ctx, c.doer, *c.spec, c.reader, id, method, params, out)
}

func openStream(ctx context.Context, doer Doer, spec mcppkg.ServerSpec) (io.ReadCloser, *bufio.Reader, error) {
	start := time.Now()
	httpReq, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, spec.URL, nil)
	if err != nil {
		debugLogf("stream build failed: server=%s url=%s err=%v", spec.Name, spec.URL, err)
		return nil, nil, fmt.Errorf("build sse stream request: %w", err)
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	for key, value := range spec.Headers {
		httpReq.Header.Set(key, value)
	}
	debugLogf("stream open begin: server=%s url=%s deadline=%s headers=%v", spec.Name, spec.URL, contextDeadlineString(ctx), httpReq.Header)

	httpResp, err := doer.Do(httpReq)
	if err != nil {
		debugLogf("stream open failed: server=%s url=%s elapsed=%s err=%v", spec.Name, spec.URL, time.Since(start), err)
		return nil, nil, fmt.Errorf("open mcp sse stream: %w", err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		debugLogf("stream status rejected: server=%s url=%s status=%d elapsed=%s payload=%s", spec.Name, spec.URL, httpResp.StatusCode, time.Since(start), string(body))
		return nil, nil, fmt.Errorf("open mcp sse stream status %d: %s", httpResp.StatusCode, string(body))
	}
	if httpResp.Body == nil {
		debugLogf("stream open failed: server=%s url=%s elapsed=%s err=empty body", spec.Name, spec.URL, time.Since(start))
		return nil, nil, fmt.Errorf("open mcp sse stream: empty body")
	}
	debugLogf("stream open success: server=%s status=%d elapsed=%s headers=%v", spec.Name, httpResp.StatusCode, time.Since(start), httpResp.Header)
	return httpResp.Body, bufio.NewReader(httpResp.Body), nil
}

func initializeSession(ctx context.Context, doer Doer, reader *bufio.Reader, spec mcppkg.ServerSpec, opts ...mcppkg.Option) (*mcppkg.ServerInfo, error) {
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
			"name":    "eino-mcp-sse",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{},
	}
	debugLogf("initialize begin: server=%s url=%s deadline=%s headers=%v params=%v", spec.Name, spec.URL, contextDeadlineString(ctx), headerKeys(spec.Headers), params)
	if err := callSSE(ctx, doer, spec, reader, 1, "initialize", params, &result); err != nil {
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

func callSSE(ctx context.Context, doer Doer, spec mcppkg.ServerSpec, reader *bufio.Reader, id int64, method string, params map[string]any, out any) error {
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
		return fmt.Errorf("build sse post request %s: %w", method, err)
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
	body, _ := io.ReadAll(httpResp.Body)
	_ = httpResp.Body.Close()
	debugLogf("post ack headers: server=%s method=%s id=%d status=%d elapsed=%s headers=%v", spec.Name, method, id, httpResp.StatusCode, time.Since(start), httpResp.Header)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		debugLogf("post ack rejected: server=%s method=%s id=%d status=%d elapsed=%s payload=%s", spec.Name, method, id, httpResp.StatusCode, time.Since(start), string(body))
		return fmt.Errorf("mcp %s http status %d: %s", method, httpResp.StatusCode, string(body))
	}
	if len(body) > 0 {
		debugLogf("post ack: server=%s method=%s id=%d elapsed=%s payload_bytes=%d payload=%s", spec.Name, method, id, time.Since(start), len(body), string(body))
	} else {
		debugLogf("post ack: server=%s method=%s id=%d elapsed=%s payload_bytes=0", spec.Name, method, id, time.Since(start))
	}

	for {
		waitStart := time.Now()
		debugLogf("event wait begin: server=%s method=%s id=%d", spec.Name, method, id)
		eventData, err := readSSEEvent(reader)
		if err != nil {
			debugLogf("event wait failed: server=%s method=%s id=%d elapsed=%s err=%v", spec.Name, method, id, time.Since(waitStart), err)
			return fmt.Errorf("read mcp sse response %s: %w", method, err)
		}
		debugLogf("event read: server=%s method=%s id=%d wait_elapsed=%s total_elapsed=%s payload_bytes=%d payload=%s", spec.Name, method, id, time.Since(waitStart), time.Since(start), len(eventData), eventData)
		var resp response
		if err := json.Unmarshal([]byte(eventData), &resp); err != nil {
			debugLogf("event decode failed: server=%s method=%s id=%d total_elapsed=%s err=%v", spec.Name, method, id, time.Since(start), err)
			return fmt.Errorf("decode mcp sse response %s: %w", method, err)
		}
		if resp.ID != id {
			debugLogf("event skip unmatched id: server=%s method=%s want=%d got=%d total_elapsed=%s", spec.Name, method, id, resp.ID, time.Since(start))
			continue
		}
		if resp.Error != nil {
			debugLogf("event rpc error: server=%s method=%s id=%d total_elapsed=%s code=%d message=%s", spec.Name, method, id, time.Since(start), resp.Error.Code, resp.Error.Message)
			return fmt.Errorf("mcp %s failed: %s", method, resp.Error.Message)
		}
		if out == nil || len(resp.Result) == 0 {
			debugLogf("event empty result: server=%s method=%s id=%d total_elapsed=%s", spec.Name, method, id, time.Since(start))
			return nil
		}
		if err := json.Unmarshal(resp.Result, out); err != nil {
			debugLogf("result decode failed: server=%s method=%s id=%d total_elapsed=%s raw=%s err=%v", spec.Name, method, id, time.Since(start), string(resp.Result), err)
			return fmt.Errorf("decode mcp result %s: %w", method, err)
		}
		debugLogf("result decode success: server=%s method=%s id=%d total_elapsed=%s result_bytes=%d", spec.Name, method, id, time.Since(start), len(resp.Result))
		return nil
	}
}

func readSSEEvent(reader *bufio.Reader) (string, error) {
	var dataLines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			debugLogf("event read line failed: err=%v", err)
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		debugLogf("event raw line: %q", line)
		if line == "" {
			if len(dataLines) == 0 {
				debugLogf("event boundary: skip empty event")
				continue
			}
			payload := strings.Join(dataLines, "\n")
			debugLogf("event boundary: assembled payload_bytes=%d", len(payload))
			return payload, nil
		}
		if strings.HasPrefix(line, ":") {
			debugLogf("event comment line skipped")
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			dataLines = append(dataLines, value)
			debugLogf("event data line appended: bytes=%d current_parts=%d", len(value), len(dataLines))
			continue
		}
		debugLogf("event line ignored: %q", line)
	}
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
	log.Printf("[mcp/sse] "+format, args...)
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

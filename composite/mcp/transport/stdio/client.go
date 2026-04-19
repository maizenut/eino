package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	mcppkg "github.com/cloudwego/eino/composite/mcp"
)

// ProcessStarter starts an MCP stdio subprocess.
type ProcessStarter interface {
	Start(ctx context.Context, spec mcppkg.ServerSpec) (io.WriteCloser, io.ReadCloser, func(context.Context) error, error)
}

// ExecProcessStarter starts a subprocess via exec.CommandContext.
type ExecProcessStarter struct{}

// Start launches the command configured in the server spec.
func (ExecProcessStarter) Start(ctx context.Context, spec mcppkg.ServerSpec) (io.WriteCloser, io.ReadCloser, func(context.Context) error, error) {
	debugLogf("start stdio process: server=%s command=%s args=%v", spec.Name, spec.Command, spec.Args)
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		debugLogf("open stdin pipe failed: server=%s err=%v", spec.Name, err)
		return nil, nil, nil, fmt.Errorf("open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		debugLogf("open stdout pipe failed: server=%s err=%v", spec.Name, err)
		return nil, nil, nil, fmt.Errorf("open stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		debugLogf("start stdio process failed: server=%s err=%v", spec.Name, err)
		return nil, nil, nil, fmt.Errorf("start stdio command %s: %w", spec.Command, err)
	}
	if cmd.Process != nil {
		debugLogf("stdio process started: server=%s pid=%d", spec.Name, cmd.Process.Pid)
	}
	stop := func(stopCtx context.Context) error {
		debugLogf("stopping stdio process: server=%s", spec.Name)
		_ = stdin.Close()
		if cmd.Process == nil {
			debugLogf("stdio process stop skipped: server=%s process=nil", spec.Name)
			return nil
		}
		if err := cmd.Process.Kill(); err != nil && stopCtx.Err() == nil {
			_ = cmd.Wait()
			debugLogf("kill stdio process failed: server=%s err=%v", spec.Name, err)
			return fmt.Errorf("kill stdio command %s: %w", spec.Command, err)
		}
		_ = cmd.Wait()
		debugLogf("stdio process stopped: server=%s", spec.Name)
		return nil
	}
	return stdin, stdout, stop, nil
}

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

// Client is a lightweight stdio MCP client skeleton.
type Client struct {
	starter ProcessStarter

	mu          sync.Mutex
	spec        *mcppkg.ServerSpec
	serverInfo  *mcppkg.ServerInfo
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	reader      *bufio.Reader
	stop        func(context.Context) error
	connected   bool
	initialized bool
	nextID      atomic.Int64
}

// NewClient creates a stdio MCP client skeleton.
func NewClient(starter ProcessStarter) *Client {
	if starter == nil {
		starter = ExecProcessStarter{}
	}
	return &Client{starter: starter}
}

// Connect starts the stdio process and keeps the pipes for future JSON-RPC traffic.
func (c *Client) Connect(ctx context.Context, spec mcppkg.ServerSpec, opts ...mcppkg.Option) error {
	debugLogf("connect begin: server=%s transport=%s command=%s args=%v", spec.Name, spec.Transport, spec.Command, spec.Args)
	if err := spec.Validate(); err != nil {
		debugLogf("connect validate failed: server=%s err=%v", spec.Name, err)
		return err
	}
	if spec.Transport != mcppkg.TransportStdio {
		debugLogf("connect unsupported transport: server=%s transport=%s", spec.Name, spec.Transport)
		return fmt.Errorf("stdio client only supports transport %s", mcppkg.TransportStdio)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connected {
		debugLogf("connect skipped: server=%s already connected", spec.Name)
		return nil
	}
	stdin, stdout, stop, err := c.starter.Start(ctx, spec)
	if err != nil {
		debugLogf("connect start failed: server=%s err=%v", spec.Name, err)
		return err
	}
	reader := bufio.NewReader(stdout)
	serverInfo, err := initializeSession(ctx, stdin, reader, spec, opts...)
	if err != nil {
		debugLogf("connect initialize failed: server=%s err=%v", spec.Name, err)
		_ = stdin.Close()
		_ = stdout.Close()
		if stop != nil {
			_ = stop(ctx)
		}
		return err
	}
	cloned := spec
	c.spec = &cloned
	c.serverInfo = serverInfo
	c.stdin = stdin
	c.stdout = stdout
	c.reader = reader
	c.stop = stop
	c.connected = true
	c.initialized = true
	debugLogf("connect success: server=%s remote_name=%s version=%s capabilities=%v", spec.Name, serverInfo.Name, serverInfo.Version, serverInfo.Capabilities)
	return nil
}

// Disconnect stops the subprocess and clears in-memory state.
func (c *Client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		debugLogf("disconnect skipped: client not connected")
		return nil
	}
	stop := c.stop
	stdin := c.stdin
	stdout := c.stdout
	serverName := ""
	if c.spec != nil {
		serverName = c.spec.Name
	}
	c.spec = nil
	c.serverInfo = nil
	c.stdin = nil
	c.stdout = nil
	c.reader = nil
	c.stop = nil
	c.connected = false
	c.initialized = false
	c.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if stdout != nil {
		_ = stdout.Close()
	}
	if stop != nil {
		err := stop(ctx)
		if err != nil {
			debugLogf("disconnect failed: server=%s err=%v", serverName, err)
			return err
		}
	}
	debugLogf("disconnect success: server=%s", serverName)
	return nil
}

// Info returns locally known server metadata before full MCP handshake exists.
func (c *Client) Info(ctx context.Context) (*mcppkg.ServerInfo, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.spec == nil {
		return nil, fmt.Errorf("mcp stdio client is not connected")
	}
	if c.serverInfo != nil {
		return cloneServerInfo(c.serverInfo), nil
	}
	return &mcppkg.ServerInfo{Name: c.spec.Name, Metadata: cloneAnyMap(c.spec.Metadata)}, nil
}

// ListTools is a transport skeleton and currently returns no remote tools.
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

// CallTool sends a JSON-RPC call_tools request over stdio.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any, opts ...mcppkg.Option) (any, error) {
	var result map[string]any
	if err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &result, opts...); err != nil {
		return nil, err
	}
	return result, nil
}

// ListResources is a transport skeleton and currently returns no remote resources.
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

// ReadResource sends a JSON-RPC resources/read request over stdio.
func (c *Client) ReadResource(ctx context.Context, uri string, opts ...mcppkg.Option) (any, error) {
	var result map[string]any
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &result, opts...); err != nil {
		return nil, err
	}
	return result, nil
}

// ListPrompts is a transport skeleton and currently returns no remote prompts.
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

// GetPrompt sends a JSON-RPC prompts/get request over stdio.
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
	resolvedOpts := applyClientOptions(opts)
	_ = resolvedOpts

	c.mu.Lock()
	stdin := c.stdin
	reader := c.reader
	connected := c.connected
	c.mu.Unlock()
	if !connected || stdin == nil || reader == nil {
		debugLogf("call rejected: method=%s connected=%v stdin_nil=%v reader_nil=%v", method, connected, stdin == nil, reader == nil)
		return fmt.Errorf("mcp stdio client is not connected")
	}

	id := c.nextID.Add(1)
	debugLogf("call dispatch: method=%s id=%d params=%v", method, id, params)
	return callWithPipes(ctx, stdin, reader, id, method, params, out, opts...)
}

func initializeSession(ctx context.Context, stdin io.WriteCloser, reader *bufio.Reader, spec mcppkg.ServerSpec, opts ...mcppkg.Option) (*mcppkg.ServerInfo, error) {
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
			"name":    "eino-mcp-stdio",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{},
	}
	debugLogf("initialize begin: server=%s params=%v", spec.Name, params)
	if err := callWithPipes(ctx, stdin, reader, 1, "initialize", params, &result, opts...); err != nil {
		debugLogf("initialize failed: server=%s err=%v", spec.Name, err)
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
	debugLogf("initialize success: server=%s remote_name=%s version=%s capabilities=%v metadata=%v", spec.Name, info.Name, info.Version, info.Capabilities, info.Metadata)
	return info, nil
}

func callWithPipes(ctx context.Context, stdin io.Writer, reader *bufio.Reader, id int64, method string, params map[string]any, out any, opts ...mcppkg.Option) error {
	_ = ctx
	resolvedOpts := applyClientOptions(opts)
	_ = resolvedOpts

	req := request{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	payload, err := json.Marshal(req)
	if err != nil {
		debugLogf("marshal request failed: method=%s id=%d err=%v", method, id, err)
		return fmt.Errorf("marshal mcp request %s: %w", method, err)
	}
	payload = append(payload, '\n')
	debugLogf("request write: method=%s id=%d payload=%s", method, id, string(payload))
	if _, writeErr := stdin.Write(payload); writeErr != nil {
		debugLogf("request write failed: method=%s id=%d err=%v", method, id, writeErr)
		return fmt.Errorf("write mcp request %s: %w", method, writeErr)
	}

	line, err := reader.ReadBytes('\n')
	if err != nil {
		debugLogf("response read failed: method=%s id=%d err=%v", method, id, err)
		return fmt.Errorf("read mcp response %s: %w", method, err)
	}
	debugLogf("response read: method=%s id=%d payload=%s", method, id, string(line))
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		debugLogf("response decode failed: method=%s id=%d err=%v", method, id, err)
		return fmt.Errorf("decode mcp response %s: %w", method, err)
	}
	if resp.Error != nil {
		debugLogf("response rpc error: method=%s id=%d code=%d message=%s", method, id, resp.Error.Code, resp.Error.Message)
		return fmt.Errorf("mcp %s failed: %s", method, resp.Error.Message)
	}
	if out == nil || len(resp.Result) == 0 {
		debugLogf("response empty result: method=%s id=%d", method, id)
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		debugLogf("result decode failed: method=%s id=%d raw=%s err=%v", method, id, string(resp.Result), err)
		return fmt.Errorf("decode mcp result %s: %w", method, err)
	}
	debugLogf("result decode success: method=%s id=%d result=%s", method, id, string(resp.Result))
	return nil
}

func applyClientOptions(opts []mcppkg.Option) map[string]any {
	_ = opts
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
	log.Printf("[mcp/stdio] "+format, args...)
}

var _ mcppkg.Client = (*Client)(nil)

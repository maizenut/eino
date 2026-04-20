package adapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/eino-contrib/jsonschema"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	mcppkg "github.com/cloudwego/eino/composite/mcp"
	"github.com/cloudwego/eino/schema"
)

// RemoteAdapter bridges remote MCP capabilities into local Eino consumables.
//
// It focuses on the "main bridge path" in docs/6.4-mcp.md:
// - MCP tool -> tool.InvokableTool (and therefore tool.BaseTool)
// - MCP prompt -> prompt.ChatTemplate via RemotePromptCatalog.ChatTemplate
// - MCP resource -> document.Loader backed by resources/read
type RemoteAdapter struct {
	ServerLabel string
}

// NewRemoteAdapter creates a default adapter that bridges one MCP server's remote capabilities.
func NewRemoteAdapter(serverLabel string) *RemoteAdapter {
	return &RemoteAdapter{ServerLabel: serverLabel}
}

func (a *RemoteAdapter) ToTools(ctx context.Context, client mcppkg.Client, opts ...mcppkg.Option) ([]tool.BaseTool, error) {
	if client == nil {
		return nil, fmt.Errorf("mcp client is required")
	}
	toolsDesc, err := client.ListTools(ctx, opts...)
	if err != nil {
		return nil, err
	}
	result := make([]tool.BaseTool, 0, len(toolsDesc))
	for _, desc := range toolsDesc {
		rt := &RemoteTool{
			ServerLabel: a.ServerLabel,
			Client:      client,
			Remote:      desc,
		}
		result = append(result, rt)
	}
	return result, nil
}

func (a *RemoteAdapter) ToPrompt(ctx context.Context, client mcppkg.Client, opts ...mcppkg.Option) (any, error) {
	_ = ctx
	_ = opts
	if client == nil {
		return nil, fmt.Errorf("mcp client is required")
	}
	return &RemotePromptCatalog{ServerLabel: a.ServerLabel, Client: client}, nil
}

func (a *RemoteAdapter) ToResource(ctx context.Context, client mcppkg.Client, opts ...mcppkg.Option) (document.Loader, error) {
	_ = ctx
	_ = opts
	if client == nil {
		return nil, fmt.Errorf("mcp client is required")
	}
	return &RemoteResourceLoader{ServerLabel: a.ServerLabel, Client: client}, nil
}

var _ mcppkg.Adapter = (*RemoteAdapter)(nil)

// RemoteTool is an invokable wrapper for one remote MCP tool.
type RemoteTool struct {
	ServerLabel string
	Client      mcppkg.Client
	Remote      mcppkg.ToolDescriptor
}

func (t *RemoteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	_ = ctx
	name := t.Remote.Name
	if t.ServerLabel != "" {
		name = t.ServerLabel + "." + name
	}

	info := &schema.ToolInfo{
		Name: name,
		Desc: t.Remote.Description,
		Extra: map[string]any{
			"mcp_server_label": t.ServerLabel,
			"mcp_tool_name":    t.Remote.Name,
		},
	}
	if len(t.Remote.Metadata) > 0 {
		info.Extra["mcp_tool_metadata"] = t.Remote.Metadata
	}

	// Best-effort schema bridge.
	if len(t.Remote.InputSchema) > 0 {
		var js jsonschema.Schema
		if err := json.Unmarshal(t.Remote.InputSchema, &js); err == nil {
			info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&js)
		}
	}
	return info, nil
}

func (t *RemoteTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	if t.Client == nil {
		return "", fmt.Errorf("mcp client is required")
	}
	args := map[string]any{}
	if argumentsInJSON != "" && argumentsInJSON != "null" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("decode tool arguments: %w", err)
		}
	}
	result, err := t.Client.CallTool(ctx, t.Remote.Name, args)
	if err != nil {
		return "", err
	}
	switch v := result.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("encode tool result: %w", err)
		}
		return string(data), nil
	}
}

var _ tool.InvokableTool = (*RemoteTool)(nil)

// RemotePromptCatalog exposes remote prompt discovery and provides prompt.ChatTemplate wrappers.
type RemotePromptCatalog struct {
	ServerLabel string
	Client      mcppkg.Client
}

func (c *RemotePromptCatalog) List(ctx context.Context) ([]mcppkg.PromptDescriptor, error) {
	if c == nil || c.Client == nil {
		return nil, fmt.Errorf("mcp client is required")
	}
	return c.Client.ListPrompts(ctx)
}

func (c *RemotePromptCatalog) Get(ctx context.Context, name string, args map[string]any) (string, error) {
	if c == nil || c.Client == nil {
		return "", fmt.Errorf("mcp client is required")
	}
	return c.Client.GetPrompt(ctx, name, args)
}

// ChatTemplate returns a prompt.ChatTemplate backed by the remote prompt name.
func (c *RemotePromptCatalog) ChatTemplate(name string) prompt.ChatTemplate {
	return &remotePromptTemplate{catalog: c, name: name}
}

type remotePromptTemplate struct {
	catalog *RemotePromptCatalog
	name    string
}

func (t *remotePromptTemplate) Format(ctx context.Context, vs map[string]any, opts ...prompt.Option) ([]*schema.Message, error) {
	_ = opts
	if t.catalog == nil {
		return nil, fmt.Errorf("prompt catalog is required")
	}
	args := map[string]any{}
	for k, v := range vs {
		args[k] = v
	}
	p, err := t.catalog.Get(ctx, t.name, args)
	if err != nil {
		return nil, err
	}
	return []*schema.Message{{Content: p}}, nil
}

var _ prompt.ChatTemplate = (*remotePromptTemplate)(nil)

// RemoteResourceLoader bridges document.Loader to MCP resources/read.
type RemoteResourceLoader struct {
	ServerLabel string
	Client      mcppkg.Client
}

func (l *RemoteResourceLoader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*schema.Document, error) {
	_ = opts
	if l == nil || l.Client == nil {
		return nil, fmt.Errorf("mcp client is required")
	}
	if src.URI == "" {
		return nil, fmt.Errorf("document source uri is required")
	}
	raw, err := l.Client.ReadResource(ctx, src.URI)
	if err != nil {
		return nil, err
	}
	content, mimeType := extractResourceText(raw)
	meta := map[string]any{
		"uri":              src.URI,
		"mcp_server_label": l.ServerLabel,
	}
	if mimeType != "" {
		meta["mime_type"] = mimeType
	}
	return []*schema.Document{{Content: content, MetaData: meta}}, nil
}

var _ document.Loader = (*RemoteResourceLoader)(nil)

func extractResourceText(raw any) (content string, mimeType string) {
	// Common "toy" server returns: {"uri": "...", "content": "..."}
	if m, ok := raw.(map[string]any); ok {
		if v, ok := m["content"].(string); ok {
			return v, ""
		}
		// MCP spec commonly returns: {"contents":[{"uri":"...","mimeType":"text/plain","text":"..."}]}
		if items, ok := m["contents"].([]any); ok {
			for _, item := range items {
				im, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if mt, ok := im["mimeType"].(string); ok {
					mimeType = mt
				}
				if text, ok := im["text"].(string); ok && text != "" {
					return text, mimeType
				}
			}
		}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Sprintf("%v", raw), ""
	}
	return string(data), ""
}

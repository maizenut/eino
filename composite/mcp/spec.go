package mcp

import (
	"encoding/json"

	declarative "github.com/cloudwego/eino/schema/declarative"
)

// ServerSpec describes how an MCP server is connected and bridged into Eino.
type ServerSpec struct {
	Name        string            `json:"name"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
	Tools       []ToolSpec        `json:"tools,omitempty"`
	Retry       *RetryPolicy      `json:"retry,omitempty"`
	AdapterRefs []declarative.Ref `json:"adapter_refs,omitempty"`
	ToolRef     *declarative.Ref  `json:"tool_ref,omitempty"`
	PromptRef   *declarative.Ref  `json:"prompt_ref,omitempty"`
	ResourceRef *declarative.Ref  `json:"resource_ref,omitempty"`
}

// ToolSpec declares one remote MCP tool that should be bridged locally.
type ToolSpec struct {
	Name        string `json:"name,omitempty"`
	RemoteName  string `json:"remote_name,omitempty"`
	RefTarget   string `json:"ref_target,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// RetryPolicy describes reconnect behavior.
type RetryPolicy struct {
	MaxAttempts int   `json:"max_attempts,omitempty"`
	BackoffMS   int64 `json:"backoff_ms,omitempty"`
}

// ServerInfo describes the advertised MCP server capabilities.
type ServerInfo struct {
	Name         string         `json:"name"`
	Version      string         `json:"version,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ToolDescriptor describes a remote MCP tool.
type ToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	// InputSchema carries the JSON Schema describing tool arguments if the MCP server provides it.
	// This mirrors MCP's `inputSchema` field and is bridged into schema.ToolInfo.ParamsOneOf.
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ResourceDescriptor describes a remote MCP resource.
type ResourceDescriptor struct {
	URI         string         `json:"uri"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	MIMEType    string         `json:"mime_type,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// PromptDescriptor describes a remote MCP prompt.
type PromptDescriptor struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Arguments   map[string]string `json:"arguments,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

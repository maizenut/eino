package mcp

import (
	"context"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/tool"
)

// RuntimeView is the assembled runtime projection of one MCP server.
type RuntimeView struct {
	Client   Client
	Adapters []Adapter
}

// Client models a session to a single MCP server.
type Client interface {
	Connect(ctx context.Context, spec ServerSpec, opts ...Option) error
	Disconnect(ctx context.Context) error

	Info(ctx context.Context) (*ServerInfo, error)

	ListTools(ctx context.Context, opts ...Option) ([]ToolDescriptor, error)
	CallTool(ctx context.Context, name string, args map[string]any, opts ...Option) (any, error)

	ListResources(ctx context.Context, opts ...Option) ([]ResourceDescriptor, error)
	ReadResource(ctx context.Context, uri string, opts ...Option) (any, error)

	ListPrompts(ctx context.Context, opts ...Option) ([]PromptDescriptor, error)
	GetPrompt(ctx context.Context, name string, args map[string]any, opts ...Option) (string, error)
}

// Adapter bridges MCP capabilities into local Eino abstractions.
type Adapter interface {
	ToTools(ctx context.Context, client Client, opts ...Option) ([]tool.BaseTool, error)
	ToPrompt(ctx context.Context, client Client, opts ...Option) (any, error)
	ToResource(ctx context.Context, client Client, opts ...Option) (document.Loader, error)
}

// Assembler materializes runtime client and adapter views from a declarative server spec.
type Assembler interface {
	Build(ctx context.Context, spec *ServerSpec) (Client, []Adapter, error)
}

// Registry manages declarative MCP server registration and client discovery.
type Registry interface {
	Register(ctx context.Context, spec *ServerSpec, opts ...Option) (string, error)
	Unregister(ctx context.Context, serverID string) error

	GetSpec(ctx context.Context, serverID string) (*ServerSpec, bool)
	List(ctx context.Context) ([]ServerSpec, error)

	GetClient(ctx context.Context, serverID string) (Client, error)
	GetRuntimeView(ctx context.Context, serverID string) (*RuntimeView, error)
}

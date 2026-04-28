package mcp

import "context"

// ToolEntry binds a remote tool descriptor to its originating server id.
type ToolEntry struct {
	ServerID   string
	Descriptor ToolDescriptor
	ServerSpec *ServerSpec
	ServerInfo *ServerInfo
}

// PromptEntry binds a remote prompt descriptor to its originating server id.
type PromptEntry struct {
	ServerID   string
	Descriptor PromptDescriptor
}

// ResourceEntry binds a remote resource descriptor to its originating server id.
type ResourceEntry struct {
	ServerID   string
	Descriptor ResourceDescriptor
}

// Catalog is an optional extension interface for registries that can aggregate remote capabilities.
//
// It is intentionally separate from Registry to avoid breaking the minimal lifecycle contract.
type Catalog interface {
	Registry

	ListAllTools(ctx context.Context, opts ...Option) ([]ToolEntry, error)
	ListAllPrompts(ctx context.Context, opts ...Option) ([]PromptEntry, error)
	ListAllResources(ctx context.Context, opts ...Option) ([]ResourceEntry, error)
}

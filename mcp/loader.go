package mcp

import "context"

// SpecLoader loads MCP server declarations from external documents.
type SpecLoader interface {
	LoadServerSpec(ctx context.Context, target string) (*ServerSpec, error)
}

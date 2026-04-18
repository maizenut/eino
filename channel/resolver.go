package channel

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

// Resolver resolves the declarative refs inside a ChannelSpec into
// concrete handler and graph objects, bridging ChannelSpec to the
// shared declarative infrastructure.
type Resolver interface {
	// ResolveHandlers resolves all HandlerRefs in the spec.
	// Each returned value is a handler, transformer, router, or tool.
	ResolveHandlers(ctx context.Context, spec *ChannelSpec) ([]any, error)

	// ResolveGraph resolves the GraphRef in the spec, if present.
	// Returns (nil, false, nil) when no GraphRef is declared.
	ResolveGraph(ctx context.Context, spec *ChannelSpec) (compose.AnyGraph, bool, error)
}

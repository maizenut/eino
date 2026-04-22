package memory

import "context"

// RuntimeMemory is the assembled, execution-ready view of a MemorySpec.
//
// Callers interact with RuntimeMemory for writing records, recalling relevant
// context, compacting stored history, and deriving child scopes. It does not
// expose the underlying MemorySpec or Store directly — those are assembler
// concerns.
type RuntimeMemory interface {
	// Info returns the metadata declared in the originating MemorySpec.
	Info() Info
	// Scope returns the active ScopeSpec for the given context.
	Scope(ctx context.Context) ScopeSpec

	// Write stores one or more records according to the configured WritePolicy.
	Write(ctx context.Context, records []*Record, opts ...WriteOption) error
	// Recall retrieves records matching the request according to the configured
	// RecallPolicy.
	Recall(ctx context.Context, req *QueryRequest, opts ...RecallOption) ([]*Record, error)
	// Compact runs the configured CompactionPolicy (summarise, evict, rolling
	// window) against the current scope.
	Compact(ctx context.Context, opts ...Option) error

	// WithScope returns a derived RuntimeMemory bound to the given scope.
	// The original RuntimeMemory is not modified.
	WithScope(scope ScopeSpec) RuntimeMemory
	// Binding returns the lifecycle binding for this memory, if one was
	// assembled. ok is false when no Binding was configured.
	Binding(ctx context.Context) (Binding, bool, error)
}

// Binding hooks a RuntimeMemory into the before/after events of graph nodes.
//
// BeforeNode typically performs recall and injects context; AfterNode typically
// writes new records produced by the node.
type Binding interface {
	// BeforeNode is called before a node executes. It may augment ctx or
	// rewrite input to inject recalled memory.
	BeforeNode(ctx context.Context, nodeKey string, input any) (context.Context, any, error)
	// AfterNode is called after a node executes. It may persist records
	// extracted from the node output.
	AfterNode(ctx context.Context, nodeKey string, output any) (context.Context, any, error)
}

// GraphBacked is an optional interface for RuntimeMemory implementations that
// expose their internal logic as an embeddable compose graph.
type GraphBacked interface {
	// Graph returns the underlying compose graph, if available.
	// The caller must not modify the returned graph.
	Graph(ctx context.Context) (any, bool, error)
}

// RecallPolicyAware is an optional interface for RuntimeMemory implementations
// that expose their effective RecallPolicy to bindings.
type RecallPolicyAware interface {
	RecallPolicy(ctx context.Context) *RecallPolicySpec
}

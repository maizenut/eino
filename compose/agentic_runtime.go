package compose

import "context"

// AgenticRuntime executes an internal agent loop while appearing as one graph node.
type AgenticRuntime interface {
	Invoke(ctx context.Context, input map[string]any, opts ...Option) (map[string]any, error)
}

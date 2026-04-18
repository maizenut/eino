package memory

// storeOptions holds resolved StoreOption values.
type storeOptions struct {
	namespace string
	extra     map[string]any
}

// StoreOption configures a single Store operation.
type StoreOption func(*storeOptions)

// WithNamespace sets the namespace for a Store operation.
func WithNamespace(ns string) StoreOption {
	return func(o *storeOptions) {
		o.namespace = ns
	}
}

// WithStoreExtra attaches arbitrary key-value pairs to a Store operation.
func WithStoreExtra(key string, value any) StoreOption {
	return func(o *storeOptions) {
		if o.extra == nil {
			o.extra = make(map[string]any)
		}
		o.extra[key] = value
	}
}

// runtimeOptions holds resolved Option values for RuntimeMemory calls.
type runtimeOptions struct {
	scope ScopeSpec
	extra map[string]any
}

// Option configures a RuntimeMemory operation.
type Option func(*runtimeOptions)

// WithScope overrides the active ScopeSpec for a single RuntimeMemory call.
func WithScope(scope ScopeSpec) Option {
	return func(o *runtimeOptions) {
		o.scope = scope
	}
}

// WithExtra attaches arbitrary key-value pairs to a RuntimeMemory operation.
func WithExtra(key string, value any) Option {
	return func(o *runtimeOptions) {
		if o.extra == nil {
			o.extra = make(map[string]any)
		}
		o.extra[key] = value
	}
}

// recallOptions holds resolved RecallOption values.
type recallOptions struct {
	topK     int
	minScore float64
	extra    map[string]any
}

// RecallOption configures a single Recall call on a RuntimeMemory.
type RecallOption func(*recallOptions)

// WithTopK overrides the TopK limit for a single Recall call.
func WithTopK(k int) RecallOption {
	return func(o *recallOptions) {
		o.topK = k
	}
}

// WithMinScore overrides the MinScore threshold for a single Recall call.
func WithMinScore(score float64) RecallOption {
	return func(o *recallOptions) {
		o.minScore = score
	}
}

// writeOptions holds resolved WriteOption values.
type writeOptions struct {
	mode  string
	extra map[string]any
}

// WriteOption configures a single Write call on a RuntimeMemory.
type WriteOption func(*writeOptions)

// WithWriteMode overrides the write mode for a single Write call.
func WithWriteMode(mode string) WriteOption {
	return func(o *writeOptions) {
		o.mode = mode
	}
}

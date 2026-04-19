package memory

import (
	"context"
	"fmt"
)

// resolvedStoreOptions merges StoreOption values.
func resolvedStoreOptions(opts []StoreOption) storeOptions {
	var out storeOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

// ResolvedStoreOptions is the public view of merged StoreOption values.
type ResolvedStoreOptions struct {
	Namespace string
	Extra     map[string]any
}

// resolvedRuntimeOptions merges RuntimeMemory Option values.
func resolvedRuntimeOptions(opts []Option) runtimeOptions {
	var out runtimeOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

// ResolvedRuntimeOptions is the public view of merged RuntimeMemory options.
type ResolvedRuntimeOptions struct {
	Scope ScopeSpec
	Extra map[string]any
}

// resolvedRecallOptions merges RecallOption values.
func resolvedRecallOptions(opts []RecallOption) recallOptions {
	var out recallOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

// ResolvedRecallOptions is the public view of merged RecallOption values.
type ResolvedRecallOptions struct {
	TopK     int
	MinScore float64
	Extra    map[string]any
}

// resolvedWriteOptions merges WriteOption values.
func resolvedWriteOptions(opts []WriteOption) writeOptions {
	var out writeOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

// ResolvedWriteOptions is the public view of merged WriteOption values.
type ResolvedWriteOptions struct {
	Mode  string
	Extra map[string]any
}

// ResolveStoreOptions merges StoreOption values into an exported result.
func ResolveStoreOptions(opts ...StoreOption) ResolvedStoreOptions {
	out := resolvedStoreOptions(opts)
	return ResolvedStoreOptions{
		Namespace: out.namespace,
		Extra:     cloneMap(out.extra),
	}
}

// ResolveRuntimeOptions merges RuntimeMemory Option values into an exported result.
func ResolveRuntimeOptions(opts ...Option) ResolvedRuntimeOptions {
	out := resolvedRuntimeOptions(opts)
	return ResolvedRuntimeOptions{
		Scope: out.scope,
		Extra: cloneMap(out.extra),
	}
}

// ResolveRecallOptions merges RecallOption values into an exported result.
func ResolveRecallOptions(opts ...RecallOption) ResolvedRecallOptions {
	out := resolvedRecallOptions(opts)
	return ResolvedRecallOptions{
		TopK:     out.topK,
		MinScore: out.minScore,
		Extra:    cloneMap(out.extra),
	}
}

// ResolveWriteOptions merges WriteOption values into an exported result.
func ResolveWriteOptions(opts ...WriteOption) ResolvedWriteOptions {
	out := resolvedWriteOptions(opts)
	return ResolvedWriteOptions{
		Mode:  out.mode,
		Extra: cloneMap(out.extra),
	}
}

// ResolveWriteScope returns the effective write scope name.
func ResolveWriteScope(scope ScopeSpec) string {
	if scope.WriteTo != "" {
		return scope.WriteTo
	}
	return scope.Name
}

// ResolveReadScopes returns the effective ordered read scope chain.
func ResolveReadScopes(scope ScopeSpec) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, 1+len(scope.ReadFrom))
	appendScope := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	appendScope(scope.Name)
	for _, item := range scope.ReadFrom {
		appendScope(item)
	}
	appendScope(scope.Parent)
	return result
}

// WriteThroughStore writes records directly to a Store using the effective scope.
func WriteThroughStore(ctx context.Context, store Store, scope ScopeSpec, records []*Record, opts ...StoreOption) error {
	if store == nil {
		return fmt.Errorf("memory store is required")
	}
	writeScope := ResolveWriteScope(scope)
	cloned := cloneRecords(records)
	for _, record := range cloned {
		if record == nil {
			continue
		}
		if record.Scope == "" {
			record.Scope = writeScope
		}
	}
	return store.Put(ctx, cloned, opts...)
}

// RecallFromStore recalls records directly from a Store using the effective scope chain.
func RecallFromStore(ctx context.Context, store Store, scope ScopeSpec, req *QueryRequest, opts ...StoreOption) ([]*Record, error) {
	if store == nil {
		return nil, fmt.Errorf("memory store is required")
	}
	query := cloneQueryRequest(req)
	if query.Scope == "" {
		query.Scope = scope.Name
	}
	if len(query.ReadScopes) == 0 {
		query.ReadScopes = ResolveReadScopes(scope)
	}
	return store.Query(ctx, query, opts...)
}

func cloneRecords(in []*Record) []*Record {
	if len(in) == 0 {
		return nil
	}
	out := make([]*Record, 0, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		cp := *item
		if item.Metadata != nil {
			cp.Metadata = cloneMap(item.Metadata)
		}
		if item.Embedding != nil {
			cp.Embedding = append([]float64(nil), item.Embedding...)
		}
		out = append(out, &cp)
	}
	return out
}

func cloneQueryRequest(in *QueryRequest) *QueryRequest {
	if in == nil {
		return &QueryRequest{}
	}
	cp := *in
	if in.ReadScopes != nil {
		cp.ReadScopes = append([]string(nil), in.ReadScopes...)
	}
	if in.Filter != nil {
		cp.Filter = cloneMap(in.Filter)
	}
	if in.TimeRange != nil {
		tr := *in.TimeRange
		cp.TimeRange = &tr
	}
	return &cp
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

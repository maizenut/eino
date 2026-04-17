package compose

import "context"

type interceptingAnyGraph struct {
	AnyGraph
	interceptors []NodeInterceptor
}

func (g *interceptingAnyGraph) compile(ctx context.Context, options *graphCompileOptions) (*composableRunnable, error) {
	if len(g.interceptors) == 0 {
		return g.AnyGraph.compile(ctx, options)
	}
	cloned := *options
	cloned.nodeInterceptors = append(append([]NodeInterceptor(nil), options.nodeInterceptors...), g.interceptors...)
	return g.AnyGraph.compile(ctx, &cloned)
}

// WithNodeInterceptors wraps an AnyGraph so compile-time interceptor defaults are preserved.
func WithNodeInterceptors(graph AnyGraph, interceptors ...NodeInterceptor) AnyGraph {
	if len(interceptors) == 0 {
		return graph
	}
	return &interceptingAnyGraph{AnyGraph: graph, interceptors: interceptors}
}

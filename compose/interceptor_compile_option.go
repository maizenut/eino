package compose

// WithNodeInterceptorsOnCompile injects node interceptors into graph compile options.
func WithNodeInterceptorsOnCompile(interceptors ...NodeInterceptor) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.nodeInterceptors = append(o.nodeInterceptors, interceptors...)
	}
}

// NodeInterceptorsFromCompileOptions returns the effective node interceptors
// declared by graph compile options, using the same ordering rules as graph
// compilation.
func NodeInterceptorsFromCompileOptions(opts ...GraphCompileOption) []NodeInterceptor {
	options := newGraphCompileOptions(opts...)
	return SortInterceptors(filterNilNodeInterceptors(options.nodeInterceptors), options.interceptorOrderStrategy, options.interceptorOrderNames)
}

func filterNilNodeInterceptors(interceptors []NodeInterceptor) []NodeInterceptor {
	if len(interceptors) == 0 {
		return nil
	}
	out := make([]NodeInterceptor, 0, len(interceptors))
	for _, interceptor := range interceptors {
		if interceptor != nil {
			out = append(out, interceptor)
		}
	}
	return out
}

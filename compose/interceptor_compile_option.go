package compose

// WithNodeInterceptorsOnCompile injects node interceptors into graph compile options.
func WithNodeInterceptorsOnCompile(interceptors ...NodeInterceptor) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.nodeInterceptors = append(o.nodeInterceptors, interceptors...)
	}
}

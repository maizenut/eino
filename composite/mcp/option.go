package mcp

// Option represents runtime access options for MCP operations.
type Option interface {
	applyMCPOption(*options)
}

type optionFunc func(*options)

func (f optionFunc) applyMCPOption(opts *options) {
	if f != nil {
		f(opts)
	}
}

type options struct {
	TimeoutMS     int64
	AutoReconnect bool
	Metadata      map[string]any
}

// WithTimeout sets the operation timeout in milliseconds.
func WithTimeout(timeoutMS int64) Option {
	return optionFunc(func(opts *options) {
		opts.TimeoutMS = timeoutMS
	})
}

// WithAutoReconnect controls reconnect behavior for the operation.
func WithAutoReconnect(enabled bool) Option {
	return optionFunc(func(opts *options) {
		opts.AutoReconnect = enabled
	})
}

// WithMetadata attaches request-scoped metadata.
func WithMetadata(metadata map[string]any) Option {
	return optionFunc(func(opts *options) {
		if len(metadata) == 0 {
			opts.Metadata = nil
			return
		}
		cloned := make(map[string]any, len(metadata))
		for k, v := range metadata {
			cloned[k] = v
		}
		opts.Metadata = cloned
	})
}

func applyOptions(opts []Option) options {
	resolved := options{}
	for _, opt := range opts {
		if opt != nil {
			opt.applyMCPOption(&resolved)
		}
	}
	return resolved
}

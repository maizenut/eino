package job

import "time"

// RunOption controls one execution attempt.
type RunOption interface {
	applyRunOption(*runOptions)
}

type runOptions struct {
	Timeout  time.Duration
	DryRun   bool
	Priority int
}

type runOptionFunc func(*runOptions)

func (f runOptionFunc) applyRunOption(opts *runOptions) {
	f(opts)
}

// WithRunTimeout sets a timeout for one run.
func WithRunTimeout(timeout time.Duration) RunOption {
	return runOptionFunc(func(opts *runOptions) {
		opts.Timeout = timeout
	})
}

// WithRunDryRun prevents the runnable from being executed.
func WithRunDryRun() RunOption {
	return runOptionFunc(func(opts *runOptions) {
		opts.DryRun = true
	})
}

// WithRunPriority stores an opaque priority value in run options.
func WithRunPriority(priority int) RunOption {
	return runOptionFunc(func(opts *runOptions) {
		opts.Priority = priority
	})
}

// RegisterOption controls task registration.
type RegisterOption interface {
	applyRegisterOption(*registerOptions)
}

type registerOptions struct {
	Enabled  bool
	Metadata map[string]any
}

type registerOptionFunc func(*registerOptions)

func (f registerOptionFunc) applyRegisterOption(opts *registerOptions) {
	f(opts)
}

// WithRegisterDisabled stores a task as disabled.
func WithRegisterDisabled() RegisterOption {
	return registerOptionFunc(func(opts *registerOptions) {
		opts.Enabled = false
	})
}

// WithRegisterMetadata stores opaque registration metadata.
func WithRegisterMetadata(metadata map[string]any) RegisterOption {
	return registerOptionFunc(func(opts *registerOptions) {
		opts.Metadata = copyMap(metadata)
	})
}

func newRunOptions(opts ...RunOption) runOptions {
	out := runOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt.applyRunOption(&out)
		}
	}
	return out
}

func newRegisterOptions(opts ...RegisterOption) registerOptions {
	out := registerOptions{Enabled: true}
	for _, opt := range opts {
		if opt != nil {
			opt.applyRegisterOption(&out)
		}
	}
	return out
}

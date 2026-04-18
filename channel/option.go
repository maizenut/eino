package channel

import "time"

// Option is a functional option for channel send/receive operations.
type Option interface {
	applyChannelOption(*options)
}

type options struct {
	Timeout  time.Duration
	Priority int
	Metadata map[string]any
}

type timeoutOption struct{ d time.Duration }

func (o timeoutOption) applyChannelOption(opts *options) { opts.Timeout = o.d }

// WithTimeout sets an operation timeout.
func WithTimeout(d time.Duration) Option { return timeoutOption{d} }

type priorityOption struct{ p int }

func (o priorityOption) applyChannelOption(opts *options) { opts.Priority = o.p }

// WithPriority sets message delivery priority.
func WithPriority(p int) Option { return priorityOption{p} }

type metadataOption struct{ m map[string]any }

func (o metadataOption) applyChannelOption(opts *options) { opts.Metadata = o.m }

// WithMetadata attaches per-operation metadata.
func WithMetadata(m map[string]any) Option { return metadataOption{m} }

func applyOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt.applyChannelOption(o)
	}
	return o
}

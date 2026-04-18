package channel

import "context"

// Assembler materializes a RuntimeChannel from a declarative ChannelSpec.
// It orchestrates endpoint construction, handler binding, and graph bridge
// setup to produce a unified runtime object.
type Assembler interface {
	Build(ctx context.Context, spec *ChannelSpec) (RuntimeChannel, error)
}

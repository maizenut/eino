package channel

import "context"

// SpecLoader loads channel declarations from external documents (files, registries).
type SpecLoader interface {
	LoadChannelSpec(ctx context.Context, target string) (*ChannelSpec, error)
}

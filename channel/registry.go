package channel

import "context"

// Registry manages declarative channel registration and runtime discovery.
type Registry interface {
	// Register stores a spec and assembles its RuntimeChannel.
	Register(ctx context.Context, spec *ChannelSpec, opts ...Option) (string, error)
	// Unregister removes a channel and closes its runtime connection.
	Unregister(ctx context.Context, channelID string) error

	// GetSpec returns the stored spec for a registered channel.
	GetSpec(ctx context.Context, channelID string) (*ChannelSpec, bool)
	// List returns info entries for all registered channels.
	List(ctx context.Context) []Info

	// GetRuntimeChannel returns the assembled runtime view for a channel.
	GetRuntimeChannel(ctx context.Context, channelID string) (RuntimeChannel, error)
}

package channel

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

// RuntimeChannel is the execution-facing view of an assembled channel.
// It only carries execution responsibility — declaration and resolution
// are handled by ChannelSpec, Resolver, and Assembler.
type RuntimeChannel interface {
	// Info returns the channel's static identity metadata.
	Info() Info
	// Capabilities returns the declared transport capabilities.
	Capabilities() Capabilities

	// Send delivers a message through the channel.
	Send(ctx context.Context, msg *Message, opts ...Option) error
	// Receive blocks until the next message arrives or ctx is cancelled.
	Receive(ctx context.Context, opts ...Option) (*Message, error)
	// ReceiveChan returns a channel that streams inbound messages.
	ReceiveChan(ctx context.Context, opts ...Option) (<-chan *Message, error)

	// Start initiates the underlying transport connection.
	Start(ctx context.Context) error
	// Stop suspends the channel without releasing resources.
	Stop(ctx context.Context) error
	// Close permanently closes the channel and releases all resources.
	Close(ctx context.Context) error
	// IsOpen reports whether the channel is currently connected.
	IsOpen() bool

	// Graph returns the associated compose graph when one was bound at
	// assembly time, enabling bridge-mode execution.
	Graph(ctx context.Context) (compose.AnyGraph, bool, error)
}

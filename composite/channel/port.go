package channel

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

// MessageInputConverter converts an inbound channel.Message into the input
// expected by a compose graph or node runnable.
type MessageInputConverter func(ctx context.Context, msg *Message) (any, error)

// MessageOutputConverter converts a graph or node output into an outbound
// channel.Message.
type MessageOutputConverter func(ctx context.Context, output any) (*Message, error)

// ChannelPort is the unified input/output boundary that maps RuntimeChannel
// message semantics onto compose graph and node I/O. It does not implement
// the compose-internal channel interface; it is a higher-level adapter that
// delegates graph scheduling to the compose runner via standard compile
// entry points.
//
// Concrete ports (graph input, graph output, node) live in the mirroru
// channel/bridge tree. The interface here pins the lifecycle contract.
type ChannelPort interface {
	// Bind associates the port with a RuntimeChannel and any port-specific
	// resources (e.g. graph runnable, node target).
	Bind(ctx context.Context, rc RuntimeChannel) error
	// Start activates message flow between the channel and its target.
	Start(ctx context.Context) error
	// Stop deactivates message flow without releasing resources.
	Stop(ctx context.Context) error
}

// PortKind identifies the direction/role of a ChannelPort instance.
type PortKind string

const (
	PortKindGraphInput  PortKind = "graph_input"
	PortKindGraphOutput PortKind = "graph_output"
	PortKindNode        PortKind = "node"
)

// PortConfig captures the declarative configuration shared by graph and node
// ports. Concrete ports may extend it with additional fields.
type PortConfig struct {
	Kind       PortKind
	NodeTarget string
	ConvertIn  MessageInputConverter
	ConvertOut MessageOutputConverter
}

// GraphPortBinding is a structural helper used by graph-oriented ports. It
// pairs a RuntimeChannel with an associated AnyGraph value so the bridge can
// compile and invoke the graph in response to inbound messages.
type GraphPortBinding struct {
	Channel RuntimeChannel
	Graph   compose.AnyGraph
}

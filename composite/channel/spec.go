package channel

import (
	"fmt"
	"time"

	declarative "github.com/cloudwego/eino/schema/declarative"
)

// Info describes channel identity and metadata.
type Info struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Version     string         `json:"version,omitempty"`
	Category    string         `json:"category,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// RetryPolicy describes reconnect behavior for a channel endpoint.
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts,omitempty"`
	Backoff     time.Duration `json:"backoff,omitempty"`
	Strategy    string        `json:"strategy,omitempty"` // fixed, linear, exponential
}

// EndpointSpec describes the transport connection for a channel.
type EndpointSpec struct {
	Transport string            `json:"transport"` // stdio, websocket, mqtt, kafka
	Address   string            `json:"address,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
	Retry     *RetryPolicy      `json:"retry,omitempty"`
}

// ChannelSpec is the static declaration of a communication channel.
// It is serializable and loadable from YAML/JSON documents.
type ChannelSpec struct {
	Info         Info              `json:"info"`
	Direction    string            `json:"direction,omitempty"` // inbound, outbound, duplex
	Endpoint     EndpointSpec      `json:"endpoint"`
	Capabilities Capabilities      `json:"capabilities,omitempty"`
	HandlerRefs  []declarative.Ref `json:"handler_refs,omitempty"`
	GraphRef     *declarative.Ref  `json:"graph_ref,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
}

// Validate checks that the spec has the required fields.
func (s *ChannelSpec) Validate() error {
	if s == nil {
		return fmt.Errorf("channel spec is required")
	}
	if s.Info.Name == "" {
		return fmt.Errorf("channel spec: info.name is required")
	}
	if s.Endpoint.Transport == "" {
		return fmt.Errorf("channel spec %s: endpoint.transport is required", s.Info.Name)
	}
	switch s.Endpoint.Transport {
	case TransportWebSocket:
		if s.Endpoint.Address == "" {
			return fmt.Errorf("channel spec %s: websocket transport requires endpoint.address", s.Info.Name)
		}
	case TransportStdio:
		if s.Endpoint.Command == "" {
			return fmt.Errorf("channel spec %s: stdio transport requires endpoint.command", s.Info.Name)
		}
	}
	return nil
}

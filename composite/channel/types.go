package channel

import "time"

const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
	DirectionDuplex   = "duplex"

	TransportStdio     = "stdio"
	TransportWebSocket = "websocket"
	TransportMQTT      = "mqtt"
	TransportKafka     = "kafka"
	TransportCronJob   = "cronjob"

	MessageTypeCronJobTick   = "cronjob.tick"
	MessageTypeCronJobManual = "cronjob.manual"
)

// scriptTransports enumerates legacy script-style transport identifiers that
// are explicitly rejected by ChannelSpec.Validate. They previously described
// shell/Python style cronjob payloads which are no longer supported: the
// cronjob channel only delivers structured messages to graph node runnables.
var scriptTransports = map[string]struct{}{
	"bash":    {},
	"python":  {},
	"shell":   {},
	"command": {},
	"script":  {},
}

// Message is the unified envelope for all channel communication.
type Message struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id,omitempty"`
	Type      string         `json:"type"`
	Content   any            `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Capabilities declares what a channel transport supports.
type Capabilities struct {
	SupportsStreaming bool     `json:"supports_streaming,omitempty"`
	SupportsBinary    bool     `json:"supports_binary,omitempty"`
	MaxMessageSize    int64    `json:"max_message_size,omitempty"`
	MessageTypes      []string `json:"message_types,omitempty"`
}

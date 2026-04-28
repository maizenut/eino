package mcp

// Option represents runtime access options for MCP operations.
type Option interface {
	ApplyMCPOption(*Options)
}

// Options is the resolved runtime option contract shared by MCP implementations.
type Options struct {
	TimeoutMS     int64
	AutoReconnect bool
	Metadata      map[string]any
}

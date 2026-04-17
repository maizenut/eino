package mcp

import "fmt"

const (
	// TransportStdio launches the MCP server as a local subprocess.
	TransportStdio = "stdio"
	// TransportHTTP sends JSON-RPC requests over HTTP.
	TransportHTTP = "http"
	// TransportSSE keeps an SSE stream open and sends requests over HTTP.
	TransportSSE = "sse"
)

// Validate checks whether the server spec has the required transport fields.
func (s ServerSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("mcp server name is required")
	}
	if s.Transport == "" {
		return fmt.Errorf("mcp server transport is required")
	}
	switch s.Transport {
	case TransportStdio:
		if s.Command == "" {
			return fmt.Errorf("mcp stdio command is required")
		}
	case TransportHTTP, TransportSSE:
		if s.URL == "" {
			return fmt.Errorf("mcp %s url is required", s.Transport)
		}
	default:
		return fmt.Errorf("unsupported mcp transport %s", s.Transport)
	}
	return nil
}

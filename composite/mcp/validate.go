package mcp

const (
	// TransportStdio launches the MCP server as a local subprocess.
	TransportStdio = "stdio"
	// TransportHTTP sends JSON-RPC requests over HTTP.
	TransportHTTP = "http"
	// TransportSSE keeps an SSE stream open and sends requests over HTTP.
	TransportSSE = "sse"
	// TransportWebSocket sends JSON-RPC requests over WebSocket.
	TransportWebSocket = "websocket"
	// TransportBytedMCP speaks the ByteDance internal MCP gateway protocol over HTTP.
	TransportBytedMCP = "bytedmcp"
)

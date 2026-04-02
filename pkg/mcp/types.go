package mcp

import "encoding/json"

// Shared MCP types used by both server and client packages

// ToolsCallResult is returned by tool executions
type ToolsCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentBlock represents a piece of content in a tool result
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError  `json:"error,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

type RPCError struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    *string `json:"data,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities  CapabilitySet          `json:"capabilities"`
	ServerInfo   ServerInfo            `json:"serverInfo"`
}

type CapabilitySet struct {
	Tools struct {
		ListChanged bool `json:"listChanged"`
	} `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

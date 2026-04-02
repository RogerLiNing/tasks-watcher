package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/rogerrlee/tasks-watcher/cmd/mcp/client"
	"github.com/rogerrlee/tasks-watcher/pkg/mcp"
)

type Server struct {
	api *client.Client
}

func New(api *client.Client) *Server {
	return &Server{api: api}
}

func (s *Server) Serve(stdin io.Reader, stdout io.Writer) error {
	dec := json.NewDecoder(stdin)
	for {
		var req mcp.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode error: %w", err)
		}

		resp := s.handle(req)

		if err := json.NewEncoder(stdout).Encode(resp); err != nil {
			return fmt.Errorf("encode error: %w", err)
		}
		if f, ok := stdout.(*os.File); ok {
			f.Sync()
		}
	}
}

func (s *Server) handle(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	ctx := context.Background()

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(ctx, req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "notifications/initialized":
		// Client is ready — no response needed
		return mcp.JSONRPCResponse{JSONRPC: "2.0"}
	case "shutdown":
		return mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  map[string]bool{"shutdown": true},
			ID:      req.ID,
		}
	default:
		return mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &mcp.RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			},
			ID: req.ID,
		}
	}
}

func (s *Server) handleInitialize(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	result := mcp.InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    mcp.CapabilitySet{},
	}
	result.Capabilities.Tools.ListChanged = false
	result.ServerInfo.Name = "tasks-watcher"
	result.ServerInfo.Version = "1.0.0"

	return mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}
}

func (s *Server) handleToolsList(ctx context.Context, req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  ToolsListResult{Tools: GetToolDefinitions()},
		ID:      req.ID,
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	result, err := ExecuteTool(ctx, s.api, params.Name, params.Arguments)
	if err != nil {
		return mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			Result: mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{{Type: "text", Text: err.Error()}},
				IsError: true,
			},
			ID: req.ID,
		}
	}

	return mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}
}

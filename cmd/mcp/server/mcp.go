package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/rogerrlee/tasks-watcher/cmd/mcp/client"
	"github.com/rogerrlee/tasks-watcher/pkg/mcp"
)

// MultiServer supports multiple concurrent MCP sessions via Unix socket.
type MultiServer struct {
	api     *client.Client
	listener net.Listener
	wg      sync.WaitGroup
}

func NewMultiServer(api *client.Client) *MultiServer {
	return &MultiServer{api: api}
}

// Listen starts a Unix socket server. Each incoming connection is handled concurrently.
func (s *MultiServer) Listen(socketPath string) error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing socket: %w", err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen on unix socket %s: %w", socketPath, err)
	}
	if err := os.Chmod(socketPath, 0770); err != nil {
		return fmt.Errorf("chmod socket: %w", err)
	}
	s.listener = ln

	go s.acceptLoop()
	return nil
}

func (s *MultiServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *MultiServer) handleConn(c net.Conn) {
	defer s.wg.Done()
	defer c.Close()
	srv := &Server{api: s.api}
	srv.ServeConn(c)
}

// Close shuts down the socket listener.
func (s *MultiServer) Close() error {
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	return nil
}

// Server is the single-session MCP server.
type Server struct {
	api *client.Client
}

func New(api *client.Client) *Server {
	return &Server{api: api}
}

// Serve handles a single stdio session.
func (s *Server) Serve(stdin io.Reader, stdout io.Writer) error {
	dec := json.NewDecoder(stdin)
	enc := json.NewEncoder(stdout)

	for {
		var req mcp.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode error: %w", err)
		}

		resp := s.handle(req)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode error: %w", err)
		}
		if f, ok := stdout.(interface{ Sync() error }); ok {
			f.Sync()
		}
	}
}

// ServeConn handles a single socket connection.
func (s *Server) ServeConn(c io.ReadWriteCloser) error {
	dec := json.NewDecoder(c)
	enc := json.NewEncoder(c)

	for {
		var req mcp.JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode error: %w", err)
		}

		resp := s.handle(req)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode error: %w", err)
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

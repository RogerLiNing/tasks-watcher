package mcp

import (
	"encoding/json"
	"testing"
)

func TestToolsCallResult_JSON(t *testing.T) {
	result := ToolsCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: "hello"},
		},
		IsError: false,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var unmarshaled ToolsCallResult
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(unmarshaled.Content) != 1 {
		t.Errorf("expected 1 content block, got %d", len(unmarshaled.Content))
	}
	if unmarshaled.Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", unmarshaled.Content[0].Text)
	}
	if unmarshaled.IsError {
		t.Error("expected IsError=false")
	}
}

func TestToolsCallResult_IsErrorOmitempty(t *testing.T) {
	// IsError=false should be omitted when marshaling (omitempty)
	result := ToolsCallResult{
		Content: []ContentBlock{{Type: "text", Text: "ok"}},
		IsError: false,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if contains(string(data), `"isError":false`) {
		t.Error("IsError=false should be omitted from JSON")
	}
}

func TestContentBlock_JSON(t *testing.T) {
	block := ContentBlock{Type: "text", Text: "result"}
	data, _ := json.Marshal(block)
	var unmarshaled ContentBlock
	json.Unmarshal(data, &unmarshaled)
	if unmarshaled.Type != "text" || unmarshaled.Text != "result" {
		t.Errorf("unexpected: %+v", unmarshaled)
	}
}

func TestJSONRPCRequest_MarshalUnmarshal(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"task_list"}`),
		ID:      float64(1),
	}
	data, _ := json.Marshal(req)
	var unmarshaled JSONRPCRequest
	json.Unmarshal(data, &unmarshaled)
	if unmarshaled.Method != "tools/call" {
		t.Errorf("expected 'tools/call', got %q", unmarshaled.Method)
	}
	if unmarshaled.ID != float64(1) {
		t.Errorf("expected ID=1, got %v", unmarshaled.ID)
	}
}

func TestJSONRPCResponse_Success(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  map[string]string{"status": "ok"},
		ID:      float64(1),
	}
	data, _ := json.Marshal(resp)
	var unmarshaled JSONRPCResponse
	json.Unmarshal(data, &unmarshaled)
	if unmarshaled.Error != nil {
		t.Error("expected no error")
	}
}

func TestJSONRPCResponse_RPCError(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &RPCError{Code: -32600, Message: "Invalid request"},
		ID:      float64(1),
	}
	data, _ := json.Marshal(resp)
	var unmarshaled JSONRPCResponse
	json.Unmarshal(data, &unmarshaled)
	if unmarshaled.Error == nil {
		t.Fatal("expected error")
	}
	if unmarshaled.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", unmarshaled.Error.Code)
	}
	if unmarshaled.Error.Message != "Invalid request" {
		t.Errorf("expected 'Invalid request', got %q", unmarshaled.Error.Message)
	}
}

func TestRPCError_WithData(t *testing.T) {
	dataStr := "extra info"
	err := RPCError{Code: -32603, Message: "Server error", Data: &dataStr}
	data, _ := json.Marshal(err)
	var unmarshaled RPCError
	json.Unmarshal(data, &unmarshaled)
	if unmarshaled.Data == nil {
		t.Fatal("expected data field")
	}
	if *unmarshaled.Data != "extra info" {
		t.Errorf("expected 'extra info', got %q", *unmarshaled.Data)
	}
}

func TestRPCError_NoData(t *testing.T) {
	err := RPCError{Code: -32600, Message: "Bad request"}
	data, _ := json.Marshal(err)
	if contains(string(data), `"data"`) {
		t.Error("data should be omitted when nil")
	}
}

func TestInitializeResult_Marshal(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    CapabilitySet{},
		ServerInfo:     ServerInfo{Name: "tasks-watcher", Version: "1.0.0"},
	}
	data, _ := json.Marshal(result)
	var unmarshaled InitializeResult
	json.Unmarshal(data, &unmarshaled)
	if unmarshaled.ProtocolVersion != "2024-11-05" {
		t.Errorf("unexpected version: %q", unmarshaled.ProtocolVersion)
	}
	if unmarshaled.ServerInfo.Name != "tasks-watcher" {
		t.Errorf("unexpected server name: %q", unmarshaled.ServerInfo.Name)
	}
	if unmarshaled.ServerInfo.Version != "1.0.0" {
		t.Errorf("unexpected server version: %q", unmarshaled.ServerInfo.Version)
	}
}

func TestCapabilitySet_JSON(t *testing.T) {
	cs := CapabilitySet{}
	cs.Tools.ListChanged = true
	data, _ := json.Marshal(cs)
	var unmarshaled CapabilitySet
	json.Unmarshal(data, &unmarshaled)
	if !unmarshaled.Tools.ListChanged {
		t.Error("expected ListChanged=true")
	}
}

func TestServerInfo_JSON(t *testing.T) {
	info := ServerInfo{Name: "test-server", Version: "2.0.0"}
	data, _ := json.Marshal(info)
	var unmarshaled ServerInfo
	json.Unmarshal(data, &unmarshaled)
	if unmarshaled.Name != "test-server" || unmarshaled.Version != "2.0.0" {
		t.Errorf("unexpected: %+v", unmarshaled)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

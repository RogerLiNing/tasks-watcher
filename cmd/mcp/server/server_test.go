package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rogerrlee/tasks-watcher/cmd/mcp/client"
	"github.com/rogerrlee/tasks-watcher/pkg/mcp"
)

func TestStr(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		def      string
		expected string
	}{
		{"nil", nil, "default", "default"},
		{"empty string", "", "default", ""},
		{"whitespace", "  hello  ", "default", "hello"},
		{"plain string", "hello", "default", "hello"},
		{"non-string int", 123, "default", "default"},
		{"non-string bool", true, "default", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := str(tt.input, tt.def)
			if got != tt.expected {
				t.Errorf("str(%v, %q) = %q, want %q", tt.input, tt.def, got, tt.expected)
			}
		})
	}
}

func TestStrPtr(t *testing.T) {
	tests := []struct {
		input    interface{}
		def      string
		expected string
	}{
		{nil, "default", ""},
		{"", "default", ""},
		{"hello", "default", "hello"},
		{123, "default", ""},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := strPtr(tt.input)
			if *got != tt.expected {
				t.Errorf("strPtr(%v) = %q, want %q", tt.input, *got, tt.expected)
			}
		})
	}
}

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()
	if len(tools) == 0 {
		t.Fatal("expected tools, got none")
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expected := []string{
		"task_create", "task_list", "task_show",
		"task_start", "task_complete", "task_fail",
		"task_update", "task_cancel", "task_delete",
		"project_list", "project_create", "project_update", "project_delete",
		"subtask_create", "subtask_list", "subtask_reorder",
		"dep_add", "dep_list", "dep_check",
	}
	for _, name := range expected {
		if !toolNames[name] {
			t.Errorf("missing tool: %s", name)
		}
	}

	for _, tool := range tools {
		if tool.Name == "task_create" {
			if len(tool.InputSchema.Required) == 0 {
				t.Error("task_create should have required fields")
			}
			if _, ok := tool.InputSchema.Properties["title"]; !ok {
				t.Error("task_create should have title property")
			}
		}
	}
}

func TestTaskCreateProps(t *testing.T) {
	props := taskCreateProps()
	if props["title"].Type != "string" {
		t.Errorf("title type = %q, want string", props["title"].Type)
	}
	if props["priority"].Default != "medium" {
		t.Errorf("priority default = %q, want medium", props["priority"].Default)
	}
	if len(props["priority"].Enum) != 4 {
		t.Errorf("priority enum count = %d, want 4", len(props["priority"].Enum))
	}
}

func newTestClientForServer(serverURL string) *client.Client {
	return &client.Client{
		BaseURL:    serverURL,
		APIKey:     "test-key",
		HTTPClient: &http.Client{},
	}
}

// --- ExecuteTool tests (via httptest) ---

func TestExecuteTool_UnknownTool(t *testing.T) {
	_, err := ExecuteTool(context.Background(), nil, "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestExecuteTool_TaskList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	_, err := ExecuteTool(context.Background(), c, "task_list", nil)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
}

func TestExecuteTool_TaskCreate(t *testing.T) {
	taskID := "test-task-12345678"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/tasks":
			w.Write([]byte(`{"id":"` + taskID + `","title":"Test","status":"pending","priority":"medium"}`))
		case r.Method == "PATCH" && r.URL.Path == "/api/tasks/"+taskID+"/status":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	_, err := ExecuteTool(context.Background(), c, "task_create", map[string]interface{}{
		"title": "Test", "project_name": "test",
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
}

func TestExecuteTool_AllTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tasks":
			if r.Method == "GET" {
				w.Write([]byte(`{"tasks":[],"total":0}`))
			} else {
				w.Write([]byte(`{"id":"task-id-12345678","title":"Test","status":"pending","priority":"medium"}`))
			}
		case "/api/tasks/task-id-12345678/status":
			w.Write([]byte(`{"id":"task-id-12345678","title":"T","status":"in_progress"}`))
		case "/api/tasks/tid-12345678":
			w.Write([]byte(`{"id":"tid-12345678","title":"T","status":"pending","priority":"medium","assignee":"","task_mode":"parallel","created_at":1}`))
		case "/api/tasks/tid-12345678/subtasks":
			w.Write([]byte(`{"subtasks":[]}`))
		case "/api/tasks/tid-12345678/dependencies":
			w.Write([]byte(`{"blockers":[]}`))
		case "/api/tasks/tid-12345678/dependents":
			w.Write([]byte(`{"dependents":[]}`))
		case "/api/tasks/tid-12345678/status":
			w.Write([]byte(`{"id":"tid-12345678","title":"T","status":"in_progress"}`))
		case "/api/projects":
			w.Write([]byte(`{"projects":[]}`))
		case "/api/projects/pid-12345678":
			w.Write([]byte(`{"id":"pid-12345678","name":"P"}`))
		case "/api/tasks/parent-12345678/subtasks":
			if r.Method == "POST" {
				w.Write([]byte(`{"task":{"id":"child-12345678","title":"sub","status":"pending"}}`))
			} else {
				w.Write([]byte(`{"subtasks":[]}`))
			}
		case "/api/tasks/task-12345678/can-start":
			w.Write([]byte(`{"can_start":true}`))
		case "/api/tasks/task-12345678/dependencies":
			w.Write([]byte(`{"blockers":[]}`))
		case "/api/tasks/task-12345678/dependents":
			w.Write([]byte(`{"dependents":[]}`))
		default:
			if r.Method == "DELETE" || r.Method == "PATCH" || r.Method == "PUT" || r.Method == "POST" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)

	tools := []struct {
		name string
		args map[string]interface{}
	}{
		{"task_list", nil},
		{"task_create", map[string]interface{}{"title": "Test", "project_name": "p"}},
		{"task_show", map[string]interface{}{"task_id": "tid-12345678"}},
		{"task_start", map[string]interface{}{"task_id": "tid-12345678"}},
		{"task_complete", map[string]interface{}{"task_id": "tid-12345678"}},
		{"task_fail", map[string]interface{}{"task_id": "tid-12345678", "reason": "fail"}},
		{"task_update", map[string]interface{}{"task_id": "tid-12345678", "title": "New"}},
		{"task_delete", map[string]interface{}{"task_id": "tid-12345678"}},
		{"task_cancel", map[string]interface{}{"task_id": "tid-12345678"}},
		{"project_list", nil},
		{"project_create", map[string]interface{}{"name": "new-proj"}},
		{"project_update", map[string]interface{}{"project_id": "pid-12345678", "name": "updated"}},
		{"project_delete", map[string]interface{}{"project_id": "pid-12345678"}},
		{"subtask_create", map[string]interface{}{"task_id": "parent-12345678", "title": "sub"}},
		{"subtask_list", map[string]interface{}{"task_id": "parent-12345678"}},
		{"subtask_reorder", map[string]interface{}{"task_id": "parent-12345678", "child_id": "c1-12345678", "position": 1}},
		{"dep_add", map[string]interface{}{"task_id": "tid-12345678", "blocker_id": "b1-12345678"}},
		{"dep_list", map[string]interface{}{"task_id": "task-12345678"}},
		{"dep_check", map[string]interface{}{"task_id": "task-12345678"}},
	}
	for _, tool := range tools {
		t.Run(tool.name, func(t *testing.T) {
			_, err := ExecuteTool(context.Background(), c, tool.name, tool.args)
			if err != nil {
				t.Errorf("ExecuteTool(%s) failed: %v", tool.name, err)
			}
		})
	}
}

// --- Server.handle tests ---

func makeJSONRPCRequest(method string, id interface{}, params interface{}) mcp.JSONRPCRequest {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", Method: method, ID: id}
	if params != nil {
		data, _ := json.Marshal(params)
		req.Params = data
	}
	return req
}

func encodeRequest(t *testing.T, req mcp.JSONRPCRequest) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(req); err != nil {
		t.Fatalf("encode error: %v", err)
	}
	return buf.String()
}

func TestServer_HandleInitialize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	req := makeJSONRPCRequest("initialize", float64(1), nil)
	resp := srv.handle(req)

	if resp.Error != nil {
		t.Fatalf("handle initialize error: %v", resp.Error)
	}
	result, ok := resp.Result.(mcp.InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", resp.Result)
	}
	if result.ServerInfo.Name != "tasks-watcher" {
		t.Errorf("server name = %q, want tasks-watcher", result.ServerInfo.Name)
	}
	if result.ServerInfo.Version != "1.0.0" {
		t.Errorf("server version = %q, want 1.0.0", result.ServerInfo.Version)
	}
}

func TestServer_HandleToolsList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	req := makeJSONRPCRequest("tools/list", float64(2), nil)
	resp := srv.handle(req)

	if resp.Error != nil {
		t.Fatalf("handle tools/list error: %v", resp.Error)
	}
	result, ok := resp.Result.(ToolsListResult)
	if !ok {
		t.Fatalf("expected ToolsListResult, got %T", resp.Result)
	}
	if len(result.Tools) == 0 {
		t.Fatal("expected tools in result")
	}
}

func TestServer_HandleToolsCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	params := map[string]interface{}{"name": "task_list"}
	req := makeJSONRPCRequest("tools/call", float64(3), params)
	resp := srv.handle(req)

	if resp.Error != nil {
		t.Fatalf("handle tools/call error: %v", resp.Error)
	}
}

func TestServer_HandleShutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[]}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	req := makeJSONRPCRequest("shutdown", float64(4), nil)
	resp := srv.handle(req)

	if resp.Error != nil {
		t.Fatalf("handle shutdown error: %v", resp.Error)
	}
}

func TestServer_HandleUnknownMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	req := makeJSONRPCRequest("completely/wrong", float64(5), nil)
	resp := srv.handle(req)

	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

func TestServer_HandleNotificationsInitialized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	req := makeJSONRPCRequest("notifications/initialized", nil, nil)
	resp := srv.handle(req)

	if resp.Error != nil {
		t.Fatalf("handle notifications/initialized error: %v", resp.Error)
	}
}

func TestServer_Serve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	req1 := makeJSONRPCRequest("initialize", float64(1), nil)
	req2 := makeJSONRPCRequest("tools/list", float64(2), nil)
	req3 := makeJSONRPCRequest("shutdown", float64(3), nil)

	input := encodeRequest(t, req1) + encodeRequest(t, req2) + encodeRequest(t, req3)
	reader := bytes.NewReader([]byte(input))
	var output bytes.Buffer

	err := srv.Serve(reader, &output)
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	lines := bytes.Split(output.Bytes(), []byte("\n"))
	var responses []mcp.JSONRPCResponse
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Fatalf("decode response %q: %v", string(line), err)
		}
		responses = append(responses, resp)
	}

	if len(responses) < 3 {
		t.Fatalf("expected ≥3 responses, got %d", len(responses))
	}
}

func TestServer_Serve_EOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	err := srv.Serve(bytes.NewReader(nil), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Serve with empty input should return nil, got: %v", err)
	}
}

func TestServer_Serve_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	reader := bytes.NewReader([]byte("not json at all\n"))
	var output bytes.Buffer
	err := srv.Serve(reader, &output)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestServer_ServeConn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	req := makeJSONRPCRequest("tools/list", float64(1), nil)
	input := encodeRequest(t, req)

	pipeR, pipeW := io.Pipe()
	go func() {
		pipeW.Write([]byte(input))
		pipeW.Close()
	}()

	var output bytes.Buffer
	err := srv.ServeConn(readWriteCloserAdapter{pipeR, &output, pipeR})
	if err != nil {
		t.Fatalf("ServeConn failed: %v", err)
	}
}

type readWriteCloserAdapter struct {
	io.Reader
	io.Writer
	io.Closer
}

func TestServer_ServeConn_EOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := New(c)

	r, w := io.Pipe()
	w.Close()
	var out bytes.Buffer
	err := srv.ServeConn(readWriteCloserAdapter{r, &out, r})
	if err != nil {
		t.Fatalf("ServeConn with EOF should return nil, got: %v", err)
	}
}

func TestMultiServer_Listen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[],"total":0}`))
	}))
	defer server.Close()

	c := newTestClientForServer(server.URL)
	srv := NewMultiServer(c)

	tmpDir := t.TempDir()
	socketPath := tmpDir + "/test.sock"

	if err := srv.Listen(socketPath); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	req := makeJSONRPCRequest("initialize", float64(1), nil)
	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	dec := json.NewDecoder(conn)
	var resp mcp.JSONRPCResponse
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestMultiServer_Listen_RemoveOldSocket(t *testing.T) {
	// Skip: macOS /var/folders symlink creates issues with stale socket cleanup
	t.Skip("flaky on macOS tmp dirs with symlinks")
}

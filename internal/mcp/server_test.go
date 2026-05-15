package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func roundTrip(t *testing.T, srv *Server, request string) map[string]interface{} {
	t.Helper()
	in := strings.NewReader(request + "\n")
	var out bytes.Buffer
	srv.r = in
	srv.w = &out

	_ = srv.Serve(context.Background())

	// Decode first non-empty line
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Fatalf("response is not valid JSON: %v\nraw: %s", err, line)
		}
		return result
	}
	t.Fatalf("no response written")
	return nil
}

func newTestServer() *Server {
	srv := New(nil, nil)
	for _, def := range AllTools() {
		switch def.Name {
		case "get_facts":
			srv.RegisterTool(def, HandleGetFacts)
		case "get_snapshot":
			srv.RegisterTool(def, HandleGetSnapshot)
		case "fact_query":
			srv.RegisterTool(def, HandleFactQuery)
		}
	}
	return srv
}

func TestServer_Initialize(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result is not an object: %v", resp["result"])
	}
	if result["protocolVersion"] == nil {
		t.Error("protocolVersion missing")
	}
}

func TestServer_ToolsList(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]interface{})
	tools, ok := result["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		t.Fatalf("tools list empty or wrong type: %v", result["tools"])
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv, `{"jsonrpc":"2.0","id":3,"method":"no_such_method","params":{}}`)

	if resp["error"] == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestServer_GetFacts(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_facts","arguments":{}}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]interface{})
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("content missing: %v", result)
	}
	item := content[0].(map[string]interface{})
	text, _ := item["text"].(string)
	if !strings.Contains(text, "OS") && !strings.Contains(text, "Arch") {
		t.Errorf("facts output looks wrong: %.200s", text)
	}
}

func TestServer_FactQuery(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"fact_query","arguments":{"query":"git_version"}}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var qr map[string]interface{}
	if err := json.Unmarshal([]byte(text), &qr); err != nil {
		t.Fatalf("fact_query result is not JSON: %v", err)
	}
	// found may be true or false depending on environment; just ensure valid JSON
	if _, ok := qr["found"]; !ok {
		t.Error("'found' field missing in fact_query result")
	}
}

func TestServer_UnknownTool(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"no_such_tool","arguments":{}}}`)

	if resp["error"] == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestServer_ParseError(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv, `not valid json`)

	if resp["error"] == nil {
		t.Fatal("expected parse error")
	}
}

func TestServer_GetSnapshot(t *testing.T) {
	srv := newTestServer()
	resp := roundTrip(t, srv,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_snapshot","arguments":{}}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)
	if len(text) == 0 {
		t.Error("snapshot text is empty")
	}
}

// TestMT25_NotificationsHaveNoResponse is a regression test for
// manual-test #25 (2026-05-15): JSON-RPC 2.0 spec says notifications
// MUST NOT receive responses, but the dispatcher used to emit the
// zero-value rpcResponse for notifications/initialized which
// marshalled as {"jsonrpc":""} (empty version field too — doubly
// invalid). Strict MCP clients could refuse to talk further.
func TestMT25_NotificationsHaveNoResponse(t *testing.T) {
	s := New(nil, nil)
	cases := []string{
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/anything"}`,
		`{"jsonrpc":"2.0","method":"initialize","id":null}`,
	}
	for _, c := range cases {
		c := c
		t.Run(c, func(t *testing.T) {
			out, err := s.DispatchBytes(context.Background(), []byte(c))
			if err != nil {
				t.Fatalf("DispatchBytes: %v", err)
			}
			if out != nil {
				t.Errorf("notification produced a response: %q", string(out))
			}
		})
	}
}

func TestMT25_RequestsStillGetResponses(t *testing.T) {
	// Regression guard the other direction: actual requests (with an
	// id) MUST still receive a response.
	s := New(nil, nil)
	out, err := s.DispatchBytes(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("ping returned no response")
	}
	if !strings.Contains(string(out), `"id":1`) {
		t.Errorf("response missing id: %s", string(out))
	}
}

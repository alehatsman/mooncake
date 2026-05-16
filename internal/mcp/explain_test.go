package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestAllTools_IncludesExplain — the explain tool ships in AllTools()
// with required:["noun"] and an optional examples_limit.
func TestAllTools_IncludesExplain(t *testing.T) {
	var def *ToolDef
	for i, d := range AllTools() {
		if d.Name == "explain" {
			def = &AllTools()[i]
			break
		}
	}
	if def == nil {
		t.Fatal("AllTools() missing 'explain'")
	}

	schema, ok := def.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("explain.InputSchema not a map: %T", def.InputSchema)
	}
	req, _ := schema["required"].([]string)
	if len(req) != 1 || req[0] != "noun" {
		t.Errorf("explain required = %v, want [\"noun\"]", req)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if _, ok := props["noun"]; !ok {
		t.Error("explain input missing 'noun' property")
	}
	if _, ok := props["examples_limit"]; !ok {
		t.Error("explain input missing 'examples_limit' property")
	}
}

// TestHandleExplain_KnownAction — a real registered action resolves to
// kind:action with metadata + schema slice + capability shapes.
func TestHandleExplain_KnownAction(t *testing.T) {
	out, err := HandleExplain(context.Background(), mustJSON(t, map[string]any{"noun": "file.write"}))
	if err != nil {
		t.Fatalf("HandleExplain: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, out)
	}
	if resp["kind"] != "action" {
		t.Errorf("kind = %v, want action", resp["kind"])
	}
	action, ok := resp["action"].(map[string]any)
	if !ok {
		t.Fatalf("action payload missing: %s", out)
	}
	if action["name"] != "file.write" {
		t.Errorf("action.name = %v, want file.write", action["name"])
	}
	if _, ok := action["metadata"].(map[string]any); !ok {
		t.Error("action.metadata missing")
	}
	if _, ok := action["diff_shape"]; !ok {
		t.Error("action.diff_shape missing")
	}
	if _, ok := action["reverse_shape"]; !ok {
		t.Error("action.reverse_shape missing")
	}
}

// TestHandleExplain_UnknownActionVerb — typed not_found with candidate
// suggestions; this is the agent-recovery path. Prefix-matching against
// the registered action set means a partial verb (e.g. "file.") should
// at least surface "file.write" / "file.copy" candidates.
func TestHandleExplain_UnknownActionVerb(t *testing.T) {
	out, err := HandleExplain(context.Background(), mustJSON(t, map[string]any{"noun": "file."}))
	if err != nil {
		t.Fatalf("HandleExplain: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, out)
	}
	if resp["kind"] != "not_found" {
		t.Errorf("kind = %v, want not_found", resp["kind"])
	}
	nf, ok := resp["not_found"].(map[string]any)
	if !ok {
		t.Fatalf("not_found payload missing: %s", out)
	}
	if nf["noun"] != "file." {
		t.Errorf("not_found.noun = %v, want file.", nf["noun"])
	}
	cands, _ := nf["candidates"].([]any)
	if len(cands) == 0 {
		t.Errorf("expected at least one candidate for 'file.'; body: %s", out)
	}
}

// TestHandleExplain_GarbageNoun — unresolvable shape (URL) returns
// not_found rather than falling through to keyword search.
func TestHandleExplain_GarbageNoun(t *testing.T) {
	out, err := HandleExplain(context.Background(), mustJSON(t, map[string]any{"noun": "http://example.com"}))
	if err != nil {
		t.Fatalf("HandleExplain: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, out)
	}
	if resp["kind"] != "not_found" {
		t.Errorf("kind = %v, want not_found", resp["kind"])
	}
}

// TestHandleExplain_MissingNoun — empty / absent noun is a hard error,
// not a not_found. This signals "you called the tool wrong" vs "the
// noun does not resolve".
func TestHandleExplain_MissingNoun(t *testing.T) {
	cases := []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`{"noun":""}`),
		json.RawMessage(`{"noun":"   "}`),
	}
	for i, args := range cases {
		_, err := HandleExplain(context.Background(), args)
		if err == nil {
			t.Errorf("case %d: expected error for missing noun, got nil", i)
		}
	}
}

// TestHandleExplain_ExamplesLimit — examples_limit is forwarded to the
// resolver. We cannot assert a specific excerpt count without fixture
// examples in the worktree, so we just confirm the field is honored at
// the bound (limit=0 must not crash, and JSON marshalling stays sound).
func TestHandleExplain_ExamplesLimit(t *testing.T) {
	out, err := HandleExplain(context.Background(),
		mustJSON(t, map[string]any{"noun": "file.write", "examples_limit": 0}))
	if err != nil {
		t.Fatalf("HandleExplain: %v", err)
	}
	if !strings.Contains(out, `"kind": "action"`) {
		t.Errorf("expected kind:action in output: %s", out)
	}
}

// TestExplainViaToolsCall — RegisterAllTools wires HandleExplain so
// MCP tools/call dispatch resolves the noun and returns the typed JSON
// inside the standard content envelope.
func TestExplainViaToolsCall(t *testing.T) {
	srv := New(nil, nil)
	RegisterAllTools(srv)

	req := []byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"explain","arguments":{"noun":"file.write"}}}`)
	got, err := srv.DispatchBytes(context.Background(), req)
	if err != nil {
		t.Fatalf("DispatchBytes: %v", err)
	}

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal(got, &resp); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, got)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call returned error: %+v", resp.Error)
	}
	if len(resp.Result.Content) == 0 {
		t.Fatalf("empty content; body: %s", got)
	}

	var inner map[string]any
	if err := json.Unmarshal([]byte(resp.Result.Content[0].Text), &inner); err != nil {
		t.Fatalf("unmarshal inner: %v\ntext: %s", err, resp.Result.Content[0].Text)
	}
	if inner["kind"] != "action" {
		t.Errorf("inner.kind = %v, want action", inner["kind"])
	}
}

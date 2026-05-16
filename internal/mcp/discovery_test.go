package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestHandleListActions_NoFilter — proposal-01: list_actions
// without arguments returns the full inventory. Asserts structural
// invariants ("actions" is non-empty, total matches len(actions),
// each row carries a non-empty Name + Category) rather than the
// exact action count, which moves whenever a new handler ships.
func TestHandleListActions_NoFilter(t *testing.T) {
	out, err := HandleListActions(context.Background(), nil)
	if err != nil {
		t.Fatalf("HandleListActions: %v", err)
	}
	var got listActionsResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Total == 0 || len(got.Actions) == 0 {
		t.Fatalf("expected non-empty inventory; got %+v", got)
	}
	if got.Total != len(got.Actions) {
		t.Errorf("Total=%d != len(Actions)=%d", got.Total, len(got.Actions))
	}
	for _, a := range got.Actions {
		if a.Name == "" {
			t.Errorf("action with empty Name: %+v", a)
		}
		if a.Category == "" {
			t.Errorf("action %q with empty Category", a.Name)
		}
	}
	// Sorted by Name — guards against a future map-iteration leak.
	for i := 1; i < len(got.Actions); i++ {
		if got.Actions[i].Name < got.Actions[i-1].Name {
			t.Errorf("actions not sorted: %q < %q", got.Actions[i].Name, got.Actions[i-1].Name)
			break
		}
	}
}

// TestHandleListActions_CategoryFilter — passing `category` narrows
// the result. At least one well-known file-category handler exists
// (file.copy / file.write) — assert it's in the file slice and that
// every returned action's Category equals the filter.
func TestHandleListActions_CategoryFilter(t *testing.T) {
	args := json.RawMessage(`{"category":"file"}`)
	out, err := HandleListActions(context.Background(), args)
	if err != nil {
		t.Fatalf("HandleListActions: %v", err)
	}
	var got listActionsResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Total == 0 {
		t.Fatal("expected at least one file-category action")
	}
	for _, a := range got.Actions {
		if a.Category != "file" {
			t.Errorf("filter leaked: action %q has category %q (want file)", a.Name, a.Category)
		}
	}
}

// TestHandleDescribeAction_KnownAction — describe_action returns the
// schemagen Definition + capability summary for a real handler.
// file.copy is a stable choice (shipped since spec-22) so the
// assertions don't drift with new handlers.
func TestHandleDescribeAction_KnownAction(t *testing.T) {
	args := json.RawMessage(`{"name":"file.copy"}`)
	out, err := HandleDescribeAction(context.Background(), args)
	if err != nil {
		t.Fatalf("HandleDescribeAction: %v", err)
	}
	var got describeActionResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Name != "file.copy" {
		t.Errorf("Name = %q, want file.copy", got.Name)
	}
	if got.Category == "" {
		t.Error("Category is empty")
	}
	if got.Schema == nil {
		t.Fatal("Schema is nil; describe_action must surface the typed Definition")
	}
	for _, want := range []string{"check", "diff", "cost", "reverse", "permissions", "sudo", "dry_run"} {
		if _, ok := got.Capabilities[want]; !ok {
			t.Errorf("Capabilities missing %q", want)
		}
	}
}

// TestHandleDescribeAction_UnknownAction — clear error, no panic.
func TestHandleDescribeAction_UnknownAction(t *testing.T) {
	args := json.RawMessage(`{"name":"made.up.action"}`)
	_, err := HandleDescribeAction(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for unknown action; got nil")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("error = %q; want it to say 'unknown action'", err.Error())
	}
}

// TestHandleDescribeAction_MissingName — same shape as
// HandleExplain's missing-noun guard: empty + whitespace + absent
// argument must all reject loudly.
func TestHandleDescribeAction_MissingName(t *testing.T) {
	cases := []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`{"name":""}`),
		json.RawMessage(`{"name":"   "}`),
	}
	for i, args := range cases {
		_, err := HandleDescribeAction(context.Background(), args)
		if err == nil {
			t.Errorf("case %d: expected error for missing name, got nil", i)
		}
	}
}

// TestHandleListPresets_Shape — the discovery tool returns valid JSON
// with a `presets` slice + `total`. We don't assert the count
// (depends on the host's preset registry) but we do assert the
// shape so a future regression breaking the wire envelope shows up.
func TestHandleListPresets_Shape(t *testing.T) {
	out, err := HandleListPresets(context.Background(), nil)
	if err != nil {
		t.Fatalf("HandleListPresets: %v", err)
	}
	var got listPresetsResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Total != len(got.Presets) {
		t.Errorf("Total=%d != len(Presets)=%d", got.Total, len(got.Presets))
	}
}

// TestDiscoveryViaToolsCall — RegisterAllTools wires all three
// discovery handlers so MCP tools/call dispatches resolve. Pre-fix
// the switch in RegisterAllTools wouldn't have a case for the new
// names and the dispatch would surface a "tool not found" — same
// shape as a missing-tool regression.
func TestDiscoveryViaToolsCall(t *testing.T) {
	srv := New(nil, nil)
	RegisterAllTools(srv)

	cases := []struct {
		name     string
		req      string
		mustHave string
	}{
		{
			name:     "list_actions",
			req:      `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_actions","arguments":{}}}`,
			mustHave: `"actions"`,
		},
		{
			name:     "describe_action",
			req:      `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"describe_action","arguments":{"name":"file.copy"}}}`,
			mustHave: `"file.copy"`,
		},
		{
			name:     "list_presets",
			req:      `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_presets","arguments":{}}}`,
			mustHave: `"presets"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, err := srv.DispatchBytes(context.Background(), []byte(c.req))
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
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("unmarshal envelope: %v\n%s", err, body)
			}
			if resp.Error != nil {
				t.Fatalf("tools/call returned error: %+v", resp.Error)
			}
			if len(resp.Result.Content) == 0 {
				t.Fatalf("empty content; body: %s", body)
			}
			if !strings.Contains(resp.Result.Content[0].Text, c.mustHave) {
				t.Errorf("response missing %q:\n%s", c.mustHave, resp.Result.Content[0].Text)
			}
		})
	}
}

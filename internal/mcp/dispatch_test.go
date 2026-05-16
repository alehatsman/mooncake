package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// dispatchOverStdio drives the stdio Serve loop with a single request line and
// returns whatever it wrote back. Used to assert DispatchBytes is byte-for-byte
// equivalent to the stdio path.
func dispatchOverStdio(t *testing.T, srv *Server, reqLine []byte) []byte {
	t.Helper()
	in := bytes.NewReader(append(reqLine, '\n'))
	var out bytes.Buffer
	stdioSrv := New(in, &out)
	stdioSrv.tools = srv.tools
	stdioSrv.hdlrs = srv.hdlrs
	if err := stdioSrv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	// Strip trailing newline json.Encoder adds.
	return bytes.TrimRight(out.Bytes(), "\n")
}

func TestDispatchBytesToolsList(t *testing.T) {
	srv := New(nil, nil)
	RegisterAllTools(srv)

	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	got, err := srv.DispatchBytes(context.Background(), req)
	if err != nil {
		t.Fatalf("DispatchBytes: %v", err)
	}

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []ToolDef `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(got, &resp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, got)
	}
	if resp.JSONRPC != "2.0" || resp.ID != 1 {
		t.Errorf("envelope wrong: %s", got)
	}
	if len(resp.Result.Tools) == 0 {
		t.Errorf("expected non-empty tools list")
	}
	wantTools := []string{"get_facts", "get_metrics", "fact_query", "get_snapshot", "check_plan", "run_plan", "query_file", "explain"}
	have := map[string]bool{}
	for _, tt := range resp.Result.Tools {
		have[tt.Name] = true
	}
	for _, w := range wantTools {
		if !have[w] {
			t.Errorf("missing tool %q in tools/list response", w)
		}
	}
}

func TestDispatchBytesEquivalentToStdio(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`),
		[]byte(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`),
		[]byte(`{"jsonrpc":"2.0","id":3,"method":"ping"}`),
		// Unknown method — both transports should return the same error.
		[]byte(`{"jsonrpc":"2.0","id":4,"method":"does/not/exist"}`),
	}
	for _, req := range cases {
		t.Run(string(req[:min(len(req), 40)]), func(t *testing.T) {
			srv := New(nil, nil)
			RegisterAllTools(srv)
			httpResp, err := srv.DispatchBytes(context.Background(), req)
			if err != nil {
				t.Fatalf("DispatchBytes: %v", err)
			}
			stdioResp := dispatchOverStdio(t, srv, req)
			if !bytes.Equal(httpResp, stdioResp) {
				t.Errorf("DispatchBytes != stdio Serve for request %s\nDispatchBytes: %s\nstdio:         %s",
					req, httpResp, stdioResp)
			}
		})
	}
}

func TestDispatchBytesParseError(t *testing.T) {
	srv := New(nil, nil)
	resp, err := srv.DispatchBytes(context.Background(), []byte(`not json`))
	if err != nil {
		t.Fatalf("DispatchBytes: %v", err)
	}
	if !strings.Contains(string(resp), "parse error") {
		t.Errorf("expected parse error in response, got: %s", resp)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

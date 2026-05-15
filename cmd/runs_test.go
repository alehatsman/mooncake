package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// httpClientFor wires an http.Client to talk to the given httptest.Server
// even though our production code targets "http://localhost/..." (the unix
// socket transport). We rewrite the dial to point at the test server's
// real TCP listener.
func httpClientFor(srv *httptest.Server) *http.Client {
	tr := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
		},
	}
	return &http.Client{Transport: tr}
}

// TestIssue29_FetchRunTerminalStatus_Failed pins the polling helper that
// fixes part A: after a stream closes with no run.completed event (the
// planner-stage failure mode), the helper must surface status=failed +
// error_message so the caller can exit non-zero.
func TestIssue29_FetchRunTerminalStatus_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/runs/abc" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintln(w, `{"status":"failed","error":"planner setup failed: missing file"}`)
	}))
	defer srv.Close()

	status, errMsg := fetchRunTerminalStatus(httpClientFor(srv), "abc")
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
	if !strings.Contains(errMsg, "planner setup failed") {
		t.Errorf("errMsg = %q, want it to contain planner-setup detail", errMsg)
	}
}

// TestIssue29_FetchRunTerminalStatus_DegradesOnHTTPError verifies the
// helper returns ("", "") on transport / non-2xx responses rather than
// surfacing a misleading status. Callers degrade to the pre-fix no-op
// behavior in those cases — we'd rather miss the new signal than synthesise
// a false failure on a flaky connection.
func TestIssue29_FetchRunTerminalStatus_DegradesOnHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	status, errMsg := fetchRunTerminalStatus(httpClientFor(srv), "abc")
	if status != "" || errMsg != "" {
		t.Errorf("expected ('', '') on 404; got (%q, %q)", status, errMsg)
	}
}

// TestIssue29_FetchRunTerminalStatus_PrefersErrorOverErrorMessage covers
// the two-field shape: agentd's records sometimes carry `error`, sometimes
// `error_message` — the helper coalesces.
func TestIssue29_FetchRunTerminalStatus_PrefersErrorOverErrorMessage(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"error_field_only", `{"status":"failed","error":"e1"}`, "e1"},
		{"error_message_field_only", `{"status":"failed","error_message":"em"}`, "em"},
		{"both_error_wins", `{"status":"failed","error":"e","error_message":"em"}`, "e"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			_, msg := fetchRunTerminalStatus(httpClientFor(srv), "abc")
			if msg != tc.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}

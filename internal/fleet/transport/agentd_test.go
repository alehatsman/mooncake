package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "test-token-pr4"

// fakeAgentd is a minimal httptest.Server mimicking the daemon endpoints
// the transport exercises. Each test installs handlers for the routes it
// cares about; unhandled routes return 404.
type fakeAgentd struct {
	*httptest.Server
	mux *http.ServeMux
}

func newFakeAgentd(t *testing.T) *fakeAgentd {
	mux := http.NewServeMux()
	srv := httptest.NewServer(authMiddleware(mux))
	t.Cleanup(srv.Close)
	return &fakeAgentd{Server: srv, mux: mux}
}

// authMiddleware mimics agentd's bearer auth — only the path matters,
// constant-time comparison is not what we're testing here.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hostFromURL strips the scheme so we can hand a plain "host:port" to
// transport.New (which prefixes http:// itself).
func hostFromURL(s string) string {
	u, _ := url.Parse(s)
	return u.Host
}

func newClient(t *testing.T, server string) *Client {
	t.Helper()
	return New("test-peer", hostFromURL(server), testToken)
}

// --- Version --------------------------------------------------------------

func TestGetVersion_RoundTrip(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     "0.9.0",
			"daemon_pid":  4242,
			"hostname":    "macbook",
			"synced_root": "/var/lib/mooncake/agentd/synced",
			"system_mode": true,
		})
	})

	c := newClient(t, srv.URL)
	v, err := c.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if v.Version != "0.9.0" || v.Hostname != "macbook" {
		t.Errorf("got %+v", v)
	}
	if v.SyncedRoot != "/var/lib/mooncake/agentd/synced" {
		t.Errorf("synced_root = %q", v.SyncedRoot)
	}
	if !v.SystemMode {
		t.Errorf("system_mode = false, want true")
	}
}

func TestGetVersion_PropagatesHTTPError(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"boom","message":"daemon exploded"}`)
	})
	c := newClient(t, srv.URL)
	_, err := c.GetVersion(context.Background())
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want HTTP 500 + structured code", err)
	}
}

func TestClient_BearerHeaderSent(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		// authMiddleware already rejected if header was wrong; if we got
		// here the bearer is correct. Send minimal response.
		w.Write([]byte(`{"version":"x"}`))
	})
	c := newClient(t, srv.URL)
	if _, err := c.GetVersion(context.Background()); err != nil {
		t.Fatalf("GetVersion: %v", err)
	}

	// Wrong token → 401.
	bad := New("bad", hostFromURL(srv.URL), "wrong-token")
	_, err := bad.GetVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("wrong token: want 401 error, got %v", err)
	}
}

// --- HEAD -----------------------------------------------------------------

func TestHead_HitAndMiss(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Match exactly on scope+path+sha for "hit"; everything else "miss".
		if r.URL.Query().Get("sha256") == "deadbeef" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	c := newClient(t, srv.URL)

	hit, err := c.Head(context.Background(), "scope/abc", "x.txt", "deadbeef")
	if err != nil || !hit {
		t.Errorf("hit case: hit=%v err=%v", hit, err)
	}
	miss, err := c.Head(context.Background(), "scope/abc", "x.txt", "cafebabe")
	if err != nil || miss {
		t.Errorf("miss case: miss=%v err=%v", miss, err)
	}
}

func TestHead_RequiresSha(t *testing.T) {
	c := newClient(t, "http://127.0.0.1:0") // server irrelevant — rejected before dial
	if _, err := c.Head(context.Background(), "s", "p", ""); err == nil {
		t.Fatal("want error on empty sha")
	}
}

func TestHead_UnexpectedStatusIsError(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/files", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := newClient(t, srv.URL)
	_, err := c.Head(context.Background(), "s", "p", "deadbeef")
	if err == nil {
		t.Fatal("want error on 500")
	}
}

// --- PUT ------------------------------------------------------------------

func TestPut_StreamsFile(t *testing.T) {
	srv := newFakeAgentd(t)
	var (
		mu       sync.Mutex
		gotBody  []byte
		gotSha   string
		gotPath  string
		gotScope string
	)
	srv.mux.HandleFunc("/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = body
		gotSha = r.Header.Get("X-Sha256")
		gotPath = r.URL.Query().Get("path")
		gotScope = r.URL.Query().Get("scope")
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})

	tmp := filepath.Join(t.TempDir(), "x.txt")
	body := []byte("hello there")
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	c := newClient(t, srv.URL)
	if err := c.Put(context.Background(), "scope/abc", "subdir/x.txt", tmp, "abc123"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !bytes.Equal(gotBody, body) {
		t.Errorf("body mismatch: got %q want %q", gotBody, body)
	}
	if gotSha != "abc123" {
		t.Errorf("sha header = %q", gotSha)
	}
	if gotPath != "subdir/x.txt" || gotScope != "scope/abc" {
		t.Errorf("query mismatch: scope=%q path=%q", gotScope, gotPath)
	}
}

func TestPut_SurfacesDaemonError(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/files", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"sha256_mismatch","message":"expected X got Y"}`))
	})

	tmp := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(tmp, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c := newClient(t, srv.URL)
	err := c.Put(context.Background(), "s", "x.txt", tmp, "deadbeef")
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "sha256_mismatch") {
		t.Errorf("err = %v, want sha256_mismatch", err)
	}
}

func TestPut_MissingSourceFile(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/files", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	c := newClient(t, srv.URL)
	err := c.Put(context.Background(), "s", "x.txt", "/does/not/exist", "")
	if err == nil {
		t.Fatal("want error on missing source")
	}
}

// --- POST /v1/runs --------------------------------------------------------

func TestSubmit_RoundTrip(t *testing.T) {
	srv := newFakeAgentd(t)
	var gotReq SubmitRequest
	srv.mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"run_id":"01JABCXXXXXXXXXXXXXXXXXXXX","status":"queued"}`))
	})
	c := newClient(t, srv.URL)
	runID, err := c.Submit(context.Background(), SubmitRequest{
		PlanPath:  "/synced/scope/abc/config.yml",
		VarsFiles: []string{"/synced/scope/abc/vars/common.yml"},
		Tags:      []string{"workstation"},
		BaseDir:   "/synced/scope/abc",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if runID != "01JABCXXXXXXXXXXXXXXXXXXXX" {
		t.Errorf("runID = %q", runID)
	}
	if gotReq.PlanPath != "/synced/scope/abc/config.yml" || gotReq.BaseDir != "/synced/scope/abc" {
		t.Errorf("body mismatch: %+v", gotReq)
	}
	if len(gotReq.VarsFiles) != 1 || gotReq.VarsFiles[0] != "/synced/scope/abc/vars/common.yml" {
		t.Errorf("vars: %v", gotReq.VarsFiles)
	}
}

func TestSubmit_EmptyRunIDIsError(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"run_id":"","status":"queued"}`))
	})
	c := newClient(t, srv.URL)
	_, err := c.Submit(context.Background(), SubmitRequest{PlanPath: "/x.yml"})
	if err == nil || !strings.Contains(err.Error(), "empty run_id") {
		t.Errorf("err = %v, want empty run_id error", err)
	}
}

func TestSubmit_SurfacesDaemonError(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"plan_path_not_found","message":"stat: no such file"}`))
	})
	c := newClient(t, srv.URL)
	_, err := c.Submit(context.Background(), SubmitRequest{PlanPath: "/missing.yml"})
	if err == nil || !strings.Contains(err.Error(), "plan_path_not_found") {
		t.Errorf("err = %v", err)
	}
}

// --- SSE Stream -----------------------------------------------------------

func TestStream_DecodesEvents(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/runs/RUN1/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f, _ := w.(http.Flusher)
		writeSSE(w, f, 1, `{"seq":1,"type":"run_started","timestamp":"2026-05-14T12:00:00Z"}`)
		writeSSE(w, f, 2, `{"seq":2,"type":"step_started","timestamp":"2026-05-14T12:00:01Z","data":{"step":"file.copy"}}`)
		writeSSE(w, f, 3, `{"seq":3,"type":"run_finished","timestamp":"2026-05-14T12:00:02Z"}`)
	})

	c := newClient(t, srv.URL)
	sink := make(chan Event, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.Stream(ctx, "RUN1", sink); err != nil {
		t.Fatalf("Stream: %v", err)
	}

	close(sink)
	var got []Event
	for ev := range sink {
		got = append(got, ev)
	}
	if len(got) != 3 {
		t.Fatalf("event count = %d, want 3 (got: %+v)", len(got), got)
	}
	if got[0].Seq != 1 || got[0].Type != "run_started" {
		t.Errorf("ev[0] = %+v", got[0])
	}
	if got[1].Seq != 2 || got[1].Type != "step_started" {
		t.Errorf("ev[1] = %+v", got[1])
	}
	if !bytes.Contains(got[1].Data, []byte(`"step":"file.copy"`)) {
		t.Errorf("ev[1].Data missing payload: %s", got[1].Data)
	}
	if got[2].Seq != 3 || got[2].Type != "run_finished" {
		t.Errorf("ev[2] = %+v", got[2])
	}
}

func TestStream_RespectsContextCancel(t *testing.T) {
	srv := newFakeAgentd(t)
	// Streamer that never ends — sends one event then blocks until ctx is
	// cancelled by the test.
	srv.mux.HandleFunc("/v1/runs/RUN2/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f, _ := w.(http.Flusher)
		writeSSE(w, f, 1, `{"seq":1,"type":"run_started","timestamp":"2026-05-14T12:00:00Z"}`)
		<-r.Context().Done()
	})

	c := newClient(t, srv.URL)
	sink := make(chan Event, 4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Stream(ctx, "RUN2", sink) }()

	// Wait for the first event, then cancel.
	select {
	case ev := <-sink:
		if ev.Seq != 1 {
			t.Errorf("first event seq = %d, want 1", ev.Seq)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first event never arrived")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("want non-nil error on cancel (ctx.Err)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stream did not return after cancel")
	}
}

func TestStream_PropagatesHTTPError(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/runs/MISSING/events", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"run_not_found","message":"unknown run id"}`))
	})

	c := newClient(t, srv.URL)
	sink := make(chan Event, 1)
	err := c.Stream(context.Background(), "MISSING", sink)
	if err == nil || !strings.Contains(err.Error(), "run_not_found") {
		t.Errorf("err = %v", err)
	}
}

func TestStream_RejectsBadContentType(t *testing.T) {
	srv := newFakeAgentd(t)
	srv.mux.HandleFunc("/v1/runs/X/events", func(w http.ResponseWriter, _ *http.Request) {
		// Lies about its content type.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})

	c := newClient(t, srv.URL)
	sink := make(chan Event, 1)
	err := c.Stream(context.Background(), "X", sink)
	if err == nil || !strings.Contains(err.Error(), "Content-Type") {
		t.Errorf("err = %v", err)
	}
}

// --- parseSSE direct tests -----------------------------------------------

func TestParseSSE_MultilineData(t *testing.T) {
	body := strings.Join([]string{
		"id: 1",
		`data: {"seq":1,"type":"step",`,
		`data: "timestamp":"2026-05-14T12:00:00Z"}`,
		"",
		"",
	}, "\n")
	sink := make(chan Event, 1)
	if err := parseSSE(context.Background(), strings.NewReader(body), sink); err != nil {
		t.Fatalf("parseSSE: %v", err)
	}
	close(sink)
	ev := <-sink
	if ev.Type != "step" || ev.Seq != 1 {
		t.Errorf("ev = %+v", ev)
	}
}

func TestParseSSE_IgnoresComments(t *testing.T) {
	body := strings.Join([]string{
		": keep-alive",
		"id: 1",
		`data: {"seq":1,"type":"x","timestamp":"2026-05-14T12:00:00Z"}`,
		"",
		": another comment",
		"id: 2",
		`data: {"seq":2,"type":"y","timestamp":"2026-05-14T12:00:01Z"}`,
		"",
	}, "\n")
	sink := make(chan Event, 4)
	if err := parseSSE(context.Background(), strings.NewReader(body), sink); err != nil {
		t.Fatalf("parseSSE: %v", err)
	}
	close(sink)
	var got []Event
	for ev := range sink {
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Errorf("event count = %d, want 2", len(got))
	}
}

func TestParseSSE_FallsBackToIDForSeq(t *testing.T) {
	// Event missing `seq` in JSON — parser should fall back to the SSE id.
	body := strings.Join([]string{
		"id: 42",
		`data: {"type":"x","timestamp":"2026-05-14T12:00:00Z"}`,
		"",
	}, "\n")
	sink := make(chan Event, 1)
	if err := parseSSE(context.Background(), strings.NewReader(body), sink); err != nil {
		t.Fatalf("parseSSE: %v", err)
	}
	close(sink)
	ev := <-sink
	if ev.Seq != 42 {
		t.Errorf("seq = %d, want 42 (from id fallback)", ev.Seq)
	}
}

func TestParseSSE_DecodeErrorPropagates(t *testing.T) {
	body := "id: 1\ndata: not-json\n\n"
	sink := make(chan Event, 1)
	if err := parseSSE(context.Background(), strings.NewReader(body), sink); err == nil {
		t.Fatal("want decode error")
	}
}

// --- helpers --------------------------------------------------------------

func writeSSE(w http.ResponseWriter, f http.Flusher, id int64, data string) {
	fmt.Fprintf(w, "id: %d\ndata: %s\n\n", id, data)
	if f != nil {
		f.Flush()
	}
}

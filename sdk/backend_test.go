package mooncake_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// ---------------------------------------------------------------------------
// Interface conformance — all three impls must satisfy CodingBackend
// ---------------------------------------------------------------------------

var _ mooncake.CodingBackend = (*mooncake.MooncakeBackend)(nil)
var _ mooncake.CodingBackend = (*mooncake.NativeBackend)(nil)
var _ mooncake.CodingBackend = (*mooncake.RemoteBackend)(nil)

// ---------------------------------------------------------------------------
// MooncakeBackend — spot-checks that reads/mutations route through the kernel
// ---------------------------------------------------------------------------

func TestMooncakeBackend_Read(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	want := []byte("mooncake read\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	b := mooncake.NewMooncakeBackend()
	got, err := b.Read(context.Background(), path, mooncake.ReadOptions{})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Read = %q; want %q", got, want)
	}
}

func TestMooncakeBackend_Grep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("needle\nskip\nneedle2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := mooncake.NewMooncakeBackend()
	matches, err := b.Grep(context.Background(), "needle", mooncake.GrepOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("len(matches)=%d; want 2", len(matches))
	}
}

func TestMooncakeBackend_Glob(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b := mooncake.NewMooncakeBackend()
	paths, err := b.Glob(context.Background(), "*.go", mooncake.GlobOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("len(paths)=%d; want 2", len(paths))
	}
}

func TestMooncakeBackend_Write(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	b := mooncake.NewMooncakeBackend()
	res, err := b.Write(context.Background(), path, []byte("hello\n"), mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello\n" {
		t.Errorf("file content = %q; want %q", got, "hello\n")
	}
}

func TestMooncakeBackend_Edit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("foo bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := mooncake.NewMooncakeBackend()
	res, err := b.Edit(context.Background(), path, "foo", "baz", mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "baz bar\n" {
		t.Errorf("file content = %q; want %q", got, "baz bar\n")
	}
}

func TestMooncakeBackend_Exec(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")

	b := mooncake.NewMooncakeBackend()
	res, err := b.Exec(context.Background(), "touch "+marker, mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("marker not created: %v", statErr)
	}
}

// ---------------------------------------------------------------------------
// NativeBackend
// ---------------------------------------------------------------------------

func TestNativeBackend_Read(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := mooncake.NewNativeBackend()

	got, err := b.Read(context.Background(), path, mooncake.ReadOptions{})
	if err != nil || string(got) != "abcdef" {
		t.Fatalf("Read full: %v / %q", err, got)
	}

	got, err = b.Read(context.Background(), path, mooncake.ReadOptions{Offset: 2, Limit: 2})
	if err != nil || string(got) != "cd" {
		t.Fatalf("Read offset+limit: %v / %q", err, got)
	}
}

func TestNativeBackend_Grep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("line1\nfoo\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := mooncake.NewNativeBackend()
	matches, err := b.Grep(context.Background(), "foo", mooncake.GrepOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 1 || matches[0].Content != "foo" {
		t.Errorf("unexpected matches: %v", matches)
	}
}

func TestNativeBackend_Glob(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.go", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b := mooncake.NewNativeBackend()
	paths, err := b.Glob(context.Background(), "*.go", mooncake.GlobOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths)=%d; want 1", len(paths))
	}
}

func TestNativeBackend_Write(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.txt")

	b := mooncake.NewNativeBackend()
	res, err := b.Write(context.Background(), path, []byte("native\n"), mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "native\n" {
		t.Errorf("content=%q; want native\\n", got)
	}
}

func TestNativeBackend_Edit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := mooncake.NewNativeBackend()
	res, err := b.Edit(context.Background(), path, "world", "earth", mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello earth" {
		t.Errorf("content=%q; want \"hello earth\"", got)
	}
}

func TestNativeBackend_Exec(t *testing.T) {
	b := mooncake.NewNativeBackend()
	res, err := b.Exec(context.Background(), "echo hello", mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
}

func TestNativeBackend_Exec_Failure(t *testing.T) {
	b := mooncake.NewNativeBackend()
	res, err := b.Exec(context.Background(), "exit 1", mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Exec returned error (should return result): %v", err)
	}
	if res.Summary.Success {
		t.Error("Summary.Success=true for exit 1; want false")
	}
}

// ---------------------------------------------------------------------------
// RemoteBackend — uses httptest.Server to mock agentd responses
// ---------------------------------------------------------------------------

// remoteReadFileServer returns a test server that handles POST /v1/mcp with
// a read_file tools/call and returns base64-encoded content.
func remoteReadFileServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/mcp" {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		var rpc struct {
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if rpc.Method != "tools/call" || rpc.Params.Name != "read_file" {
			http.Error(w, "unexpected tool", http.StatusBadRequest)
			return
		}
		envelope := map[string]any{
			"path":     "/test/file",
			"size":     len(content),
			"encoding": "base64",
			"content":  base64.StdEncoding.EncodeToString(content),
		}
		text, _ := json.Marshal(envelope)
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(text)},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRemoteBackend_Read(t *testing.T) {
	want := []byte("remote content\n")
	srv := remoteReadFileServer(t, want)
	defer srv.Close()

	b := mooncake.NewRemoteBackend(mooncake.RemoteConfig{BaseURL: srv.URL})
	got, err := b.Read(context.Background(), "/test/file", mooncake.ReadOptions{})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q; want %q", got, want)
	}
}

// remoteGrepServer returns a test server that handles grep_files tool calls.
func remoteGrepServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matches := []map[string]any{
			{"path": "/dir/a.txt", "line": 3, "content": "needle line"},
		}
		text, _ := json.Marshal(map[string]any{"matches": matches})
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": string(text)}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRemoteBackend_Grep(t *testing.T) {
	srv := remoteGrepServer(t)
	defer srv.Close()

	b := mooncake.NewRemoteBackend(mooncake.RemoteConfig{BaseURL: srv.URL})
	matches, err := b.Grep(context.Background(), "needle", mooncake.GrepOptions{})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 1 || matches[0].Line != 3 {
		t.Errorf("unexpected matches: %v", matches)
	}
}

// remoteGlobServer returns a test server that handles glob_files tool calls.
func remoteGlobServer(t *testing.T, paths []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		text, _ := json.Marshal(map[string]any{"paths": paths})
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": string(text)}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRemoteBackend_Glob(t *testing.T) {
	want := []string{"/dir/a.go", "/dir/b.go"}
	srv := remoteGlobServer(t, want)
	defer srv.Close()

	b := mooncake.NewRemoteBackend(mooncake.RemoteConfig{BaseURL: srv.URL})
	paths, err := b.Glob(context.Background(), "*.go", mooncake.GlobOptions{})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("len(paths)=%d; want 2", len(paths))
	}
}

// remoteMutationServer handles the Write/Edit/Exec flow:
// PUT /v1/files (200), GET /v1/version (synced_root), POST /v1/runs (run_id),
// GET /v1/runs/{id} (success terminal), GET /v1/runs/{id}/result (KernelResult).
func remoteMutationServer(t *testing.T) *httptest.Server {
	t.Helper()
	var runID string
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/files":
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodGet && r.URL.Path == "/v1/version":
			syncedRoot := t.TempDir()
			_ = os.MkdirAll(filepath.Join(syncedRoot, "sdk-remote"), 0o755)
			resp := map[string]any{
				"version":     "test",
				"synced_root": syncedRoot,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			runID = "test-run-1"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"run_id": runID, "status": "queued"})

		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/result"):
			result := map[string]any{
				"summary": map[string]any{
					"success":     true,
					"total_steps": 1,
					"ok":          1,
					"changed":     1,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result)

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/v1/runs/"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": runID, "status": "success"})

		default:
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusBadRequest)
		}
	}))
}

func TestRemoteBackend_Write(t *testing.T) {
	srv := remoteMutationServer(t)
	defer srv.Close()

	b := mooncake.NewRemoteBackend(mooncake.RemoteConfig{BaseURL: srv.URL})
	res, err := b.Write(context.Background(), "/tmp/test.txt", []byte("hello"), mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !res.Summary.Success {
		t.Errorf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
}

func TestRemoteBackend_Edit(t *testing.T) {
	srv := remoteMutationServer(t)
	defer srv.Close()

	b := mooncake.NewRemoteBackend(mooncake.RemoteConfig{BaseURL: srv.URL})
	res, err := b.Edit(context.Background(), "/tmp/test.txt", "old", "new", mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !res.Summary.Success {
		t.Errorf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
}

func TestRemoteBackend_Exec(t *testing.T) {
	srv := remoteMutationServer(t)
	defer srv.Close()

	b := mooncake.NewRemoteBackend(mooncake.RemoteConfig{BaseURL: srv.URL})
	res, err := b.Exec(context.Background(), "echo hi", mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !res.Summary.Success {
		t.Errorf("Summary.Success=false: %s", res.Summary.ErrorMessage)
	}
}

// ---------------------------------------------------------------------------
// Swap-seam invariant: same driver, different backend, zero call-site change
// ---------------------------------------------------------------------------

// runDriver exercises the backend through the CodingBackend interface.
// It reads, searches, globs, and mutates. This proves a driver can swap
// backend with zero interface change.
func runDriver(t *testing.T, b mooncake.CodingBackend, dir string) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(dir, "swap.txt")

	if _, err := b.Write(ctx, path, []byte("hello world\n"), mooncake.ApplyOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := b.Read(ctx, path, mooncake.ReadOptions{})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(string(got), "hello") {
		t.Errorf("Read = %q; want content with 'hello'", got)
	}
	matches, err := b.Grep(ctx, "hello", mooncake.GrepOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) == 0 {
		t.Error("Grep returned no matches")
	}
	paths, err := b.Glob(ctx, "*.txt", mooncake.GlobOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) == 0 {
		t.Error("Glob returned no paths")
	}
}

func TestBackendSwapSeam_Mooncake(t *testing.T) {
	runDriver(t, mooncake.NewMooncakeBackend(), t.TempDir())
}

func TestBackendSwapSeam_Native(t *testing.T) {
	runDriver(t, mooncake.NewNativeBackend(), t.TempDir())
}

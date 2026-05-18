package agentd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- resolveSyncPath pure-function tests ---------------------------------

func TestResolveSyncPath_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"foo/../../etc/passwd",
		"foo/../../..",
	}
	for _, c := range cases {
		if _, err := resolveSyncPath(root, "alice/dir1", c); err == nil {
			t.Errorf("rel=%q: want rejection, got pass", c)
		}
	}
}

func TestResolveSyncPath_RejectsAbsolute(t *testing.T) {
	root := t.TempDir()
	if _, err := resolveSyncPath(root, "alice/dir1", "/etc/passwd"); err == nil {
		t.Fatal("want rejection of absolute path")
	}
}

func TestResolveSyncPath_RejectsNullByte(t *testing.T) {
	root := t.TempDir()
	if _, err := resolveSyncPath(root, "alice/dir1", "foo\x00.txt"); err == nil {
		t.Fatal("want rejection of null byte in path")
	}
}

func TestResolveSyncPath_RejectsBadScope(t *testing.T) {
	root := t.TempDir()
	cases := map[string]string{
		"empty":                "",
		"contains slash slash": "a//b",
		"too many segments":    "a/b/c",
		"bad char":             "alice;rm",
		"space":                "with space",
		"unicode":              "Ünicode",
		"way too long":         strings.Repeat("a", maxScopeBytes+1),
		"segment too long":     strings.Repeat("a", maxScopeSegBytes+1),
	}
	for name, scope := range cases {
		if _, err := resolveSyncPath(root, scope, "foo.txt"); err == nil {
			t.Errorf("%s: scope=%q should have been rejected", name, scope)
		}
	}
}

func TestResolveSyncPath_AcceptsValidScopes(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"alice",
		"alice/dir1",
		"a-b_c",
		"a-b_c/d-e_f",
		strings.Repeat("a", maxScopeSegBytes),
	}
	for _, scope := range cases {
		if _, err := resolveSyncPath(root, scope, "foo.txt"); err != nil {
			t.Errorf("scope=%q should have been accepted, got %v", scope, err)
		}
	}
}

func TestResolveSyncPath_AcceptsValidRelPaths(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"foo.txt",
		"presets/bar.yml",
		"a/b/c/d/e.txt",
		"./normalized/by/clean.txt",
		"file.with.dots.txt",
	}
	for _, rel := range cases {
		got, err := resolveSyncPath(root, "alice/dir1", rel)
		if err != nil {
			t.Errorf("rel=%q: want accept, got error %v", rel, err)
			continue
		}
		if !strings.HasPrefix(got, filepath.Join(root, "alice/dir1")) {
			t.Errorf("rel=%q: resolved to %q, want under scope root", rel, got)
		}
	}
}

func TestResolveSyncPath_RejectsOverlongPath(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("a", maxRelPathBytes+1)
	if _, err := resolveSyncPath(root, "alice/dir1", long); err == nil {
		t.Fatal("want rejection of path > maxRelPathBytes")
	}
}

func TestResolveSyncPath_RejectsSymlinkInExistingPrefix(t *testing.T) {
	root := t.TempDir()
	scope := "alice/dir1"
	scopeRoot := filepath.Join(root, scope)
	if err := os.MkdirAll(scopeRoot, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create a symlinked subdirectory inside the scope root.
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	link := filepath.Join(scopeRoot, "linkdir")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := resolveSyncPath(root, scope, "linkdir/evil.txt"); err == nil {
		t.Fatal("want rejection when path traverses a symlink")
	}
}

// --- PUT /v1/files handler tests -----------------------------------------

func TestPutFile_RoundTrip(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	body := []byte("hello, fleet!")
	u := tcpURL(cfg, "alice/dir1", "config.yml", "")
	resp := doPut(t, cfg.Token, u, body, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	got, err := os.ReadFile(filepath.Join(cfg.SyncedRoot(), "alice/dir1", "config.yml"))
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("file contents differ:\nwant: %q\ngot:  %q", body, got)
	}

	info, err := os.Stat(filepath.Join(cfg.SyncedRoot(), "alice/dir1", "config.yml"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 0600", got)
	}
}

func TestPutFile_CreatesNestedDirs(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	body := []byte("nested!")
	u := tcpURL(cfg, "alice/dir1", "deeply/nested/thing/here.txt", "")
	resp := doPut(t, cfg.Token, u, body, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	got, err := os.ReadFile(filepath.Join(cfg.SyncedRoot(), "alice/dir1", "deeply/nested/thing/here.txt"))
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("contents differ")
	}
}

func TestPutFile_VerifiesSha256(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	body := []byte("hash-me")
	sum := sha256.Sum256(body)
	correctHex := hex.EncodeToString(sum[:])

	// Correct hash → 204.
	u := tcpURL(cfg, "alice/dir1", "ok.txt", "")
	resp := doPut(t, cfg.Token, u, body, correctHex)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("correct hash: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Wrong hash → 422 and no file left behind.
	wrongHex := strings.Repeat("0", 64)
	u2 := tcpURL(cfg, "alice/dir1", "wrong.txt", "")
	resp = doPut(t, cfg.Token, u2, body, wrongHex)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("wrong hash: want 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	if _, err := os.Stat(filepath.Join(cfg.SyncedRoot(), "alice/dir1", "wrong.txt")); !os.IsNotExist(err) {
		t.Errorf("file should not have been committed on hash mismatch (err=%v)", err)
	}
}

func TestPutFile_RejectsMalformedSha(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	u := tcpURL(cfg, "alice/dir1", "x.txt", "")
	resp := doPut(t, cfg.Token, u, []byte("x"), "not-hex!!!")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 on malformed sha, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPutFile_EnforcesSizeLimit(t *testing.T) {
	// Start server with a tiny size cap.
	t.Setenv("MOONCAKE_TEST_SMALL_LIMIT", "1")
	cfg, _, _, stop := startTestServerTCPWithLimit(t, 16)
	defer stop()

	body := bytes.Repeat([]byte("x"), 32) // 2x the cap
	u := tcpURL(cfg, "alice/dir1", "big.txt", "")
	resp := doPut(t, cfg.Token, u, body, "")
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPutFile_RequiresParams(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	// scope missing
	u := "http://" + cfg.BindAddr + "/v1/files?path=foo.txt"
	resp := doPut(t, cfg.Token, u, []byte("x"), "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing scope: want 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// path missing
	u = "http://" + cfg.BindAddr + "/v1/files?scope=alice/dir1"
	resp = doPut(t, cfg.Token, u, []byte("x"), "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing path: want 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPutFile_RejectsTraversal(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	u := tcpURL(cfg, "alice/dir1", "../../etc/passwd", "")
	resp := doPut(t, cfg.Token, u, []byte("x"), "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 on traversal, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPutFile_RequiresAuth(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	u := tcpURL(cfg, "alice/dir1", "x.txt", "")
	// No bearer header.
	resp := doPut(t, "", u, []byte("x"), "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- HEAD /v1/files handler tests ----------------------------------------

func TestHeadFile_Match(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	body := []byte("already here")
	target := filepath.Join(cfg.SyncedRoot(), "alice/dir1", "exists.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sum := sha256.Sum256(body)
	correctHex := hex.EncodeToString(sum[:])

	u := tcpURL(cfg, "alice/dir1", "exists.txt", correctHex)
	resp := doHead(t, cfg.Token, u)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestHeadFile_HashMismatch(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	body := []byte("seeded content")
	target := filepath.Join(cfg.SyncedRoot(), "alice/dir1", "exists.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	u := tcpURL(cfg, "alice/dir1", "exists.txt", strings.Repeat("0", 64))
	resp := doHead(t, cfg.Token, u)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 on hash mismatch, got %d", resp.StatusCode)
	}
}

func TestHeadFile_NotPresent(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	u := tcpURL(cfg, "alice/dir1", "missing.txt", strings.Repeat("a", 64))
	resp := doHead(t, cfg.Token, u)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 on missing file, got %d", resp.StatusCode)
	}
}

func TestHeadFile_RequiresSha(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	// No sha256 param.
	u := "http://" + cfg.BindAddr + "/v1/files?scope=alice/dir1&path=foo.txt"
	resp := doHead(t, cfg.Token, u)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 on missing sha, got %d", resp.StatusCode)
	}
}

func TestHeadFile_RequiresAuth(t *testing.T) {
	cfg, _, _, stop := startTestServerTCP(t)
	defer stop()

	u := tcpURL(cfg, "alice/dir1", "x.txt", strings.Repeat("a", 64))
	resp := doHead(t, "", u)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

// --- helpers --------------------------------------------------------------

func tcpURL(cfg Config, scope, path, sha string) string {
	q := url.Values{}
	q.Set("scope", scope)
	q.Set("path", path)
	if sha != "" {
		q.Set("sha256", sha)
	}
	return "http://" + cfg.BindAddr + "/v1/files?" + q.Encode()
}

func doPut(t *testing.T, token, u string, body []byte, expectedSha string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if expectedSha != "" {
		req.Header.Set("X-Sha256", expectedSha)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", u, err)
	}
	return resp
}

func doHead(t *testing.T, token, u string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodHead, u, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HEAD %s: %v", u, err)
	}
	// HEAD responses sometimes have no body, but io.ReadAll on a nil/empty
	// body is safe and lets `defer resp.Body.Close()` work cleanly.
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp
}

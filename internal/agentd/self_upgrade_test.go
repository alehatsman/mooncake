package agentd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeMooncakeBinary writes a tiny shell script to <dir>/mooncake that
// prints "mooncake version <v>" and exits 0, mimicking the surface
// `sanityCheckBinary` looks for. Test-only — the real binary is a Go
// executable; the script is enough to satisfy the --version probe.
func fakeMooncakeBinary(t *testing.T, dir, version string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fakeMooncakeBinary uses POSIX shell")
	}
	path := filepath.Join(dir, "mooncake")
	body := "#!/bin/sh\necho mooncake version " + version + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func sha256Bytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// putBinary streams body to PUT /v1/self/binary on the test server.
func putBinary(t *testing.T, client *http.Client, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, "http://localhost/v1/self/binary", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func TestSelfBinary_AcceptsValidBinary(t *testing.T) {
	cfg, client, stop := startTestServer(t)
	defer stop()

	dir := t.TempDir()
	binPath := fakeMooncakeBinary(t, dir, "1.2.3")
	body, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}

	resp := putBinary(t, client, body, map[string]string{
		"X-Mooncake-Binary-SHA256": sha256Bytes(body),
		"X-Mooncake-Binary-OS":     runtime.GOOS,
		"X-Mooncake-Binary-Arch":   runtime.GOARCH,
		"Content-Type":             "application/octet-stream",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 202; body=%s", resp.StatusCode, string(b))
	}
	var out stageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(out.StagedPath, filepath.Join(cfg.StateDir, "upgrade")) {
		t.Errorf("staged path %q not under upgrade/", out.StagedPath)
	}
	if out.SHA256 != sha256Bytes(body) {
		t.Errorf("sha mismatch: got %s want %s", out.SHA256, sha256Bytes(body))
	}
	if _, err := os.Stat(out.StagedPath); err != nil {
		t.Errorf("staged file missing: %v", err)
	}
}

func TestSelfBinary_RejectsSHAMismatch(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	dir := t.TempDir()
	binPath := fakeMooncakeBinary(t, dir, "1.2.3")
	body, _ := os.ReadFile(binPath)

	// Use a sha for "different bytes" so the header doesn't match the body.
	wrongSHA := sha256Bytes([]byte("not the body bytes"))

	resp := putBinary(t, client, body, map[string]string{
		"X-Mooncake-Binary-SHA256": wrongSHA,
		"X-Mooncake-Binary-OS":     runtime.GOOS,
		"X-Mooncake-Binary-Arch":   runtime.GOARCH,
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	bodyOut, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(bodyOut), "sha_mismatch") {
		t.Errorf("body doesn't mention sha_mismatch: %s", bodyOut)
	}
}

func TestSelfBinary_RejectsOSMismatch(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	dir := t.TempDir()
	binPath := fakeMooncakeBinary(t, dir, "1.2.3")
	body, _ := os.ReadFile(binPath)

	resp := putBinary(t, client, body, map[string]string{
		"X-Mooncake-Binary-SHA256": sha256Bytes(body),
		"X-Mooncake-Binary-OS":     "plan9", // never the daemon's runtime
		"X-Mooncake-Binary-Arch":   runtime.GOARCH,
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	bodyOut, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(bodyOut), "os_mismatch") {
		t.Errorf("body doesn't mention os_mismatch: %s", bodyOut)
	}
}

// Garbage bytes that aren't a valid executable get caught by the
// --version sanity check. The daemon should respond 400 binary_unhealthy
// and the staged file should be cleaned up.
func TestSelfBinary_RejectsUnhealthyBinary(t *testing.T) {
	cfg, client, stop := startTestServer(t)
	defer stop()

	body := []byte("this is not a valid executable\n")
	resp := putBinary(t, client, body, map[string]string{
		"X-Mooncake-Binary-SHA256": sha256Bytes(body),
		"X-Mooncake-Binary-OS":     runtime.GOOS,
		"X-Mooncake-Binary-Arch":   runtime.GOARCH,
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	bodyOut, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(bodyOut), "binary_unhealthy") {
		t.Errorf("body doesn't mention binary_unhealthy: %s", bodyOut)
	}
	// No staged file should remain after a failed sanity check.
	entries, _ := os.ReadDir(filepath.Join(cfg.StateDir, "upgrade"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "staged-") {
			t.Errorf("staged file %q lingered after failed sanity check", e.Name())
		}
	}
}

func TestSelfReplace_RejectsPathOutsideUpgradeDir(t *testing.T) {
	cfg, client, stop := startTestServer(t)
	defer stop()

	// Even a file that exists, but lives outside the upgrade dir, must
	// be refused — closes the obvious "swap in whatever file you want"
	// abuse channel.
	outside := filepath.Join(t.TempDir(), "evil.bin")
	if err := os.WriteFile(outside, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(replaceRequest{
		StagedPath: outside,
		SHA256:     sha256Bytes([]byte("x")),
	})
	req, _ := http.NewRequest(http.MethodPost, "http://localhost/v1/self/replace", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	bodyOut, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(bodyOut), "bad_staged_path") {
		t.Errorf("body doesn't mention bad_staged_path: %s", bodyOut)
	}
	// Nothing should have changed in upgrade/.
	_, _ = os.ReadDir(filepath.Join(cfg.StateDir, "upgrade"))
}

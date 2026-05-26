package main

import (
	"io"
	"os"
	"testing"
)

// captureStdout runs fn, capturing what it writes to os.Stdout, and
// returns the captured bytes. Lives in cmd/ root so any *_test.go in
// package main can use it — the original definition in cmd/tool was
// the lone copy until cmd/tool got sub-packaged out; this is the
// surviving cmd/-side replacement for callers in package main (e.g.
// cmd/query_cmd_test.go).
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
		_ = w.Close()
	}()

	out, _ := io.ReadAll(r)
	return string(out), <-errCh
}

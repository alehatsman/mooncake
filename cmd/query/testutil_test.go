package query

import (
	"io"
	"os"
	"testing"
)

// captureStdout runs fn, capturing what it writes to os.Stdout, and
// returns the captured bytes. Local copy for query package tests; a
// sibling copy lives at cmd/testutil_test.go for package-main tests.
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

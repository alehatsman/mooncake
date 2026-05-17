package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/queryio"
	"github.com/urfave/cli/v2"
)

// TestPickFormat covers extension auto-detect + explicit override.
func TestPickFormat(t *testing.T) {
	cases := []struct {
		path, override, want, wantErr string
	}{
		{"foo.json", "", "json", ""},
		{"foo.yaml", "", "yaml", ""},
		{"foo.yml", "", "yaml", ""},
		{"foo.JSON", "", "json", ""},
		{"foo.unknown", "", "", "cannot infer format"},
		{"foo.unknown", "json", "json", ""},
		{"foo.json", "yaml", "yaml", ""},
		{"foo.json", "toml", "", "must be json or yaml"},
	}
	for _, c := range cases {
		t.Run(c.path+"|"+c.override, func(t *testing.T) {
			got, err := queryio.PickFormat(c.path, c.override, "--as")
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want err containing %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestParseDoc round-trips both formats and rejects multi-document YAML.
func TestParseDoc(t *testing.T) {
	v, err := queryio.ParseDoc([]byte(`{"a":1}`), "json")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	m, _ := v.(map[string]any)
	if m["a"] != float64(1) {
		t.Errorf("json round-trip: got %v", v)
	}

	v, err = queryio.ParseDoc([]byte("a: 1\n"), "yaml")
	if err != nil {
		t.Fatalf("yaml: %v", err)
	}
	m, _ = v.(map[string]any)
	if m["a"] != 1 {
		t.Errorf("yaml round-trip: got %v", v)
	}

	_, err = queryio.ParseDoc([]byte("a: 1\n---\nb: 2\n"), "yaml")
	if err == nil || !strings.Contains(err.Error(), "multi-document YAML not supported") {
		t.Errorf("expected multi-doc rejection, got %v", err)
	}
}

// TestReadBoundedFile covers happy path + missing + oversize.
func TestReadBoundedFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"k":"v"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := queryio.ReadBounded(good, 1024)
	if err != nil {
		t.Fatalf("read good: %v", err)
	}
	if !strings.Contains(string(data), `"k"`) {
		t.Errorf("data missing: %q", data)
	}

	_, err = queryio.ReadBounded(filepath.Join(dir, "missing.json"), 1024)
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected not-found, got %v", err)
	}

	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, bytes.Repeat([]byte("x"), 200), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = queryio.ReadBounded(big, 50)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected oversize, got %v", err)
	}
}

// TestPrintQueryValue captures stdout to assert output shape per type.
func TestPrintQueryValue(t *testing.T) {
	cases := []struct {
		name   string
		v      any
		pretty bool
		want   string
	}{
		{"string", "hello", false, "hello\n"},
		{"bool", true, false, "true\n"},
		{"int", 42, false, "42\n"},
		{"null", nil, false, "null\n"},
		{"map_compact", map[string]any{"a": 1}, false, "{\"a\":1}\n"},
		{"map_pretty", map[string]any{"a": 1}, true, "{\n  \"a\": 1\n}\n"},
		{"array_compact", []any{1, 2}, false, "[1,2]\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := captureStdout(t, func() error {
				return printQueryValue(c.v, c.pretty)
			})
			if err != nil {
				t.Fatalf("printQueryValue: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestRunQuery_E2E drives the full subcommand via cli.App and asserts
// exit-code semantics (0 found, 1 path-miss, 2 parse/format error).
func TestRunQuery_E2E(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "p.json")
	if err := os.WriteFile(jsonPath, []byte(`{"v":"ok"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  string
	}{
		{"found", []string{"query", jsonPath, "v"}, 0, "ok\n"},
		{"miss", []string{"query", jsonPath, "missing"}, 1, ""},
		{"file-missing", []string{"query", filepath.Join(dir, "nope.json"), "v"}, 2, ""},
		{"bad-extension", []string{"query", filepath.Join(dir, "p.txt"), "v"}, 2, ""},
	}

	if err := os.WriteFile(filepath.Join(dir, "p.txt"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := &cli.App{
				Commands: []*cli.Command{queryCommand()},
				// urfave/cli writes ExitCoder messages to ErrWriter;
				// silence them so test output stays clean.
				ErrWriter: io.Discard,
			}
			var gotExit int
			cli.OsExiter = func(code int) { gotExit = code }
			t.Cleanup(func() { cli.OsExiter = os.Exit })

			gotOut, _ := captureStdout(t, func() error {
				err := app.Run(append([]string{"mooncake"}, c.args...))
				if ec, ok := err.(cli.ExitCoder); ok {
					gotExit = ec.ExitCode()
				} else if err != nil && gotExit == 0 {
					gotExit = 1
				}
				return nil
			})

			if gotExit != c.wantExit {
				t.Errorf("exit=%d want %d", gotExit, c.wantExit)
			}
			if c.wantOut != "" && gotOut != c.wantOut {
				t.Errorf("stdout=%q want %q", gotOut, c.wantOut)
			}
		})
	}
}

// Sanity: parsed JSON values round-trip through json.Marshal cleanly so
// downstream consumers can mix structured + scalar results.
func TestPrintQueryValue_RoundTripsThroughJSON(t *testing.T) {
	v := map[string]any{"a": 1, "b": []any{"x", "y"}}
	got, _ := captureStdout(t, func() error { return printQueryValue(v, false) })

	var back map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &back); err != nil {
		t.Fatalf("json round-trip: %v\nraw: %q", err, got)
	}
	if back["a"] != float64(1) {
		t.Errorf("round-trip lost data: %v", back)
	}
}

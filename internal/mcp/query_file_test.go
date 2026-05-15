package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeQueryFixtures drops fresh JSON + YAML test files in t.TempDir
// and returns the directory + their paths.
func writeQueryFixtures(t *testing.T) (dir, jsonPath, yamlPath string) {
	t.Helper()
	dir = t.TempDir()
	jsonPath = filepath.Join(dir, "pkg.json")
	yamlPath = filepath.Join(dir, "svc.yaml")

	jsonBody := []byte(`{
		"name": "mooncake",
		"version": "1.2.3",
		"tools": [{"name": "go"}, {"name": "make"}]
	}`)
	yamlBody := []byte("service:\n  port: 8080\n  tls: true\n")

	if err := os.WriteFile(jsonPath, jsonBody, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yamlPath, yamlBody, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, jsonPath, yamlPath
}

func callQueryFile(t *testing.T, args map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	out, err := HandleQueryFile(context.Background(), raw)
	if err != nil {
		t.Fatalf("HandleQueryFile: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("response not JSON: %v\nraw: %s", err, out)
	}
	return env
}

// TestQueryFile_JSONScalar: dotted path against a JSON file returns
// {"found":true,"value":<scalar>}.
func TestQueryFile_JSONScalar(t *testing.T) {
	_, jsonPath, _ := writeQueryFixtures(t)

	env := callQueryFile(t, map[string]any{
		"path":  jsonPath,
		"query": "version",
	})
	if env["found"] != true {
		t.Errorf("expected found=true, got %v", env)
	}
	if env["value"] != "1.2.3" {
		t.Errorf("expected value=1.2.3, got %v", env["value"])
	}
	if env["format"] != "json" {
		t.Errorf("expected format=json, got %v", env["format"])
	}
}

// TestQueryFile_JSONNested: bracketed array index works on JSON.
func TestQueryFile_JSONNested(t *testing.T) {
	_, jsonPath, _ := writeQueryFixtures(t)

	env := callQueryFile(t, map[string]any{
		"path":  jsonPath,
		"query": "tools[1].name",
	})
	if env["value"] != "make" {
		t.Errorf("expected tools[1].name=make, got %v", env["value"])
	}
}

// TestQueryFile_YAMLAutoDetect: .yaml extension routes through YAML
// parser without an explicit format override.
func TestQueryFile_YAMLAutoDetect(t *testing.T) {
	_, _, yamlPath := writeQueryFixtures(t)

	env := callQueryFile(t, map[string]any{
		"path":  yamlPath,
		"query": "service.port",
	})
	if env["found"] != true {
		t.Errorf("expected found=true, got %v", env)
	}
	// YAML port is an int — decodes to float64 when round-tripped
	// through JSON. Either is acceptable; just check it stringifies
	// to "8080".
	if got := stringify(env["value"]); got != "8080" {
		t.Errorf("expected value=8080, got %v", env["value"])
	}
	if env["format"] != "yaml" {
		t.Errorf("expected format=yaml, got %v", env["format"])
	}
}

// TestQueryFile_PathMiss: unknown key returns {"found":false} with no
// "value" field — distinct from an error.
func TestQueryFile_PathMiss(t *testing.T) {
	_, jsonPath, _ := writeQueryFixtures(t)

	env := callQueryFile(t, map[string]any{
		"path":  jsonPath,
		"query": "does.not.exist",
	})
	if env["found"] != false {
		t.Errorf("expected found=false, got %v", env)
	}
	if _, hasValue := env["value"]; hasValue {
		t.Errorf("expected no value key on miss, got %v", env)
	}
}

// TestQueryFile_FileNotFound: missing file returns an envelope with
// "error" set — not a transport-layer error.
func TestQueryFile_FileNotFound(t *testing.T) {
	env := callQueryFile(t, map[string]any{
		"path": "/no/such/file.json",
	})
	errStr, _ := env["error"].(string)
	if !strings.Contains(errStr, "file not found") {
		t.Errorf("expected file-not-found error envelope, got %v", env)
	}
}

// TestQueryFile_UnknownExtensionRequiresFormat: extension we don't
// recognize must demand the explicit override.
func TestQueryFile_UnknownExtensionRequiresFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "blob.unknown")
	if err := os.WriteFile(p, []byte(`{"k":"v"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	env := callQueryFile(t, map[string]any{"path": p, "query": "k"})
	errStr, _ := env["error"].(string)
	if !strings.Contains(errStr, "cannot infer format") {
		t.Errorf("expected format-required error, got %v", env)
	}

	// With override, it should succeed.
	env = callQueryFile(t, map[string]any{
		"path":   p,
		"query":  "k",
		"format": "json",
	})
	if env["value"] != "v" {
		t.Errorf("expected value=v after override, got %v", env)
	}
}

// TestQueryFile_NoPathRequired: missing path arg is a hard error
// (returned via the second slot, not the envelope), since the request
// itself is malformed.
func TestQueryFile_NoPathRequired(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"query": "k"})
	_, err := HandleQueryFile(context.Background(), raw)
	if err == nil {
		t.Fatalf("expected error when path is missing")
	}
	if !strings.Contains(err.Error(), "path parameter required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestQueryFile_MaxBytesEnforced: oversize file returns an envelope
// error, not a successful parse.
func TestQueryFile_MaxBytesEnforced(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.json")
	big := append([]byte(`{"a":"`), make([]byte, 2048)...)
	big = append(big, []byte(`"}`)...)
	for i := 6; i < 6+2048; i++ {
		big[i] = 'x'
	}
	if err := os.WriteFile(p, big, 0o600); err != nil {
		t.Fatal(err)
	}

	env := callQueryFile(t, map[string]any{
		"path":      p,
		"query":     "a",
		"max_bytes": 100,
	})
	errStr, _ := env["error"].(string)
	if !strings.Contains(errStr, "exceeds max_bytes") {
		t.Errorf("expected oversize error, got %v", env)
	}
}

// TestQueryFile_RootDocument: empty query returns the whole parsed
// document (matches CLI behavior).
func TestQueryFile_RootDocument(t *testing.T) {
	_, jsonPath, _ := writeQueryFixtures(t)

	env := callQueryFile(t, map[string]any{"path": jsonPath})
	if env["found"] != true {
		t.Errorf("expected found=true for root, got %v", env)
	}
	root, ok := env["value"].(map[string]any)
	if !ok {
		t.Fatalf("root value should be an object, got %T", env["value"])
	}
	if root["version"] != "1.2.3" {
		t.Errorf("root.version mismatch: %v", root["version"])
	}
}

// TestQueryFile_BadFormatOverride: unknown format string is an error.
func TestQueryFile_BadFormatOverride(t *testing.T) {
	_, jsonPath, _ := writeQueryFixtures(t)

	env := callQueryFile(t, map[string]any{
		"path":   jsonPath,
		"format": "toml",
	})
	errStr, _ := env["error"].(string)
	if !strings.Contains(errStr, "format must be json or yaml") {
		t.Errorf("expected bad-format error, got %v", env)
	}
}

// TestAllTools_IncludesQueryFile: the tool is registered in AllTools
// so MCP clients can discover it.
func TestAllTools_IncludesQueryFile(t *testing.T) {
	var found bool
	for _, tool := range AllTools() {
		if tool.Name == "query_file" {
			found = true
			// path must be required; query optional
			schema, ok := tool.InputSchema.(map[string]interface{})
			if !ok {
				t.Fatalf("InputSchema is not a map: %T", tool.InputSchema)
			}
			req, _ := schema["required"].([]string)
			hasPath := false
			for _, k := range req {
				if k == "path" {
					hasPath = true
				}
			}
			if !hasPath {
				t.Errorf("query_file tool: path not in required: %v", req)
			}
		}
	}
	if !found {
		t.Errorf("query_file not registered in AllTools()")
	}
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		s := string(b)
		// trim trailing decimal-zero so 8080.0 reads as 8080
		if dot := strings.IndexByte(s, '.'); dot >= 0 {
			tail := s[dot+1:]
			allZero := tail != ""
			for _, c := range tail {
				if c != '0' {
					allZero = false
					break
				}
			}
			if allZero {
				return s[:dot]
			}
		}
		return s
	}
}

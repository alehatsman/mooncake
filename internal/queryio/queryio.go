// Package queryio holds the read+parse+format-detection helpers
// shared between `mooncake query` (cmd/query_cmd.go) and the MCP
// query_file tool (internal/mcp/query_file.go). Both surfaces accept
// a path + optional format override + a pathquery expression, then
// extract a value from a JSON or YAML document.
//
// Lives outside cmd/ because internal/mcp can't import cmd/. Lives
// outside internal/actions/read_common because that package owns
// the *action*-shaped read pipeline (Validate, Run, secret redaction,
// max_bytes from config.ReadFile); the helpers here are the smaller
// "raw bytes + parse + pick format" surface the CLI/MCP wrappers need.
package queryio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadBounded reads up to limit bytes from path. Reads limit+1 so an
// oversize file errors without first slurping the whole thing into
// memory. Returns "file not found" for ErrNotExist, a wrapped open
// error otherwise.
func ReadBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- path is a user-supplied argument by design
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds max_bytes=%d", path, limit)
	}
	return data, nil
}

// ParseDoc decodes data as either "json" or "yaml" into a generic Go
// value (any nested combination of map[string]any, []any, and
// scalars). YAML rejects multi-document files to match read.yaml's
// behavior (spec-38 Open Q3). The format argument must be lowercase.
func ParseDoc(data []byte, format string) (any, error) {
	var v any
	switch format {
	case "json":
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	case "yaml":
		dec := yaml.NewDecoder(bytes.NewReader(data))
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		var second any
		switch err := dec.Decode(&second); {
		case errors.Is(err, io.EOF):
			return v, nil
		case err == nil:
			return nil, fmt.Errorf("multi-document YAML not supported")
		default:
			return nil, fmt.Errorf("trailing-document parse: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

// PickFormat resolves the file format, preferring an explicit
// override when set. Returns "json" or "yaml", or an error if neither
// override nor the path extension is decisive. `flagLabel` (e.g.
// "--as" for the CLI, "format" for the MCP tool) is interpolated into
// the user-facing error so each surface keeps its own argument shape.
func PickFormat(path, override, flagLabel string) (string, error) {
	switch override {
	case "":
		// fall through to extension sniffing
	case "json", "yaml":
		return override, nil
	default:
		return "", fmt.Errorf("%s must be json or yaml (got %q)", flagLabel, override)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "json", nil
	case ".yml", ".yaml":
		return "yaml", nil
	default:
		return "", fmt.Errorf("cannot infer format from extension; pass %s json|yaml", flagLabel)
	}
}

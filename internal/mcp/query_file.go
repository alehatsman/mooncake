package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/pathquery"
)

// queryFileDefaultMaxBytes mirrors read_common.DefaultMaxBytes and the
// CLI's defaultQueryMaxBytes — 4 MiB. Kept duplicated rather than
// imported so MCP doesn't pull internal/actions just for a constant.
const queryFileDefaultMaxBytes int64 = 4 << 20

// queryFileParams is the deserialized argument shape for the
// `query_file` MCP tool. Mirrors the CLI flags so tools-clients can
// switch between the two surfaces without remapping.
type queryFileParams struct {
	Path     string `json:"path"`
	Query    string `json:"query"`
	Format   string `json:"format"`
	MaxBytes int64  `json:"max_bytes"`
}

// HandleQueryFile reads a JSON/YAML file, extracts a value by dotted
// path, and returns a structured JSON envelope to the LLM:
//
//	{"found": true,  "path": "/abs/path", "format": "json", "value": ...}
//	{"found": false, "path": "/abs/path", "format": "json"}
//	{"error":  "...","path": "..."}
//
// The envelope is friendlier than a raw value for LLM consumers — they
// can distinguish "key not present" from "errored out" without parsing
// the error string.
func HandleQueryFile(_ context.Context, args json.RawMessage) (string, error) {
	var params queryFileParams
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("query_file: parse args: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("query_file: path parameter required")
	}
	if params.MaxBytes == 0 {
		params.MaxBytes = queryFileDefaultMaxBytes
	}

	format, err := pickQueryFormat(params.Path, params.Format)
	if err != nil {
		return mcpError(params.Path, err.Error())
	}

	if err := pathquery.Validate(params.Query); err != nil {
		return mcpError(params.Path, "invalid query: "+err.Error())
	}

	data, err := readQueryFileBounded(params.Path, params.MaxBytes)
	if err != nil {
		return mcpError(params.Path, err.Error())
	}

	parsed, err := parseQueryDoc(data, format)
	if err != nil {
		return mcpError(params.Path, fmt.Sprintf("parse %s: %v", format, err))
	}

	value, found, err := pathquery.Extract(parsed, params.Query)
	if err != nil {
		return mcpError(params.Path, err.Error())
	}

	envelope := map[string]any{
		"path":   params.Path,
		"format": format,
		"found":  found,
	}
	if found {
		envelope["value"] = value
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("query_file: marshal envelope: %w", err)
	}
	return string(b), nil
}

func mcpError(path, msg string) (string, error) {
	b, err := json.Marshal(map[string]any{"path": path, "error": msg})
	if err != nil {
		// fall back to a plain string so the caller still gets an
		// observable failure
		return "", fmt.Errorf("query_file: %s", msg)
	}
	return string(b), nil
}

// pickQueryFormat is the MCP-side twin of cmd/query_cmd.go:pickFormat.
// Duplication is intentional — MCP doesn't depend on the cmd package
// and the function is tiny enough that a shared helper isn't worth a
// dedicated package.
func pickQueryFormat(path, override string) (string, error) {
	switch override {
	case "":
	case "json", "yaml":
		return override, nil
	default:
		return "", fmt.Errorf("format must be json or yaml (got %q)", override)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return "json", nil
	case ".yml", ".yaml":
		return "yaml", nil
	default:
		return "", fmt.Errorf("cannot infer format from extension; pass format=json|yaml")
	}
}

func readQueryFileBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 — path is a tool argument by design
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

func parseQueryDoc(data []byte, format string) (any, error) {
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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alehatsman/mooncake/internal/pathquery"
	"github.com/alehatsman/mooncake/internal/queryio"
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

	format, err := queryio.PickFormat(params.Path, params.Format, "format")
	if err != nil {
		return mcpError(params.Path, err.Error())
	}

	if err := pathquery.Validate(params.Query); err != nil {
		return mcpError(params.Path, "invalid query: "+err.Error())
	}

	data, err := queryio.ReadBounded(params.Path, params.MaxBytes)
	if err != nil {
		return mcpError(params.Path, err.Error())
	}

	parsed, err := queryio.ParseDoc(data, format)
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

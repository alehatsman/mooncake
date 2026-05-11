// Package mcp implements a minimal MCP (Model Context Protocol) server over
// stdio transport using JSON-RPC 2.0. Only the subset of the protocol needed
// for tool calls is implemented.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// rpcRequest is a JSON-RPC 2.0 request.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// rpcResponse is a JSON-RPC 2.0 response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolHandler is a function that handles a tool call.
// args is the raw JSON arguments object. Returns content text or error.
type ToolHandler func(ctx context.Context, args json.RawMessage) (string, error)

// ToolDef describes a tool for the tools/list response.
type ToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// Server is a minimal MCP stdio server.
type Server struct {
	r     io.Reader
	w     io.Writer
	tools []ToolDef
	hdlrs map[string]ToolHandler
}

// New creates a new MCP server reading from r and writing to w.
func New(r io.Reader, w io.Writer) *Server {
	return &Server{
		r:     r,
		w:     w,
		hdlrs: make(map[string]ToolHandler),
	}
}

// RegisterTool registers a tool with a definition and handler.
func (s *Server) RegisterTool(def ToolDef, h ToolHandler) {
	s.tools = append(s.tools, def)
	s.hdlrs[def.Name] = h
}

// Serve reads JSON-RPC requests from stdin and writes responses to stdout
// until ctx is cancelled or the reader is closed.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	enc := json.NewEncoder(s.w)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			return scanner.Err() // nil on clean EOF
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}

		resp := s.dispatch(ctx, &req)
		_ = enc.Encode(resp)
	}
}

func (s *Server) dispatch(ctx context.Context, req *rpcRequest) rpcResponse {
	base := rpcResponse{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		base.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "mooncake", "version": "0.2.0"},
		}

	case "notifications/initialized", "initialized":
		// notification — no response ID; return nothing useful
		return rpcResponse{}

	case "tools/list":
		base.Result = map[string]interface{}{"tools": s.tools}

	case "tools/call":
		return s.handleToolCall(ctx, req, base)

	case "ping":
		base.Result = map[string]interface{}{}

	default:
		base.Error = &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	return base
}

func (s *Server) handleToolCall(ctx context.Context, req *rpcRequest, base rpcResponse) rpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		base.Error = &rpcError{Code: -32602, Message: "invalid params"}
		return base
	}

	h, ok := s.hdlrs[params.Name]
	if !ok {
		base.Error = &rpcError{Code: -32601, Message: fmt.Sprintf("unknown tool: %s", params.Name)}
		return base
	}

	text, err := h(ctx, params.Arguments)
	if err != nil {
		base.Error = &rpcError{Code: -32000, Message: err.Error()}
		return base
	}

	base.Result = map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	}
	return base
}

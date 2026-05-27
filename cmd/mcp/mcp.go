// Package mcp implements the `mooncake mcp` CLI — start the MCP
// (Model Context Protocol) server over stdio for LLM tool-call
// clients (Claude Desktop, etc.).
package mcp

import (
	"os"

	"github.com/alehatsman/mooncake/internal/mcp"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "mcp",
		Usage: "Start MCP server (stdio transport) for use with Claude Desktop and other MCP clients",
		Action: func(c *cli.Context) error {
			srv := mcp.New(os.Stdin, os.Stdout)
			mcp.RegisterAllTools(srv)
			return srv.Serve(c.Context)
		},
	}
}

package main

import (
	"os"

	"github.com/alehatsman/mooncake/internal/mcp"
	"github.com/urfave/cli/v2"
)

func mcpCommand() *cli.Command {
	return &cli.Command{
		Name:  "mcp",
		Usage: "Start MCP server (stdio transport) for use with Claude Desktop and other MCP clients",
		Action: func(c *cli.Context) error {
			srv := mcp.New(os.Stdin, os.Stdout)
			for _, def := range mcp.AllTools() {
				switch def.Name {
				case "get_facts":
					srv.RegisterTool(def, mcp.HandleGetFacts)
				case "get_snapshot":
					srv.RegisterTool(def, mcp.HandleGetSnapshot)
				case "fact_query":
					srv.RegisterTool(def, mcp.HandleFactQuery)
				case "run_plan":
					srv.RegisterTool(def, mcp.HandleRunPlan)
				case "check_plan":
					srv.RegisterTool(def, mcp.HandleCheckPlan)
				}
			}
			return srv.Serve(c.Context)
		},
	}
}

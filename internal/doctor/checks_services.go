package doctor

import (
	"context"
	"net"

	"github.com/alehatsman/mooncake/internal/agentd"
)

// checkAgentd probes the per-user agentd socket. Not running is the
// normal default (info, not warning) since agentd is opt-in.
type checkAgentd struct{}

func (checkAgentd) Section() string { return "services" }
func (checkAgentd) Name() string    { return "agentd" }
func (checkAgentd) Run(ctx Context) Result {
	r := Result{Section: "services", Name: "agentd"}
	cfg, err := agentd.Default(false)
	if err != nil {
		r.Status = StatusInfo
		r.Message = "agentd socket path unresolvable"
		return r
	}
	r.Detail = cfg.SocketPath
	err = withDeadline(ctx.Ctx, func(c context.Context) error {
		d := net.Dialer{}
		conn, dialErr := d.DialContext(c, "unix", cfg.SocketPath)
		if dialErr != nil {
			return dialErr
		}
		return conn.Close()
	})
	if err != nil {
		r.Status = StatusInfo
		r.Message = "agentd socket: not running"
		r.Fix = "run `mooncake agentd` if you want the daemon"
		return r
	}
	r.Status = StatusOK
	r.Message = "agentd socket: listening"
	return r
}

// checkMCP is an info-only stub. The MCP server runs in-process via
// `mooncake mcp`; there's no out-of-band socket to dial. We surface this
// as a reminder rather than do anything dynamic.
type checkMCP struct{}

func (checkMCP) Section() string { return "services" }
func (checkMCP) Name() string    { return "mcp" }
func (checkMCP) Run(_ Context) Result {
	return Result{
		Section: "services", Name: "mcp",
		Status:  StatusInfo,
		Message: "MCP server is invoked on demand via `mooncake mcp`",
	}
}

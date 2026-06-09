package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/observe"
)

// fleetObserveCommand registers `mooncake fleet observe <kind> [args]`
// (spec-64). Fans out a spec-59 / spec-60 / spec-62 observation across
// selected peers and renders the typed result as a table (default) or
// JSONL (`--format json`).
//
// CLI shape:
//
//	mooncake fleet observe port --port 80
//	mooncake fleet observe port :80                (shorthand: positional)
//	mooncake fleet observe http --url https://x/health --expect-status 200
//	mooncake fleet observe service --name nginx
//	mooncake fleet observe cpu
//	mooncake fleet observe disk --path /var
//	mooncake fleet observe gpu
func fleetObserveCommand() *cli.Command {
	commonFlags := []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "peer",
			Usage: "Select peers: repeat to UNION. Each value is a name, `key=value` filter (`tag=production`), or `@k=v,k2=v2` AND-group. Default: every peer in peers.toml.",
		},
		&cli.StringFlag{Name: "format", Usage: "Output format: table (default) or json", Value: "table"},
	}

	return &cli.Command{
		Name:  "observe",
		Usage: "Read typed state across peers (spec-64): port, process, http, service, cpu, memory, disk, gpu",
		Description: "Fan-out a single observation across selected peers; renders the " +
			"typed result as a comparison table. Compose with --peer for " +
			"selective queries (e.g. `fleet observe gpu --peer tag=inference`).",
		Subcommands: []*cli.Command{
			observePortSubcommand(commonFlags),
			observeProcessSubcommand(commonFlags),
			observeHTTPSubcommand(commonFlags),
			observeServiceSubcommand(commonFlags),
			observeCPUSubcommand(commonFlags),
			observeMemorySubcommand(commonFlags),
			observeDiskSubcommand(commonFlags),
			observeGPUSubcommand(commonFlags),
		},
	}
}

// --- Per-kind subcommands ---------------------------------------------------

func observePortSubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:      "port",
		Usage:     "Observe a TCP/UDP port across peers",
		ArgsUsage: "[:PORT] | [tcp:PORT] | [udp:PORT]",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: "host", Usage: "Host to probe (default: localhost)"},
			&cli.IntFlag{Name: "port", Usage: "Port (1..65535)"},
			&cli.StringFlag{Name: "protocol", Usage: "tcp (default) or udp"},
			&cli.StringFlag{Name: "timeout", Usage: "Dial timeout (default 2s)"},
		}, common...),
		Action: func(c *cli.Context) error {
			port := c.Int("port")
			proto := c.String("protocol")
			if port == 0 && c.NArg() > 0 {
				p, pr := observe.ParsePortShorthand(c.Args().First())
				if p == 0 {
					return cli.Exit("fleet observe port: invalid port shorthand", 2)
				}
				port = p
				if proto == "" && pr != "" {
					proto = pr
				}
			}
			return runFleetObserve(c, observe.SynthOptions{
				Kind:     "port",
				Host:     c.String("host"),
				Port:     port,
				Protocol: proto,
				Timeout:  c.String("timeout"),
			})
		},
	}
}

func observeProcessSubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:  "process",
		Usage: "Observe a process across peers (name or argv-regex)",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Exact match against process basename"},
			&cli.StringFlag{Name: "pattern", Usage: "Regex against full argv"},
		}, common...),
		Action: func(c *cli.Context) error {
			return runFleetObserve(c, observe.SynthOptions{
				Kind:           "process",
				ProcessName:    c.String("name"),
				ProcessPattern: c.String("pattern"),
			})
		},
	}
}

func observeHTTPSubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:  "http",
		Usage: "Observe an HTTP endpoint across peers",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: "url", Usage: "Target URL (required)"},
			&cli.StringFlag{Name: "method", Usage: "HTTP method (default GET)"},
			&cli.StringFlag{Name: "timeout", Usage: "Request timeout (default 3s)"},
			&cli.IntFlag{Name: "expect-status", Usage: "Found=false if status != this"},
			&cli.StringSliceFlag{Name: "capture-header", Usage: "Response header(s) to expose in headers map"},
			&cli.BoolFlag{Name: "skip-tls-verify", Usage: "Disable cert verification (insecure)"},
			// Issue #18: --follow-redirects=0 lets operators see the
			// redirect itself ("is the HTTP→HTTPS redirect still up?").
			// Pointer-shape on SynthOptions distinguishes "unset" (default
			// 10) from explicit "0" (stop on first 3xx); urfave/cli's
			// IntFlag.IsSet() is the gate.
			&cli.IntFlag{Name: "follow-redirects", Usage: "Max redirects to follow (default 10; 0 = stop on first 3xx)"},
		}, common...),
		Action: func(c *cli.Context) error {
			opts := observe.SynthOptions{
				Kind:           "http",
				URL:            c.String("url"),
				Method:         c.String("method"),
				Timeout:        c.String("timeout"),
				ExpectStatus:   c.Int("expect-status"),
				CaptureHeaders: c.StringSlice("capture-header"),
				SkipTLSVerify:  c.Bool("skip-tls-verify"),
			}
			if c.IsSet("follow-redirects") {
				n := c.Int("follow-redirects")
				opts.FollowRedirects = &n
			}
			return runFleetObserve(c, opts)
		},
	}
}

func observeServiceSubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:  "service",
		Usage: "Observe an init-system service across peers (systemd / launchd)",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Service unit name (required)"},
			&cli.StringFlag{Name: "manager", Usage: "systemd | launchd | auto"},
		}, common...),
		Action: func(c *cli.Context) error {
			return runFleetObserve(c, observe.SynthOptions{
				Kind:           "service",
				ServiceName:    c.String("name"),
				ServiceManager: c.String("manager"),
			})
		},
	}
}

func observeCPUSubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:   "cpu",
		Usage:  "Observe CPU utilization + load averages across peers",
		Flags:  common,
		Action: func(c *cli.Context) error { return runFleetObserve(c, observe.SynthOptions{Kind: "cpu"}) },
	}
}

func observeMemorySubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:   "memory",
		Usage:  "Observe memory + swap across peers",
		Flags:  common,
		Action: func(c *cli.Context) error { return runFleetObserve(c, observe.SynthOptions{Kind: "memory"}) },
	}
}

func observeDiskSubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:  "disk",
		Usage: "Observe disk usage for a path across peers",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: "path", Usage: "Filesystem path (default /)"},
		}, common...),
		Action: func(c *cli.Context) error {
			return runFleetObserve(c, observe.SynthOptions{
				Kind:     "disk",
				DiskPath: c.String("path"),
			})
		},
	}
}

func observeGPUSubcommand(common []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:  "gpu",
		Usage: "Observe GPU utilization + memory across peers",
		Flags: append([]cli.Flag{
			&cli.IntFlag{Name: "index", Usage: "Specific GPU index", Value: -1},
		}, common...),
		Action: func(c *cli.Context) error {
			opts := observe.SynthOptions{Kind: "gpu"}
			if c.IsSet("index") {
				i := c.Int("index")
				opts.GPUIndex = &i
			}
			return runFleetObserve(c, opts)
		},
	}
}

// --- Common orchestration ---------------------------------------------------

func runFleetObserve(c *cli.Context, synth observe.SynthOptions) error {
	planBytes, err := observe.Synthesize(synth)
	if err != nil {
		return cli.Exit("fleet observe: "+err.Error(), 2)
	}

	peers, err := resolveObservePeers(c)
	if err != nil {
		return err
	}

	controllerID, err := fleet.EnsureControllerID()
	if err != nil {
		return fmt.Errorf("controller id: %w", err)
	}

	ctx, cancel := installSigintCancel(c.Context, c.App.Writer)
	defer cancel()

	outcomes, runErr := observe.Observe(ctx, observe.Options{
		Peers:        toObservePeers(peers),
		PlanBytes:    planBytes,
		ControllerID: controllerID,
		Parallel:     c.Int("parallel"),
	})
	if runErr != nil {
		return cli.Exit("fleet observe: "+runErr.Error(), 2)
	}

	switch c.String("format") {
	case "json":
		return renderObserveJSON(c.App.Writer, outcomes)
	case "table", "":
		return renderObserveTable(c.App.Writer, synth.Kind, outcomes)
	default:
		return cli.Exit("fleet observe: unknown --format "+c.String("format"), 2)
	}
}

func resolveObservePeers(c *cli.Context) ([]execPeerEntry, error) {
	return resolveAgentdPeers(c, "fleet observe")
}

func toObservePeers(in []execPeerEntry) []observe.Peer {
	out := make([]observe.Peer, len(in))
	for i, p := range in {
		out[i] = observe.Peer{Name: p.Peer.Name, Client: p.Client}
	}
	return out
}

// --- Output renderers -------------------------------------------------------

func renderObserveJSON(w io.Writer, outs []observe.PeerOutcome) error {
	enc := json.NewEncoder(w)
	for _, o := range outs {
		if err := enc.Encode(o); err != nil {
			return err
		}
	}
	return exitCodeFromOutcomes(outs)
}

func renderObserveTable(w io.Writer, kind string, outs []observe.PeerOutcome) error {
	sorted := make([]observe.PeerOutcome, len(outs))
	copy(sorted, outs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Peer < sorted[j].Peer })

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush() //nolint:errcheck
	// Per-kind columns. Falls back to a generic STATUS / FOUND / NOTE view.
	switch kind {
	case "port":
		fmt.Fprintln(tw, "PEER\tSTATUS\tFOUND\tOPEN\tPID\tLISTENER\tNOTE")
	case "process":
		fmt.Fprintln(tw, "PEER\tSTATUS\tFOUND\tRUNNING\tPID\tUSER\tNOTE")
	case "http":
		fmt.Fprintln(tw, "PEER\tSTATUS\tFOUND\tSTATUS_CODE\tLATENCY_MS\tREACHABLE\tNOTE")
	case "service":
		fmt.Fprintln(tw, "PEER\tSTATUS\tFOUND\tEXISTS\tACTIVE\tENABLED\tSUB_STATE\tNOTE")
	case "cpu":
		fmt.Fprintln(tw, "PEER\tSTATUS\tCORES\tUSAGE%\tLOAD_1M\tNOTE")
	case "memory":
		fmt.Fprintln(tw, "PEER\tSTATUS\tTOTAL_BYTES\tUSED_BYTES\tAVAILABLE_BYTES\tNOTE")
	case "disk":
		fmt.Fprintln(tw, "PEER\tSTATUS\tPATH\tTOTAL_BYTES\tUSED_BYTES\tFREE_BYTES\tNOTE")
	case "gpu":
		fmt.Fprintln(tw, "PEER\tSTATUS\tCOUNT\tVENDOR\tMAX_UTIL%\tMEM_USED_BYTES\tNOTE")
	default:
		fmt.Fprintln(tw, "PEER\tSTATUS\tFOUND\tNOTE")
	}
	for _, o := range sorted {
		note := o.Error
		if note == "" && o.Result != nil {
			if e, _ := o.Result["error"].(string); e != "" {
				note = e
			}
		}
		val := extractValueMap(o.Result)
		switch kind {
		case "port":
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o), boolStr(found(o.Result)),
				anyStr(val["open"]), anyStr(val["pid"]),
				anyStr(val["listener"]), trunc(note, 60))
		case "process":
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o), boolStr(found(o.Result)),
				anyStr(val["running"]), anyStr(val["pid"]),
				anyStr(val["user"]), trunc(note, 60))
		case "http":
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o), boolStr(found(o.Result)),
				anyStr(val["status_code"]), anyStr(val["latency_ms"]),
				anyStr(val["reachable"]), trunc(note, 60))
		case "service":
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o), boolStr(found(o.Result)),
				anyStr(val["exists"]), anyStr(val["active"]),
				anyStr(val["enabled"]), anyStr(val["sub_state"]), trunc(note, 60))
		case "cpu":
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o),
				anyStr(val["cores"]), anyStr(val["usage_pct"]),
				anyStr(val["load_1m"]), trunc(note, 60))
		case "memory":
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o),
				anyStr(val["total_bytes"]), anyStr(val["used_bytes"]),
				anyStr(val["available_bytes"]), trunc(note, 60))
		case "disk":
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o),
				anyStr(val["path"]), anyStr(val["total_bytes"]),
				anyStr(val["used_bytes"]), anyStr(val["free_bytes"]),
				trunc(note, 60))
		case "gpu":
			agg, _ := val["aggregate"].(map[string]any)
			maxUtil, memUsed := "", ""
			if agg != nil {
				maxUtil = anyStr(agg["max_utilization_pct"])
				memUsed = anyStr(agg["memory_used_bytes"])
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				o.Peer, status(o),
				anyStr(val["count"]), anyStr(val["vendor"]),
				maxUtil, memUsed, trunc(note, 60))
		default:
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", o.Peer, status(o), boolStr(found(o.Result)), trunc(note, 60))
		}
	}
	return exitCodeFromOutcomes(outs)
}

// --- helpers ---------------------------------------------------------------

func extractValueMap(result map[string]any) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	// step.completed event: result.data.value (current shape)
	if data, ok := result["data"].(map[string]any); ok {
		if v, ok := data["value"].(map[string]any); ok {
			return v
		}
	}
	// fallback: result.value (legacy / flat shape)
	if v, ok := result["value"].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

func found(result map[string]any) bool {
	if result == nil {
		return false
	}
	// step.completed event: result.data.found
	if data, ok := result["data"].(map[string]any); ok {
		b, _ := data["found"].(bool)
		return b
	}
	// fallback: result.found
	b, _ := result["found"].(bool)
	return b
}

func status(o observe.PeerOutcome) string {
	if o.Status == "" {
		return "error"
	}
	return o.Status
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func anyStr(v any) string {
	if v == nil {
		return "-"
	}
	switch x := v.(type) {
	case string:
		if x == "" {
			return "-"
		}
		return x
	case bool:
		return boolStr(x)
	case float64:
		// JSON numbers come in as float64; render integers without trailing zeros.
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', 2, 64)
	}
	return fmt.Sprintf("%v", v)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// exitCodeFromOutcomes returns a non-zero CLI exit if any peer failed
// (status != "success"). The error is empty so urfave's formatter
// doesn't print a redundant message — the table/JSON already shows
// the per-peer status.
func exitCodeFromOutcomes(outs []observe.PeerOutcome) error {
	for _, o := range outs {
		if o.Status != "success" {
			return cli.Exit("", 1)
		}
	}
	_ = context.Canceled // keep import set stable
	return nil
}

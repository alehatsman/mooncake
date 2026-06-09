package fleet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/observe"
)

func fleetTopCommand() *cli.Command {
	return &cli.Command{
		Name:  "top",
		Usage: "Live resource dashboard: CPU, memory, disk, GPU across fleet peers",
		Description: "Polls each selected peer every --interval and renders a " +
			"refreshing table of CPU%, memory%, disk%, GPU%, and load average. " +
			"^C exits cleanly; remote runs are unaffected.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers: repeat to UNION. Each value is a name, `key=value` filter, or `@k=v,k2=v2` AND-group. Default: all peers in peers.toml.",
			},
			&cli.DurationFlag{
				Name:  "interval",
				Usage: "Refresh interval",
				Value: 5 * time.Second,
			},
		},
		Action: fleetTopAction,
	}
}

func fleetTopAction(c *cli.Context) error {
	peers, err := resolveAgentdPeers(c, "fleet top")
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		return cli.Exit("fleet top: no agentd peers selected", 1)
	}

	controllerID, err := fleet.EnsureControllerID()
	if err != nil {
		return fmt.Errorf("controller id: %w", err)
	}

	obsPeers := toObservePeers(peers)
	interval := c.Duration("interval")
	useColor := fleet.ShouldColor(c.App.Writer, c.Bool("no-color"))
	parallel := c.Int("parallel")
	w := c.App.Writer
	isTTY := isTerminalWriter(w)

	ctx, cancel := installWatchSigint(c.Context, nil)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		rows := gatherTopRows(ctx, obsPeers, controllerID, parallel)
		renderTopFrame(w, rows, useColor, isTTY, interval)

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// topRow holds the merged per-peer metric snapshot for one refresh tick.
type topRow struct {
	peer    string
	cpuPct  *float64
	memPct  *float64
	diskPct *float64
	gpuPct  *float64
	load1m  *float64
	errMsg  string
}

// gatherTopRows fans out four observe kinds (cpu, memory, disk, gpu)
// concurrently across all peers and merges the results into one row per peer.
func gatherTopRows(ctx context.Context, peers []observe.Peer, controllerID string, parallel int) []topRow {
	const (
		kindCPU    = 0
		kindMemory = 1
		kindDisk   = 2
		kindGPU    = 3
	)
	kinds := []string{"cpu", "memory", "disk", "gpu"}
	allResults := make([][]observe.PeerOutcome, len(kinds))

	var wg sync.WaitGroup
	for i, kind := range kinds {
		i, kind := i, kind
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			planBytes, err := observe.Synthesize(observe.SynthOptions{Kind: kind})
			if err != nil {
				return
			}
			outs, err := observe.Observe(ctx, observe.Options{
				Peers:        peers,
				PlanBytes:    planBytes,
				ControllerID: controllerID,
				Parallel:     parallel,
			})
			if err == nil {
				allResults[i] = outs
			}
		}()
	}
	wg.Wait()

	rows := make([]topRow, len(peers))
	idx := make(map[string]int, len(peers))
	for i, p := range peers {
		rows[i].peer = p.Name
		idx[p.Name] = i
	}

	// CPU: usage_pct + load_1m
	for _, o := range allResults[kindCPU] {
		i, ok := idx[o.Peer]
		if !ok {
			continue
		}
		if o.Status != "success" {
			if rows[i].errMsg == "" {
				rows[i].errMsg = o.Error
			}
			continue
		}
		val := extractValueMap(o.Result)
		if v, ok2 := val["usage_pct"].(float64); ok2 {
			rows[i].cpuPct = ptr(v)
		}
		if v, ok2 := val["load_1m"].(float64); ok2 {
			rows[i].load1m = ptr(v)
		}
	}

	// Memory: used_bytes / total_bytes → pct
	for _, o := range allResults[kindMemory] {
		i, ok := idx[o.Peer]
		if !ok {
			continue
		}
		if o.Status != "success" {
			continue
		}
		val := extractValueMap(o.Result)
		total, _ := val["total_bytes"].(float64)
		used, _ := val["used_bytes"].(float64)
		if total > 0 {
			rows[i].memPct = ptr(used / total * 100)
		}
	}

	// Disk: used_bytes / total_bytes → pct (default path /)
	for _, o := range allResults[kindDisk] {
		i, ok := idx[o.Peer]
		if !ok {
			continue
		}
		if o.Status != "success" {
			continue
		}
		val := extractValueMap(o.Result)
		total, _ := val["total_bytes"].(float64)
		used, _ := val["used_bytes"].(float64)
		if total > 0 {
			rows[i].diskPct = ptr(used / total * 100)
		}
	}

	// GPU: aggregate.max_utilization_pct
	for _, o := range allResults[kindGPU] {
		i, ok := idx[o.Peer]
		if !ok {
			continue
		}
		if o.Status != "success" {
			continue
		}
		val := extractValueMap(o.Result)
		agg, _ := val["aggregate"].(map[string]any)
		if agg != nil {
			if v, ok2 := agg["max_utilization_pct"].(float64); ok2 {
				rows[i].gpuPct = ptr(v)
			}
		}
	}

	return rows
}

// renderTopFrame clears the terminal (when isTTY) and redraws the metrics table.
func renderTopFrame(w io.Writer, rows []topRow, useColor, isTTY bool, interval time.Duration) {
	if isTTY {
		fmt.Fprint(w, "\033[H\033[2J")
	}

	now := time.Now().Format("15:04:05")
	fmt.Fprintf(w, "fleet top  %d peer(s)  %s  (every %s)\n\n", len(rows), now, interval)

	buf := &bytes.Buffer{}
	tw := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PEER\tCPU%\tMEM%\tDISK%\tGPU%\tLOAD_1M\tSTATUS")

	for _, r := range rows {
		st := "ok"
		if r.errMsg != "" {
			st = "unreachable"
		} else if r.cpuPct == nil && r.memPct == nil {
			st = "partial"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.peer,
			topColorPct(r.cpuPct, 60, 85, useColor),
			topColorPct(r.memPct, 70, 85, useColor),
			topColorPct(r.diskPct, 70, 90, useColor),
			topColorPct(r.gpuPct, 60, 85, useColor),
			topFmtLoad(r.load1m),
			topFmtStatus(st, useColor),
		)
	}
	_ = tw.Flush()
	_, _ = w.Write(buf.Bytes())

	if useColor {
		fmt.Fprintf(w, "\n\x1b[2m^C to quit\x1b[0m\n")
	} else {
		fmt.Fprint(w, "\n^C to quit\n")
	}
}

func topColorPct(v *float64, warnAt, critAt float64, useColor bool) string {
	if v == nil {
		return "—"
	}
	s := fmt.Sprintf("%.1f%%", *v)
	if !useColor {
		return s
	}
	switch {
	case *v >= critAt:
		return "\x1b[31m" + s + "\x1b[0m"
	case *v >= warnAt:
		return "\x1b[33m" + s + "\x1b[0m"
	default:
		return "\x1b[32m" + s + "\x1b[0m"
	}
}

func topFmtLoad(v *float64) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f", *v)
}

func topFmtStatus(s string, useColor bool) string {
	if !useColor {
		return s
	}
	switch s {
	case "ok":
		return "\x1b[32m" + s + "\x1b[0m"
	case "unreachable":
		return "\x1b[31m" + s + "\x1b[0m"
	case "partial":
		return "\x1b[33m" + s + "\x1b[0m"
	}
	return s
}

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func ptr(v float64) *float64 { return &v }

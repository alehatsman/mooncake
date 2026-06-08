package fleet

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
)

func fleetStatusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show one-line-per-peer health for the configured fleet",
		Description: "Probes each peer's /v1/version, /v1/runs, and /v1/facts " +
			"in parallel and renders a STATE column (ok / running / failed / " +
			"unreachable) alongside OS, mooncake version, queue depth, and " +
			"the last run's outcome. --json switches to JSONL for scripts.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers: repeat to UNION. Each value is a name, `key=value` filter, or `@k=v,k2=v2` AND-group. Default: every peer in peers.toml.",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "Per-peer probe timeout (covers the three GETs)",
				Value: 3 * time.Second,
			},
		},
		Action: fleetStatusAction,
	}
}

// fleetStatusAction loads peers.toml, probes each selected peer in
// parallel, and renders either a table or JSONL. Exit code aggregates
// across peers: 0 if all ok-or-running, 1 if any failed, 2 if any
// unreachable. Matches `fleet apply`'s exit-code shape.
func fleetStatusAction(c *cli.Context) error {
	peersPath := c.String("peers-file")
	if peersPath == "" {
		var err error
		peersPath, err = fleet.DefaultPeersPath()
		if err != nil {
			return err
		}
	}
	cfgPeers, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return err
	}
	if len(cfgPeers.Peers) == 0 {
		return cli.Exit("fleet status: no peers configured. Edit "+peersPath+
			" or run `mooncake fleet bootstrap` / `mooncake fleet pair`.", 1)
	}

	// Fleet DX proposal-01: single unified --peer flag.
	peerFlag := c.StringSlice("peer")
	var osFor peerOSResolver
	if peerFlagsReferenceOSKey(peerFlag) {
		osFor = newPeerOSCache(c.Context, cfgPeers.Peers, c.App.Writer)
	}
	sel, err := resolvePeers(cfgPeers.Peers, peerFlag, osFor)
	if err != nil {
		return cli.Exit("fleet status: "+err.Error(), 2)
	}
	selected, unknown := sel.Matched, sel.UnknownNames
	if len(selected) == 0 {
		msg := "fleet status: --peer selected 0 of " + strconv.Itoa(len(cfgPeers.Peers)) + " peer(s)"
		if len(unknown) > 0 {
			msg += " (unknown: " + strings.Join(unknown, ", ") + ")"
		}
		return cli.Exit(msg, 1)
	}

	rows := fleet.ProbeAll(c.Context, selected, c.Duration("timeout"), c.Int("parallel"))

	w := c.App.Writer
	if c.Bool("json") {
		if err := emitJSON(w, rows); err != nil {
			return err
		}
	} else {
		useColor := fleet.ShouldColor(w, c.Bool("no-color"))
		renderStatusTable(w, rows, useColor)
		// Print the unknown-peer warning AFTER the table so the table
		// stays parseable for tools that aren't using --json.
		if len(unknown) > 0 {
			fmt.Fprintf(w, "\nwarning: unknown peer name(s) in --peers: %s\n",
				strings.Join(unknown, ", "))
		}
	}

	return statusExitCode(rows)
}

// statusExitCode maps the per-peer state mix to the same 0/1/2 code shape
// that `fleet apply` uses: unreachable dominates, then failed, then ok.
// Running peers count as healthy for exit-code purposes — they just
// haven't finished yet.
func statusExitCode(rows []fleet.Status) error {
	var unreachable, failed []string
	for _, r := range rows {
		switch r.State {
		case fleet.StateUnreachable:
			unreachable = append(unreachable, r.Name)
		case fleet.StateFailed:
			failed = append(failed, r.Name)
		}
	}
	switch {
	case len(unreachable) > 0:
		return cli.Exit("fleet status: unreachable peer(s): "+strings.Join(unreachable, ", "), 2)
	case len(failed) > 0:
		return cli.Exit("fleet status: failed peer(s): "+strings.Join(failed, ", "), 1)
	}
	return nil
}

// renderStatusTable lays out [host addr accessible running os mooncake
// queue last_run] via tabwriter and tail-summarises the counts. ANSI
// color on the ACCESSIBLE (green/red) and RUNNING (yellow/dim) cells
// when useColor is true.
func renderStatusTable(w io.Writer, rows []fleet.Status, useColor bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tADDR\tACCESSIBLE\tRUNNING\tOS\tMOONCAKE\tQUEUE\tLAST RUN")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Name,
			r.Addr,
			colorAccessible(r.Accessible, useColor),
			colorRunning(r.Running, useColor),
			dash(osColumn(r)),
			dash(r.Mooncake),
			queueColumn(r),
			lastRunColumn(r),
		)
	}
	_ = tw.Flush()

	// Tail summary: lead with accessibility, then call out subsets
	// (running + last-failed) and the unreachable count.
	var accessible, running, lastFailed, unreachable int
	for _, r := range rows {
		if !r.Accessible {
			unreachable++
			continue
		}
		accessible++
		if r.Running {
			running++
		}
		if r.State == fleet.StateFailed {
			lastFailed++
		}
	}
	parts := make([]string, 0, 4)
	parts = append(parts, fmt.Sprintf("%d/%d accessible", accessible, len(rows)))
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d running", running))
	}
	if lastFailed > 0 {
		parts = append(parts, fmt.Sprintf("%d last-failed", lastFailed))
	}
	if unreachable > 0 {
		parts = append(parts, fmt.Sprintf("%d unreachable", unreachable))
	}
	tickOrCross := "✔"
	if unreachable > 0 || lastFailed > 0 {
		tickOrCross = "✗"
	}
	fmt.Fprintln(w, tickOrCross+" "+strings.Join(parts, ", "))

	// Print probe errors as a footnote so the table itself stays narrow.
	// When we have a persisted last-seen timestamp for the unreachable
	// peer, append it so the user can tell "freshly broken" from
	// "never worked" without checking another tool.
	now := time.Now().UTC()
	for _, r := range rows {
		if !r.Accessible && r.Error != "" {
			fmt.Fprintf(w, "  %s: %s%s\n", r.Name, oneLineErr(r.Error), lastSeenSuffix(r.LastSeenAt, now))
		}
	}
}

// lastSeenSuffix returns " [last seen Xh ago]" when ts is non-zero, or
// " [no prior contact on this controller]" otherwise.
func lastSeenSuffix(ts, now time.Time) string {
	if ts.IsZero() {
		return " [no prior contact on this controller]"
	}
	d := now.Sub(ts)
	if d < 0 {
		d = 0
	}
	return " [last seen " + fleet.HumanDuration(d) + " ago]"
}

// colorAccessible renders the ACCESSIBLE cell as green "yes" / red "no".
func colorAccessible(ok bool, useColor bool) string {
	word := "no"
	if ok {
		word = "yes"
	}
	if !useColor {
		return word
	}
	const reset = "\x1b[0m"
	if ok {
		return "\x1b[32m" + word + reset
	}
	return "\x1b[31m" + word + reset
}

// colorRunning renders the RUNNING cell as yellow "yes" (run in flight)
// or dim "no" (idle, or peer unreachable so we can't tell).
func colorRunning(running bool, useColor bool) string {
	word := "no"
	if running {
		word = "yes"
	}
	if !useColor {
		return word
	}
	const reset = "\x1b[0m"
	if running {
		return "\x1b[33m" + word + reset
	}
	return "\x1b[2m" + word + reset
}

// osColumn collapses OS + arch into one cell. Arch is the parenthetical
// hint that's mostly useful for distinguishing aarch64 vs amd64 on macOS.
func osColumn(r fleet.Status) string {
	if r.OS == "" {
		return ""
	}
	if r.Arch == "" {
		return r.OS
	}
	return r.OS + " (" + r.Arch + ")"
}

func queueColumn(r fleet.Status) string {
	if r.QueueDepth < 0 {
		return "—"
	}
	if r.RunsRunning > 0 {
		// Show "running+queued" so the user sees there's both an
		// in-flight run AND a backlog.
		return fmt.Sprintf("%d (+%d running)", r.QueueDepth, r.RunsRunning)
	}
	return fmt.Sprintf("%d", r.QueueDepth)
}

func lastRunColumn(r fleet.Status) string {
	switch {
	case r.State == fleet.StateUnreachable:
		return "—"
	case r.LastRunStatus == "" && r.LastRunAge == "":
		return "—"
	case r.LastRunAge == "in flight":
		return "in flight"
	default:
		return r.LastRunStatus + " " + r.LastRunAge
	}
}

// dash returns "—" when s is empty, so the table doesn't have visually
// blank cells for fields the probe couldn't fill.
func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func oneLineErr(s string) string {
	out := strings.ReplaceAll(s, "\n", " ")
	out = strings.ReplaceAll(out, "\t", " ")
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return strings.TrimSpace(out)
}

// emitJSON writes one JSON object per row to w (JSONL). Designed for
// `mooncake fleet status --json | jq` and similar pipelines.
func emitJSON(w io.Writer, rows []fleet.Status) error {
	enc := json.NewEncoder(w)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("emit json: %w", err)
		}
	}
	return nil
}

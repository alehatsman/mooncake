package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fleetPsCommand registers `mooncake fleet ps` (spec-54). Fans out
// `GET /v1/runs?status=running` to every selected peer and renders a
// one-line-per-run table.
func fleetPsCommand() *cli.Command {
	return &cli.Command{
		Name:        "ps",
		Usage:       "List in-flight (or recent) runs across fleet peers",
		ArgsUsage:   "",
		Description: "Read-only fan-out across all selected peers. Default shows running runs; --all also shows recently terminal ones. Pair with jq via --json for scripting.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers: repeat to UNION. Each value is a name (`main_pc`), `key=value` filter (`tag=production`), or `@k=v,k2=v2` AND-group. Default (no --peer): every peer in peers.toml.",
			},
			&cli.StringFlag{Name: "peers-file", Usage: "Override the peers.toml path"},
			&cli.IntFlag{Name: "parallel", Usage: "Max peers in flight (0 = unbounded)", Value: 0},
			&cli.DurationFlag{Name: "timeout", Usage: "Per-peer probe timeout", Value: 3 * time.Second},
			&cli.StringFlag{
				Name:  "status",
				Usage: "Comma-separated statuses: running,queued,success,failed,interrupted (default: running unless --all)",
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Show recently-completed runs too (no status filter; per-peer limit applies)",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Per-peer record cap (active only with --all or non-running --status)",
				Value: 5,
			},
			&cli.StringFlag{
				Name:  "sort",
				Usage: "Sort order: peer (group by peer, default) | age (oldest first)",
				Value: "peer",
			},
			&cli.BoolFlag{Name: "short", Usage: "Truncate RUN_ID column to the last 10 chars"},
			&cli.BoolFlag{Name: "no-color", Usage: "Disable ANSI colors"},
			&cli.BoolFlag{Name: "json", Usage: "Emit one JSONL record per run instead of a table"},
		},
		Action: fleetPsAction,
	}
}

func fleetPsAction(c *cli.Context) error {
	peers, err := loadPsPeers(c)
	if err != nil {
		return err
	}

	statuses, perPeerLimit, err := derivePsStatusFilter(c.String("status"), c.Bool("all"), c.Int("limit"))
	if err != nil {
		return cli.Exit("fleet ps: "+err.Error(), 2)
	}

	results := fleet.FetchRunsAll(c.Context, peers, fleet.FetchOpts{
		Statuses:     statuses,
		LimitPerPeer: perPeerLimit,
		Timeout:      c.Duration("timeout"),
		MaxParallel:  c.Int("parallel"),
	})

	rows := buildPsRows(results, c.String("sort"))

	if c.Bool("json") {
		return renderPsJSON(c.App.Writer, results)
	}
	useColor := fleet.ShouldColor(c.App.Writer, c.Bool("no-color"))
	renderPsTable(c.App.Writer, rows, results, useColor, c.Bool("short"))

	if allUnreachable(results) {
		return cli.Exit("", 2)
	}
	return nil
}

// loadPsPeers handles the shared peer selection + filter pipeline. Same
// shape as fleet status / fleet apply / fleet exec.
func loadPsPeers(c *cli.Context) ([]fleet.Peer, error) {
	peersPath := c.String("peers-file")
	if peersPath == "" {
		p, err := fleet.DefaultPeersPath()
		if err != nil {
			return nil, err
		}
		peersPath = p
	}
	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return nil, err
	}
	if len(cfg.Peers) == 0 {
		return nil, cli.Exit("fleet ps: no peers configured. Run `mooncake fleet bootstrap` or edit "+peersPath, 1)
	}

	// Fleet DX proposal-01: single unified --peer flag. resolvePeers
	// builds the os-probe cache lazily (only when an `os=` predicate
	// is in scope) and returns matched + any bare-name typos.
	peerFlag := c.StringSlice("peer")
	var osFor peerOSResolver
	if peerFlagsReferenceOSKey(peerFlag) {
		osFor = newPeerOSCache(c.Context, cfg.Peers, c.App.Writer)
	}
	sel, err := resolvePeers(cfg.Peers, peerFlag, osFor)
	if err != nil {
		return nil, cli.Exit("fleet ps: "+err.Error(), 2)
	}
	if len(sel.UnknownNames) > 0 {
		fmt.Fprintf(c.App.Writer, "fleet ps: unknown peer name(s): %s\n", strings.Join(sel.UnknownNames, ", "))
	}
	selected := sel.Matched
	if len(selected) == 0 {
		return nil, cli.Exit("fleet ps: --peer selected 0 of "+strconv.Itoa(len(cfg.Peers))+" peer(s)", 1)
	}

	// Skip non-agentd peers; ps is a /v1/runs probe.
	out := make([]fleet.Peer, 0, len(selected))
	for _, p := range selected {
		if p.Transport == fleet.TransportAgentd {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, cli.Exit("fleet ps: no agentd peers after filters", 1)
	}
	return out, nil
}

// derivePsStatusFilter turns the user's flags into the FetchRuns inputs.
// Default: status=running, no per-peer limit (the daemon's default
// applies). --all: empty status list (all), per-peer limit honored.
// --status x,y: split + dedup; per-peer limit honored.
func derivePsStatusFilter(statusFlag string, all bool, limit int) ([]string, int, error) {
	switch {
	case statusFlag == "" && !all:
		return []string{"running"}, 0, nil
	case all && statusFlag == "":
		return nil, limit, nil // empty list = no filter
	}
	parts := strings.Split(statusFlag, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, raw := range parts {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		switch p {
		case "running", "queued", "success", "failed", "interrupted":
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		default:
			return nil, 0, fmt.Errorf("unknown status %q (valid: running, queued, success, failed, interrupted)", p)
		}
	}
	if len(out) == 0 {
		return []string{"running"}, 0, nil
	}
	return out, limit, nil
}

// psRow is one rendered run row, carrying the peer name + the run
// record plus a precomputed `pickTimestamp` so sort.Slice doesn't
// re-parse strings on every comparison.
type psRow struct {
	Peer string
	Run  transport.RunRecord
	When time.Time // pickTimestamp result; used by sort
}

func buildPsRows(results []fleet.PeerRuns, sortMode string) []psRow {
	var rows []psRow
	for _, pr := range results {
		for _, r := range pr.Runs {
			rows = append(rows, psRow{
				Peer: pr.Name,
				Run:  r,
				When: psPickTimestamp(r),
			})
		}
	}
	switch sortMode {
	case "age":
		// Oldest first — "which peer is taking so long?" use case.
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].When.Before(rows[j].When)
		})
	default:
		// Group by peer (preserve input order), then newest-first within peer.
		// SliceStable + a single key derives both at once: equal peer name
		// → newer-first.
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].Peer != rows[j].Peer {
				return false // preserve input order across peers
			}
			return rows[i].When.After(rows[j].When)
		})
	}
	return rows
}

// psPickTimestamp returns the most relevant timestamp on a run for sort
// + AGE rendering. Mirrors humanRunAge's recipe: finished > started >
// queued. Falls back to zero if all three are empty.
func psPickTimestamp(r transport.RunRecord) time.Time {
	for _, s := range []string{r.FinishedAt, r.StartedAt, r.QueuedAt} {
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// renderPsTable writes the tabwriter table + a summary line + per-peer
// error footnotes.
func renderPsTable(w io.Writer, rows []psRow, results []fleet.PeerRuns, useColor, shortIDs bool) {
	reachable, unreachable := splitReachable(results)

	if len(rows) == 0 {
		fmt.Fprintf(w, "no in-flight runs (%d peer(s) accessible, %d unreachable)\n",
			len(reachable), len(unreachable))
		writeFootnotes(w, results)
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tRUN_ID\tSTATUS\tAGE\tPLAN")
	now := time.Now().UTC()
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			row.Peer,
			runIDDisplay(row.Run.ID, shortIDs),
			colorStatus(row.Run.Status, useColor),
			ageDisplay(row.When, now),
			planDisplay(row.Run.PlanPath),
		)
	}
	_ = tw.Flush()

	fmt.Fprintln(w, psSummary(rows, results))
	writeFootnotes(w, results)
}

// renderPsJSON emits JSONL: one object per run, plus one sentinel
// object per unreachable peer with `"error"` set.
func renderPsJSON(w io.Writer, results []fleet.PeerRuns) error {
	enc := json.NewEncoder(w)
	for _, pr := range results {
		if pr.Error != nil {
			_ = enc.Encode(map[string]any{"peer": pr.Name, "error": pr.Error.Error()})
			continue
		}
		for _, r := range pr.Runs {
			_ = enc.Encode(map[string]any{"peer": pr.Name, "run": r})
		}
	}
	return nil
}

func splitReachable(results []fleet.PeerRuns) (reachable, unreachable []fleet.PeerRuns) {
	for _, pr := range results {
		if pr.Error != nil {
			unreachable = append(unreachable, pr)
		} else {
			reachable = append(reachable, pr)
		}
	}
	return
}

func allUnreachable(results []fleet.PeerRuns) bool {
	for _, pr := range results {
		if pr.Error == nil {
			return false
		}
	}
	return len(results) > 0
}

func writeFootnotes(w io.Writer, results []fleet.PeerRuns) {
	for _, pr := range results {
		if pr.Error != nil {
			fmt.Fprintf(w, "  %s: %s\n", pr.Name, oneLineErr(pr.Error.Error()))
		}
	}
}

// psSummary builds the trailing one-line summary banner. Splits running
// from terminal so the operator sees at-a-glance counts.
func psSummary(rows []psRow, results []fleet.PeerRuns) string {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Run.Status]++
	}
	reachable, unreachable := splitReachable(results)
	verdict := "✔"
	if len(unreachable) > 0 {
		verdict = "✗"
	}
	parts := []string{fmt.Sprintf("%d run(s) across %d peer(s)", len(rows), len(reachable))}
	if counts["running"] > 0 {
		parts = append(parts, fmt.Sprintf("%d running", counts["running"]))
	}
	if counts["queued"] > 0 {
		parts = append(parts, fmt.Sprintf("%d queued", counts["queued"]))
	}
	if counts["success"] > 0 {
		parts = append(parts, fmt.Sprintf("%d success", counts["success"]))
	}
	if counts["failed"] > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", counts["failed"]))
	}
	if counts["interrupted"] > 0 {
		parts = append(parts, fmt.Sprintf("%d interrupted", counts["interrupted"]))
	}
	if len(unreachable) > 0 {
		parts = append(parts, fmt.Sprintf("%d unreachable", len(unreachable)))
	}
	return verdict + " " + strings.Join(parts, ", ")
}

// runIDDisplay returns the full ID or the last-10-char tail when short=true.
func runIDDisplay(id string, short bool) string {
	if !short || len(id) <= 10 {
		return id
	}
	return "…" + id[len(id)-10:]
}

// ageDisplay renders the AGE column. Empty when no timestamp is available.
func ageDisplay(when, now time.Time) string {
	if when.IsZero() {
		return "—"
	}
	d := now.Sub(when)
	if d < 0 {
		d = 0
	}
	return fleet.HumanDuration(d)
}

// planDisplay trims the synced-root prefix when it shows up in a path,
// then middle-truncates to keep the column narrow on a TTY.
func planDisplay(p string) string {
	const maxWidth = 60
	if i := strings.Index(p, "/synced/"); i >= 0 {
		// Strip up to and including the next "/" after "/synced/<scope>".
		// Typical shape: /var/lib/.../synced/<controller>/<dirhash>/path.yml
		tail := p[i+len("/synced/"):]
		if slash := strings.IndexByte(tail, '/'); slash >= 0 {
			tail = tail[slash+1:]
		}
		if slash := strings.IndexByte(tail, '/'); slash >= 0 {
			tail = tail[slash+1:]
		}
		p = tail
	}
	if len(p) <= maxWidth {
		return p
	}
	half := (maxWidth - 1) / 2
	return p[:half] + "…" + p[len(p)-half:]
}

// colorStatus paints the STATUS column. Mirrors fleet status's color
// choices; intentionally muted.
func colorStatus(s string, useColor bool) string {
	if !useColor {
		return s
	}
	const (
		reset  = "\x1b[0m"
		yellow = "\x1b[33m"
		green  = "\x1b[32m"
		red    = "\x1b[31m"
		dim    = "\x1b[2m"
	)
	switch s {
	case "running":
		return yellow + s + reset
	case "success":
		return green + s + reset
	case "failed", "interrupted":
		return red + s + reset
	case "queued":
		return dim + s + reset
	default:
		return s
	}
}

// Context is referenced for the future watch-mode follow-up. Kept here
// so the import isn't dropped on the next refactor.
var _ context.Context

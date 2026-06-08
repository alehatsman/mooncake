package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fleetFactsCommand defines `mooncake fleet facts`. Three surfaces per
// spec-46:
//
//	mooncake fleet facts <peer>            # full facts JSON for peer
//	mooncake fleet facts <peer> <key>      # one fact (dot-path)
//	mooncake fleet facts --query <key>     # one fact across ALL peers, tabular
//
// Dot-path resolution uses the same convention as `mooncake query` (dots in
// the user-facing key map to underscores in the flat fact map — see
// cmd/query.go:queryMap).
func fleetFactsCommand() *cli.Command {
	return &cli.Command{
		Name:      "facts",
		Usage:     "Read facts from a peer, or compare a key across the fleet",
		ArgsUsage: "<peer> [key] | --query <key>",
		Description: "Without --query: fetch one peer's facts and either " +
			"pretty-print the full map (1 arg) or extract a single dot-path " +
			"key (2 args). With --query: fan out across every peer and " +
			"render a side-by-side table — the fast way to spot divergence.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "query",
				Usage: "Dot-path key (e.g. go_version, os.distribution) to compare across all peers",
			},
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers (only with --query): repeat to UNION. Each value is a name, `key=value` filter, or `@k=v,k2=v2` AND-group. Default: every peer in peers.toml.",
			},
		},
		Action: fleetFactsAction,
	}
}

func fleetFactsAction(c *cli.Context) error {
	query := c.String("query")
	args := c.Args().Slice()

	peersPath := c.String("peers-file")
	if peersPath == "" {
		var err error
		peersPath, err = fleet.DefaultPeersPath()
		if err != nil {
			return err
		}
	}
	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return err
	}
	if len(cfg.Peers) == 0 {
		return cli.Exit("fleet facts: no peers configured. Run `mooncake fleet bootstrap` or edit "+peersPath, 1)
	}

	w := c.App.Writer

	// --query: fan-out across all/selected peers, render table.
	if query != "" {
		if len(args) > 0 {
			return cli.Exit("fleet facts: --query takes no positional args", 2)
		}
		// Fleet DX proposal-01: single unified --peer flag.
		peerFlag := c.StringSlice("peer")
		var osFor peerOSResolver
		if peerFlagsReferenceOSKey(peerFlag) {
			osFor = newPeerOSCache(c.Context, cfg.Peers, c.App.Writer)
		}
		sel, err := resolvePeers(cfg.Peers, peerFlag, osFor)
		if err != nil {
			return cli.Exit("fleet facts: "+err.Error(), 2)
		}
		if len(sel.Matched) == 0 {
			return cli.Exit("fleet facts: --peer selected 0 peers", 1)
		}
		if len(sel.UnknownNames) > 0 {
			fmt.Fprintln(c.App.ErrWriter, "warning: unknown peer name(s): "+strings.Join(sel.UnknownNames, ", "))
		}
		return renderFactsQuery(c.Context, w, sel.Matched, query)
	}

	// Single-peer mode: 1 or 2 args.
	if len(args) < 1 || len(args) > 2 {
		return cli.Exit("fleet facts: expected <peer> [key], or --query <key>", 2)
	}
	peerName := args[0]
	peer, ok := findPeer(cfg.Peers, peerName)
	if !ok {
		return cli.Exit(fmt.Sprintf("fleet facts: peer %q not found in %s", peerName, peersPath), 1)
	}
	if peer.Transport != fleet.TransportAgentd {
		return cli.Exit(fmt.Sprintf("fleet facts: peer %q transport %q is not agentd", peer.Name, peer.Transport), 1)
	}

	facts, err := fetchFacts(c.Context, peer, defaultFactsTimeout)
	if err != nil {
		return cli.Exit("fleet facts: "+oneLineErr(err.Error()), 1)
	}

	if len(args) == 2 {
		key := args[1]
		val, ok := lookupFact(facts, key)
		if !ok {
			return cli.Exit(fmt.Sprintf("fleet facts: key %q not found on peer %s", key, peer.Name), 1)
		}
		fmt.Fprintln(w, val)
		return nil
	}

	// Full pretty-print. Stable key order (alphabetical) so reruns diff
	// cleanly — json.MarshalIndent on a map is already deterministic since
	// Go 1.12, but the explicit sort makes intent obvious.
	return printFactsJSON(w, facts)
}

// defaultFactsTimeout is the per-peer probe timeout for fan-out facts
// reads. Matches the inspect.go status-probe timeout — facts payloads are
// typically a few KB, so 3s is generous.
const defaultFactsTimeout = 3 * time.Second

// fetchFacts is a single-peer wrapper around transport.Client.GetFacts.
// Distinct from inspect.Probe (which combines version + runs + facts) —
// `fleet facts` only needs the facts call, and a single timeout per call
// gives a tighter error message.
func fetchFacts(ctx context.Context, peer fleet.Peer, timeout time.Duration) (map[string]any, error) {
	client := transport.New(peer.Name, peer.Addr, peer.Token)
	pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.GetFacts(pCtx)
}

// lookupFact resolves a dot-path key against a flat facts map, matching
// the convention in cmd/query.go:queryMap — dots map to underscores. Returns
// the value as a display string (empty string if value is empty/false/nil).
// Reuses the same rendering switch as queryMap so the output shapes match
// between `mooncake query` and `mooncake fleet facts`.
func lookupFact(m map[string]any, key string) (string, bool) {
	norm := strings.ReplaceAll(key, ".", "_")
	val, ok := m[norm]
	if !ok || val == nil || val == "" || val == false {
		return "", false
	}
	return formatFactValue(val), true
}

// formatFactValue is the same renderer queryMap uses. Kept package-private
// here; if a third caller appears, lift to a shared file. Two callers
// (queryMap + this) is still under the rule-of-three threshold.
func formatFactValue(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case bool:
		return "true"
	case int, int64, float64:
		return fmt.Sprintf("%v", v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// printFactsJSON renders the full map as indented JSON. The daemon emits
// the map order non-deterministically; encoding/json sorts map keys when
// marshaling, so this output is stable across calls.
func printFactsJSON(w io.Writer, facts map[string]any) error {
	b, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return fmt.Errorf("encode facts: %w", err)
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// renderFactsQuery does the parallel facts probe across peers and renders
// a `text/tabwriter` table. Unreachable peers show "—" in the value column
// (matching the inspect-table convention) so divergence is the visual
// signal, not blank cells.
//
// Exit code is 0 even with partial reachability — the user asked for a
// comparison, partial results are still informative. An unreachable peer
// is surfaced as a non-empty value but visually distinct.
func renderFactsQuery(ctx context.Context, w io.Writer, peers []fleet.Peer, key string) error {
	type row struct {
		name  string
		value string
	}
	rows := make([]row, len(peers))
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Add(1)
		go func(i int, p fleet.Peer) {
			defer wg.Done()
			rows[i].name = p.Name
			if p.Transport != fleet.TransportAgentd {
				rows[i].value = "—  (transport " + string(p.Transport) + ")"
				return
			}
			facts, err := fetchFacts(ctx, p, defaultFactsTimeout)
			if err != nil {
				rows[i].value = "—  (" + oneLineErr(err.Error()) + ")"
				return
			}
			v, ok := lookupFact(facts, key)
			if !ok {
				rows[i].value = "—"
				return
			}
			rows[i].value = v
		}(i, p)
	}
	wg.Wait()

	// Stable order: alphabetical by peer name. Matches inspect.go's table
	// preservation rules even though the underlying probe is parallel.
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "HOST\t%s\n", strings.ToUpper(strings.ReplaceAll(key, ".", "_")))
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\n", r.name, r.value)
	}
	return tw.Flush()
}

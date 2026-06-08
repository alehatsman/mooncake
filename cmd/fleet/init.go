// Package fleet implements the `mooncake fleet` subcommand tree. This
// file owns `fleet init` (spec-45 PR13) — the interactive wrapper that
// adds discovered candidates to peers.toml in one command instead of
// hand-editing TOML. The discovery pieces — mDNS browse, SSH-config
// parser, peers.toml loader, agentd probe — live in internal/fleet
// (shipped in spec-45 PRs 12 / 13.5 / 13.6).
package fleet

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/discovery"
)

// fleetInitCommand registers `mooncake fleet init`. Runs discovery,
// prints a candidate table, and walks the operator through adding
// new ones to peers.toml. Closes the last item of the original
// personal-fleet 14-PR plan; pure UX polish on top of pieces that
// already exist.
func fleetInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Interactive setup — discover peers and add them to peers.toml",
		Description: "Aggregates discovery sources (mDNS, ~/.ssh/config, " +
			"existing peers.toml) and prompts the operator to add each " +
			"new candidate to peers.toml. The hard step — moving each " +
			"peer's bearer token to the controller — is a manual paste " +
			"in v1; --from-bootstrap / --ssh-fetch are spec-47 follow-ups.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "no-mdns", Usage: "Skip the mDNS browse source"},
			&cli.BoolFlag{Name: "no-ssh-config", Usage: "Skip the ~/.ssh/config source"},
			&cli.DurationFlag{Name: "mdns-timeout", Usage: "Cap mDNS browse wall-clock", Value: 3 * time.Second},
			&cli.BoolFlag{
				Name: "dry-run",
				Usage: "Print the candidate list + planned actions without writing peers.toml " +
					"and without prompting. Useful for seeing what `fleet init` would do.",
			},
			&cli.BoolFlag{
				Name: "accept-all",
				Usage: "Skip prompts. Every agentd candidate that already has a token (e.g. " +
					"already in peers.toml) is left alone. Candidates that need a token error " +
					"out — pair them via `fleet bootstrap` instead.",
			},
		},
		Action: fleetInitAction,
	}
}

// fleetInitAction is the entry point — one big function on purpose so
// the prompt control flow is straightforward to read top-to-bottom.
func fleetInitAction(c *cli.Context) error {
	peersPath := c.String("peers-file")
	if peersPath == "" {
		p, err := fleet.DefaultPeersPath()
		if err != nil {
			return fmt.Errorf("resolve peers.toml path: %w", err)
		}
		peersPath = p
	}

	w := c.App.Writer
	fmt.Fprintln(w, "discovering candidates…")

	mdns := !c.Bool("no-mdns")
	mdnsPtr := &mdns
	sshPath := ""
	if c.Bool("no-ssh-config") {
		sshPath = "-" // discovery.Aggregate's sentinel for "skip"
	}
	cands, err := discovery.Aggregate(c.Context, discovery.Options{
		PeersPath:     peersPath,
		SSHConfigPath: sshPath,
		MDNS:          mdnsPtr,
		MDNSTimeout:   c.Duration("mdns-timeout"),
	})
	if err != nil {
		return cli.Exit("fleet init: "+err.Error(), 1)
	}

	fmt.Fprintln(w)
	renderCandidateTable(w, cands)

	plan := buildInitPlan(cands)
	fmt.Fprintln(w)
	renderInitPlan(w, plan)

	if c.Bool("dry-run") {
		return nil
	}
	if len(plan.NewCandidates) == 0 && len(plan.SSHCandidates) == 0 {
		fmt.Fprintln(w, "fleet init: nothing to add. Run `mooncake fleet status` to verify the existing fleet.")
		return nil
	}

	added, skipped, err := runInitPrompts(c.Context, w, os.Stdin, plan, peersPath, c.Bool("accept-all"))
	if err != nil {
		return cli.Exit("fleet init: "+err.Error(), 1)
	}

	fmt.Fprintf(w, "\nfleet init: wrote %s (%d added, %d skipped).\n", peersPath, added, skipped)
	if added > 0 {
		fmt.Fprintln(w, "✓ fleet ready: `mooncake fleet status` to verify.")
	}
	return nil
}

// initPlan separates discovery output into the categories the prompt
// flow treats differently:
//
//   - ConfiguredAgentd: already in peers.toml with a working probe.
//     No prompt; print only.
//   - ConfiguredAgentdBroken: in peers.toml but probe failed. Print +
//     reference doctor.
//   - NewCandidates: mDNS responders or peers.toml entries that aren't
//     usable (probe failed) — operator prompted to add or skip.
//   - SSHCandidates: ssh-config-only entries — operator prompted to
//     trigger fleet bootstrap, deferred.
type initPlan struct {
	ConfiguredAgentd       []discovery.Candidate
	ConfiguredAgentdBroken []discovery.Candidate
	NewCandidates          []discovery.Candidate
	SSHCandidates          []discovery.Candidate
}

func buildInitPlan(cands []discovery.Candidate) initPlan {
	var plan initPlan
	for _, c := range cands {
		switch {
		case c.HasSource(discovery.SourcePeersTOML) && c.AgentdOK:
			plan.ConfiguredAgentd = append(plan.ConfiguredAgentd, c)
		case c.HasSource(discovery.SourcePeersTOML):
			plan.ConfiguredAgentdBroken = append(plan.ConfiguredAgentdBroken, c)
		case c.HasSource(discovery.SourceMDNS):
			plan.NewCandidates = append(plan.NewCandidates, c)
		case c.HasSource(discovery.SourceSSHConfig):
			plan.SSHCandidates = append(plan.SSHCandidates, c)
		}
	}
	return plan
}

// renderCandidateTable lays out the discovered candidates in a single
// tabwriter table, one row per Candidate, mirroring the spec's "source
// / name / addr / status" shape.
func renderCandidateTable(w io.Writer, cands []discovery.Candidate) {
	if len(cands) == 0 {
		fmt.Fprintln(w, "  (no candidates found)")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  SOURCE\tNAME\tADDR\tSTATUS")
	for _, c := range cands {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
			strings.Join(c.Sources, "+"),
			c.Name,
			dashIfEmpty(c.Addr),
			candidateStatus(c),
		)
	}
	_ = tw.Flush()
}

// renderInitPlan prints the operator-facing summary of what fleet init
// is about to do. Distinct from the candidate table so the operator
// can see at a glance "3 new boxes to add, 1 ssh-only to bootstrap".
func renderInitPlan(w io.Writer, plan initPlan) {
	fmt.Fprintf(w, "  %d already configured (agentd OK)\n", len(plan.ConfiguredAgentd))
	if n := len(plan.ConfiguredAgentdBroken); n > 0 {
		fmt.Fprintf(w, "  %d configured but unreachable (try `mooncake fleet doctor <peer>`)\n", n)
	}
	fmt.Fprintf(w, "  %d new candidate(s) to add\n", len(plan.NewCandidates))
	if n := len(plan.SSHCandidates); n > 0 {
		fmt.Fprintf(w, "  %d ssh-config candidate(s) — defer to `mooncake fleet bootstrap`\n", n)
	}
}

// runInitPrompts iterates the plan's NewCandidates, prompting the
// operator per-candidate for name / tags / token. Skipped candidates
// don't write a peers.toml row. SSHCandidates are surfaced one-by-one
// with a "y/N: bootstrap now?" prompt that prints the suggested
// command rather than running it (v1 keeps `fleet bootstrap` invocation
// out of `fleet init`'s scope — spec-47 owns that integration).
func runInitPrompts(ctx context.Context, w io.Writer, in io.Reader, plan initPlan, peersPath string, acceptAll bool) (added, skipped int, err error) {
	reader := bufio.NewReader(in)

	if len(plan.NewCandidates) > 0 {
		fmt.Fprintln(w)
		ok, err := promptYesNo(w, reader, "Add new peers to "+peersPath+"?", true)
		if err != nil {
			return 0, 0, err
		}
		if !ok {
			fmt.Fprintln(w, "fleet init: skipping new-candidate prompts.")
		} else {
			for _, cand := range plan.NewCandidates {
				if ctx.Err() != nil {
					return added, skipped, ctx.Err()
				}
				addedNow, err := promptOneCandidate(w, reader, cand, peersPath, acceptAll)
				if err != nil {
					return added, skipped, err
				}
				if addedNow {
					added++
				} else {
					skipped++
				}
			}
		}
	}

	for _, cand := range plan.SSHCandidates {
		fmt.Fprintln(w)
		if acceptAll {
			fmt.Fprintf(w, "fleet init: skipping ssh-config candidate %q (--accept-all + no token source).\n", cand.Name)
			skipped++
			continue
		}
		bootstrap, err := promptYesNo(w, reader, fmt.Sprintf("? %s — bootstrap over SSH now (runs `mooncake fleet bootstrap`)?", cand.Name), false)
		if err != nil {
			return added, skipped, err
		}
		if !bootstrap {
			sshTarget := sshTargetFor(cand)
			fmt.Fprintf(w, "   skipped; run `mooncake fleet bootstrap %s` later.\n", sshTarget)
			skipped++
			continue
		}
		fmt.Fprintf(w, "   `fleet bootstrap` integration with `fleet init` is a spec-47 follow-up.\n")
		fmt.Fprintf(w, "   run `mooncake fleet bootstrap %s` to add this peer.\n", sshTargetFor(cand))
		skipped++
	}

	return added, skipped, nil
}

// promptOneCandidate walks the operator through one new candidate's
// name + tags + token, then calls fleet.Upsert. Returns whether a peer
// was actually written. Empty token = skip.
func promptOneCandidate(w io.Writer, reader *bufio.Reader, cand discovery.Candidate, peersPath string, acceptAll bool) (bool, error) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  candidate %s (%s, %s):\n", cand.Name, cand.Addr, strings.Join(cand.Sources, "+"))

	if acceptAll {
		// No token source available without prompting — surface the
		// expected failure instead of silently skipping. Spec §"Non-
		// interactive mode": --accept-all errors on mDNS-only candidates.
		return false, fmt.Errorf("--accept-all: candidate %q has no token source (run `mooncake fleet bootstrap` to pair it)", cand.Name)
	}

	name, err := promptDefault(w, reader, fmt.Sprintf("? %s — peer name in peers.toml", cand.Name), cand.Name)
	if err != nil {
		return false, err
	}
	tags, err := promptDefault(w, reader, fmt.Sprintf("? %s — tags (comma-separated, optional)", cand.Name), "")
	if err != nil {
		return false, err
	}
	tagList := parseTagInput(tags)
	token, err := promptSecret(w, reader, fmt.Sprintf("? %s — paste bearer token (cat /etc/mooncake/agentd.token on %s)", cand.Name, cand.Name))
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(token) == "" {
		fmt.Fprintf(w, "   skipped %s — no token provided.\n", cand.Name)
		return false, nil
	}

	peer := fleet.Peer{
		Name:      name,
		Addr:      cand.Addr,
		Transport: fleet.TransportAgentd,
		Token:     strings.TrimSpace(token),
		Tags:      tagList,
	}
	addedRow, diff, err := fleet.Upsert(peersPath, peer)
	if err != nil {
		return false, err
	}
	switch {
	case addedRow:
		fmt.Fprintf(w, "   ✓ added %s\n", peer.Name)
	case len(diff) > 0:
		fmt.Fprintf(w, "   ✓ updated %s: %s\n", peer.Name, strings.Join(diff, "; "))
	default:
		fmt.Fprintf(w, "   = %s already configured (no changes)\n", peer.Name)
	}
	return true, nil
}

// promptDefault prints `prompt (default: <def>): ` and returns the
// user's response. Empty input returns def.
func promptDefault(w io.Writer, reader *bufio.Reader, prompt, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(w, "%s (default %s): ", prompt, def)
	} else {
		fmt.Fprintf(w, "%s: ", prompt)
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return def, nil
	}
	return line, nil
}

// promptSecret reads a value without echoing it to the terminal.
// When stdin is a TTY, the terminal is put into raw mode for the read.
// When stdin is not a TTY (pipe, CI), falls back to reading from reader
// and emits a warning — the caller decides whether to refuse instead.
func promptSecret(w io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(w, prompt+": ")
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fmt.Fprintln(os.Stderr, "(stdin is not a TTY; token may be visible to the terminal)")
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	raw, err := term.ReadPassword(fd)
	fmt.Fprintln(w)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

// promptYesNo asks a Y/n (or y/N) question and returns the boolean.
// defaultYes controls the casing of the prompt and the empty-input
// default.
func promptYesNo(w io.Writer, reader *bufio.Reader, question string, defaultYes bool) (bool, error) {
	suffix := " [y/N] "
	if defaultYes {
		suffix = " [Y/n] "
	}
	fmt.Fprint(w, question+suffix)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	switch line {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return defaultYes, nil
	}
}

// parseTagInput splits "a, b ,c" into ["a","b","c"], trims each entry,
// drops empties, dedupes.
func parseTagInput(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// sshTargetFor builds the `user@host:port` string `fleet bootstrap`
// expects, falling back to bare host when User / Port are empty.
func sshTargetFor(c discovery.Candidate) string {
	host := c.Addr
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		host = c.Name
	}
	target := host
	if c.SSHUser != "" {
		target = c.SSHUser + "@" + host
	}
	if c.SSHPort != 0 && c.SSHPort != 22 {
		target = fmt.Sprintf("%s -p %d", target, c.SSHPort)
	}
	return target
}

// candidateStatus is the right-most column of the candidate table.
// Keeps the wide-format spec rendering close to what `fleet status`
// users already recognize.
func candidateStatus(c discovery.Candidate) string {
	switch {
	case c.AgentdOK:
		v := c.AgentdVersion
		if v == "" {
			v = "agentd up"
		} else {
			v = "agentd up (mooncake " + v + ")"
		}
		return "✓ " + v
	case c.HasSource(discovery.SourcePeersTOML):
		if c.ProbeError != "" {
			return "✗ " + c.ProbeError
		}
		return "✗ agentd unreachable"
	case c.HasSource(discovery.SourceMDNS):
		return "mdns responder; needs token"
	case c.HasSource(discovery.SourceSSHConfig):
		return "ssh-only, not bootstrapped"
	default:
		return "—"
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

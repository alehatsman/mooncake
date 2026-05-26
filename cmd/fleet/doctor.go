package fleet

// fleet_doctor.go implements `mooncake fleet doctor <peer>` — an
// opinionated probe ladder that turns "peer is unreachable" into a
// curated walk-through of where the failure actually lives. Each rung
// runs only if the prior one succeeded, so the user sees the first
// thing that's wrong rather than three pages of cascading errors.
//
// Hints live next to the rung that emits them; this is on purpose. The
// table-driven layout (probe → hint) keeps "what went wrong" and "what
// to try next" within eye-distance, so future contributors don't drift
// them apart.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
)

func fleetDoctorCommand() *cli.Command {
	return &cli.Command{
		Name:      "doctor",
		Usage:     "Run an opinionated probe ladder against one peer",
		ArgsUsage: "<peer-name>",
		Description: "Walks Resolve → TCP → HTTP (anonymous) → Auth → " +
			"Facts, stopping at the first failing rung and printing a " +
			"curated hint. Designed for the moment `fleet status` " +
			"reports unreachable and you want to know *why*.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "Per-rung deadline (DNS, TCP, each HTTP call)",
				Value: 3 * time.Second,
			},
			&cli.BoolFlag{
				Name:  "no-color",
				Usage: "Disable ANSI colors (also honors NO_COLOR)",
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Emit one JSON object summarising all rungs; skip the table",
			},
		},
		Action: fleetDoctorAction,
	}
}

func fleetDoctorAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return cli.Exit("fleet doctor: exactly one <peer-name> argument required", 2)
	}
	target := c.Args().First()

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
	var peer *fleet.Peer
	for i := range cfgPeers.Peers {
		if cfgPeers.Peers[i].Name == target {
			peer = &cfgPeers.Peers[i]
			break
		}
	}
	if peer == nil {
		return cli.Exit(fmt.Sprintf("fleet doctor: peer %q not found in %s", target, peersPath), 1)
	}

	w := c.App.Writer
	report := runDoctorLadder(c.Context, *peer, c.Duration("timeout"))

	if c.Bool("json") {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		useColor := fleet.ShouldColor(w, c.Bool("no-color"))
		renderDoctorReport(w, report, useColor)
	}

	if !report.Healthy {
		return cli.Exit("", 1)
	}
	return nil
}

// rungName identifies one step in the diagnostic ladder. Stringly-typed
// because it ends up in the JSON output and gets read by humans.
type rungName string

const (
	rungResolve rungName = "resolve"
	rungTCP     rungName = "tcp"
	rungHTTP    rungName = "http"
	rungAuth    rungName = "auth"
	rungFacts   rungName = "facts"
	// rungSSHDiag is added at the end when peer.SSH is set AND a prior
	// rung failed — it shells in over SSH and runs the OS-appropriate
	// "is mooncake-agentd healthy" probe. Strictly supplementary; the
	// ladder remains useful without an SSH fallback configured.
	rungSSHDiag rungName = "ssh-diag"
)

// rungResult is one row of the doctor table. Detail is the short
// machine-flavoured "what we saw" string; Hint is the human-flavoured
// "what to try next" text shown only on failure.
type rungResult struct {
	Name    rungName      `json:"name"`
	OK      bool          `json:"ok"`
	Skipped bool          `json:"skipped,omitempty"`
	Detail  string        `json:"detail,omitempty"`
	Hint    string        `json:"hint,omitempty"`
	Took    time.Duration `json:"took_ns,omitempty"`
}

// doctorReport is the whole output of one `fleet doctor <peer>` run.
type doctorReport struct {
	Peer    string       `json:"peer"`
	Addr    string       `json:"addr"`
	Healthy bool         `json:"healthy"`
	Rungs   []rungResult `json:"rungs"`
}

// runDoctorLadder is the meat. Each rung gets a fresh sub-context
// bounded by perRung, so a hung HTTP read doesn't eat the budget for
// later (skipped, but still recorded) rungs.
func runDoctorLadder(ctx context.Context, peer fleet.Peer, perRung time.Duration) doctorReport {
	if perRung <= 0 {
		perRung = 3 * time.Second
	}
	r := doctorReport{Peer: peer.Name, Addr: peer.Addr}

	// Tracks whether *any* rung failed; gates the SSH supplementary rung.
	anyFailed := func() bool {
		for _, rg := range r.Rungs {
			if !rg.OK && !rg.Skipped {
				return true
			}
		}
		return false
	}

	// finalize: when the (agentd) ladder is done — successful or not —
	// optionally tack on an SSH-fallback rung if the user configured one
	// AND something earlier failed. Reaches into the OS to ask "what
	// does systemctl/launchctl/Task Scheduler think?"
	finalize := func() doctorReport {
		if peer.SSH != "" && anyFailed() {
			r.Rungs = append(r.Rungs, sshDiagRung(ctx, peer, perRung))
		}
		return r
	}

	host, _, err := net.SplitHostPort(peer.Addr)
	if err != nil {
		r.Rungs = append(r.Rungs, rungResult{
			Name:   rungResolve,
			OK:     false,
			Detail: "invalid addr in peers.toml: " + err.Error(),
			Hint:   "Fix the [[peers]] entry — addr must be host:port (e.g. 192.168.1.10:7878).",
		})
		return finalize()
	}

	// === Rung 1: Resolve ===
	res := resolveRung(ctx, host, perRung)
	r.Rungs = append(r.Rungs, res)
	if !res.OK {
		return finalize()
	}

	// === Rung 2: TCP ===
	tcp := tcpRung(ctx, peer.Addr, perRung)
	r.Rungs = append(r.Rungs, tcp)
	if !tcp.OK {
		r.Rungs = append(r.Rungs, skipped(rungHTTP), skipped(rungAuth), skipped(rungFacts))
		return finalize()
	}

	// === Rung 3: HTTP (anonymous) ===
	httpR := httpAnonRung(ctx, peer.Addr, perRung)
	r.Rungs = append(r.Rungs, httpR)
	if !httpR.OK {
		r.Rungs = append(r.Rungs, skipped(rungAuth), skipped(rungFacts))
		return finalize()
	}

	// === Rung 4: Auth ===
	auth := authRung(ctx, peer.Addr, peer.Token, perRung)
	r.Rungs = append(r.Rungs, auth)
	if !auth.OK {
		r.Rungs = append(r.Rungs, skipped(rungFacts))
		return finalize()
	}

	// === Rung 5: Facts ===
	facts := factsRung(ctx, peer.Addr, peer.Token, perRung)
	r.Rungs = append(r.Rungs, facts)
	if !facts.OK {
		return finalize()
	}

	r.Healthy = true
	return finalize()
}

// sshDiagRung wraps fleet.RunSSHDiagnostic into a doctor rung. Always
// stamped as OK=true when the SSH call itself succeeded — the captured
// stdout *is* the diagnostic, whether the remote command exited 0 or 3
// (systemctl's "inactive" code). Hard SSH failures become OK=false with
// the dial error in Detail.
func sshDiagRung(ctx context.Context, peer fleet.Peer, perRung time.Duration) rungResult {
	start := time.Now()
	diag, err := fleet.RunSSHDiagnostic(ctx, peer, perRung)
	took := time.Since(start)
	if err != nil {
		return rungResult{
			Name:   rungSSHDiag,
			OK:     false,
			Detail: err.Error(),
			Hint:   "Could not reach the peer over SSH for fallback diagnostics. Confirm `ssh " + peer.SSH + "` works from this controller.",
			Took:   took,
		}
	}
	detail := "os=" + string(diag.OS) + " exit=" + fmt.Sprintf("%d", diag.ExitCode)
	if diag.Stdout != "" {
		// Indent stdout one extra level so it visually nests under the rung line.
		detail += "\n" + indentLines(diag.Stdout, "    ")
	}
	if diag.Stderr != "" {
		detail += "\n  stderr:\n" + indentLines(diag.Stderr, "    ")
	}
	return rungResult{Name: rungSSHDiag, OK: true, Detail: detail, Took: took}
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = prefix + ln
	}
	return strings.Join(lines, "\n")
}

func skipped(n rungName) rungResult {
	return rungResult{Name: n, Skipped: true, Detail: "(skipped because a prior rung failed)"}
}

func resolveRung(ctx context.Context, host string, timeout time.Duration) rungResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// If the host part is already an IP literal, skip the resolver call
	// (it'd succeed trivially but the rung label would be misleading).
	if ip := net.ParseIP(host); ip != nil {
		return rungResult{Name: rungResolve, OK: true, Detail: ip.String() + " (literal)", Took: time.Since(start)}
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	took := time.Since(start)
	if err != nil {
		return rungResult{
			Name:   rungResolve,
			OK:     false,
			Detail: err.Error(),
			Hint: "DNS lookup failed. Check /etc/hosts, the resolver, " +
				"or replace the hostname with an IP literal in peers.toml.",
			Took: took,
		}
	}
	if len(ips) == 0 {
		return rungResult{
			Name:   rungResolve,
			OK:     false,
			Detail: "no addresses returned",
			Hint:   "The resolver succeeded but returned zero addresses — likely a stale entry.",
			Took:   took,
		}
	}
	names := make([]string, 0, len(ips))
	for _, ip := range ips {
		names = append(names, ip.IP.String())
	}
	return rungResult{Name: rungResolve, OK: true, Detail: strings.Join(names, ", "), Took: took}
}

func tcpRung(ctx context.Context, addr string, timeout time.Duration) rungResult {
	start := time.Now()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	took := time.Since(start)
	if err == nil {
		_ = conn.Close()
		return rungResult{Name: rungTCP, OK: true, Detail: fmt.Sprintf("connected in %s", roundMillis(took)), Took: took}
	}
	return rungResult{
		Name:   rungTCP,
		OK:     false,
		Detail: err.Error(),
		Hint:   tcpHintFor(err),
		Took:   took,
	}
}

// tcpHintFor matches errors to the most actionable next step. Covers
// the four shapes we actually see in practice; falls back to the
// generic "check the obvious" message for anything else.
func tcpHintFor(err error) string {
	if isErrno(err, "connection refused") {
		return "TCP RST from the peer: the host is up, but nothing is listening on this port.\n" +
			"  - On Linux: `systemctl status mooncake-agentd` or `ss -tlnp | grep <port>`\n" +
			"  - On Windows: check the `Mooncake-Agentd-Autostart` scheduled task " +
			"(`Get-ScheduledTaskInfo Mooncake-Agentd-Autostart`)."
	}
	if isErrno(err, "no route to host") || isErrno(err, "host is unreachable") {
		return "No route to the host. Likely the box is off the network or on a different subnet."
	}
	if isErrno(err, "network is unreachable") {
		return "Network unreachable from this controller — check the local routing table / VPN."
	}
	if isTimeout(err) {
		return "TCP handshake timed out. Two common causes:\n" +
			"  - host is powered off / sleeping (try `ping` if ICMP isn't blocked)\n" +
			"  - firewall is dropping SYN packets (filtered, not closed).\n" +
			"  For a WSL peer: `wsl --exec true` from the Windows host wakes a sleeping VM."
	}
	return "Generic dial failure. Confirm the addr in peers.toml matches what the peer binds."
}

func httpAnonRung(ctx context.Context, addr string, timeout time.Duration) rungResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/v1/version", nil)
	resp, err := client.Do(req)
	took := time.Since(start)
	if err != nil {
		return rungResult{
			Name:   rungHTTP,
			OK:     false,
			Detail: err.Error(),
			Hint: "TCP connected but HTTP didn't complete. Likely agentd " +
				"crashed mid-request or is hung. Check journalctl / agentd.stderr.log.",
			Took: took,
		}
	}
	_ = resp.Body.Close()
	// 401 here means "daemon is alive, middleware is working" — that's
	// a green light for this rung. Any 2xx is also fine (means auth
	// isn't required, e.g. unix-socket mode). 5xx means agentd is
	// broken at the app layer; we treat that as a failure with hint.
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return rungResult{Name: rungHTTP, OK: true,
			Detail: fmt.Sprintf("HTTP %d (auth required — expected)", resp.StatusCode), Took: took}
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return rungResult{Name: rungHTTP, OK: true,
			Detail: fmt.Sprintf("HTTP %d (auth not required)", resp.StatusCode), Took: took}
	case resp.StatusCode >= 500:
		return rungResult{Name: rungHTTP, OK: false,
			Detail: fmt.Sprintf("HTTP %d (daemon error)", resp.StatusCode),
			Hint:   "Agentd answered with 5xx — middleware reached, app broke. Check the daemon's stderr.",
			Took:   took}
	default:
		return rungResult{Name: rungHTTP, OK: false,
			Detail: fmt.Sprintf("HTTP %d (unexpected)", resp.StatusCode),
			Hint:   "Unexpected status from /v1/version. Confirm the addr really points at an agentd instance.",
			Took:   took}
	}
}

func authRung(ctx context.Context, addr, token string, timeout time.Duration) rungResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if token == "" {
		return rungResult{Name: rungAuth, OK: false,
			Detail: "peers.toml has no token for this peer",
			Hint:   "Set `token = \"...\"` on the [[peers]] entry. Fetch it with `mooncake fleet pair` or read it on the peer at /etc/mooncake/agentd.token."}
	}
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/v1/version", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	took := time.Since(start)
	if err != nil {
		return rungResult{Name: rungAuth, OK: false, Detail: err.Error(),
			Hint: "Could not complete the authenticated request — same shape as the anonymous rung failing.",
			Took: took}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var v struct {
			Version   string `json:"version"`
			UptimeSec int64  `json:"uptime_sec"`
		}
		_ = json.Unmarshal(body, &v)
		detail := "HTTP 200"
		if v.Version != "" {
			detail += " mooncake=" + v.Version
		}
		if v.UptimeSec > 0 {
			detail += " uptime=" + (time.Duration(v.UptimeSec) * time.Second).String()
		}
		return rungResult{Name: rungAuth, OK: true, Detail: detail, Took: took}
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return rungResult{Name: rungAuth, OK: false,
			Detail: fmt.Sprintf("HTTP %d (token rejected)", resp.StatusCode),
			Hint: "Agentd rejected the configured token. Re-pair with `mooncake fleet pair`, " +
				"or read the peer's current token from /etc/mooncake/agentd.token and update peers.toml.",
			Took: took}
	}
	return rungResult{Name: rungAuth, OK: false,
		Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
		Hint:   "Unexpected status with auth — the daemon is alive but not happy. Check its logs.",
		Took:   took}
}

func factsRung(ctx context.Context, addr, token string, timeout time.Duration) rungResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/v1/facts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	took := time.Since(start)
	if err != nil {
		return rungResult{Name: rungFacts, OK: false, Detail: err.Error(),
			Hint: "Facts endpoint is failing where version succeeded — likely a facts-collection bug.",
			Took: took}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return rungResult{Name: rungFacts, OK: false,
			Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
			Hint:   "Facts endpoint should return 200; non-200 here suggests partial daemon dysfunction.",
			Took:   took}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var f map[string]any
	if err := json.Unmarshal(body, &f); err != nil {
		return rungResult{Name: rungFacts, OK: false,
			Detail: "200 but body undecodable: " + err.Error(),
			Hint:   "Body shape changed — check controller/agentd version skew.",
			Took:   took}
	}
	os, _ := f["os"].(string)
	arch, _ := f["arch"].(string)
	bits := []string{"HTTP 200"}
	if os != "" {
		bits = append(bits, "os="+os)
	}
	if arch != "" {
		bits = append(bits, "arch="+arch)
	}
	return rungResult{Name: rungFacts, OK: true, Detail: strings.Join(bits, " "), Took: took}
}

// renderDoctorReport prints the human-readable ladder. Each rung is one
// line, then hints are indented two spaces below their rung.
func renderDoctorReport(w io.Writer, r doctorReport, useColor bool) {
	fmt.Fprintf(w, "%s → %s\n", r.Peer, r.Addr)
	for _, rg := range r.Rungs {
		fmt.Fprintf(w, "%s %-7s %s\n", rungMarker(rg, useColor), rg.Name, rg.Detail)
		if rg.Hint != "" {
			for _, ln := range strings.Split(rg.Hint, "\n") {
				fmt.Fprintf(w, "  hint:   %s\n", strings.TrimSpace(ln))
			}
		}
	}
	fmt.Fprintln(w)
	if r.Healthy {
		fmt.Fprintln(w, "→ healthy")
	} else {
		fmt.Fprintln(w, "→ unhealthy (see first failing rung above)")
	}
}

func rungMarker(rg rungResult, useColor bool) string {
	switch {
	case rg.Skipped:
		if useColor {
			return "\x1b[2m·\x1b[0m"
		}
		return "·"
	case rg.OK:
		if useColor {
			return "\x1b[32m✓\x1b[0m"
		}
		return "✓"
	default:
		if useColor {
			return "\x1b[31m✗\x1b[0m"
		}
		return "✗"
	}
}

func roundMillis(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

// isErrno matches a network error against a substring of its message.
// Coarse on purpose — the precise syscall constants differ slightly
// across linux/darwin/windows but the kernel-localised message strings
// are stable enough for hint dispatch.
func isErrno(err error, needle string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), needle)
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return false
}

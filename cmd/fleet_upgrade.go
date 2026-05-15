package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// supportedSelfUpgradeOS lists peer OSes that v1 of self-upgrade can
// drive. Linux peers use syscall.Exec for in-place process-image
// replacement; Windows peers use the MoveFile-the-running-exe trick
// plus a scheduled-task Stop/Start cycle driven by a detached helper.
// macOS works in principle (shares the Unix syscall.Exec path) but
// isn't smoke-tested, so the CLI defaults to refusing it until
// someone trials it with --include-os darwin.
var supportedSelfUpgradeOS = map[string]struct{}{
	"linux":   {},
	"windows": {},
}

func fleetUpgradeCommand() *cli.Command {
	return &cli.Command{
		Name:      "upgrade",
		Usage:     "Push a new agentd binary to fleet peers and trigger re-exec",
		ArgsUsage: "",
		Description: "Streams the local mooncake binary to each selected peer's " +
			"/v1/self/binary, verifies it on the peer side (--version sanity " +
			"check), then asks /v1/self/replace to swap the on-disk binary and " +
			"restart. The controller polls /v1/version until the daemon comes " +
			"back (PID changes on Windows scheduled-task restart, or uptime " +
			"resets on Linux syscall.Exec).\n\n" +
			"Linux and Windows peers are supported by default. macOS peers are " +
			"skipped (untested) — override with --include-os darwin if you've " +
			"validated the peer's behaviour yourself.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "peers",
				Usage: "Comma-separated peer names to upgrade (default: all agentd peers)",
			},
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
			&cli.StringFlag{
				Name: "binary",
				Usage: "Path to the binary to push. Default: this process's executable. " +
					"On a mismatched controller/peer arch you must pre-build with " +
					"GOOS=... GOARCH=... go build and point --binary at the result.",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "Per-peer deadline covering upload + replace + restart-watch",
				Value: 90 * time.Second,
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Replace even when the peer has runs in flight (will be killed)",
			},
			&cli.StringSliceFlag{
				Name: "include-os",
				Usage: "Allow upgrading peers running an OS outside the v1-supported " +
					"set (default: linux). Repeat to allow multiple, e.g. --include-os darwin",
			},
			&cli.BoolFlag{
				Name:  "no-color",
				Usage: "Disable ANSI colors (also honors NO_COLOR env)",
			},
		},
		Action: fleetUpgradeAction,
	}
}

// fleetUpgradeAction runs the per-peer upgrade pipeline serially. v1
// keeps it sequential rather than parallel — the failure mode of "new
// binary doesn't start" is bad enough that walking through one peer at
// a time lets the operator notice and abort before clobbering the rest.
func fleetUpgradeAction(c *cli.Context) error {
	binPath := c.String("binary")
	if binPath == "" {
		var err error
		binPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("locate self binary: %w (pass --binary)", err)
		}
	}
	binSHA, binSize, err := sha256AndSize(binPath)
	if err != nil {
		return fmt.Errorf("hash %s: %w", binPath, err)
	}

	peersPath := c.String("peers-file")
	if peersPath == "" {
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
		return cli.Exit("fleet upgrade: no peers configured", 1)
	}

	selected, unknown := selectPeers(cfgPeers.Peers, c.String("peers"))
	if len(selected) == 0 {
		msg := "fleet upgrade: no peers matched filter " + c.String("peers")
		if len(unknown) > 0 {
			msg += " (unknown: " + strings.Join(unknown, ", ") + ")"
		}
		return cli.Exit(msg, 1)
	}

	allowedOS := map[string]struct{}{}
	for k := range supportedSelfUpgradeOS {
		allowedOS[k] = struct{}{}
	}
	for _, extra := range c.StringSlice("include-os") {
		allowedOS[strings.ToLower(strings.TrimSpace(extra))] = struct{}{}
	}

	w := c.App.Writer
	fmt.Fprintf(w, "fleet upgrade: %s (%d bytes, sha256 %s…) → %d peer(s)\n",
		binPath, binSize, binSHA[:12], len(selected))
	if len(unknown) > 0 {
		fmt.Fprintf(w, "  warning: unknown peer name(s) in --peers: %s\n", strings.Join(unknown, ", "))
	}

	timeout := c.Duration("timeout")
	force := c.Bool("force")

	type result struct {
		name  string
		state string // ok | skipped | failed
		msg   string
	}
	results := make([]result, 0, len(selected))

	for _, p := range selected {
		if p.Transport != fleet.TransportAgentd {
			results = append(results, result{p.Name, "skipped",
				fmt.Sprintf("transport %q not supported (agentd only)", p.Transport)})
			fmt.Fprintf(w, "[%s] skipped: %s\n", p.Name, results[len(results)-1].msg)
			continue
		}

		peerCtx, cancel := context.WithTimeout(c.Context, timeout)
		err := upgradeOnePeer(peerCtx, w, p, binPath, binSHA, allowedOS, force)
		cancel()

		switch {
		case errors.Is(err, errUpgradeSkipped):
			results = append(results, result{p.Name, "skipped", err.Error()})
		case err != nil:
			results = append(results, result{p.Name, "failed", err.Error()})
			fmt.Fprintf(w, "[%s] ✗ %s\n", p.Name, err.Error())
		default:
			results = append(results, result{p.Name, "ok", ""})
		}
	}

	// Tail summary.
	var ok, skipped, failed int
	for _, r := range results {
		switch r.state {
		case "ok":
			ok++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
	}
	tick := "✔"
	if failed > 0 {
		tick = "✗"
	}
	fmt.Fprintf(w, "%s fleet upgrade: %d ok, %d skipped, %d failed (of %d)\n",
		tick, ok, skipped, failed, len(selected))

	if failed > 0 {
		return cli.Exit("", 1)
	}
	return nil
}

// errUpgradeSkipped sentinels a non-error reason to walk past a peer
// (wrong OS, in-flight runs without --force, etc.). Surfaced in the
// summary but doesn't drive the exit code red.
var errUpgradeSkipped = errors.New("skipped")

// upgradeOnePeer runs the four-step pipeline against a single peer:
// version probe, upload, replace, post-restart probe.
func upgradeOnePeer(ctx context.Context, w io.Writer, p fleet.Peer, binPath, binSHA string, allowedOS map[string]struct{}, force bool) error {
	client := transport.New(p.Name, p.Addr, p.Token)

	fmt.Fprintf(w, "[%s] probe…\n", p.Name)
	before, err := client.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("version probe: %w", err)
	}

	peerOS := strings.ToLower(before.OS)
	if peerOS == "" {
		return errors.New("peer didn't report OS (pre-spec-50 daemon?); upgrade refused — verify the target manually")
	}
	if _, ok := allowedOS[peerOS]; !ok {
		// Skipped, not failed — the user knows; we tell them how to override.
		msg := fmt.Sprintf("OS %q not in supported set (linux, windows). Pass --include-os %s to override.", peerOS, peerOS)
		fmt.Fprintf(w, "[%s] skipped: %s\n", p.Name, msg)
		return fmt.Errorf("%w: %s", errUpgradeSkipped, msg)
	}

	// Arch isn't on the version response yet — we trust the controller
	// to have built a binary that matches the peer. The daemon
	// re-verifies on its side before staging, so a mismatch errors out
	// with a clear "arch_mismatch" code from the agentd.
	peerArch := runtime.GOARCH
	if a := strings.TrimSpace(os.Getenv("MOONCAKE_FLEET_UPGRADE_ARCH")); a != "" {
		peerArch = a
	}

	fmt.Fprintf(w, "[%s] upload %s/%s …\n", p.Name, peerOS, peerArch)
	staged, err := client.UploadBinary(ctx, binPath, binSHA, peerOS, peerArch)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	fmt.Fprintf(w, "[%s] staged at %s (sha %s…)\n", p.Name, staged.StagedPath, staged.SHA256[:12])

	fmt.Fprintf(w, "[%s] replace…\n", p.Name)
	rep, err := client.SelfReplace(ctx, staged.StagedPath, staged.SHA256, force)
	if err != nil {
		return fmt.Errorf("replace: %w", err)
	}
	fmt.Fprintf(w, "[%s] daemon old_pid=%d, waiting for restart…\n", p.Name, rep.OldPID)

	if err := waitForRestart(ctx, client, before.DaemonPID, before.UptimeSec); err != nil {
		return fmt.Errorf("await restart: %w", err)
	}

	after, err := client.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("post-restart probe: %w", err)
	}
	fmt.Fprintf(w, "[%s] ✓ upgrade complete (pid %d → %d, version %s → %s)\n",
		p.Name, before.DaemonPID, after.DaemonPID, before.Version, after.Version)
	return nil
}

// waitForRestart polls /v1/version every 500ms until the peer's daemon
// confirms a restart. Two valid restart signals:
//
//   - PID changed — what happens on the Windows scheduled-task-restart
//     path (the new process inherits the task slot, not the PID).
//   - uptime_sec dropped below the pre-restart value — what happens on
//     the Linux syscall.Exec path. exec replaces the process image but
//     keeps the same PID, so the only reliable signal is the daemon's
//     startedAt resetting (uptime collapses from N seconds to ~0).
//
// Returns ctx.Err() if the deadline expires before either signal fires.
func waitForRestart(ctx context.Context, client *transport.Client, oldPID int, oldUptimeSec int64) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			v, err := client.GetVersion(ctx)
			if err != nil {
				// Transient — daemon is still restarting. Keep
				// polling until ctx expires.
				continue
			}
			if v.DaemonPID != 0 && v.DaemonPID != oldPID {
				return nil
			}
			if v.UptimeSec < oldUptimeSec {
				return nil
			}
		}
	}
}

func sha256AndSize(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), st.Size(), nil
}


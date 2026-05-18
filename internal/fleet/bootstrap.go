package fleet

// bootstrap.go is the controller-side SSH entry point for
// `mooncake fleet bootstrap user@host`. It owns the SSH-shaped
// concerns:
//
//	1. transport.Connect + auth
//	2. Session.DetectPlatform (Linux/macOS/Windows fan-out)
//	8. peers.toml upsert (caller's responsibility — cmd/fleet.go)
//
// Steps 3-7 (existing-install probe through token read) live in
// internal/fleet/install, called via install.Bootstrap. Spec-70
// extracted that orchestration so `mooncake agentd bootstrap`
// (cmd/agentd.go) reuses it against a LocalExecutor.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alehatsman/mooncake/internal/fleet/install"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// BootstrapOptions configures one fleet-bootstrap invocation.
type BootstrapOptions struct {
	Target transport.SSHTarget // user@host (port via SSHTarget.Port)
	Name   string              // peer name written to peers.toml
	Tags   []string            // peer tags written to peers.toml
	Port   int                 // agentd TCP bind port on remote (default 7878)

	// SSHOpts is forwarded to transport.Connect. Most callers leave this
	// zero — defaults (ssh-agent + ~/.ssh/known_hosts) match the system
	// ssh client's behavior.
	SSHOpts transport.ConnectOptions

	// LocalBinary is the path to the mooncake binary to SCP to the
	// remote. Caller passes os.Executable() when empty isn't desired.
	LocalBinary string

	// ControllerVersion is the version string the controller's binary
	// reports. Compared against the remote's `mooncake --version` to
	// decide whether step 3 short-circuits. Empty disables the
	// short-circuit; the bootstrap always reinstalls in that case.
	ControllerVersion string

	// Upgrade, when true, replaces a different-version install on the
	// remote without prompting. Without it, version mismatch errors.
	Upgrade bool

	// AsUser, when true (Linux only), installs the agentd as a user-scope
	// systemd unit running as the SSH user. Binary → ~/.local/bin/mooncake,
	// unit → ~/.config/systemd/user/mooncake-agentd.service, token →
	// ~/.config/mooncake/agentd.token. No sudo, `loginctl enable-linger`
	// called so the unit survives logout. Ignored on macOS / Windows
	// (those platforms already pick a per-user shape elsewhere).
	AsUser bool

	// Writer receives one-line progress updates. nil = silent.
	Writer io.Writer
}

// BootstrapResult summarizes what bootstrap did.
type BootstrapResult struct {
	Peer      Peer   // entry to write to peers.toml
	OS        string // "linux" or "darwin"
	Arch      string // "amd64" or "arm64"
	AlreadyOK bool   // true if step 3 short-circuited (same version, active)
}

// Bootstrap installs mooncake on a remote box reachable via SSH and
// registers it as an agentd peer. Idempotent: re-running against an
// already-bootstrapped peer with the same version is a no-op (returns
// AlreadyOK=true).
//
// Caller is responsible for writing peers.toml from BootstrapResult.Peer.
func Bootstrap(ctx context.Context, opts BootstrapOptions) (BootstrapResult, error) {
	if opts.Target.Host == "" {
		return BootstrapResult{}, errors.New("bootstrap: target host is empty")
	}
	if opts.Name == "" {
		opts.Name = opts.Target.Host
		// Be friendlier than `192.168.1.68` — collapse dots so the name
		// is a valid peers.toml identifier and looks OK as a log prefix.
		opts.Name = strings.ReplaceAll(opts.Name, ".", "-")
	}
	if opts.Port == 0 {
		opts.Port = 7878
	}
	if opts.LocalBinary == "" {
		return BootstrapResult{}, errors.New("bootstrap: LocalBinary is empty")
	}
	w := opts.Writer
	if w == nil {
		w = io.Discard
	}
	report := func(format string, args ...any) {
		fmt.Fprintf(w, "[%s] ", opts.Name)
		fmt.Fprintf(w, format+"\n", args...)
	}

	// === Step 1: Connect + auth ===
	report("connecting via ssh")
	sess, err := transport.Connect(ctx, opts.Target, opts.SSHOpts)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 1 (connect): %w", err)
	}
	defer func() { _ = sess.Close() }()

	// === Step 2: Detect platform ===
	osName, arch, err := sess.DetectPlatform(ctx)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 2 (detect platform): %w", err)
	}
	report("detected %s %s", osName, arch)

	// Windows takes a distinct path: no sudo, paths under %LOCALAPPDATA%,
	// scheduled task instead of systemd unit, firewall handled inline.
	// The shape diverges enough from Linux/macOS that sharing the helper
	// functions would add more branching than clarity. spec-56 has the
	// rationale; the Windows flow is encapsulated in bootstrapWindows
	// below so future spec changes touch one self-contained block.
	if osName == "windows" {
		return bootstrapWindows(ctx, sess, opts, arch, report)
	}

	// Linux/macOS path: delegate steps 3-7 to install.Bootstrap.
	exec := install.NewSSHExecutor(sess)
	res, err := install.Bootstrap(ctx, exec, install.BootstrapOptions{
		OS:                osName,
		Arch:              arch,
		Port:              opts.Port,
		AsUser:            opts.AsUser,
		LocalBinary:       opts.LocalBinary,
		ControllerVersion: opts.ControllerVersion,
		Upgrade:           opts.Upgrade,
		ReachableHost:     opts.Target.Host,
		Writer:            w,
		LogPrefix:         opts.Name,
	})
	if err != nil {
		return BootstrapResult{}, err
	}

	// === Step 8 lite: build the Peer entry the caller writes to
	// peers.toml. install.Bootstrap intentionally doesn't know about
	// peers — this is the controller-side concern.
	peer := Peer{
		Name:      opts.Name,
		Addr:      fmt.Sprintf("%s:%d", opts.Target.Host, opts.Port),
		Transport: TransportAgentd,
		Tags:      opts.Tags,
		Token:     res.Token,
	}
	return BootstrapResult{
		Peer:      peer,
		OS:        res.OS,
		Arch:      res.Arch,
		AlreadyOK: res.AlreadyOK,
	}, nil
}

// EnsureLocalBinaryPath returns the absolute path of the mooncake
// binary that's currently running, suitable as
// BootstrapOptions.LocalBinary. Falls back to a clearer error than
// os.Executable's defaults.
func EnsureLocalBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate own binary: %w", err)
	}
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("stat own binary at %s: %w", exe, err)
	}
	return exe, nil
}

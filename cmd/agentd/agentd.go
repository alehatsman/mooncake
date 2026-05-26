package agentd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"strings"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/agentd"
	"github.com/alehatsman/mooncake/internal/fleet/install"
)

// Version is the binary's controller/agentd version stamped at build
// time by goreleaser linker flags into cmd/mooncake.go's `version` and
// copied here from main() before Command() or RunsCommand() is invoked.
// Read by `agentd run` (daemon startup) and by `agentd bootstrap`
// (local-install rendering).
var Version = "dev"

// Command is the parent for the daemon-side verbs.
//
// Spec-70 split the leaf `mooncake agentd` into two subcommands:
//
//   - `mooncake agentd run` — the daemon (was the old leaf Action).
//   - `mooncake agentd bootstrap` — install agentd locally without
//     the SSH-to-self detour.
//
// Pre-spec-70 unit files that say `ExecStart=...mooncake agentd ...`
// will fail to start on a post-split binary — re-bootstrap them
// (`mooncake fleet bootstrap` or `mooncake agentd bootstrap`) to
// re-render the unit with the new `agentd run` form.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "agentd",
		Usage: "Mooncake host daemon (run / install) (experimental)",
		Description: "Subcommands:\n" +
			" run        — run the daemon (foreground; what systemd / launchd execs).\n" +
			" bootstrap  — install agentd on this machine and print the bearer token.",
		Subcommands: []*cli.Command{
			agentdRunCommand(),
			agentdBootstrapCommand(),
		},
	}
}

func agentdRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Run the agentd daemon in the foreground",
		Description: "Started by the systemd unit / launchd plist / Task Scheduler entry " +
			"that `agentd bootstrap` or `fleet bootstrap` installs. Operators rarely " +
			"invoke this directly outside of debugging.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "system",
				Usage: "Run in system mode (Unix: /run/mooncake, /var/lib/mooncake. Windows: %ProgramData%\\Mooncake\\). Default: per-user.",
			},
			&cli.StringFlag{
				Name:  "socket",
				Usage: "Unix socket path. Pass --socket=\"\" to disable the unix listener (TCP-only mode, requires --bind).",
			},
			&cli.StringFlag{
				Name:  "state-dir",
				Usage: "Override the state directory",
			},
			&cli.StringFlag{
				Name:  "bind",
				Usage: "TCP bind address for fleet access (e.g. 0.0.0.0:7878). Empty disables TCP unless --socket=\"\" forces TCP-only mode.",
			},
			&cli.StringFlag{
				Name:  "token-file",
				Usage: "Bearer-token file path. Generated on first start if missing.",
			},
			&cli.Int64Flag{
				Name:  "max-sync-bytes",
				Usage: "Per-file size cap for PUT /v1/files (default 100 MiB).",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Value: "info",
				Usage: "Log level: debug, info, warn, error",
			},
			&cli.BoolFlag{
				Name:  "no-mdns",
				Usage: "Disable mDNS advertise (`_mooncake._tcp.local`). Default: advertise when --bind is set.",
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "Override the mDNS instance name. Defaults to the OS hostname with `.local` stripped. Useful on macOS to dodge Bonjour collision renames.",
			},
		},
		Action: agentdRun,
	}
}

func agentdRun(c *cli.Context) error {
	cfg, err := agentd.Default(c.Bool("system"))
	if err != nil {
		return err
	}
	// --socket: when the flag is explicitly set we honor its literal value
	// (including the empty string, which disables the unix listener).
	// When the flag isn't passed at all, we keep the platform default.
	if c.IsSet("socket") {
		cfg.SocketPath = c.String("socket")
	}
	if v := c.String("state-dir"); v != "" {
		cfg.StateDir = v
	}
	if v := c.String("bind"); v != "" {
		cfg.BindAddr = v
	}
	if v := c.String("token-file"); v != "" {
		cfg.TokenPath = v
	}
	if v := c.Int64("max-sync-bytes"); v > 0 {
		cfg.MaxSyncBytes = v
	}
	cfg.LogLevel = c.String("log-level")

	// Spec-45 §Task 1: mDNS advertise defaults on whenever TCP is bound.
	// --no-mdns disables it (operator-driven opt-out: privacy, captive-
	// network behaviour, etc.). --name overrides the auto-derived
	// instance name; useful on macOS where Bonjour aggressively renames
	// collisions.
	cfg.AdvertiseMDNS = !c.Bool("no-mdns")
	cfg.AdvertiseName = c.String("name")

	// Windows convenience: if the user didn't ask for either listener
	// explicitly, default to TCP-only on loopback. Spec-49 §"CLI changes".
	// Loopback (not 0.0.0.0) so first-time launches don't unexpectedly
	// accept LAN traffic before a firewall rule is in place.
	if runtime.GOOS == "windows" && !c.IsSet("socket") && !c.IsSet("bind") {
		cfg.SocketPath = ""
		cfg.BindAddr = "127.0.0.1:7878"
	}

	// Load (or create) the bearer token when TCP is enabled. The unix
	// listener is gated by filesystem perms and doesn't need it.
	if cfg.BindAddr != "" {
		tok, err := agentd.LoadOrCreateToken(cfg.TokenPath)
		if err != nil {
			return fmt.Errorf("load bearer token: %w", err)
		}
		cfg.Token = tok
	}

	log, err := newDaemonLogger(cfg.LogLevel)
	if err != nil {
		return err
	}

	srv, err := agentd.New(cfg, log, Version)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(c.Context, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return srv.Serve(ctx)
}

// agentdBootstrapCommand drives the local-install path added in
// spec-70: install the systemd unit / launchd plist for the running
// mooncake binary, enable + start it, and print the bearer token + a
// `fleet pair` one-liner. Reuses internal/fleet/install.Bootstrap
// against a LocalExecutor — same orchestration the SSH path uses,
// just without the SSH-to-self detour.
func agentdBootstrapCommand() *cli.Command {
	return &cli.Command{
		Name:  "bootstrap",
		Usage: "Install agentd on this machine (no SSH detour)",
		Description: "Install the systemd unit (Linux) or launchd plist (macOS) for " +
			"the running mooncake binary, enable + start it, and print the bearer " +
			"token + a `fleet pair` one-liner for the controller. Use " +
			"`mooncake fleet bootstrap user@host` when the target is a different machine.\n\n" +
			"Idempotent: rerun with the same version + active service is a no-op. " +
			"Different version requires --upgrade.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "user",
				Usage: "Linux only: install as user-scope systemd unit (default: system-scope).",
			},
			&cli.IntFlag{
				Name:  "port",
				Value: 7878,
				Usage: "agentd TCP bind port",
			},
			&cli.StringFlag{
				Name:  "binary",
				Usage: "mooncake binary to install (default: this process)",
			},
			&cli.BoolFlag{
				Name:  "upgrade",
				Usage: "Replace mismatched-version install",
			},
		},
		Action: agentdBootstrapAction,
	}
}

func agentdBootstrapAction(c *cli.Context) error {
	osName := runtime.GOOS
	if osName == "windows" {
		// Out of scope for v1 (spec-70 §Non-goals). The Linux user/
		// system split this command untangles doesn't have an analog
		// on Windows yet — Task Scheduler S4U principal lookup is
		// the bit that needs design.
		return cli.Exit("agentd bootstrap: not supported on Windows; use `mooncake fleet bootstrap` from a Linux/macOS controller", 2)
	}
	if osName != "linux" && osName != "darwin" {
		return cli.Exit(fmt.Sprintf("agentd bootstrap: unsupported os %q", osName), 2)
	}
	asUser := c.Bool("user")
	if asUser && osName != "linux" {
		return cli.Exit("agentd bootstrap: --user is Linux-only (macOS LaunchAgents are a follow-up)", 2)
	}

	binPath := c.String("binary")
	if binPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate own binary: %w", err)
		}
		binPath = exe
	}
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("stat binary at %s: %w", binPath, err)
	}

	port := c.Int("port")

	w := c.App.Writer
	exec := install.NewLocalExecutor()
	res, err := install.Bootstrap(c.Context, exec, install.BootstrapOptions{
		OS:                osName,
		Arch:              runtime.GOARCH,
		Port:              port,
		AsUser:            asUser,
		LocalBinary:       binPath,
		ControllerVersion: Version,
		Upgrade:           c.Bool("upgrade"),
		ReachableHost:     "127.0.0.1",
		Writer:            w,
	})
	if err != nil {
		return err
	}

	printAgentdBootstrapResult(w, osName, asUser, port, res)
	return nil
}

// printAgentdBootstrapResult is the spec-70 §Design 3 output shape:
// a few lines of facts about what's installed, the bearer token, and
// a copy-pasteable `fleet pair` one-liner for the controller.
//
// The pair hint uses `--token-via stdin` (heredoc) so the copy-paste
// doesn't leave the token in shell history.
func printAgentdBootstrapResult(w io.Writer, osName string, asUser bool, port int, res install.BootstrapResult) {
	inst := install.Installer{OS: osName, Port: port, AsUser: asUser && osName == "linux"}

	mode := "system"
	if inst.AsUser {
		mode = "user"
	}

	if res.AlreadyOK {
		fmt.Fprintf(w, "\n✓ agentd already installed at %s (%s-mode), same version — refreshed token only\n",
			inst.BinaryInstallPath(), mode)
	} else {
		fmt.Fprintf(w, "\n✓ agentd installed at %s (%s-mode)\n", inst.BinaryInstallPath(), mode)
		fmt.Fprintf(w, "✓ unit at %s\n", inst.UnitPath())
		if inst.AsUser {
			if u, err := user.Current(); err == nil {
				fmt.Fprintf(w, "✓ linger enabled for %s\n", u.Username)
			}
		}
		fmt.Fprintf(w, "✓ agentd reachable at 0.0.0.0:%d\n", port)
	}

	host, _ := os.Hostname()
	if host == "" {
		host = "<this-host>"
	}
	// Strip trailing `.local` for the pair-hint — `.local` is just
	// the mDNS advertise zone; the controller can resolve the
	// shorter form via mDNS or its hosts file.
	host = strings.TrimSuffix(host, ".local")

	fmt.Fprintf(w, "\nbearer token:\n  %s\n", res.Token)
	fmt.Fprintf(w, "\npair from the controller:\n")
	fmt.Fprintf(w, "  mooncake fleet pair %s:%d --token-via stdin <<<'%s'\n", host, port, res.Token)
}

func newDaemonLogger(level string) (*slog.Logger, error) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info", "":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log level: %s", level)
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h), nil
}

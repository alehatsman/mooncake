package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/alehatsman/mooncake/internal/agentd"
	"github.com/urfave/cli/v2"
)

func agentdCommand() *cli.Command {
	return &cli.Command{
		Name:  "agentd",
		Usage: "Run the mooncake host daemon (experimental)",
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

	srv, err := agentd.New(cfg, log, version)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(c.Context, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return srv.Serve(ctx)
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

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
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
				Usage: "Run in system mode (socket /run/mooncake, state /var/lib/mooncake). Default: per-user.",
			},
			&cli.StringFlag{
				Name:  "socket",
				Usage: "Override the unix socket path",
			},
			&cli.StringFlag{
				Name:  "state-dir",
				Usage: "Override the state directory",
			},
			&cli.StringFlag{
				Name:  "bind",
				Usage: "TCP bind address for fleet access (e.g. 0.0.0.0:7878). Empty disables TCP.",
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
		},
		Action: agentdRun,
	}
}

func agentdRun(c *cli.Context) error {
	cfg, err := agentd.Default(c.Bool("system"))
	if err != nil {
		return err
	}
	if v := c.String("socket"); v != "" {
		cfg.SocketPath = v
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

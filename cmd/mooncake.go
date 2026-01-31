package main

import (
	"log"
	"os"
	"strings"

	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/urfave/cli/v2"
)

func run(c *cli.Context) error {
	raw := c.Bool("raw")
	logLevel := c.String("log-level")

	var log logger.Logger

	// Use animated TUI by default if supported, unless --raw is specified
	if !raw && logger.IsTUISupported() {
		tuiLogger, err := logger.NewTUILogger(logger.InfoLevel)
		if err != nil {
			// Fallback to console logger if TUI initialization fails
			log = logger.NewLogger(logger.InfoLevel)
		} else {
			log = tuiLogger
			tuiLogger.Start()
			defer tuiLogger.Stop()
		}
	} else {
		// Use console logger for raw output or when TUI is not supported
		log = logger.NewLogger(logger.InfoLevel)
	}

	if err := log.SetLogLevelStr(logLevel); err != nil {
		return err
	}

	// Parse tags from comma-separated string
	var tags []string
	tagsStr := c.String("tags")
	if tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ",") {
			trimmed := strings.TrimSpace(tag)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	return executor.Start(executor.StartConfig{
		ConfigFilePath: c.String("config"),
		VarsFilePath:   c.String("vars"),
		SudoPass:       c.String("sudo-pass"),
		Tags:           tags,
	}, log)
}

func createApp() *cli.App {
	app := &cli.App{
		Name:                 "mooncake",
		Usage:                "Space fighters provisioning tool, Chookity!",
		EnableBashCompletion: true,

		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Run a space fighter",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "config",
						Aliases:  []string{"c"},
						Required: true,
						Usage:    "Path to configuration file",
					},
					&cli.StringFlag{
						Name:    "vars",
						Aliases: []string{"v"},
						Usage:   "Path to variables file",
					},
					&cli.StringFlag{
						Name:    "log-level",
						Aliases: []string{"l"},
						Value:   "info",
						Usage:   "Log level (debug, info, error)",
					},
					&cli.StringFlag{
						Name:    "sudo-pass",
						Aliases: []string{"s"},
						Usage:   "Sudo password for steps with become: true",
					},
					&cli.StringFlag{
						Name:    "tags",
						Aliases: []string{"t"},
						Usage:   "Filter steps by tags (comma-separated)",
					},
					&cli.BoolFlag{
						Name:    "raw",
						Aliases: []string{"r"},
						Value:   false,
						Usage:   "Disable animated TUI and use raw console output",
					},
				},
				Action: run,
			},
		},
	}

	return app
}

func main() {
	app := createApp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

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
	log := logger.NewLogger(logger.InfoLevel)

	logLevel := c.String("log-level")

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

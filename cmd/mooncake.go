package main

import (
	"fmt"
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
		VarsFilePath:   c.String("variables"),
		SudoPass:       c.String("sudo"),
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
						Name:    "config",
						Aliases: []string{"c"},
					},
					&cli.StringFlag{
						Name:    "variables",
						Aliases: []string{"v"},
					},
					&cli.StringFlag{
						Name:    "log-level",
						Aliases: []string{"l"},
						Value:   "info",
					},
					&cli.StringFlag{
						Name:    "sudo",
						Aliases: []string{"s"},
						Value:   "false",
					},
					&cli.StringFlag{
						Name:    "tags",
						Aliases: []string{"t"},
						Usage:   "Filter steps by tags (comma-separated)",
					},
				},
				Action: run,
			},
			{
				Name:  "watch",
				Usage: "Watch a space fighter",
				Action: func(c *cli.Context) error {
					fmt.Println("Running space fighter...")
					return nil
				},
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

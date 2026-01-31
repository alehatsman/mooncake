package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/urfave/cli/v2"
)

func run(c *cli.Context) error {
	logger := logger.GetInstance()

	logLevel := c.String("log-level")

	if err := logger.SetLogLevelStr(logLevel); err != nil {
		return err
	}
	return executor.Start(executor.StartConfig{
		ConfigFilePath: c.String("config"),
		VarsFilePath:   c.String("variables"),
	})
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

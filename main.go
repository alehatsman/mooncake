package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
)

func run(c *cli.Context) error {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Fighter provisioning..."),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionSpinnerType(76),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionEnableColorCodes(true),
	)
	for i := 0; i < 100; i++ {
		bar.Add(1)
		time.Sleep(40 * time.Millisecond)
	}
	bar.Finish()

	fmt.Println("Chookity!")
	return nil
}

func main() {
	app := &cli.App{
		Name:                 "mooncake",
		Usage:                "Space fighters provisioning tool, Chookity!",
		EnableBashCompletion: true,
		Action:               run,
		Commands: []*cli.Command{
			{
				Name:   "run",
				Usage:  "Run a space fighter",
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

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// Package main provides the mooncake CLI application.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/explain"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/urfave/cli/v2"
)

func run(c *cli.Context) error {
	raw := c.Bool("raw")
	dryRun := c.Bool("dry-run")
	logLevel := c.String("log-level")

	var log logger.Logger

	// Force raw mode for dry-run
	if dryRun {
		raw = true
	}

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
		DryRun:         dryRun,
	}, log)
}

func explainCommand(_ *cli.Context) error {
	// Collect system facts
	f := facts.Collect()

	// Display facts
	explain.DisplayFacts(f)

	return nil
}

func validateCommand(c *cli.Context) error {
	configPath := c.String("config")
	format := c.String("format")

	// Read and validate configuration
	_, diagnostics, err := config.ReadConfigWithValidation(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(3) // Runtime error
	}

	// Check for validation errors
	hasErrors := config.HasErrors(diagnostics)

	// Output diagnostics
	if format == "json" {
		// JSON output
		type ValidationResult struct {
			Valid       bool                  `json:"valid"`
			Diagnostics []config.Diagnostic   `json:"diagnostics,omitempty"`
		}
		result := ValidationResult{
			Valid:       !hasErrors,
			Diagnostics: diagnostics,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(3)
		}
	} else {
		// Text output
		if len(diagnostics) > 0 {
			fmt.Println(config.FormatDiagnosticsWithContext(diagnostics))
		}

		if hasErrors {
			fmt.Println("\n❌ Validation failed")
		} else if len(diagnostics) > 0 {
			fmt.Println("\n⚠️  Validation passed with warnings")
		} else {
			fmt.Println("✓ Configuration is valid")
		}
	}

	// Exit with appropriate code
	if hasErrors {
		os.Exit(2) // Validation error
	}

	return nil
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
					&cli.BoolFlag{
						Name:  "dry-run",
						Value: false,
						Usage: "Preview what would be executed without making changes",
					},
				},
				Action: run,
			},
			{
				Name:    "explain",
				Aliases: []string{"info"},
				Usage:   "Explain machine state - show system information",
				Action:  explainCommand,
			},
			{
				Name:  "validate",
				Usage: "Validate configuration file without executing",
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
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "text",
						Usage:   "Output format: text or json",
					},
				},
				Action: validateCommand,
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

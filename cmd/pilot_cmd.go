package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/pilot"
)

// pilotCommand returns the `mooncake pilot` parent with its single
// `run` subcommand.
func pilotCommand() *cli.Command {
	return &cli.Command{
		Name:  "pilot",
		Usage: "Pilot operations",
		Subcommands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Execute pilot iteration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "goal",
						Aliases:  []string{"g"},
						Required: true,
						Usage:    "Goal description",
					},
					&cli.StringFlag{
						Name:    "plan",
						Aliases: []string{"p"},
						Usage:   "Path to plan YAML file",
					},
					&cli.BoolFlag{
						Name:  "stdin",
						Usage: "Read plan from stdin",
					},
					&cli.StringFlag{
						Name:  "provider",
						Usage: "LLM provider: anthropic-cli, anthropic-http, or openai-shape (omit for auto-discovery)",
					},
					&cli.StringFlag{
						Name:  "endpoint",
						Usage: "OpenAI-compatible /v1 base URL (e.g. http://localhost:11434/v1); required for --provider openai-shape unless MOONCAKE_PILOT_ENDPOINT is set",
					},
					&cli.StringFlag{
						Name:  "model",
						Usage: "Model name (when using --provider)",
					},
					&cli.IntFlag{
						Name:  "max-iterations",
						Value: 5,
						Usage: "Maximum iterations for loop mode",
					},
					&cli.BoolFlag{
						Name:  "auto-apply",
						Usage: "Skip the plan-confirm gate (required for unattended/CI runs; spec-67 §10)",
					},
				},
				Action: pilotRunAction,
			},
		},
	}
}

func pilotRunAction(c *cli.Context) error {
	goal := c.String("goal")
	planPath := c.String("plan")
	useStdin := c.Bool("stdin")
	provider := c.String("provider")
	model := c.String("model")
	maxIterations := c.Int("max-iterations")

	if goal == "" {
		return fmt.Errorf("--goal is required")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	opts := pilot.RunOptions{
		Goal:          goal,
		PlanPath:      planPath,
		UseStdin:      useStdin,
		RepoRoot:      repoRoot,
		Provider:      provider,
		Endpoint:      c.String("endpoint"),
		Model:         model,
		MaxIterations: maxIterations,
		AutoApply:     c.Bool("auto-apply"),
	}

	if planPath == "" && !useStdin {
		result, loopErr := pilot.RunLoop(opts)
		if loopErr != nil {
			fmt.Fprintf(os.Stderr, "Pilot loop failed: %v\n", loopErr)
			if result != nil && result.FinalLog != nil {
				printPilotSummary(result.FinalLog)
			}
			return loopErr
		}

		fmt.Printf("Pilot completed: %d iterations\n", len(result.Iterations))
		fmt.Printf("Stop reason: %s\n", result.StopReason)
		if result.FinalLog != nil {
			fmt.Println()
			printPilotSummary(result.FinalLog)
		}
		return nil
	}

	if planPath != "" && useStdin {
		return fmt.Errorf("cannot specify both --plan and --stdin")
	}

	if planPath != "" && !filepath.IsAbs(planPath) {
		planPath = filepath.Join(repoRoot, planPath)
	}

	opts.PlanPath = planPath

	log, err := pilot.Run(opts)
	if err != nil {
		return err
	}

	printPilotSummary(log)
	return nil
}

func printPilotSummary(log *pilot.IterationLog) {
	fmt.Printf("Iteration: %d\n", log.Iteration)
	fmt.Printf("Status: %s\n", log.Status)
	fmt.Printf("Files touched: %d\n", log.DiffStat.Files)
	fmt.Printf("Insertions: +%d\n", log.DiffStat.Insertions)
	fmt.Printf("Deletions: -%d\n", log.DiffStat.Deletions)

	if len(log.ChangedFiles) > 0 {
		fmt.Println("\nChanged files:")
		for _, file := range log.ChangedFiles {
			fmt.Printf("  %s\n", file)
		}
	}

	if len(log.Artifacts) > 0 {
		fmt.Println("\nArtifacts:")
		for _, artifact := range log.Artifacts {
			fmt.Printf("  %s\n", artifact)
		}
	}
}

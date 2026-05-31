package kernel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/pilot"
)

// PilotCommand returns the `mooncake pilot` parent with its single
// `run` subcommand.
func PilotCommand() *cli.Command {
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
					&cli.StringFlag{
						Name:  "style",
						Usage: "Planning style: plan (single complete plan, default) or step (one action per turn). Overrides MOONCAKE_PILOT_STYLE.",
					},
					&cli.StringFlag{
						Name:  "output-format",
						Value: "text",
						Usage: "Output format: text (human-readable, default) or json (NDJSON event stream, one events.Event per line, terminated by a pilot.completed event).",
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

	style, err := resolvePilotStyle(c.String("style"), os.Getenv("MOONCAKE_PILOT_STYLE"))
	if err != nil {
		return err
	}

	outputFormat, err := resolvePilotOutputFormat(c.String("output-format"))
	if err != nil {
		return err
	}
	jsonOut := outputFormat == pilot.OutputFormatJSON

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
		Style:         style,
		OutputFormat:  outputFormat,
	}

	if planPath == "" && !useStdin {
		result, loopErr := pilot.RunLoop(opts)
		if loopErr != nil {
			// The loop's own NDJSON/text stream already conveyed per-step
			// detail; the human-readable summary goes to stdout only in
			// text mode (it would corrupt the JSON stream otherwise — the
			// error itself is on stderr).
			fmt.Fprintf(os.Stderr, "Pilot loop failed: %v\n", loopErr)
			if result != nil && result.FinalLog != nil && !jsonOut {
				printPilotSummary(result.FinalLog)
			}
			return loopErr
		}

		if jsonOut {
			emitPilotCompleted(os.Stdout, result)
		} else {
			fmt.Printf("Pilot completed: %d iterations\n", len(result.Iterations))
			fmt.Printf("Stop reason: %s\n", result.StopReason)
			if result.FinalLog != nil {
				fmt.Println()
				printPilotSummary(result.FinalLog)
			}
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

	if jsonOut {
		emitPilotCompletedFromLog(os.Stdout, log)
	} else {
		printPilotSummary(log)
	}
	return nil
}

// resolvePilotOutputFormat validates the --output-format flag. Empty
// defaults to text; only text and json are accepted (a typo is a hard
// error so a consumer asking for json never silently gets prose).
func resolvePilotOutputFormat(raw string) (string, error) {
	switch raw {
	case "", pilot.OutputFormatText:
		return pilot.OutputFormatText, nil
	case pilot.OutputFormatJSON:
		return pilot.OutputFormatJSON, nil
	default:
		return "", fmt.Errorf("invalid --output-format %q (want text or json)", raw)
	}
}

// emitPilotCompleted writes the terminal pilot.completed event for a loop
// run. It carries the same outcome the text summary prints so a JSON
// consumer can finalize without parsing prose.
func emitPilotCompleted(w io.Writer, result *pilot.LoopResult) {
	data := pilot.PilotCompletedData{
		Iterations: len(result.Iterations),
		StopReason: string(result.StopReason),
	}
	if result.FinalLog != nil {
		data.Status = result.FinalLog.Status
		data.DiffStat = result.FinalLog.DiffStat
		data.ChangedFiles = result.FinalLog.ChangedFiles
	}
	emitPilotEvent(w, events.EventPilotCompleted, data)
}

// emitPilotCompletedFromLog is the single-shot (--plan / --stdin) analogue
// of emitPilotCompleted: that path applies exactly one iteration and
// returns its log directly, so it's iteration 1 with a success stop.
func emitPilotCompletedFromLog(w io.Writer, log *pilot.IterationLog) {
	emitPilotEvent(w, events.EventPilotCompleted, pilot.PilotCompletedData{
		Iterations:   1,
		StopReason:   string(pilot.StopSuccess),
		Status:       log.Status,
		DiffStat:     log.DiffStat,
		ChangedFiles: log.ChangedFiles,
	})
}

// emitPilotEvent encodes one events.Event as a single NDJSON line,
// matching the shape the ConsoleSubscriber's renderJSON emits for the
// per-step stream so a consumer parses every line uniformly.
func emitPilotEvent(w io.Writer, t events.Type, data interface{}) {
	ev := events.Event{Type: t, Timestamp: time.Now(), Data: data}
	if err := json.NewEncoder(w).Encode(ev); err != nil {
		fmt.Fprintf(os.Stderr, "pilot: encode %s event: %v\n", t, err)
	}
}

// resolvePilotStyle implements the precedence chain documented in
// spec-67 §6.1 + plan §6: CLI --style > MOONCAKE_PILOT_STYLE env >
// built-in default `plan`. Anything outside the {plan, step}
// whitelist is a hard error so a typo doesn't silently downgrade the
// run.
func resolvePilotStyle(cliVal, envVal string) (pilot.Style, error) {
	raw := cliVal
	if raw == "" {
		raw = envVal
	}
	if raw == "" {
		return pilot.StylePlan, nil
	}
	switch raw {
	case string(pilot.StylePlan):
		return pilot.StylePlan, nil
	case string(pilot.StyleStep):
		return pilot.StyleStep, nil
	default:
		return "", fmt.Errorf("invalid --style %q (want plan or step)", raw)
	}
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

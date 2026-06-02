package kernel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/agent"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// AgentCommand returns the `mooncake agent` parent with its single
// `run` subcommand.
func AgentCommand() *cli.Command {
	return &cli.Command{
		Name:  "agent",
		Usage: "Agent operations",
		Subcommands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Execute agent iteration",
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
						Usage: "OpenAI-compatible /v1 base URL (e.g. http://localhost:11434/v1); required for --provider openai-shape unless MOONCAKE_AGENT_ENDPOINT is set",
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
						Usage: "Planning style: plan (single complete plan, default) or step (one action per turn). Overrides MOONCAKE_AGENT_STYLE. Plan-style completes as soon as an iteration makes no new agent-attributable file changes, so a one-shot task (print, run a test, echo) finishes in a single iteration rather than re-planning; files already dirty in the workspace when the run started don't count (#87).",
					},
					&cli.StringSliceFlag{
						Name:  "allow-action",
						Usage: "Permissions-as-contract allowlist (#11): action types the agent may use (repeatable). Empty = any action unless denied.",
					},
					&cli.StringSliceFlag{
						Name:  "deny-action",
						Usage: "Permissions-as-contract denylist (#11): action types the agent may NOT use (repeatable), e.g. --deny-action shell --deny-action cmd. Wins over the allowlist.",
					},
					&cli.BoolFlag{
						Name:  "deny-network",
						Usage: "Refuse any step that declares network egress (pkg install, download, http.request, remote git clone).",
					},
					&cli.IntFlag{
						Name:  "max-risk",
						Usage: "Refuse any step whose estimated risk band (1..10) exceeds this cap. 0 = no cap.",
					},
					&cli.StringFlag{
						Name:  "output-format",
						Value: "text",
						Usage: "Output format: text (human-readable, default) or json (NDJSON event stream, one events.Event per line, terminated by a agent.completed event).",
					},
					&cli.DurationFlag{
						Name:    "llm-timeout",
						EnvVars: []string{"MOONCAKE_AGENT_LLM_TIMEOUT"},
						Usage:   "Per-iteration LLM plan-generation budget (e.g. 5m, 20m). 0 = built-in default (5m). Raise for thinking-heavy plans that would otherwise be SIGKILLed mid-stream (#80).",
					},
				},
				Action: agentRunAction,
			},
		},
	}
}

func agentRunAction(c *cli.Context) error {
	goal := c.String("goal")
	planPath := c.String("plan")
	useStdin := c.Bool("stdin")
	provider := c.String("provider")
	model := c.String("model")
	maxIterations := c.Int("max-iterations")

	if goal == "" {
		return fmt.Errorf("--goal is required")
	}

	style, err := resolveAgentStyle(c.String("style"), os.Getenv("MOONCAKE_AGENT_STYLE"))
	if err != nil {
		return err
	}

	outputFormat, err := resolveAgentOutputFormat(c.String("output-format"))
	if err != nil {
		return err
	}
	jsonOut := outputFormat == agent.OutputFormatJSON

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	opts := agent.RunOptions{
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
		Policy:        buildAgentPolicy(c),
		OutputFormat:  outputFormat,
		LLMTimeout:    c.Duration("llm-timeout"),
	}

	if planPath == "" && !useStdin {
		result, loopErr := agent.RunLoop(opts)
		if loopErr != nil {
			// The loop's own NDJSON/text stream already conveyed per-step
			// detail; the human-readable summary goes to stdout only in
			// text mode (it would corrupt the JSON stream otherwise — the
			// error itself is on stderr).
			fmt.Fprintf(os.Stderr, "Agent loop failed: %v\n", loopErr)
			if result != nil && result.FinalLog != nil && !jsonOut {
				printAgentSummary(result.FinalLog)
			}
			return loopErr
		}

		if jsonOut {
			emitAgentCompleted(os.Stdout, result)
		} else {
			fmt.Printf("Agent completed: %d iterations\n", len(result.Iterations))
			fmt.Printf("Stop reason: %s\n", result.StopReason)
			if result.FinalLog != nil {
				fmt.Println()
				printAgentSummary(result.FinalLog)
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

	log, err := agent.Run(opts)
	if err != nil {
		return err
	}

	if jsonOut {
		emitAgentCompletedFromLog(os.Stdout, log)
	} else {
		printAgentSummary(log)
	}
	return nil
}

// resolveAgentOutputFormat validates the --output-format flag. Empty
// defaults to text; only text and json are accepted (a typo is a hard
// error so a consumer asking for json never silently gets prose).
func resolveAgentOutputFormat(raw string) (string, error) {
	switch raw {
	case "", agent.OutputFormatText:
		return agent.OutputFormatText, nil
	case agent.OutputFormatJSON:
		return agent.OutputFormatJSON, nil
	default:
		return "", fmt.Errorf("invalid --output-format %q (want text or json)", raw)
	}
}

// emitAgentCompleted writes the terminal agent.completed event for a loop
// run. It carries the same outcome the text summary prints so a JSON
// consumer can finalize without parsing prose.
func emitAgentCompleted(w io.Writer, result *agent.LoopResult) {
	data := agent.AgentCompletedData{
		Iterations: len(result.Iterations),
		StopReason: string(result.StopReason),
	}
	if result.FinalLog != nil {
		// Status is the worst outcome across all iterations, not just the
		// last (#64): a later no-op/success iteration must not mask an
		// earlier failed (or failed-rollback) one for a consumer keying on
		// agent.completed.status. DiffStat/ChangedFiles still reflect the
		// final iteration's artifact.
		data.Status = result.TerminalStatus()
		data.DiffStat = result.FinalLog.DiffStat
		data.ChangedFiles = result.FinalLog.ChangedFiles
	}
	emitAgentEvent(w, events.EventAgentCompleted, data)
}

// emitAgentCompletedFromLog is the single-shot (--plan / --stdin) analogue
// of emitAgentCompleted: that path applies exactly one iteration and
// returns its log directly, so it's iteration 1 with a success stop.
func emitAgentCompletedFromLog(w io.Writer, log *agent.IterationLog) {
	emitAgentEvent(w, events.EventAgentCompleted, agent.AgentCompletedData{
		Iterations:   1,
		StopReason:   string(agent.StopSuccess),
		Status:       log.Status,
		DiffStat:     log.DiffStat,
		ChangedFiles: log.ChangedFiles,
	})
}

// emitAgentEvent encodes one events.Event as a single NDJSON line,
// matching the shape the ConsoleSubscriber's renderJSON emits for the
// per-step stream so a consumer parses every line uniformly.
func emitAgentEvent(w io.Writer, t events.Type, data interface{}) {
	ev := events.Event{Type: t, Timestamp: time.Now(), Data: data}
	if err := json.NewEncoder(w).Encode(ev); err != nil {
		fmt.Fprintf(os.Stderr, "agent: encode %s event: %v\n", t, err)
	}
}

// resolveAgentStyle implements the precedence chain documented in
// spec-67 §6.1 + plan §6: CLI --style > MOONCAKE_AGENT_STYLE env >
// built-in default `plan`. Anything outside the {plan, step}
// whitelist is a hard error so a typo doesn't silently downgrade the
// run.
func resolveAgentStyle(cliVal, envVal string) (agent.Style, error) {
	raw := cliVal
	if raw == "" {
		raw = envVal
	}
	if raw == "" {
		return agent.StylePlan, nil
	}
	switch raw {
	case string(agent.StylePlan):
		return agent.StylePlan, nil
	case string(agent.StyleStep):
		return agent.StyleStep, nil
	default:
		return "", fmt.Errorf("invalid --style %q (want plan or step)", raw)
	}
}

// buildAgentPolicy assembles the per-run permissions-as-contract gate
// (#11) from the policy flags. Returns nil when no policy flag is set,
// so an unrestricted agent run stays byte-identical to the pre-flag
// behavior (the executor treats a nil Policy as "enforce nothing").
// This is the surface a moongit-spawned agent container invokes to drop
// the shell escape hatch: `mooncake agent run ... --deny-action shell
// --deny-action cmd`.
func buildAgentPolicy(c *cli.Context) *executor.Policy {
	allow := c.StringSlice("allow-action")
	deny := c.StringSlice("deny-action")
	denyNet := c.Bool("deny-network")
	maxRisk := c.Int("max-risk")

	if len(allow) == 0 && len(deny) == 0 && !denyNet && maxRisk <= 0 {
		return nil
	}
	return &executor.Policy{
		AllowedActions: allow,
		DeniedActions:  deny,
		DenyNetwork:    denyNet,
		MaxRisk:        maxRisk,
	}
}

func printAgentSummary(log *agent.IterationLog) {
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

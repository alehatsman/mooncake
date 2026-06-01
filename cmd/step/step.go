// Package step implements the `mooncake step` CLI — execute a
// single inline step (one action invocation, no plan) and return
// a JSON result. The MCP server and the agent agent loop drive
// this verb to invoke handlers without going through plan/apply.
package step

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
	"github.com/urfave/cli/v2"
)

// buildStepJSON assembles the JSON payload for `mooncake step`. Mirrors
// the shape `apply --output-format json` exposes under `result.*`: the
// shared scalar fields (stdout / stderr / changed / …) plus every
// action-specific key the handler called SetData() with. MT-22: prior to
// this generalization the output was a hard-coded subset (changed,
// action, stdout, stderr, error, duration_ms), which silently dropped
// typed actions' payloads — `repo.search` results, `read.json` value,
// etc. — and made `step` useless for agents that need the structured
// output, while shell still worked by accident because stdout is in the
// subset.
func buildStepJSON(actionType string, result *executor.Result, execErr error) map[string]any {
	var payload map[string]any
	if result != nil {
		// RegisteredResult.ToMap is the canonical surface — same shape
		// `apply --output-format json` ships in run records and the
		// same shape templates see via `{{ result.foo }}`.
		payload = result.ToRegisteredResult().ToMap()
	} else {
		payload = map[string]any{}
	}
	payload["action"] = actionType
	if execErr != nil {
		payload["error"] = execErr.Error()
		// MT-61: keep `failed` consistent with the presence of an
		// error. Without this `mooncake step` emits {failed: false,
		// error: "..."} on wait.* timeouts (and any handler that
		// returns an error without first setting result.Failed),
		// which agents parsing the JSON read as success. Apply
		// mode already marks the step failed; step mode now matches.
		payload["failed"] = true
	}
	return payload
}

func Command() *cli.Command {
	return &cli.Command{
		Name:      "step",
		Usage:     "Execute a single inline step and return JSON result",
		ArgsUsage: "<yaml-step>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Show what would be done without making changes",
			},
			&cli.BoolFlag{
				Name:    "become",
				Aliases: []string{"b"},
				Usage:   "Run step with sudo",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return fmt.Errorf("step YAML argument required (e.g. 'shell: {cmd: echo hi}')")
			}

			raw := c.Args().First()

			// MT-83: enforce the same `additionalProperties: false`
			// strictness that `mooncake apply` runs at config-read time
			// (MT-44). Without this, typos like `expected_exit:`
			// (canonical: `expect_exit:`) get silently dropped by
			// yaml.v3's permissive decode and the action handler runs
			// with the default value — agents see a confusing timeout
			// instead of a "field unknown" error.
			var step config.Step
			if err := config.DecodeAutoStrict([]byte(raw), &step); err != nil {
				return fmt.Errorf("failed to parse step: %w", err)
			}

			if c.Bool("become") && step.AsUser == "" {
				step.AsUser = "root"
			}

			actionType := step.DetermineActionType()
			if actionType == "" {
				return fmt.Errorf("no recognized action type in step YAML")
			}

			renderer, err := template.NewPongo2Renderer()
			if err != nil {
				return err
			}
			evaluator := expression.NewGovaluateEvaluator()
			pathExpander := pathutil.NewPathExpander(renderer)
			fileTreeWalker := filetree.NewWalker(pathExpander)
			redactor := security.NewRedactor()
			publisher := events.NewSyncPublisher()

			log := logger.NewLogger(logger.ErrorLevel)

			cwd, _ := os.Getwd()

			mode := actions.ModeApply
			if c.Bool("dry-run") {
				mode = actions.ModePlan
			}
			scope := executor.NewVariableScope()
			ec := &executor.ExecutionContext{
				Svc: &executor.RunServices{
					Logger:         log,
					Mode:           mode,
					Stats:          executor.NewExecutionStats(),
					Template:       renderer,
					Evaluator:      evaluator,
					PathUtil:       pathExpander,
					FileTree:       fileTreeWalker,
					Redactor:       redactor,
					EventPublisher: publisher,
				},
				Scope:      scope,
				CurrentDir: cwd,
				Level:      0,
			}

			executor.AddGlobalVariables(ec.Scope)

			start := time.Now()
			execErr := executor.DispatchStepAction(step, ec)
			durationMs := time.Since(start).Milliseconds()

			payload := buildStepJSON(actionType, ec.CurrentResult, execErr)
			// duration_ms in the registered-result map is sourced from
			// Result.Duration which the dispatcher does not always set
			// (the per-action handlers fill it for shell, but not all
			// typed actions). Use the controller-side wall clock so the
			// field is always present and meaningful, matching what
			// `apply --output-format json` does at the run level.
			payload["duration_ms"] = durationMs

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(payload)

			if execErr != nil {
				os.Exit(1)
			}
			return nil
		},
	}
}

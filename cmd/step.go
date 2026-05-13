package main

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
	"gopkg.in/yaml.v3"
)

type stepResult struct {
	Changed    bool   `json:"changed"`
	Action     string `json:"action"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

func stepCommand() *cli.Command {
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

			var step config.Step
			if err := yaml.Unmarshal([]byte(raw), &step); err != nil {
				return fmt.Errorf("failed to parse step YAML: %w", err)
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

			res := stepResult{
				Action:     actionType,
				DurationMs: durationMs,
			}
			if ec.CurrentResult != nil {
				res.Changed = ec.CurrentResult.Changed
				res.Stdout = ec.CurrentResult.Stdout
				res.Stderr = ec.CurrentResult.Stderr
			}
			if execErr != nil {
				res.Error = execErr.Error()
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(res)

			if execErr != nil {
				os.Exit(1)
			}
			return nil
		},
	}
}

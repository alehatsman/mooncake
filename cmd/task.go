package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/urfave/cli/v2"
)

// taskCommand registers `mooncake task`. Bare invocation lists every
// task defined in the discovered config; `mooncake task <name>` runs
// the named task through the regular plan→apply pipeline by setting
// apply.Config.TaskName.
//
// Discovery: ./tasks.yml (or .yaml) wins. Otherwise the apply-config
// search path (mooncake.yml, mooncake/main.yml) is used IF the file
// defines at least one task. When both are present and the apply-
// config also has a tasks: block, the tasks file wins and a stderr
// warning names the shadowed apply-config so the user knows their
// definitions are being ignored.
func taskCommand() *cli.Command {
	return &cli.Command{
		Name:      "task",
		Usage:     "Run a named task from tasks.yml (or `tasks:` in mooncake.yml)",
		ArgsUsage: "[name]",
		Description: "Without an argument, lists every task defined in the discovered " +
			"config with its description. With a name, runs that task through the " +
			"same planner + executor that `mooncake apply` uses — only the step list " +
			"and the var overlay differ.\n\n" +
			"Discovery: ./tasks.yml is preferred, otherwise the apply-config " +
			"search path is used (must contain a `tasks:` block). When both a " +
			"tasks file and a mooncake.yml with `tasks:` exist, the tasks file " +
			"wins and a warning is printed.\n\n" +
			"Variable precedence (highest first): --vars files, task-level `vars:`, " +
			"file-level `vars:`.",
		Flags: taskFlags(),
		// MT-66-style: keep the default urfave/cli quiet handler but
		// intercept the common --dry-run typo and steer users at
		// --plan. Apply has --dry-run; task chose --plan deliberately
		// (see ADR in docs-working/decisions, if added). The hint here
		// is the only place that gap shows up to a user.
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			if err != nil && strings.Contains(err.Error(), "dry-run") {
				return fmt.Errorf("%w — for a preview use: mooncake task <name> --plan", err)
			}
			return quietUsageError(c, err, isSubcommand)
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return listTasksAction(c)
			}
			if c.NArg() > 1 {
				return fmt.Errorf("mooncake task takes at most one positional argument; got %d — run one task at a time", c.NArg())
			}
			return runTaskAction(c, c.Args().First())
		},
	}
}

// taskFlags is the trimmed flag set for `mooncake task`. Two modes:
//
//   - run mode (default): execute the task through the in-memory apply
//     runner. --vars, --tags, sudo flags apply.
//   - plan mode (--plan): build the plan via the planner and print it
//     without executing. --format, --diff, --show-origins apply.
//
// We deliberately drop apply-only flags that don't make sense for a
// task (--from-plan, --host / --overlays, --allow-stale, --max-plan-age,
// --tui — tasks are dev loop, not interactive operator UX). We also
// drop --dry-run; preview is `--plan`, no overload. A user typing
// --dry-run gets a usage error with the suggestion.
func taskFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Path to tasks file (default: ./tasks.yml, then ./mooncake.yml)",
		},
		&cli.StringSliceFlag{
			Name:    "vars",
			Aliases: []string{"v"},
			Usage:   "Path to a variables file. Repeat to layer; later wins on key collision and overrides task-level vars.",
		},
		&cli.StringFlag{
			Name:    "log-level",
			Aliases: []string{"l"},
			// Tasks are dev-loop: users want to see the underlying
			// shell-step stdout/stderr by default, not just step-start
			// / step-end markers. debug streams stdout (prefixed `|`);
			// apply's run path keeps info as default since operator
			// workflows tolerate quieter output. Pass --log-level info
			// here to suppress the per-line stream.
			Value: "debug",
			Usage: "Log level (debug, info, error). Default debug so shell-step stdout streams.",
		},

		// Preview / plan mode.
		&cli.BoolFlag{
			Name:    "plan",
			Aliases: []string{"p"},
			Usage:   "Print the task's plan without executing it (paired with --format, --diff, --show-origins)",
		},
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value:   "text",
			Usage:   "Plan output format (with --plan): text, json, or yaml",
		},
		&cli.BoolFlag{
			Name:    "diff",
			Aliases: []string{"d"},
			Usage:   "With --plan: show unified diff for file steps that would change content",
		},
		&cli.BoolFlag{
			Name:  "show-origins",
			Usage: "With --plan: include file:line:col for each step",
		},

		// Filtering — applies to both modes.
		&cli.StringFlag{
			Name:    "tags",
			Aliases: []string{"t"},
			Usage:   "Filter the task's steps by tags (comma-separated)",
		},
		&cli.StringFlag{
			Name:  "skip-tags",
			Usage: "Exclude steps whose tags appear in this list (comma-separated). Composes with --tags via AND.",
		},

		// Sudo handoffs — run mode only, but cheap to accept in plan mode too
		// so the dispatcher's preflight gets the password during inspection.
		&cli.StringFlag{Name: "sudo-pass-file", Usage: "Read sudo password from file (must have 0600 permissions)"},
		&cli.StringFlag{Name: "sudo-pass", Aliases: []string{"s"}, Usage: "Sudo password (requires --insecure-sudo-pass)"},
		&cli.BoolFlag{Name: "ask-become-pass", Aliases: []string{"K"}, Usage: "Prompt for sudo password interactively"},
		&cli.BoolFlag{Name: "insecure-sudo-pass", Usage: "Allow --sudo-pass flag (WARNING: visible in shell history)"},
	}
}

// resolveTasksConfigPath returns the explicit --config flag when set,
// otherwise the result of config.DiscoverTasksConfig. Mirrors
// cmdutil.ResolveConfigPath's shape but routes through the task-specific
// discovery so ./tasks.yml takes precedence. The second return value
// is the shadowed apply-config path (empty when no shadowing).
func resolveTasksConfigPath(c *cli.Context) (path, shadowed string, err error) {
	if explicit := c.String("config"); explicit != "" {
		return explicit, "", nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	return config.DiscoverTasksConfig(cwd)
}

// listTasksAction prints the available tasks one per line, sorted by
// name, in a "name — description" table layout. Tasks with no Desc
// render as "name — (no description)" so the column stays aligned.
func listTasksAction(c *cli.Context) error {
	configPath, shadowed, err := resolveTasksConfigPath(c)
	if err != nil {
		var nfe *config.ErrNoConfigFound
		if errors.As(err, &nfe) {
			fmt.Fprint(os.Stderr, config.HintNoConfigFound(nfe, "task"))
			os.Exit(exitCodeValidationError)
			return nil
		}
		return err
	}

	parsed, err := config.ReadConfig(configPath)
	if err != nil {
		return err
	}

	warnIfShadowed(configPath, shadowed)
	rejectStepsInDedicatedTasksFile(configPath, parsed)

	if len(parsed.Tasks) == 0 {
		fmt.Fprintf(os.Stderr, "no tasks defined in %s\n", configPath)
		return nil
	}

	names := make([]string, 0, len(parsed.Tasks))
	for name := range parsed.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintf(os.Stdout, "Tasks defined in %s:\n\n", configPath)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, name := range names {
		desc := parsed.Tasks[name].Desc
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Fprintf(tw, "  %s\t%s\n", name, desc)
	}
	_ = tw.Flush()
	fmt.Fprintln(os.Stdout, "\nRun a task with:  mooncake task <name>")
	return nil
}

// runTaskAction is the run / preview dispatcher for `mooncake task <name>`.
// Both modes share the plan-build pipeline; they diverge only at the
// final step (render vs. execute). Apply stays unaware of tasks — the
// planner does the task→steps splice and we hand the resulting plan
// to apply.NewRunnerFromInMemoryPlan for execution.
func runTaskAction(c *cli.Context, name string) error {
	configPath, shadowed, err := resolveTasksConfigPath(c)
	if err != nil {
		var nfe *config.ErrNoConfigFound
		if errors.As(err, &nfe) {
			fmt.Fprint(os.Stderr, config.HintNoConfigFound(nfe, "task"))
			os.Exit(exitCodeValidationError)
			return nil
		}
		return err
	}

	parsed, err := config.ReadConfig(configPath)
	if err != nil {
		return err
	}

	warnIfShadowed(configPath, shadowed)
	rejectStepsInDedicatedTasksFile(configPath, parsed)

	if _, ok := parsed.Tasks[name]; !ok {
		return fmt.Errorf("task %q not found in %s (defined: %s)",
			name, configPath, joinSortedTaskNames(parsed.Tasks))
	}

	// Build the plan in-process. The planner's TaskName field does the
	// step + vars splice; everything downstream (loop expansion, secret
	// resolution, template rendering) runs against the task's body
	// exactly as if it were the top-level steps.
	planData, err := buildTaskPlan(c, configPath, name)
	if err != nil {
		return err
	}

	if c.Bool("plan") {
		return renderTaskPlan(c, planData)
	}

	return executeTaskPlan(c, configPath, name, planData)
}

// buildTaskPlan reads any --vars overlays, then builds the plan via
// plan.NewPlanner().BuildPlan with TaskName set. Shared between the
// run path and the --plan path so var-precedence and tag filtering
// stay identical across modes.
func buildTaskPlan(c *cli.Context, configPath, name string) (*plan.Plan, error) {
	variables := make(map[string]interface{})
	for _, varsPath := range c.StringSlice("vars") {
		if varsPath == "" {
			continue
		}
		vars, err := config.ReadVariables(varsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read variables from %s: %w", varsPath, err)
		}
		for k, v := range vars {
			variables[k] = v
		}
	}

	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, err
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: configPath,
		Variables:  variables,
		Tags:       cmdutil.ParseTags(c.String("tags")),
		SkipTags:   cmdutil.ParseTags(c.String("skip-tags")),
		TaskName:   name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build plan: %w", err)
	}
	return planData, nil
}

// renderTaskPlan handles `mooncake task <name> --plan`. Mirrors the
// terminal branch of planCommand: optional inspection pass, then
// format-dispatch into the shared formatPlan{Text,JSON,YAML} helpers.
// We deliberately do not write the plan to a file here — saving plans
// is `mooncake plan -o`'s job; tasks are dev loop, not the spec-16
// "build a plan now, apply it later" workflow.
func renderTaskPlan(c *cli.Context, planData *plan.Plan) error {
	// Inspection pass so the rendered plan carries would-change
	// predictions for spec-16 Runner handlers. Sudo password feeds the
	// dispatcher's preflight the same way `mooncake plan` does it.
	sudoPass, err := security.ResolvePassword(security.PasswordConfig{
		CLIPassword:    c.String("sudo-pass"),
		AskInteractive: c.Bool("ask-become-pass"),
		PasswordFile:   c.String("sudo-pass-file"),
		InsecureCLI:    c.Bool("insecure-sudo-pass"),
	})
	if err != nil {
		return fmt.Errorf("sudo password setup failed: %w", err)
	}
	internalLog := logger.NewLogger(logger.ErrorLevel)
	inspections, ierr := executor.InspectPlan(planData, sudoPass, internalLog)
	if ierr != nil {
		return fmt.Errorf("failed to inspect plan: %w", ierr)
	}
	planData.Inspections = inspections

	format := c.String("format")
	switch format {
	case "json":
		return formatPlanJSON(planData)
	case "yaml":
		return formatPlanYAML(planData)
	case "text", "":
		return formatPlanText(planData, c.Bool("show-origins"), c.Bool("diff"))
	default:
		return fmt.Errorf("unsupported --format: %s (use text, json, or yaml)", format)
	}
}

// executeTaskPlan hands the already-built plan to the in-memory apply
// runner. RootFile is the config path for runlog/op-linkage attribution;
// without it the history entry for a task run would be unattributable.
func executeTaskPlan(c *cli.Context, configPath, name string, planData *plan.Plan) error {
	opts := apply.InMemoryPlanOptions{
		SudoPass: c.String("sudo-pass"),
		LogLevel: c.String("log-level"),
		OpID:     recordOp("task "+name, configPath, false),
		RootFile: configPath,
	}
	return runWithSignalCtx(c.Context, func(ctx context.Context) error {
		_, runErr := apply.NewRunnerFromInMemoryPlan(planData, opts).Run(ctx)
		return runErr
	})
}

// warnIfShadowed emits a single stderr line when the chosen task
// config is a dedicated tasks file but the apply-config in the same
// directory ALSO defines tasks. The warning is informational, not an
// error — the user's intent (the dedicated file) is honored.
func warnIfShadowed(chosen, shadowed string) {
	if shadowed == "" {
		return
	}
	fmt.Fprintf(os.Stderr,
		"warning: %s wins; %s also defines `tasks:` and is ignored. "+
			"Move definitions into one file to silence this warning.\n",
		filepath.Base(chosen), filepath.Base(shadowed))
}

// rejectStepsInDedicatedTasksFile enforces the "tasks.yml is tasks-
// only" rule: a dedicated tasks file that also declares top-level
// `steps:` is almost certainly a user mistake (they meant for those
// steps to be a task body). We warn rather than hard-error so a file
// in progress isn't blocked, but the warning is loud.
func rejectStepsInDedicatedTasksFile(configPath string, parsed *config.ParsedConfig) {
	base := filepath.Base(configPath)
	isDedicated := false
	for _, name := range config.TasksSearchPaths {
		if base == name {
			isDedicated = true
			break
		}
	}
	if !isDedicated {
		return
	}
	if len(parsed.Steps) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr,
		"warning: %s defines top-level `steps:` outside any task — "+
			"these will be ignored by `mooncake task`. Either move them "+
			"into a task or rename the file to mooncake.yml.\n",
		base)
}

// joinSortedTaskNames returns the sorted, comma-separated list of task
// names in the parsed config, for unknown-task error messages. Mirrors
// plan.joinTaskNames but operates on the CLI-visible ParsedConfig.
func joinSortedTaskNames(tasks map[string]config.Task) string {
	if len(tasks) == 0 {
		return "<none>"
	}
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

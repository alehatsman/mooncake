// Package observe_command implements the observe.command action: run a
// command and report its exit status as an observation.
//
// Successor to wait.command. The polling that action existed for is now the
// shared `wait:` modifier, so this handler only has to run the command once
// and let actions.ObserveWait do the looping.
package observe_command

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"

	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName     = "observe.command"
	defaultTimeout = 30 * time.Second

	// outputSampleBytes caps stdout/stderr retained in the observation. The
	// point is a diagnostic sample, not a transcript.
	outputSampleBytes = 2048
)

// CommandObservation is the typed Value payload for observe.command.
type CommandObservation struct {
	Cmd        string `json:"cmd"`                   // Command that was run
	ExitCode   int    `json:"exit_code"`             // Observed exit code; -1 if it never started
	ExpectExit int    `json:"expect_exit"`           // Exit code that means "found"
	Stdout     string `json:"stdout,omitempty"`      // Sampled stdout
	Stderr     string `json:"stderr,omitempty"`      // Sampled stderr
	DurationMs int64  `json:"duration_ms,omitempty"` // Wall-clock run time
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Run a command and observe its exit code (non-zero is data, not failure)",
		Category:           actions.CategoryCommand,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      false, // running a command in plan mode is a side effect
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObserveCommand
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if strings.TrimSpace(o.Cmd) == "" {
		return fmt.Errorf("%s: cmd is required", actionName)
	}
	if o.Timeout != "" {
		if _, err := time.ParseDuration(o.Timeout); err != nil {
			return fmt.Errorf("%s: invalid timeout %q: %w", actionName, o.Timeout, err)
		}
	}
	return actions.ValidateWait(actionName, o.Wait)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveCommand

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("%s: invalid context type", actionName)
	}

	cmd, err := ctx.Template().Render(o.Cmd, ctx.Variables())
	if err != nil {
		return nil, &executor.RenderError{Field: actionName + ".cmd", Cause: err}
	}

	timeout := defaultTimeout
	if o.Timeout != "" {
		timeout, _ = time.ParseDuration(o.Timeout) // Validate guarantees this parses
	}

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Unlike the other observe handlers, this one shells out — so plan mode
	// must not probe at all, not even "deferred but harmless". A command is a
	// side effect whatever its author intended it to read.
	if ctx.Mode() == actions.ModePlan {
		envelope := actions.PlanDeferred(CommandObservation{
			Cmd: cmd, ExpectExit: o.ExpectExit, ExitCode: -1,
		})
		result.PublishObservation(envelope, cmd)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe exit code of %q (deferred to apply)", cmd)
		return result, nil
	}

	probeOnce := func() actions.ObserveResult {
		return observeCommand(ctx.Ctx(), cmd, ec.CurrentDir, o.ExpectExit, timeout)
	}

	envelope := probeOnce()
	if o.Wait != nil {
		var waitErr error
		// ObserveWait returns the last observation either way, so a timed-out
		// wait still publishes real data for a downstream `as:` capture.
		envelope, waitErr = actions.ObserveWait(ctx, actionName, cmd, o.Wait, probeOnce)
		if waitErr != nil {
			result.PublishObservation(envelope, cmd)
			return result, waitErr
		}
	}
	result.PublishObservation(envelope, cmd)

	ctx.Logger().Debugf("%s %q = found:%v", actionName, cmd, envelope.Found)
	return result, nil
}

// observeCommand runs cmd once and reports its exit status.
//
// A non-zero exit is Found=false with NO Error: the exit status is the thing
// being observed, so treating it as a probe failure would make this unusable
// for "is the service healthy yet?". Error is reserved for the command failing
// to start at all.
func observeCommand(parent context.Context, cmd, dir string, expectExit int,
	timeout time.Duration) actions.ObserveResult {

	obs := CommandObservation{Cmd: cmd, ExpectExit: expectExit, ExitCode: -1}

	// The per-attempt timeout chains onto the run context, so both SIGINT and
	// a hung command abort this attempt rather than the whole wait budget.
	runCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	start := time.Now()
	// #nosec G204 -- shell command from user config is the point of this action
	shell := exec.CommandContext(runCtx, "bash", "-c", cmd)
	shell.Dir = dir

	var stdout, stderr strings.Builder
	shell.Stdout = &stdout
	shell.Stderr = &stderr

	runErr := shell.Run()
	obs.DurationMs = time.Since(start).Milliseconds()
	obs.Stdout = sample(stdout.String())
	obs.Stderr = sample(stderr.String())

	switch {
	case runErr == nil:
		obs.ExitCode = 0
	default:
		exitErr, isExit := errors.AsType[*exec.ExitError](runErr)
		if !isExit {
			// Failed to start: missing binary, cancelled context. That IS a
			// probe failure, so it goes in Error.
			return actions.ObserveResult{
				Found: false, Value: obs, AsOf: time.Now(), Error: runErr.Error(),
			}
		}
		obs.ExitCode = exitErr.ExitCode()
	}

	return actions.ObserveResult{
		Found: obs.ExitCode == expectExit,
		Value: obs,
		AsOf:  time.Now(),
	}
}

func sample(s string) string {
	if len(s) <= outputSampleBytes {
		return s
	}
	return s[:outputSampleBytes]
}

// --- Read-only ABI specialization -------------------------------------------
//
// No Reverse method by design: an observation has nothing to undo, and a no-op
// would make `actions list` report REVERSE=yes. See specs/observe.md.
//
// Risk is 2, not the 1 every other observe handler reports: this one runs an
// arbitrary command, and honesty about that is the whole point of the ABI.

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: false, Risk: 2}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		RequiredBinaries: []string{"bash"},
		Notes: []string{
			"runs an arbitrary command; mooncake cannot know whether it mutates",
		},
	}
}

func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObserveCommand
	if o == nil {
		return actions.Diff{}, nil
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: o.Cmd,
			Attributes: map[string]string{"observe_kind": "command"},
		},
		Operation: actions.OpNoop,
	}, nil
}

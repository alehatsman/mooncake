// Package pilot provides autonomous agent functionality for iterative plan generation and execution.
package pilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pilot/llm"
	"github.com/alehatsman/mooncake/internal/snapshot"
)

const defaultMaxIterations = 5

// planGenTimeout bounds a single LLM plan-generation call. The
// previous shape relied on http.Client.Timeout=60s which silently
// truncated thinking-model runs (F040). Five minutes is generous —
// real generations finish in 5-90s — and lets long-context agent
// runs complete without a fixed-window cutoff. Operators who need a
// different budget can fork the value if/when --llm-timeout lands.
const planGenTimeout = 5 * time.Minute

type LoopResult struct {
	Iterations []IterationLog
	StopReason StopReason
	FinalLog   *IterationLog
}

func RunLoop(opts RunOptions) (*LoopResult, error) {
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = defaultMaxIterations
	}

	if opts.AutoApply {
		fmt.Fprintln(os.Stderr, AutoApplyWarning)
	}

	client, err := llm.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude client: %w", err)
	}

	var iterations []IterationLog
	var lastIteration *IterationSummary

	for i := 1; i <= opts.MaxIterations; i++ {
		iterNum, err := NextIterationNumber(opts.RepoRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to get iteration number: %w", err)
		}

		snap, err := snapshot.Collect(opts.RepoRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to collect snapshot: %w", err)
		}

		snapJSON, err := json.Marshal(snap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal snapshot: %w", err)
		}

		systemPrompt, userPrompt, err := BuildPrompt(PlanInput{
			Goal:          opts.Goal,
			Snapshot:      snapJSON,
			LastIteration: lastIteration,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build prompt: %w", err)
		}

		// F040(a): bound a single generation with a generous deadline.
		// The Claude client no longer carries a 60s overall timeout;
		// ctx is the budget. Per-iteration cancel keeps a stuck call
		// from blocking the rest of the agent loop.
		genCtx, cancelGen := context.WithTimeout(context.Background(), planGenTimeout)
		rawPlan, err := client.GeneratePlan(genCtx, systemPrompt, userPrompt, opts.Model)
		cancelGen()
		if err != nil {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, "", "generation_failed", err.Error())
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopFailed,
				FinalLog:   log,
			}, err
		}

		planBytes, err := SanitizePlan(rawPlan)
		if err != nil {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, "", "sanitization_failed", err.Error())
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopFailed,
				FinalLog:   log,
			}, err
		}

		planHash := ComputePlanHash(planBytes)

		if lastIteration != nil && planHash == lastIteration.PlanHash {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "no_progress", "plan identical to previous iteration")
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopNoProgress,
				FinalLog:   log,
			}, nil
		}

		wrappedBytes, err := WrapInTransaction(planBytes)
		if err != nil {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "wrap_failed", err.Error())
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopFailed,
				FinalLog:   log,
			}, err
		}

		log := logger.NewLogger(logger.ErrorLevel)

		// Set up the plan-confirm gate (spec-67 §10). nil gate means
		// "skip gate; behave like before" — the --auto-apply path. TTY
		// is only required when a gate is needed, so --auto-apply runs
		// stay unaffected on non-interactive shells.
		var gate planGate
		if !opts.AutoApply {
			if ttyErr := EnsureInteractive(os.Stdin); ttyErr != nil {
				return &LoopResult{Iterations: iterations, StopReason: StopFailed}, ttyErr
			}
			gate = func() (ConfirmResult, error) {
				return ConfirmPlan(os.Stdin, os.Stderr, planBytes)
			}
		}

		outcome, err := applyPlanIteration(wrappedBytes, opts.RepoRoot, log, gate)
		if err != nil {
			return nil, err
		}
		if !outcome.ValidationOK {
			errMsg := ""
			if outcome.ValidationErr != nil {
				errMsg = outcome.ValidationErr.Error()
			} else {
				errMsg = config.FormatDiagnostics(outcome.Diagnostics)
			}
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "validation_failed", errMsg)
			iterations = append(iterations, *log)
			lastIteration = &IterationSummary{
				Iteration:    iterNum,
				PlanHash:     planHash,
				Status:       "validation_failed",
				ErrorMessage: errMsg,
			}
			continue
		}

		// Plan-confirm gate ran inside applyPlanIteration. On a non-Apply
		// outcome it short-circuited execution; pick that up here and
		// either feed the iteration forward or stop the loop.
		switch outcome.GateOutcome {
		case OutcomeReject:
			const msg = "operator rejected plan at confirm gate"
			rejLog := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "user_rejected", msg)
			iterations = append(iterations, *rejLog)
			lastIteration = &IterationSummary{
				Iteration:    iterNum,
				PlanHash:     planHash,
				Status:       "user_rejected",
				ErrorMessage: msg,
			}
			continue
		case OutcomeAbort:
			const msg = "operator aborted at confirm gate"
			abLog := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "aborted", msg)
			iterations = append(iterations, *abLog)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopAborted,
				FinalLog:   abLog,
			}, nil
		}
		// On OutcomeApply (or no gate when --auto-apply), executor ran.
		// If the operator edited the plan, persist the edited bytes
		// (not the LLM's original) as the audit artifact.
		if len(outcome.EditedPlanBytes) > 0 {
			planBytes = outcome.EditedPlanBytes
		}

		execErr := outcome.ExecErr
		changedFiles := outcome.ChangedFiles
		diffStat := outcome.DiffStat

		planPath, savePlanErr := savePlan(opts.RepoRoot, iterNum, planBytes)
		if savePlanErr != nil {
			// Plan already executed via the tempfile path; failure to
			// persist the artifact is non-fatal to the iteration. Surface
			// it so the operator notices instead of seeing a silent
			// missing-artifact in the iteration log (F039).
			log.Errorf("pilot: save plan iteration %d: %v", iterNum, savePlanErr)
		}

		iterLog := &IterationLog{
			Iteration:    iterNum,
			Goal:         opts.Goal,
			PlanHash:     planHash,
			Provider:     opts.Provider,
			Model:        opts.Model,
			ChangedFiles: changedFiles,
			DiffStat:     diffStat,
		}

		if planPath != "" {
			iterLog.Artifacts = append(iterLog.Artifacts, planPath)
		}

		if execErr != nil {
			iterLog.Status = "execution_failed"
			iterLog.ExecutionError = execErr.Error()
			_, _ = WriteIterationLog(opts.RepoRoot, iterLog)
			iterations = append(iterations, *iterLog)
			lastIteration = &IterationSummary{
				Iteration:    iterNum,
				PlanHash:     planHash,
				Status:       "execution_failed",
				ChangedFiles: changedFiles,
				ErrorMessage: execErr.Error(),
			}
			continue
		}

		iterLog.Status = "success"
		_, _ = WriteIterationLog(opts.RepoRoot, iterLog)
		iterations = append(iterations, *iterLog)

		if len(changedFiles) == 0 {
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopSuccess,
				FinalLog:   iterLog,
			}, nil
		}

		lastIteration = &IterationSummary{
			Iteration:    iterNum,
			PlanHash:     planHash,
			Status:       "success",
			ChangedFiles: changedFiles,
		}
	}

	finalLog := &iterations[len(iterations)-1]
	return &LoopResult{
		Iterations: iterations,
		StopReason: StopMaxReached,
		FinalLog:   finalLog,
	}, nil
}

// SavePlan persists an LLM-generated plan to `<repoRoot>/.mooncake/
// iterations/<n>.plan.yml` and returns the path. The directory is
// created at 0700 and the file at 0600 — plan artifacts can contain
// resolved !secret values (F037 resolves markers at plan time before
// the YAML is serialized) and the operator's goal/prompt, so world-
// readable permissions on a shared host (CI runner, dev box with
// multiple users) would leak them. Matches the 0700/0600 convention
// already in use by internal/agentd/store.go for run state (F039).
//
// Returns the saved path on success, or an empty string and an error
// on failure. The error is non-fatal to the loop iteration (the plan
// already ran via the tempfile path); callers should log+continue.
func SavePlan(repoRoot string, iterNum int, planBytes []byte) (string, error) {
	dir := filepath.Join(repoRoot, ".mooncake", "iterations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create iterations dir: %w", err)
	}
	filename := filepath.Join(dir, fmt.Sprintf("%05d.plan.yml", iterNum))
	if err := os.WriteFile(filename, planBytes, 0o600); err != nil {
		return "", fmt.Errorf("write plan: %w", err)
	}
	return filename, nil
}

// savePlan is the internal call site used by RunLoop. Kept as a thin
// wrapper so the caller doesn't have to import the same package as
// itself when testing the loop with a different persistence policy.
func savePlan(repoRoot string, iterNum int, planBytes []byte) (string, error) {
	return SavePlan(repoRoot, iterNum, planBytes)
}

func writeLoopFailureLog(repoRoot string, iterNum int, opts RunOptions, planHash, status, errMsg string) *IterationLog {
	log := &IterationLog{
		Iteration:       iterNum,
		Goal:            opts.Goal,
		PlanHash:        planHash,
		Status:          status,
		Provider:        opts.Provider,
		Model:           opts.Model,
		ChangedFiles:    []string{},
		DiffStat:        DiffStat{},
		ValidationError: errMsg,
	}
	_, _ = WriteIterationLog(repoRoot, log)
	return log
}

// iterationOutcome carries one RunLoop iteration's tempfile-validate-
// execute result back to the caller. The tempfile itself is owned by
// applyPlanIteration and is gone by the time this struct returns.
type iterationOutcome struct {
	Diagnostics   []config.Diagnostic
	ValidationErr error
	ValidationOK  bool
	ExecErr       error
	ChangedFiles  []string
	DiffStat      DiffStat
	// GateOutcome carries the plan-confirm gate disposition (spec-67
	// §10). OutcomeApply (the zero value) means either the gate said
	// "y" or there was no gate (--auto-apply). On Reject/Abort,
	// executor.Start was skipped and the caller decides loop semantics.
	GateOutcome ConfirmOutcome
	// EditedPlanBytes is non-nil iff the operator picked `edit` and
	// saved a plan different from the LLM's original. The caller
	// persists this as the iteration's audit artifact instead of the
	// LLM bytes.
	EditedPlanBytes []byte
}

// planGate is the callback applyPlanIteration invokes between
// validation success and executor.Start. Returning OutcomeReject or
// OutcomeAbort short-circuits execution; OutcomeApply (possibly with
// edited PlanBytes) proceeds to executor.Start.
type planGate func() (ConfirmResult, error)

// applyPlanIteration writes wrappedBytes to a temp file, validates the
// plan, optionally runs the plan-confirm gate (spec-67 §10), runs the
// executor against the plan, and returns the outcome. The temp file
// lifecycle (create + defer-Remove) is scoped to this function —
// F039(a) — so the file is cleaned up immediately when the function
// returns, instead of accumulating on disk across iterations until
// RunLoop exits (which is what the pre-fix defer-in-for-loop pattern
// did). F039(b) — the variable-capture risk in that pattern is also
// gone by extraction, since tmpFile no longer outlives the defer.
//
// gate is the plan-confirm gate callback. Pass nil to skip the gate
// (--auto-apply path). When gate returns an edited plan, the tempfile
// is rewritten with the edited bytes before executor.Start runs.
func applyPlanIteration(wrappedBytes []byte, repoRoot string, log logger.Logger, gate planGate) (iterationOutcome, error) {
	var out iterationOutcome
	tmpFile, err := os.CreateTemp("", "mooncake-plan-*.yml")
	if err != nil {
		return out, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, writeErr := tmpFile.Write(wrappedBytes); writeErr != nil {
		return out, fmt.Errorf("failed to write temp file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return out, fmt.Errorf("failed to close temp file: %w", closeErr)
	}

	_, diagnostics, vErr := config.ReadConfigWithValidation(tmpFile.Name())
	out.Diagnostics = diagnostics
	out.ValidationErr = vErr
	if vErr != nil || config.HasErrors(diagnostics) {
		return out, nil
	}
	out.ValidationOK = true

	if gate != nil {
		gateResult, gateErr := gate()
		if gateErr != nil {
			return out, fmt.Errorf("plan-confirm gate: %w", gateErr)
		}
		out.GateOutcome = gateResult.Outcome
		if gateResult.Outcome != OutcomeApply {
			return out, nil
		}
		// Operator picked apply. If they edited the plan, re-wrap and
		// overwrite the tempfile so executor.Start runs the edited
		// version. ConfirmPlan already re-validated the edited form;
		// only the wrap step can still fail here.
		if len(gateResult.PlanBytes) > 0 {
			editedWrapped, werr := WrapInTransaction(gateResult.PlanBytes)
			if werr != nil {
				return out, fmt.Errorf("re-wrap edited plan: %w", werr)
			}
			if !bytes.Equal(editedWrapped, wrappedBytes) {
				if err := os.WriteFile(tmpFile.Name(), editedWrapped, 0o600); err != nil {
					return out, fmt.Errorf("rewrite tempfile with edited plan: %w", err)
				}
				out.EditedPlanBytes = gateResult.PlanBytes
			}
		}
	}

	publisher := events.NewPublisher()
	out.ExecErr = executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: tmpFile.Name(),
	}, log, publisher)
	publisher.Close()

	out.ChangedFiles, _ = CollectChangedFiles(repoRoot)
	out.DiffStat, _ = CollectDiffStat(repoRoot)
	return out, nil
}

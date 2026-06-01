// Package agent provides autonomous agent functionality for iterative plan generation and execution.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/agent/llm"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/snapshot"
)

const defaultMaxIterations = 5

// maxNoChangeStreak bounds how many consecutive step-style iterations may
// apply zero mutations before the loop stops with StopNoProgress (#77). A
// single no-op is allowed — the model may legitimately propose a diagnostic
// step and continue — so the guard trips on the 2nd in a row, where the
// planner is clearly not advancing. Complements the byte-identical planHash
// guard: this catches cosmetically-different but effect-free re-plans.
const maxNoChangeStreak = 2

// defaultPlanGenTimeout bounds a single LLM plan-generation call when
// the run doesn't override it. The previous shape relied on
// http.Client.Timeout=60s which silently truncated thinking-model runs
// (F040). Five minutes covers typical generations (5-90s) with headroom,
// but a thinking-heavy plan can run past it and get SIGKILLed mid-stream
// (#80) — so RunOptions.LLMTimeout lets an operator raise or lower the
// budget via `mooncake agent run --llm-timeout` / MOONCAKE_AGENT_LLM_TIMEOUT.
const defaultPlanGenTimeout = 5 * time.Minute

// planGenTimeout resolves the per-iteration LLM budget for this run: the
// explicit LLMTimeout override when positive, else the built-in default.
func (opts RunOptions) planGenTimeout() time.Duration {
	if opts.LLMTimeout > 0 {
		return opts.LLMTimeout
	}
	return defaultPlanGenTimeout
}

// newClient is the package-level LLM-client factory so tests can swap
// in a stub. Mirrors the editorRunner pattern in confirm.go.
var newClient = llm.NewClientWithOptions

type LoopResult struct {
	Iterations []IterationLog
	StopReason StopReason
	FinalLog   *IterationLog
}

// statusSeverity ranks an iteration status by how bad its outcome is.
// Higher is worse. The two terminal-good states ("success", "step_done")
// share rank 0; every other status is a non-success of escalating severity,
// with execution_failed the worst (a step actually ran and failed, possibly
// with a failed rollback on top — see #35). Unknown statuses are treated as
// a mid-grade failure so a future status string never silently reads green.
func statusSeverity(status string) int {
	switch status {
	case "success", "step_done":
		return 0
	case "no_progress":
		return 1
	case "user_rejected", "aborted":
		return 2
	case "step_contract_violation":
		return 3
	case "generation_failed", "sanitization_failed", "wrap_failed":
		return 4
	case "validation_failed":
		return 5
	case "execution_failed":
		return 6
	default:
		return 4
	}
}

// TerminalStatus returns the worst outcome across every iteration of the
// run, not just the last one (#64). The loop continues past a failed
// iteration (execution_failed / validation_failed / user_rejected), so a
// later no-op or success iteration would otherwise reset FinalLog.Status to
// "success" and mask the earlier failure — exactly the case where a consumer
// keying on agent.completed.status (moongit #110) rendered green over
// visibly-failed work. Ties keep the earliest occurrence.
func (r *LoopResult) TerminalStatus() string {
	worst := ""
	worstRank := -1
	for i := range r.Iterations {
		s := r.Iterations[i].Status
		if s == "" {
			continue
		}
		if rank := statusSeverity(s); rank > worstRank {
			worstRank = rank
			worst = s
		}
	}
	if worst == "" && r.FinalLog != nil {
		return r.FinalLog.Status
	}
	return worst
}

func RunLoop(opts RunOptions) (*LoopResult, error) {
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = defaultMaxIterations
	}

	if opts.AutoApply {
		fmt.Fprintln(os.Stderr, AutoApplyWarning)
	}

	client, err := newClient(llm.ClientOptions{
		Provider: opts.Provider,
		Endpoint: opts.Endpoint,
		Model:    opts.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Single per-loop state for the step-style bulk-approve gate.
	// Allocated unconditionally; only ConfirmPlanStep consults it.
	stepGate := &StepGateState{}

	var iterations []IterationLog
	var lastIteration *IterationSummary
	// lastFailureFingerprint detects the planner re-proposing a step that
	// already failed the same way (#71). When an iteration fails on a step
	// whose fingerprint matches the previous iteration's failure, the loop
	// stops early instead of re-running the whole plan to rediscover the
	// same error (observed: 10 identical failures exhausting max_iterations).
	var lastFailureFingerprint string
	// noChangeStreak counts consecutive step-style iterations that succeeded
	// but applied zero mutations (#77). It trips maxNoChangeStreak to stop a
	// planner that keeps emitting no-op plans; any iteration that changes
	// something resets it.
	var noChangeStreak int

	// terminate writes a failure-log entry, appends it to the run's
	// iteration list, and returns the stop result. Collapses the
	// log+append+return triplet repeated by every terminal exit below.
	terminate := func(iterNum int, planHash, reason, msg string, stop StopReason, retErr error) (*LoopResult, error) {
		log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, reason, msg)
		iterations = append(iterations, *log)
		return &LoopResult{Iterations: iterations, StopReason: stop, FinalLog: log}, retErr
	}

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
			Style:         opts.Style,
			Policy:        opts.Policy,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build prompt: %w", err)
		}

		// #74/#76: a plan-phase publisher live for the whole planner call.
		// It emits the plan.generating "started" bracket and then streams
		// planner.delta chunks as the LLM generates, so a --output-format
		// json consumer sees live content instead of dead air until
		// plan.loaded (moongit#133). Scoped to the plan phase and Closed
		// before sanitize/apply so it never races applyPlanIteration's own
		// publisher on stdout (cf. MT-53 events-drop-on-close). No-op-ish in
		// text mode for the bracket; deltas do render live there.
		planPub := events.NewPublisher()
		planPub.Subscribe(logger.NewConsoleSubscriber(logger.InfoLevel, consoleLogFormat(opts.OutputFormat), false))

		// F040(a): bound a single generation with a generous deadline.
		// The Claude client no longer carries a 60s overall timeout;
		// ctx is the budget. Per-iteration cancel keeps a stuck call
		// from blocking the rest of the agent loop. The budget is the
		// run's LLMTimeout override, or the built-in default (#80).
		genCtx, cancelGen := context.WithTimeout(context.Background(), opts.planGenTimeout())
		rawPlan, err := streamPlan(genCtx, client, planPub, iterNum, opts, systemPrompt, userPrompt)
		cancelGen()
		// Close flushes the bracket + every delta to stdout and joins the
		// publisher goroutine BEFORE applyPlanIteration's publisher can
		// start writing — keeping the two phases' stdout strictly ordered.
		planPub.Close()
		if opts.OutputFormat != OutputFormatJSON {
			// Terminate the live-streamed (un-newlined) planner output with a
			// newline so the following step markers / confirm prompt don't
			// glue onto the last delta. Must NOT run in JSON mode — a raw byte
			// on stdout would corrupt the NDJSON event stream.
			_, _ = fmt.Fprintln(os.Stdout)
		}
		if err != nil {
			return terminate(iterNum, "", "generation_failed", err.Error(), StopFailed, err)
		}

		planBytes, err := SanitizePlan(rawPlan)
		if err != nil {
			return terminate(iterNum, "", "sanitization_failed", err.Error(), StopFailed, err)
		}

		planHash := ComputePlanHash(planBytes)

		if lastIteration != nil && planHash == lastIteration.PlanHash {
			return terminate(iterNum, planHash, "no_progress", "plan identical to previous iteration", StopNoProgress, nil)
		}

		// Step-style contract enforcement (spec-67 §12.3, plan §4 + §8).
		// Helper decides between done / continue / violation; the loop
		// reacts. Extracted so RunLoop's gocyclo stays under cap.
		if opts.Style == StyleStep {
			disp, doneLog, summary := stepContractDispatch(planBytes, opts, iterNum, planHash)
			switch disp {
			case stepDispDone:
				iterations = append(iterations, *doneLog)
				return &LoopResult{
					Iterations: iterations,
					StopReason: StopStepDone,
					FinalLog:   doneLog,
				}, nil
			case stepDispViolation:
				iterations = append(iterations, *doneLog)
				lastIteration = summary
				continue
			}
		}

		wrappedBytes, err := WrapInTransaction(planBytes)
		if err != nil {
			return terminate(iterNum, planHash, "wrap_failed", err.Error(), StopFailed, err)
		}

		log := executorLogger(opts.OutputFormat)

		// Set up the plan-confirm gate (spec-67 §10). nil gate means
		// "skip gate; behave like before" — the --auto-apply path. TTY
		// is only required when a gate is needed, so --auto-apply runs
		// stay unaffected on non-interactive shells.
		var gate planGate
		if !opts.AutoApply {
			// Step-style with an already-engaged auto-approve gate can
			// skip the TTY check entirely — ConfirmPlanStep short-
			// circuits without reading stdin. Plan-style and the first
			// step call still need an interactive terminal.
			needsTTY := true
			if opts.Style == StyleStep && (stepGate.ApprovedThread || stepGate.RemainingAutoApprovals > 0) {
				needsTTY = false
			}
			if needsTTY {
				if ttyErr := EnsureInteractive(os.Stdin); ttyErr != nil {
					return &LoopResult{Iterations: iterations, StopReason: StopFailed}, ttyErr
				}
			}
			if opts.Style == StyleStep {
				gate = func() (ConfirmResult, error) {
					return ConfirmPlanStep(os.Stdin, os.Stderr, planBytes, stepGate)
				}
			} else {
				gate = func() (ConfirmResult, error) {
					return ConfirmPlan(os.Stdin, os.Stderr, planBytes)
				}
			}
		}

		outcome, err := applyPlanIteration(wrappedBytes, opts.RepoRoot, log, gate, opts.Policy, opts.OutputFormat)
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
			return terminate(iterNum, planHash, "aborted", "operator aborted at confirm gate", StopAborted, nil)
		}
		// On OutcomeApply (or no gate when --auto-apply), executor ran.
		// If the operator edited the plan, persist the edited bytes
		// (not the LLM's original) as the audit artifact.
		if len(outcome.EditedPlanBytes) > 0 {
			planBytes = outcome.EditedPlanBytes
		}

		planPath, savePlanErr := savePlan(opts.RepoRoot, iterNum, planBytes)
		if savePlanErr != nil {
			// Plan already executed via the tempfile path; failure to
			// persist the artifact is non-fatal to the iteration. Surface
			// it so the operator notices instead of seeing a silent
			// missing-artifact in the iteration log (F039).
			log.Errorf("agent: save plan iteration %d: %v", iterNum, savePlanErr)
		}

		iterLog := &IterationLog{
			Iteration:    iterNum,
			Goal:         opts.Goal,
			PlanHash:     planHash,
			Provider:     opts.Provider,
			Model:        opts.Model,
			ChangedFiles: outcome.ChangedFiles,
			DiffStat:     outcome.DiffStat,
		}
		if planPath != "" {
			iterLog.Artifacts = append(iterLog.Artifacts, planPath)
		}

		terminal, summary, fingerprint, streak := concludeIteration(opts, iterLog, &iterations, outcome, planHash, lastFailureFingerprint, noChangeStreak, log)
		if terminal != nil {
			return terminal, nil
		}
		lastFailureFingerprint = fingerprint
		noChangeStreak = streak
		lastIteration = summary
	}

	finalLog := &iterations[len(iterations)-1]
	return &LoopResult{
		Iterations: iterations,
		StopReason: StopMaxReached,
		FinalLog:   finalLog,
	}, nil
}

// streamPlan runs the planner call for one iteration, emitting the
// plan.generating "started" bracket and (when the provider implements
// llm.StreamingClient) live planner.delta events through pub, and returns the
// final plan text (#76). Providers without streaming fall back to the
// buffered GeneratePlan and get one synthetic planner.delta carrying the full
// text, so the event contract is uniform across providers. pub is owned by
// the caller, which Closes it once this returns. The loop owns the
// delta→events.Event translation here so providers stay transport-only.
func streamPlan(ctx context.Context, client llm.Client, pub events.Publisher, iterNum int, opts RunOptions, systemPrompt, userPrompt string) (string, error) {
	pub.Publish(events.Event{
		Type:      events.EventPlanGenerating,
		Timestamp: time.Now(),
		Data: events.PlanGeneratingData{
			Iteration: iterNum,
			Provider:  opts.Provider,
			Model:     opts.Model,
		},
	})

	onDelta := func(text, kind string) {
		if text == "" {
			return
		}
		pub.Publish(events.Event{
			Type:      events.EventPlannerDelta,
			Timestamp: time.Now(),
			Data: events.PlannerDeltaData{
				Iteration: iterNum,
				Text:      text,
				Kind:      kind,
			},
		})
	}

	if sc, ok := client.(llm.StreamingClient); ok {
		return sc.GeneratePlanStream(ctx, systemPrompt, userPrompt, opts.Model, onDelta)
	}

	// Buffered fallback: one synthetic delta with the full text once the
	// call returns, so a non-streaming provider still rides the planner.delta
	// contract (a consumer keying on it sees the plan, just not live).
	full, err := client.GeneratePlan(ctx, systemPrompt, userPrompt, opts.Model)
	if err != nil {
		return "", err
	}
	onDelta(full, "text")
	return full, nil
}

// concludeIteration persists this iteration's log (status + error), appends it
// to *iterations, and decides the loop's next move. It returns a non-nil
// terminal *LoopResult when the loop should stop now — a #71 repeated-failure
// short-circuit (the same step failed the same way as last time, so the
// planner isn't adapting), a plan-style success that changed nothing, or a
// step-style no-change stall (#77) — otherwise the summary to feed into the
// next prompt plus the failure fingerprint and the updated no-change streak to
// carry forward. Extracted from RunLoop to keep its cyclomatic complexity
// under cap.
func concludeIteration(
	opts RunOptions,
	iterLog *IterationLog,
	iterations *[]IterationLog,
	outcome iterationOutcome,
	planHash, lastFingerprint string,
	noChangeStreak int,
	log logger.Logger,
) (terminal *LoopResult, summary *IterationSummary, fingerprint string, newStreak int) {
	if outcome.ExecErr != nil {
		iterLog.Status = "execution_failed"
		iterLog.ExecutionError = outcome.ExecErr.Error()
		_, _ = WriteIterationLog(opts.RepoRoot, iterLog)
		*iterations = append(*iterations, *iterLog)

		// #71: stop re-running the whole plan to rediscover the same
		// failure. iterLog (with the real execution error) is already the
		// terminal log; StopNoProgress explains *why we stopped early* while
		// TerminalStatus still reports the failure.
		fingerprint = outcome.FailedStep.Fingerprint()
		if fingerprint != "" && fingerprint == lastFingerprint {
			log.Errorf("agent: step %q failed identically twice (exit %d); stopping — planner is not adapting to the failure",
				outcome.FailedStep.Name, outcome.FailedStep.ExitCode)
			return &LoopResult{Iterations: *iterations, StopReason: StopNoProgress, FinalLog: iterLog}, nil, fingerprint, 0
		}
		summary = &IterationSummary{
			Iteration:      iterLog.Iteration,
			PlanHash:       planHash,
			Status:         "execution_failed",
			ChangedFiles:   outcome.ChangedFiles,
			ErrorMessage:   outcome.ExecErr.Error(),
			LastStepStdout: outcome.LastStepStdout,
			StepSummaries:  outcome.StepSummaries,
			FailedStep:     outcome.FailedStep,
		}
		// A failed iteration isn't a successful no-op; reset the no-change
		// streak so the #77 guard only counts consecutive clean no-ops.
		return nil, summary, fingerprint, 0
	}

	// Success clears the repeated-failure tracker (fingerprint ""): a later
	// failure on the same step is a fresh problem, not a repeat.
	iterLog.Status = "success"
	_, _ = WriteIterationLog(opts.RepoRoot, iterLog)
	*iterations = append(*iterations, *iterLog)

	// A mutation resets the no-change streak; a clean no-op advances it.
	newStreak = noChangeStreak + 1
	if outcome.ChangedSteps > 0 {
		newStreak = 0
	}

	// Under --style step the goal-reached signal is an empty plan (handled
	// upstream); a no-op step is not a stop condition — the model may
	// legitimately propose a diagnostic step and continue. Only plan-style
	// treats "no files changed" as "we're done".
	if len(outcome.ChangedFiles) == 0 && opts.Style != StyleStep {
		return &LoopResult{Iterations: *iterations, StopReason: StopSuccess, FinalLog: iterLog}, nil, "", newStreak
	}

	// #77: a step-style planner that keeps proposing no-op steps never sends
	// the empty-plan done signal and would otherwise burn every iteration.
	// One no-op is tolerated above; maxNoChangeStreak consecutive means the
	// planner isn't advancing — stop with StopNoProgress. iterLog keeps its
	// real "success" status (the steps ran fine); StopReason explains the
	// early stop, mirroring the #71 repeated-failure pattern. The byte-
	// identical planHash guard (in RunLoop) only catches verbatim re-plans;
	// this catches cosmetically-different but effect-free ones.
	if opts.Style == StyleStep && newStreak >= maxNoChangeStreak {
		log.Errorf("agent: %d consecutive iterations changed nothing; stopping — planner is not making progress (no empty plan to signal done)",
			newStreak)
		return &LoopResult{Iterations: *iterations, StopReason: StopNoProgress, FinalLog: iterLog}, nil, "", newStreak
	}
	summary = &IterationSummary{
		Iteration:      iterLog.Iteration,
		PlanHash:       planHash,
		Status:         "success",
		ChangedFiles:   outcome.ChangedFiles,
		LastStepStdout: outcome.LastStepStdout,
		StepSummaries:  outcome.StepSummaries,
	}
	return nil, summary, "", newStreak
}

// createPlanTempFile creates the temp plan file the executor reads
// inside `repoRoot` itself (NOT a subdirectory) so the executor's
// plan-relative path resolution honors operator intent. The executor
// sets `CurrentDir = filepath.Dir(rootConfigFile)` and resolves plan
// paths (e.g. `file.write: { path: hello.txt }`) against that dir.
// Before this fix the tempfile lived in `$TMPDIR`, so a plan with
// `path: hello.txt` wrote to `/tmp/hello.txt` instead of the
// operator's project — the silent-wrong-path bug behind
// agent-tmpfile-cwd. Anchoring on `repoRoot` (not `repoRoot/.mooncake`)
// is the point: with `.mooncake/` we'd just trade one wrong location
// for another (the file would land in `<repo>/.mooncake/hello.txt`).
//
// The pattern keeps the `.yml` suffix so config.ReadConfigWithValidation
// sees a recognized extension. The file is short-lived — the caller
// deletes it via defer once the executor returns.
func createPlanTempFile(repoRoot string) (*os.File, error) {
	if err := os.MkdirAll(repoRoot, 0o755); err != nil { // #nosec G301 -- standard directory permissions
		return nil, fmt.Errorf("create repoRoot dir: %w", err)
	}
	return os.CreateTemp(repoRoot, ".mooncake-plan-*.yml")
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
	// LastStepStdout is the 4 KiB-tail-truncated stdout from the LAST
	// cmd/shell-family step that completed during executor.Start. The
	// caller forwards this into the next iteration's PlanInput so the
	// model sees the result of the action it proposed. Empty when the
	// plan ran no cmd/shell steps or those steps produced no stdout.
	LastStepStdout string
	// StepSummaries is the per-step summary list from stdoutCapture
	// — one line per completed step, all action types. Forwarded into
	// IterationSummary so the next prompt renders a "Step Outcomes:"
	// block under LAST ITERATION. Nil/empty when the plan ran nothing
	// captureable (validation-only / pre-gate-reject iterations).
	StepSummaries []string
	// FailedStep is the step.failed detail (name/action/exit/stderr) when
	// ExecErr != nil, captured from the executor's step.failed event. The
	// caller forwards it into the next prompt so the planner sees what
	// broke (#71). nil on success or when no step.failed was observed.
	FailedStep *FailedStepInfo
	// ChangedSteps is how many steps reported a mutation this iteration.
	// The step-style stall guard (#77) keys on zero vs non-zero, not the
	// cumulative ChangedFiles diff (which counts pre-existing runner-
	// injected files and so never reads zero in an agent container).
	ChangedSteps int
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
func applyPlanIteration(wrappedBytes []byte, repoRoot string, log logger.Logger, gate planGate, policy *executor.Policy, outputFormat string) (iterationOutcome, error) {
	var out iterationOutcome
	tmpFile, err := createPlanTempFile(repoRoot)
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
	// Part 1 — stream shell-step stdout/stderr to the operator's
	// terminal during agent apply, matching `mooncake task` (cmd/task.go
	// sets StreamStepOutput:true for the same reason). Without this
	// subscriber the executor publishes step.stdout events into the
	// void and the operator sees nothing between "applying…" and the
	// next prompt — making goal-reached-but-loop-doesn't-stop look like
	// a silent hang.
	// outputFormat threads the run's --output-format through: "json"
	// flips this to the NDJSON event stream, and (below) silences the
	// capture's terminal printout so it can't corrupt that stream.
	publisher.Subscribe(logger.NewConsoleSubscriber(logger.InfoLevel, consoleLogFormat(outputFormat), true))
	// Part 2 — capture the last cmd/shell step's stdout so the next
	// iteration's prompt can feed it back to the model (the loop-
	// termination half of this work — see output_capture.go).
	capture := newStdoutCapture(captureWriter(outputFormat))
	publisher.Subscribe(capture)
	out.ExecErr = executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: tmpFile.Name(),
		Policy:         policy,
	}, log, publisher)
	publisher.Close()
	out.LastStepStdout = capture.Last()
	out.StepSummaries = capture.Summaries()
	out.FailedStep = capture.FailedStep()
	out.ChangedSteps = capture.ChangedCount()

	out.ChangedFiles, _ = CollectChangedFiles(repoRoot)
	out.DiffStat, _ = CollectDiffStat(repoRoot)
	return out, nil
}

// stepDisposition is the three-way result of stepContractDispatch.
type stepDisposition int

const (
	// stepDispProceed: plan parsed and has exactly one step — let the
	// rest of the iteration run as normal. Also the safe fallback when
	// decode fails — the regular validation path will surface that.
	stepDispProceed stepDisposition = iota
	// stepDispDone: empty plan, the documented goal-reached signal
	// (spec-67 §12.3). Caller should write the done log and return
	// StopStepDone.
	stepDispDone
	// stepDispViolation: >1 step, contract violation (plan §8 dec. 2).
	// Caller appends the failure log and continues the loop so the
	// next prompt carries the error back to the model.
	stepDispViolation
)

func stepContractDispatch(planBytes []byte, opts RunOptions, iterNum int, planHash string) (stepDisposition, *IterationLog, *IterationSummary) {
	steps, _, _, err := decodePlan(planBytes)
	if err != nil {
		return stepDispProceed, nil, nil
	}
	switch {
	case len(steps) == 0:
		doneLog := &IterationLog{
			Iteration: iterNum,
			Goal:      opts.Goal,
			PlanHash:  planHash,
			Status:    "step_done",
			Provider:  opts.Provider,
			Model:     opts.Model,
		}
		_, _ = WriteIterationLog(opts.RepoRoot, doneLog)
		return stepDispDone, doneLog, nil
	case len(steps) > 1:
		errMsg := fmt.Sprintf("--style step requires exactly one step, got %d", len(steps))
		rejLog := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "step_contract_violation", errMsg)
		summary := &IterationSummary{
			Iteration:    iterNum,
			PlanHash:     planHash,
			Status:       "step_contract_violation",
			ErrorMessage: errMsg,
		}
		return stepDispViolation, rejLog, summary
	}
	return stepDispProceed, nil, nil
}

package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/agent/llm"
	"github.com/alehatsman/mooncake/internal/executor"
)

type Snapshot struct {
	Branch       string   `json:"branch"`
	Head         string   `json:"head"`
	Clean        bool     `json:"clean"`
	TopLevelDirs []string `json:"top_level_dirs"`
	Actions      []string `json:"actions"`
}

type IterationLog struct {
	Iteration        int      `json:"iteration"`
	Goal             string   `json:"goal"`
	PlanHash         string   `json:"plan_hash"`
	Status           string   `json:"status"`
	ChangedFiles     []string `json:"changed_files"`
	DiffStat         DiffStat `json:"diff_stat"`
	Artifacts        []string `json:"artifacts"`
	Provider         string   `json:"provider,omitempty"`
	Model            string   `json:"model,omitempty"`
	ValidationError  string   `json:"validation_error,omitempty"`
	ExecutionError   string   `json:"execution_error,omitempty"`
	AssertionsFailed int      `json:"assertions_failed,omitempty"`
}

type DiffStat struct {
	Files      int `json:"files"`
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
}

type RunOptions struct {
	Goal          string
	PlanPath      string
	UseStdin      bool
	RepoRoot      string
	Provider      string
	Endpoint      string
	Model         string
	MaxIterations int
	// AutoApply skips the plan-confirm gate (spec-67 §10). Required for
	// unattended runs (CI, scripted). Emits a warning at thread start.
	AutoApply bool
	// Style selects the planning style (spec-67 §12.3). Zero value is
	// StylePlan (the historical default).
	Style Style
	// OutputFormat selects how the run streams to stdout. "" / "text" is
	// the human-readable rendering; "json" emits the same NDJSON event
	// stream `apply --output-format json` does (one events.Event per
	// line), capped by a terminal agent.completed event. Used by
	// programmatic consumers (e.g. moongit spawns agent in a container).
	OutputFormat string
	// Policy is the per-run permissions-as-contract gate (#11) applied
	// to every plan the loop executes. nil = enforce nothing. This is
	// how an unattended agent run (moongit spawning a containerized
	// Claude) drops the shell escape hatch: Policy{DeniedActions:
	// ["shell","cmd"]} lets the model propose a shell step but the
	// executor refuses it before any side effect. See executor.Policy.
	Policy *executor.Policy
	// LLMTimeout bounds a single plan-generation call (one claude/LLM
	// invocation per iteration). Zero falls back to defaultPlanGenTimeout.
	// A thinking-heavy plan can run past the historical 5m wall; this lets
	// an operator (or a moongit-spawned run aligning to its own per-turn
	// budget) raise or lower the cutoff. See loop.go and #80.
	LLMTimeout time.Duration
	// Approver, when non-nil, replaces the built-in stdin confirm gate
	// (#102). RunLoop calls it once per generated plan, before execution,
	// with the run ctx and the plan bytes, and runs the plan iff it returns
	// OutcomeApply (optionally carrying edited bytes). This is the seam an
	// external driver (moongit) plugs into to approve over its own channel
	// instead of a TTY; #103 layers the stdin control protocol on top. The
	// ctx lets the approver block on an out-of-band decision and unblock on
	// cancellation. Ignored under AutoApply (no gate at all); nil keeps the
	// interactive stdin gate. Honored by RunLoop only — the single-shot
	// Run path keeps its own stdin gate.
	Approver func(ctx context.Context, planBytes []byte) (ConfirmResult, error)
	// Registry, when non-nil, is the action registry the loop plans and
	// executes against. nil uses the process-wide global (every CLI/agentd
	// caller). The agent-framework path sets a custom registry (built-ins +
	// the consumer's own typed handlers, e.g. moongit.issue) so those actions
	// both surface in the planner's vocabulary (BuildSchemaChunk reads this
	// registry) and resolve at execution time. Threaded into the planner and
	// executor via executor.StartConfig.Registry. See actions.GlobalRegistry
	// and the public facade package.
	Registry *actions.Registry
	// LLMClient, when non-nil, is the reasoning backend RunLoop generates
	// plans with — bypassing the Provider/Endpoint/Model resolution chain.
	// This is the seam for a fully custom or offline backend (a local
	// ollama/vLLM client, or a deterministic mock in tests) without going
	// through env/provider strings. nil keeps the default resolution from
	// Provider/Endpoint/Model. Honored by RunLoop only (the single-shot Run
	// path executes a provided plan and never calls a backend).
	LLMClient llm.Client
}

// Output format values for RunOptions.OutputFormat. Mirrors the
// text/json split apply uses (internal/apply/runner.go).
const (
	OutputFormatText = "text"
	OutputFormatJSON = "json"
)

// AgentCompletedData is the Data of the terminal events.EventAgentCompleted
// event emitted in JSON mode. It carries the same fields the text summary
// (printAgentSummary + the "Agent completed" lines) prints, so a
// programmatic consumer gets the run outcome without parsing prose.
type AgentCompletedData struct {
	Iterations   int      `json:"iterations"`
	StopReason   string   `json:"stop_reason"`
	Status       string   `json:"status"`
	DiffStat     DiffStat `json:"diff_stat"`
	ChangedFiles []string `json:"changed_files"`
}

type PlanInput struct {
	Goal          string
	Snapshot      []byte
	LastIteration *IterationSummary
	// Style picks the trailing TASK STYLE block in the system prompt
	// (spec-67 §12.3). Zero value is StylePlan.
	Style Style
	// Policy is the run's permissions-as-contract gate (#11). When set,
	// BuildPrompt renders a PERMISSIONS CONTRACT block so the planner
	// proposes plans within the same contract the executor enforces at
	// preflight. nil = no contract block (unrestricted run) — the loop's
	// opts.Policy forwarded so the contract is visible, not just enforced
	// after the fact.
	Policy *executor.Policy
	// Registry, when non-nil and not the global, makes BuildPrompt derive
	// the action-vocabulary chunk from this live registry (so consumer-
	// registered custom actions surface to the planner) instead of the
	// embedded schema.json. nil / global keeps the byte-identical default.
	Registry *actions.Registry
}

type IterationSummary struct {
	Iteration    int
	PlanHash     string
	Status       string
	ChangedFiles []string
	ErrorMessage string
	// LastStepStdout is the captured stdout from the LAST cmd/shell-
	// family step that completed during this iteration's apply (capped
	// at 4 KB tail). The next iteration's prompt surfaces it so the
	// model can decide whether the goal is answered; without this signal
	// step-style loops re-propose the same diagnostic step forever.
	// Empty when the iteration ran no cmd/shell steps or produced no
	// stdout (e.g. only file/template actions).
	LastStepStdout string
	// StepSummaries holds one short line per step that completed during
	// this iteration's apply, regardless of action type. Closes the
	// "non-cmd/shell actions produce no LLM-visible signal" gap that
	// made --style step loops re-propose the same file.write / pkg.* /
	// os.service step (PICKUP item #1, 2026-05-27): for those actions
	// LastStepStdout stays empty, so without these summaries the
	// prompt's LAST ITERATION block has nothing positive to show.
	// Each line is independently capped (~240 B); the slice is capped
	// at 30 entries with a trailing "... N more" sentinel.
	StepSummaries []string
	// FailedStep carries the step that broke this iteration's plan (the
	// step.failed event the executor emits before rolling the transaction
	// back). Non-nil only when Status == "execution_failed". Without it the
	// next planning prompt sees only the top-level execErr string and the
	// last step's *stdout* — never the failing step's stderr/exit code, so
	// the planner can't tell *what* failed and re-proposes the same plan
	// (#71). Rendered as a "Failed Step" block by BuildPrompt.
	FailedStep *FailedStepInfo
}

// FailedStepInfo is the subset of events.StepFailedData the agent feeds back
// to the planner. Captured from the step.failed event (see output_capture.go)
// and rendered under LAST ITERATION so the next plan can course-correct
// instead of reproducing the failing step (#71).
type FailedStepInfo struct {
	Name     string
	Action   string
	ExitCode int
	// Stderr is tail-truncated to the prompt budget (agentStdoutTailBytes).
	Stderr string
	// Message is the executor's error string (events.StepFailedData.ErrorMessage).
	Message string
}

// Fingerprint is the stable signature used to detect "the same step failed
// the same way again". Deliberately excludes stderr — that often carries
// volatile timestamps/paths/PIDs that would defeat the match — and keys on
// the planner-chosen step name, action, and exit code, which stay constant
// when the planner re-proposes the same failing step (#71). Empty receiver
// returns "".
func (f *FailedStepInfo) Fingerprint() string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%s\x00%s\x00%d", f.Name, f.Action, f.ExitCode)
}

type StopReason string

const (
	StopSuccess    StopReason = "success"
	StopNoProgress StopReason = "no_progress"
	StopNoChange   StopReason = "no_change"
	StopFailed     StopReason = "failed"
	StopMaxReached StopReason = "max_iterations"
	// StopAborted is set when the operator picks `abort` at the plan-
	// confirm gate (spec-67 §10).
	StopAborted StopReason = "aborted"
	// StopStepDone fires under --style step when the model emits an
	// empty plan, the documented "goal reached" signal (spec-67 §12.3).
	StopStepDone StopReason = "step_done"
	// StopCanceled fires when the run's context is cancelled — an
	// operator Ctrl-C, a moongit-driven `stop` control message, or a
	// parent timeout. The loop stops at the next safe point: between
	// iterations it returns immediately; mid-apply the executor stops
	// between steps and (for an in-step interrupt) runs the
	// transaction's LIFO rollback before this fires (#101).
	StopCanceled StopReason = "canceled"
)

package pilot

import (
	"fmt"

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
	// line), capped by a terminal pilot.completed event. Used by
	// programmatic consumers (e.g. moongit spawns pilot in a container).
	OutputFormat string
	// Policy is the per-run permissions-as-contract gate (#11) applied
	// to every plan the loop executes. nil = enforce nothing. This is
	// how an unattended agent run (moongit spawning a containerized
	// Claude) drops the shell escape hatch: Policy{DeniedActions:
	// ["shell","cmd"]} lets the model propose a shell step but the
	// executor refuses it before any side effect. See executor.Policy.
	Policy *executor.Policy
}

// Output format values for RunOptions.OutputFormat. Mirrors the
// text/json split apply uses (internal/apply/runner.go).
const (
	OutputFormatText = "text"
	OutputFormatJSON = "json"
)

// PilotCompletedData is the Data of the terminal events.EventPilotCompleted
// event emitted in JSON mode. It carries the same fields the text summary
// (printPilotSummary + the "Pilot completed" lines) prints, so a
// programmatic consumer gets the run outcome without parsing prose.
type PilotCompletedData struct {
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

// FailedStepInfo is the subset of events.StepFailedData the pilot feeds back
// to the planner. Captured from the step.failed event (see output_capture.go)
// and rendered under LAST ITERATION so the next plan can course-correct
// instead of reproducing the failing step (#71).
type FailedStepInfo struct {
	Name     string
	Action   string
	ExitCode int
	// Stderr is tail-truncated to the prompt budget (pilotStdoutTailBytes).
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
)

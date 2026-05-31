package pilot

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

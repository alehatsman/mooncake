// Package events provides the event system for Mooncake execution lifecycle.
// Events are emitted during execution and consumed by subscribers for logging,
// artifacts, and observability.
package events

import (
	"time"
)

// Event represents a single event in the execution lifecycle
type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// EventType identifies the type of event
type EventType string

// Event types for run lifecycle
const (
	EventRunStarted   EventType = "run.started"
	EventPlanLoaded   EventType = "plan.loaded"
	EventRunCompleted EventType = "run.completed"
)

// Event types for step lifecycle
const (
	EventStepStarted   EventType = "step.started"
	EventStepCompleted EventType = "step.completed"
	EventStepSkipped   EventType = "step.skipped"
	EventStepFailed    EventType = "step.failed"
	EventStepChecked   EventType = "step.checked"
)

// Event types for output streaming
const (
	EventStepStdout EventType = "step.stdout"
	EventStepStderr EventType = "step.stderr"
	EventStepDebug  EventType = "step.debug"
)

// Event types for file operations
const (
	EventFileCreated        EventType = "file.created"
	EventFileUpdated        EventType = "file.updated"
	EventFileRemoved        EventType = "file.removed"
	EventFileCopied         EventType = "file.copied"
	EventFileDownloaded     EventType = "file.downloaded"
	EventHTTPRequested      EventType = "http.requested"
	EventDirCreated         EventType = "directory.created"
	EventDirRemoved         EventType = "directory.removed"
	EventLinkCreated        EventType = "link.created"
	EventPermissionsChanged EventType = "permissions.changed"
	EventTemplateRender     EventType = "template.rendered"
	EventArchiveExtracted   EventType = "archive.extracted"
)

// Event types for variables
const (
	EventVarsSet    EventType = "variables.set"
	EventVarsLoaded EventType = "variables.loaded"
)

// Event types for service management
const (
	EventServiceManaged EventType = "service.managed"
)

// Event types for package management
const (
	EventPackageManaged EventType = "package.managed"
)

// Event types for assertions
const (
	EventAssertPassed EventType = "assert.passed"
	EventAssertFailed EventType = "assert.failed"
)

// Event types for presets
const (
	EventPresetExpanded  EventType = "preset.expanded"
	EventPresetCompleted EventType = "preset.completed"
)

// Event types for agent loop
const (
	EventAgentIterationStarted EventType = "agent.iteration.started"
	EventAgentPlanGenerated    EventType = "agent.plan.generated"
	EventAgentPlanValidating   EventType = "agent.plan.validating"
	EventAgentPlanValid        EventType = "agent.plan.valid"
	EventAgentPlanInvalid      EventType = "agent.plan.invalid"
	EventAgentPlanExecuting    EventType = "agent.plan.executing"
	EventAgentIterationDone    EventType = "agent.iteration.done"
	EventAgentLoopComplete     EventType = "agent.loop.complete"
)

// Event types for artifact capture
const (
	EventArtifactCaptureStart    EventType = "artifact_capture.start"
	EventArtifactCaptureComplete EventType = "artifact_capture.complete"
)

// Event types for print
const (
	EventPrintMessage EventType = "print.message"
)

// Event types for transaction rollback (F054 / spec-30).
//
// Spec-30 §"Key files" promised a six-event surface for transaction
// lifecycle (begin / commit / rollback_begin / step_reversed /
// rollback_complete / rollback_failed); the implementation shipped
// the executor-side semantics but never wired the event emit. F054
// closes the visibility half: four events at natural emit boundaries
// inside handleTxnBodyFailure + the inverse-step dispatch path. The
// two missing events (TransactionBegin / TransactionCommit) need
// compound-parent step.started semantics that don't exist yet —
// deferred until the executor exposes per-compound-parent lifecycle.
const (
	EventTransactionRollbackBegin    EventType = "transaction.rollback_begin"
	EventTransactionStepReversed     EventType = "transaction.step_reversed"
	EventTransactionRollbackComplete EventType = "transaction.rollback_complete"
	EventTransactionRollbackFailed   EventType = "transaction.rollback_failed"
)

// RunStartedData contains data for run.started events
type RunStartedData struct {
	RootFile   string   `json:"root_file"`
	Tags       []string `json:"tags,omitempty"`
	DryRun     bool     `json:"dry_run"`
	TotalSteps int      `json:"total_steps"`
}

// PlanLoadedData contains data for plan.loaded events
type PlanLoadedData struct {
	RootFile   string   `json:"root_file"`
	TotalSteps int      `json:"total_steps"`
	Tags       []string `json:"tags,omitempty"`
}

// RunCompletedData contains data for run.completed events
type RunCompletedData struct {
	TotalSteps   int `json:"total_steps"`
	SuccessSteps int `json:"success_steps"`
	// OkSteps counts steps that completed successfully without
	// changing the system (F6). Mutually exclusive with ChangedSteps
	// at the executor's decision sites; for the typical case
	// OkSteps + ChangedSteps == SuccessSteps. Lifts the renderer-side
	// derivation onto a first-class field so downstream consumers
	// (agentd, MCP, SDK) don't have to subtract.
	OkSteps      int `json:"ok_steps,omitempty"`
	FailedSteps  int `json:"failed_steps"`
	SkippedSteps int `json:"skipped_steps"`
	ChangedSteps int `json:"changed_steps"`
	// RevertedSteps counts steps whose Changed=true result was undone
	// by a transaction's LIFO Reverse() pass. Already subtracted from
	// ChangedSteps so callers don't double-count (MT-45).
	RevertedSteps int `json:"reverted_steps,omitempty"`
	// CancelledSteps counts steps interrupted mid-execution (SIGINT,
	// fleet kill, timeout) per proposal-02. Tracked separately from
	// FailedSteps so the exit-code aggregator can map cancelled>0 to
	// 130 (the SIGINT-equivalent exit) rather than a generic failure.
	CancelledSteps int `json:"cancelled_steps,omitempty"`
	// HealedSteps counts assert steps that failed on first dispatch,
	// ran their declared heal: child plan, and passed the re-check
	// (proposal-11). Distinct from FailedSteps and ChangedSteps —
	// the assert step itself never changes state, but a non-zero
	// HealedSteps means the system was drifting and was restored
	// in-band.
	HealedSteps  int    `json:"healed_steps,omitempty"`
	DurationMs   int64  `json:"duration_ms"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	CheckMode    bool   `json:"check_mode,omitempty"`
}

// StepStartedData contains data for step.started events
type StepStartedData struct {
	StepID     string            `json:"step_id"`
	Name       string            `json:"name"`
	Level      int               `json:"level"`
	GlobalStep int               `json:"global_step"`
	Action     string            `json:"action"`
	Tags       []string          `json:"tags,omitempty"`
	When       string            `json:"when,omitempty"`
	Vars       map[string]string `json:"vars,omitempty"`
	Depth      int               `json:"depth,omitempty"` // Directory depth for filetree items
	DryRun     bool              `json:"dry_run"`
	// TriggeredBy is set on steps expanded from an on_change child
	// (spec-23 §1). Carries the parent step's ID so consumers can render
	// parent→child relationships in logs / UIs / replays.
	TriggeredBy string `json:"triggered_by,omitempty"`
	// TryParent / TryRole are set on steps expanded from a try-block
	// branch (spec-23 §2). TryParent is the compound parent's step ID;
	// TryRole is one of "try", "catch", "finally". Consumers can render
	// the compound shape and attribute outcomes back to the wrapper.
	TryParent string `json:"try_parent,omitempty"`
	TryRole   string `json:"try_role,omitempty"`
}

// StepCompletedData contains data for step.completed events
type StepCompletedData struct {
	StepID     string                 `json:"step_id"`
	Name       string                 `json:"name"`
	Level      int                    `json:"level"`
	DurationMs int64                  `json:"duration_ms"`
	Changed    bool                   `json:"changed"`
	Result     map[string]interface{} `json:"result,omitempty"`
	Depth      int                    `json:"depth,omitempty"` // Directory depth for filetree items
	DryRun     bool                   `json:"dry_run"`
	// TriggeredBy mirrors StepStartedData.TriggeredBy. Repeating it on
	// completed events lets log-only consumers attribute changes back to
	// the triggering parent without holding state across event types.
	TriggeredBy string `json:"triggered_by,omitempty"`
	// TryParent / TryRole mirror StepStartedData (spec-23 §2).
	TryParent string `json:"try_parent,omitempty"`
	TryRole   string `json:"try_role,omitempty"`
}

// StepSkippedData contains data for step.skipped events
type StepSkippedData struct {
	StepID string `json:"step_id"`
	Name   string `json:"name"`
	Level  int    `json:"level"`
	Reason string `json:"reason"`
	Depth  int    `json:"depth,omitempty"` // Directory depth for filetree items
	// TriggeredBy mirrors the started/completed fields. Most useful here
	// for the "parent didn't change → skipped" path: the consumer sees
	// both the reason string and the structured parent reference.
	TriggeredBy string `json:"triggered_by,omitempty"`
	// TryParent / TryRole mirror StepStartedData (spec-23 §2). Most
	// useful here for "try-block already failed" / "try succeeded
	// (catch skipped)" paths.
	TryParent string `json:"try_parent,omitempty"`
	TryRole   string `json:"try_role,omitempty"`
}

// StepFailedData contains data for step.failed events
type StepFailedData struct {
	StepID       string `json:"step_id"`
	Name         string `json:"name"`
	Action       string `json:"action,omitempty"`
	Level        int    `json:"level"`
	ErrorMessage string `json:"error_message"`
	ExitCode     int    `json:"exit_code"`
	Stdout       string `json:"stdout,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	DurationMs   int64  `json:"duration_ms"`
	Depth        int    `json:"depth,omitempty"` // Directory depth for filetree items
	DryRun       bool   `json:"dry_run"`
}

// StepOutputData contains data for step.stdout/stderr events
type StepOutputData struct {
	StepID     string `json:"step_id"`
	Stream     string `json:"stream"` // "stdout" or "stderr"
	Line       string `json:"line"`
	LineNumber int    `json:"line_number"`
}

// FileOperationData contains data for file operation events
type FileOperationData struct {
	Path           string `json:"path"`
	Mode           string `json:"mode,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`  // After-write size (synonym for SizeAfter)
	SizeBefore     int64  `json:"size_before,omitempty"` // Pre-write size; 0 for new files
	Changed        bool   `json:"changed"`
	DryRun         bool   `json:"dry_run"`
	ChecksumBefore string `json:"checksum_before,omitempty"` // SHA256 before modification (empty for new files)
	ChecksumAfter  string `json:"checksum_after,omitempty"`  // SHA256 after modification
	// ContentBefore carries the pre-write bytes for downstream consumers
	// (e.g. artifact.capture with capture_content:true) that want before/
	// after diffs without re-reading the file off disk. Not serialised into
	// the JSON event stream — too bulky and only useful in-process.
	ContentBefore []byte `json:"-"`
	// ContentAfter mirrors ContentBefore for symmetry. Same JSON-omit reasoning.
	ContentAfter []byte `json:"-"`
}

// FileRemovedData contains data for file/directory removal events
type FileRemovedData struct {
	Path      string `json:"path"`
	WasDir    bool   `json:"was_dir"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	DryRun    bool   `json:"dry_run"`
}

// LinkCreatedData contains data for link creation events
type LinkCreatedData struct {
	Src    string `json:"src"`
	Dest   string `json:"dest"`
	Type   string `json:"type"` // "symlink" or "hardlink"
	DryRun bool   `json:"dry_run"`
}

// PermissionsChangedData contains data for permissions.changed events
type PermissionsChangedData struct {
	Path      string `json:"path"`
	Mode      string `json:"mode,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Group     string `json:"group,omitempty"`
	Recursive bool   `json:"recursive"`
	DryRun    bool   `json:"dry_run"`
}

// FileCopiedData contains data for file.copied events
type FileCopiedData struct {
	Src       string `json:"src"`
	Dest      string `json:"dest"`
	SizeBytes int64  `json:"size_bytes"`
	Mode      string `json:"mode"`
	Checksum  string `json:"checksum,omitempty"`
	DryRun    bool   `json:"dry_run"`
}

// FileDownloadedData contains data for file.downloaded events
type FileDownloadedData struct {
	URL       string `json:"url"`
	Dest      string `json:"dest"`
	SizeBytes int64  `json:"size_bytes"`
	Mode      string `json:"mode"`
	Checksum  string `json:"checksum,omitempty"`
	DryRun    bool   `json:"dry_run"`
}

// HTTPRequestedData carries event payload for http.requested. proposal-16
// keeps this deliberately minimal: scheme/host/path (no query string),
// method, status, duration. The request and response bodies live in the
// registered fact, not in the audit event — bodies may contain secrets,
// PII, or large payloads that don't belong in the event stream. Auth
// headers and other sensitive headers are already redacted before the
// event is emitted.
type HTTPRequestedData struct {
	Method     string `json:"method"`
	URL        string `json:"url"`         // scheme://host/path (no query)
	StatusCode int    `json:"status_code"` // 0 if request never received a response
	DurationMs int64  `json:"duration_ms"`
	Attempts   int    `json:"attempts"`
	DryRun     bool   `json:"dry_run"`
}

// TemplateRenderData contains data for template.rendered events
type TemplateRenderData struct {
	TemplatePath string `json:"template_path"`
	DestPath     string `json:"dest_path"`
	SizeBytes    int64  `json:"size_bytes"`
	Changed      bool   `json:"changed"`
	DryRun       bool   `json:"dry_run"`
}

// VarsSetData contains data for variables.set events
type VarsSetData struct {
	Count  int      `json:"count"`
	Keys   []string `json:"keys"`
	DryRun bool     `json:"dry_run"`
}

// VarsLoadedData contains data for variables.loaded events
type VarsLoadedData struct {
	FilePath string   `json:"file_path"`
	Count    int      `json:"count"`
	Keys     []string `json:"keys"`
	DryRun   bool     `json:"dry_run"`
}

// ArchiveExtractedData contains data for archive.extracted events
type ArchiveExtractedData struct {
	Src             string `json:"src"`
	Dest            string `json:"dest"`
	Format          string `json:"format"`
	FilesExtracted  int    `json:"files_extracted"`
	DirsCreated     int    `json:"dirs_created"`
	BytesExtracted  int64  `json:"bytes_extracted"`
	StripComponents int    `json:"strip_components,omitempty"`
	DurationMs      int64  `json:"duration_ms"`
	DryRun          bool   `json:"dry_run"`
}

// ServiceManagementData contains data for service.managed events
type ServiceManagementData struct {
	Service    string   `json:"service"`              // Service name
	State      string   `json:"state,omitempty"`      // Desired state (started/stopped/restarted/reloaded)
	Enabled    *bool    `json:"enabled,omitempty"`    // Enabled status
	Changed    bool     `json:"changed"`              // Whether changes were made
	Operations []string `json:"operations,omitempty"` // List of operations performed
	DryRun     bool     `json:"dry_run"`
}

// AssertionData contains data for assert.passed and assert.failed events
type AssertionData struct {
	Type     string `json:"type"`              // Assertion type: "command", "file", or "http"
	Expected string `json:"expected"`          // What was expected
	Actual   string `json:"actual"`            // What was found
	Failed   bool   `json:"failed"`            // Whether the assertion failed
	StepID   string `json:"step_id,omitempty"` // Step ID (added by event bus)
}

// PresetData contains data for preset events
type PresetData struct {
	Name       string                 `json:"name"`                 // Preset name
	Parameters map[string]interface{} `json:"parameters,omitempty"` // Parameters passed to preset
	StepsCount int                    `json:"steps_count"`          // Number of steps in preset
	Changed    bool                   `json:"changed,omitempty"`    // Whether any step changed (only in completed event)
}

// ArtifactCaptureData contains data for artifact_capture events
type ArtifactCaptureData struct {
	Name         string `json:"name"`                    // Artifact name
	OutputDir    string `json:"output_dir"`              // Output directory path
	StepsCount   int    `json:"steps_count"`             // Number of steps executed
	FilesChanged int    `json:"files_changed,omitempty"` // Number of files changed (only in complete event)
	DurationMs   int64  `json:"duration_ms,omitempty"`   // Duration in milliseconds (only in complete event)
}

// PrintData contains data for print.message events.
//
// Message is the fully-rendered output line(s) — always populated and
// preserves the legacy contract for consumers that only care about the
// human-readable form.
//
// Title, Format, and Data carry the structured-log payload from the
// epic-read-and-report D3 deliverable. They are nil/empty for legacy
// msg-only steps, and populated when the YAML used the structured form
// (title / data / format). Agent JSONL consumers can read Data to get
// the typed payload without re-parsing Message.
type PrintData struct {
	Message string `json:"message"`
	Title   string `json:"title,omitempty"`
	Format  string `json:"format,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// PackageManagedData contains data for package.managed events
type PackageManagedData struct {
	Manager        string   `json:"manager"`
	Installed      []string `json:"installed,omitempty"`
	AlreadyPresent []string `json:"already_present,omitempty"`
	Removed        []string `json:"removed,omitempty"`
}

// StepCheckedData contains data for step.checked events (check mode)
type StepCheckedData struct {
	StepID      string `json:"step_id"`
	Name        string `json:"name"`
	Action      string `json:"action"`
	WouldChange bool   `json:"would_change"`
	Checkable   bool   `json:"checkable"`
	Reason      string `json:"reason,omitempty"`
	Level       int    `json:"level"`
	Depth       int    `json:"depth,omitempty"`
	// Detail carries action-specific plan data (e.g. effects.ContentDiff).
	Detail any `json:"detail,omitempty"`
	// Diff is the spec-22 structural delta when the dispatching handler
	// implements actions.Differ. Carried as `any` so this package
	// doesn't import internal/actions; the executor populates it with
	// a `*actions.Diff`, the inspectionCollector type-asserts back.
	// Empty (nil) for handlers that haven't opted into Differ.
	Diff any `json:"diff,omitempty"`

	// Cost is the spec-22 phase 6 informational cost estimate when
	// the dispatching handler implements actions.Coster. Same any-
	// typing rationale as Diff above (events doesn't import actions);
	// the executor stashes a `*actions.CostEstimate`, the
	// inspectionCollector type-asserts back. Nil when the handler
	// doesn't implement Coster.
	Cost any `json:"cost,omitempty"`
}

// TransactionRollbackBeginData fires once at the start of a
// transaction's LIFO rollback walk — i.e. the first time a body
// child fails. Carries the failed step's identity + the originating
// error so machine-readable consumers (runlog, agent telemetry) can
// trace which body step triggered the unwind.
type TransactionRollbackBeginData struct {
	TxnParentID    string `json:"txn_parent_id"`
	FailedStepID   string `json:"failed_step_id"`
	FailedStepName string `json:"failed_step_name,omitempty"`
	ErrorMessage   string `json:"error_message"`
	// CompletedSteps is the count of body children that ran to
	// completion before the failure — i.e. the count of inverse
	// steps the rollback will attempt. Lets consumers pre-allocate
	// or size progress UI.
	CompletedSteps int `json:"completed_steps"`
}

// TransactionStepReversedData fires after each successful inverse
// step dispatch during rollback. Identifies the ORIGINAL body
// step (the one whose effect just got undone), not the inverse step
// — readers of `mooncake history` care which user-authored step
// was reverted, not which synthesized inverse handler ran.
type TransactionStepReversedData struct {
	TxnParentID string `json:"txn_parent_id"`
	StepID      string `json:"step_id"`
	Name        string `json:"name,omitempty"`
	// Action is the original step's action type (e.g. "file.write").
	// The inverse's action is usually the same but with a different
	// shape (state=absent for a create, state=present with captured
	// bytes for a delete). Identifying via the original lets log
	// readers correlate to the original step in the plan.
	Action string `json:"action,omitempty"`
	// DurationMs is how long the inverse Run took.
	DurationMs int64 `json:"duration_ms"`
}

// TransactionRollbackCompleteData fires at the end of a fully-
// successful rollback (every previously-completed body child got
// reversed cleanly).
type TransactionRollbackCompleteData struct {
	TxnParentID   string `json:"txn_parent_id"`
	ReversedSteps int    `json:"reversed_steps"`
}

// TransactionRollbackFailedData fires when at least one Reverse()
// failed during the LIFO walk. ReversedSteps counts the inverses
// that ran successfully BEFORE the first failure — i.e. the steps
// that ARE rolled back. The system state past that step is
// indeterminate; the spec-30 "ROLLBACK INCOMPLETE — manual
// intervention required" UX is built from this event.
type TransactionRollbackFailedData struct {
	TxnParentID           string `json:"txn_parent_id"`
	ReversedSteps         int    `json:"reversed_steps"`
	FailedReverseStepID   string `json:"failed_reverse_step_id"`
	FailedReverseStepName string `json:"failed_reverse_step_name,omitempty"`
	ErrorMessage          string `json:"error_message"`
}

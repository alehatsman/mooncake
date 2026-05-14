package plan

import (
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Plan represents a fully expanded, deterministic execution plan.
//
// Spec 16 adds:
//   - Inspections: per-step state predictions
//   - GeneratedOn: host facts subset for stale-plan detection
//   - InputFiles + InputFilesHash: source-file integrity check so
//     `apply --from-plan` refuses to run plans that no longer match
//     the YAML they were built from.
type Plan struct {
	Version        string                 `json:"version" yaml:"version"`
	GeneratedAt    time.Time              `json:"generated_at" yaml:"generated_at"`
	GeneratedOn    HostFacts              `json:"generated_on,omitempty" yaml:"generated_on,omitempty"`
	RootFile       string                 `json:"root_file" yaml:"root_file"`
	InputFiles     []string               `json:"input_files,omitempty" yaml:"input_files,omitempty"`
	InputFilesHash string                 `json:"input_files_hash,omitempty" yaml:"input_files_hash,omitempty"`
	Steps          []config.Step          `json:"steps" yaml:"steps"`
	Inspections    []StepInspection       `json:"inspections,omitempty" yaml:"inspections,omitempty"`
	InitialVars    map[string]interface{} `json:"initial_vars,omitempty" yaml:"initial_vars,omitempty"`
	Tags           []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// StepInspection is the result of running a handler in ModePlan against
// a single step. One inspection per Plan.Steps entry (matched by StepID).
type StepInspection struct {
	StepID      string `json:"step_id" yaml:"step_id"`
	ActionType  string `json:"action_type,omitempty" yaml:"action_type,omitempty"`
	WouldChange bool   `json:"would_change" yaml:"would_change"`
	Checkable   bool   `json:"checkable" yaml:"checkable"`
	Reason      string `json:"reason,omitempty" yaml:"reason,omitempty"`
	// Skipped reflects when/tag filtering decisions made at plan time.
	// Skipped steps have WouldChange=false and Reason explains why.
	Skipped bool `json:"skipped,omitempty" yaml:"skipped,omitempty"`
	// Detail carries action-specific plan data (e.g. effects.ContentDiff for file writes).
	Detail any `json:"detail,omitempty" yaml:"detail,omitempty"`

	// Diff is the spec-22 structural per-step delta when the handler
	// implements actions.Differ. nil for handlers that haven't opted
	// in (assert, shell, cmd, etc.) — we deliberately don't synthesize
	// a default-Differ Diff here because the coarse "Operation=update,
	// Resource.Kind=other" fallback would appear on every non-Differ
	// step and add noise to JSON output without information.
	//
	// Consumers wanting structured information about steps that don't
	// implement Differ should look at Reason + Detail instead.
	Diff *actions.Diff `json:"diff,omitempty" yaml:"diff,omitempty"`
}

// HostFacts captures the minimum set of facts needed to detect a
// stale plan being applied on the wrong host. Spec 16's stale-plan
// policy compares these at apply time and refuses on mismatch unless
// --allow-stale is set.
//
// Deliberately small. Hostname, kernel version, package versions etc.
// are too strict and would make plans annoyingly unportable across
// similar dev machines.
type HostFacts struct {
	OsFamily     string `json:"os_family,omitempty" yaml:"os_family,omitempty"`
	Arch         string `json:"arch,omitempty" yaml:"arch,omitempty"`
	DistroFamily string `json:"distro_family,omitempty" yaml:"distro_family,omitempty"`
}

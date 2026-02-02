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
)

// Event types for output streaming
const (
	EventStepStdout EventType = "step.stdout"
	EventStepStderr EventType = "step.stderr"
	EventStepDebug  EventType = "step.debug"
)

// Event types for file operations
const (
	EventFileCreated         EventType = "file.created"
	EventFileUpdated         EventType = "file.updated"
	EventFileRemoved         EventType = "file.removed"
	EventFileCopied          EventType = "file.copied"
	EventFileDownloaded      EventType = "file.downloaded"
	EventDirCreated          EventType = "directory.created"
	EventDirRemoved          EventType = "directory.removed"
	EventLinkCreated         EventType = "link.created"
	EventPermissionsChanged  EventType = "permissions.changed"
	EventTemplateRender      EventType = "template.rendered"
	EventArchiveExtracted    EventType = "archive.extracted"
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

// Event types for Ollama management
const (
	EventOllamaManaged     EventType = "ollama.managed"
	EventOllamaInstalled   EventType = "ollama.installed"
	EventOllamaRemoved     EventType = "ollama.removed"
	EventOllamaModelPulled EventType = "ollama.model_pulled"
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
	TotalSteps    int    `json:"total_steps"`
	SuccessSteps  int    `json:"success_steps"`
	FailedSteps   int    `json:"failed_steps"`
	SkippedSteps  int    `json:"skipped_steps"`
	ChangedSteps  int    `json:"changed_steps"`
	DurationMs    int64  `json:"duration_ms"`
	Success       bool   `json:"success"`
	ErrorMessage  string `json:"error_message,omitempty"`
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
}

// StepSkippedData contains data for step.skipped events
type StepSkippedData struct {
	StepID string `json:"step_id"`
	Name   string `json:"name"`
	Level  int    `json:"level"`
	Reason string `json:"reason"`
	Depth  int    `json:"depth,omitempty"` // Directory depth for filetree items
}

// StepFailedData contains data for step.failed events
type StepFailedData struct {
	StepID       string `json:"step_id"`
	Name         string `json:"name"`
	Level        int    `json:"level"`
	ErrorMessage string `json:"error_message"`
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
	Path      string `json:"path"`
	Mode      string `json:"mode,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Changed   bool   `json:"changed"`
	DryRun    bool   `json:"dry_run"`
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

// OllamaData contains data for ollama.managed events
type OllamaData struct {
	State          string   `json:"state"`                     // present|absent
	ServiceEnabled bool     `json:"service_enabled"`           // Service was configured/started
	Method         string   `json:"method,omitempty"`          // Installation method used (script|package)
	ModelsDir      string   `json:"models_dir,omitempty"`      // Custom models directory
	ModelsPulled   []string `json:"models_pulled,omitempty"`   // Models that were pulled (changed)
	ModelsSkipped  []string `json:"models_skipped,omitempty"`  // Models already present (unchanged)
	Operations     []string `json:"operations,omitempty"`      // Operations performed
}

// AssertionData contains data for assert.passed and assert.failed events
type AssertionData struct {
	Type     string `json:"type"`               // Assertion type: "command", "file", or "http"
	Expected string `json:"expected"`           // What was expected
	Actual   string `json:"actual"`             // What was found
	Failed   bool   `json:"failed"`             // Whether the assertion failed
	StepID   string `json:"step_id,omitempty"`  // Step ID (added by event bus)
}

// PresetData contains data for preset events
type PresetData struct {
	Name       string                 `json:"name"`                  // Preset name
	Parameters map[string]interface{} `json:"parameters,omitempty"`  // Parameters passed to preset
	StepsCount int                    `json:"steps_count"`           // Number of steps in preset
	Changed    bool                   `json:"changed,omitempty"`     // Whether any step changed (only in completed event)
}

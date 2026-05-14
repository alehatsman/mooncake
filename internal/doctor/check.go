// Package doctor runs a fixed battery of health checks and renders a single
// scannable report. See spec-41-mooncake-doctor.md.
package doctor

import "context"

// Status is one of OK / Info / Warning / Error, in increasing order of
// severity. Higher values indicate worse outcomes; consumers can compare
// numerically.
type Status int

const (
	StatusOK Status = iota
	StatusInfo
	StatusWarning
	StatusError
)

// String renders the status as a stable lowercase token used in JSON output.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusInfo:
		return "info"
	case StatusWarning:
		return "warning"
	case StatusError:
		return "error"
	}
	return "unknown"
}

// Result is one check's outcome. Fields are JSON-marshalable; field names
// are the public contract for `doctor --format json` consumers (CI tooling).
type Result struct {
	Section string   `json:"section"`
	Name    string   `json:"name"`
	Status  Status   `json:"-"`
	StatusS string   `json:"status"`
	Message string   `json:"message"`
	Detail  string   `json:"detail,omitempty"`
	Fix     string   `json:"fix,omitempty"`
	UsedBy  []string `json:"used_by,omitempty"`
}

// Check is the contract every check satisfies. Section + Name uniquely
// identify a check in the catalogue and drive deterministic ordering.
type Check interface {
	Section() string
	Name() string
	Run(ctx Context) Result
}

// Context bundles shared state passed to every Check.Run call. Heavy
// resources (facts, preset paths) are collected once at startup and shared
// here so individual checks don't re-do work.
type Context struct {
	Ctx         context.Context
	HomeDir     string   // absolute path to ~/.mooncake/ (may not exist yet)
	Cwd         string   // absolute path to the directory invocation ran from
	SkipProject bool
	PresetPaths []string // ordered preset search paths (see internal/presets)
}

// PerCheckTimeout caps each check's filesystem / network probe so the global
// <1s wall-time promise survives a wedged dependency. See spec-41 §"Timeouts".
const PerCheckTimeout = 200 // milliseconds

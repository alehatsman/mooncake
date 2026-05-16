// Package resolver implements the kernel-side pre-execute walk that
// resolves `!secret <ref>` typed references to their provider-backed
// values.
//
// # What it is
//
// Mooncake is a typed mutation kernel (see
// docs-working/vision/kernel.md). Every step in a compiled plan
// carries typed action fields. The YAML pre-pass in
// internal/config/secret_tag.go rewrites `!secret env:FOO` tagged
// scalars into sentinel-marker strings; those markers flow through
// plan compilation untouched so plan output (and `mooncake plan
// --format json`) can re-render them as `"!secret env:FOO"` without
// ever materialising the resolved value.
//
// Resolve() is the apply-time inverse: walk a step's typed action
// struct, replace markers with the value returned by
// security.DefaultRegistry, and append every resolved value to the
// supplied Redactor's denylist so it cannot leak into events,
// runlog, or step stdout.
//
// # Why it's a kernel service
//
// Resolution is a pure traversal over the typed plan. It does not
// need executor state (no ExecutionContext, no Result, no
// per-handler context). That makes it callable from frontends that
// want to pre-process a plan before submission:
//
//   - executor's apply path (today's caller; the executor decides
//     whether the run is in plan-mode and supplies a redactor from
//     the run's services)
//   - MCP check_plan (so dry-run can predict the resolved action)
//   - agent loop's "validate before propose" step
//   - future apply_approved hash-then-sign flow
//
// None of these need to drag in 3,270 LOC of internal/executor; they
// just need the walk.
//
// # What this package does NOT do
//
//   - Decide whether to run. Callers handle plan-mode skip themselves
//     (the executor's apply path checks ec.Mode() == ModePlan).
//   - Manage the redactor. Callers supply one (or nil) and own its
//     lifecycle.
//   - Resolve markers outside step action fields. Plan-time variables,
//     metadata, and template inputs are out of scope; their resolution
//     lives in internal/plan and internal/template respectively.
//
// See spec-23 §3 (!secret typed refs) and
// docs-working/vision/kernel.md for the kernel framing.
package resolver

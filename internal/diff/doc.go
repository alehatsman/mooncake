// Package diff renders per-step plan diffs into operator-readable
// text. See spec-66.
//
// The renderer consumes whatever a handler's Differ.Diff returned (or
// the JSON-decoded equivalent from a saved plan) and emits text /
// JSON / YAML output. Wave 1 (this package skeleton) lifts the
// existing file-diff path from cmd/mooncake.go without changing
// output. Subsequent waves add render_<kind>.go files for
// package / user / group / firewall / service / cron / mount /
// git / repo / transaction.
//
// Plumbing only — handlers stay format-agnostic. The renderer's
// kernel-facing contract is: take an opaque `detail any` (the
// StepInspection.Detail value), dispatch on its dynamic type via
// Lookup, and Render to the requested Format. A nil Lookup result
// means "no renderer for this detail" and callers should fall back
// to whatever placeholder text they want.
package diff

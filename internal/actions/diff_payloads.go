package actions

// Per-kind typed payloads carried in Diff.Before / Diff.After. Spec-66
// wave 2+ lives here: each new action category that ships a typed
// rendered diff adds its payload type to this file. Handlers populate
// these; internal/diff renders them.
//
// Convention:
//
//   - Field tags are stable wire JSON; agent consumers lock to them
//     via `mooncake plan --format json`.
//   - Pointer Before/After is OK when a payload tracks before/after
//     state with versions / versions-likely-to-change fields. For
//     intent-only payloads (PackageDiff today) the handler reports
//     the desired terminal state on After and leaves Before nil.
//   - When the JSON-decoded plan path matters (saved plan re-read
//     from disk), the renderer accepts both the typed-pointer and
//     map[string]interface{} shapes — see internal/diff/render_*.go.

// PackageDiff is the typed Before/After payload when an actions.Diff
// has Resource.Kind == ResourcePackage. Today's pkg handler reports
// intent only — it does not probe the package manager from Diff —
// so After describes the desired terminal state and Before is left
// nil. A future "with measurement" mode (e.g. `plan --probe`) could
// fill in Before.Installed once probing is in scope.
type PackageDiff struct {
	// Names are the package names this step targets. Set even when
	// the step uses singular `name:` — we always emit a slice so
	// consumers don't need two code paths.
	Names []string `json:"names,omitempty"`

	// State is the desired terminal state: "present" / "absent" /
	// "latest". Empty maps to "present" by handler convention.
	State string `json:"state,omitempty"`

	// Manager is the explicit package manager when the step pins
	// one (apt/dnf/brew/winget/...). Empty when auto-detect is in
	// play.
	Manager string `json:"manager,omitempty"`
}

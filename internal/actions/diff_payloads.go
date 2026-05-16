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

// UserDiff is the typed Before/After payload when an actions.Diff
// describes an os.user mutation. The handler emits intent only
// (no getent probe at plan time); Before stays nil. Resource.Kind
// is ResourceOther on the wire today (no dedicated kind); consumers
// dispatch on Resource.Attributes["kind"] == "os.user".
type UserDiff struct {
	// Name is the account name (idempotency identity).
	Name string `json:"name,omitempty"`

	// State is the desired terminal state: "present" or "absent".
	// Empty maps to "present" by handler convention.
	State string `json:"state,omitempty"`

	// UID, when set, is the requested numeric uid.
	UID *int `json:"uid,omitempty"`

	// Shell is the requested login shell.
	Shell string `json:"shell,omitempty"`

	// Home is the requested home directory path.
	Home string `json:"home,omitempty"`

	// Groups is the supplementary group list. The primary group, if
	// set on the step, is folded in as the first entry so consumers
	// don't need two code paths to enumerate "all groups this user
	// will be in."
	Groups []string `json:"groups,omitempty"`

	// System mirrors the system-account flag.
	System bool `json:"system,omitempty"`
}

// GroupDiff is the typed Before/After payload when an actions.Diff
// describes an os.group mutation. Same intent-only convention as
// UserDiff. Consumers dispatch on Resource.Attributes["kind"] ==
// "os.group".
type GroupDiff struct {
	// Name is the group name (idempotency identity).
	Name string `json:"name,omitempty"`

	// State is the desired terminal state: "present" or "absent".
	State string `json:"state,omitempty"`

	// GID, when set, is the requested numeric gid.
	GID *int `json:"gid,omitempty"`

	// System mirrors the system-group flag.
	System bool `json:"system,omitempty"`
}

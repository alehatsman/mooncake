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

// FirewallDiff is the typed Before/After payload when an actions.Diff
// describes an os.firewall mutation. The handler reports intent only
// — rule details are intentionally NOT echoed in After because rule
// lists may carry security-sensitive ports / source addresses. The
// renderer surfaces backend + count + state. Consumers dispatch on
// Resource.Attributes["kind"] == "os.firewall".
type FirewallDiff struct {
	// Backend is the firewall backend (v1: "ufw"; "auto" when the
	// step did not pin one).
	Backend string `json:"backend,omitempty"`

	// State is the desired terminal state: "present" or "absent".
	State string `json:"state,omitempty"`

	// RuleCount is the number of rules in the step (1 for single
	// `rule:` form, len(`rules:`) otherwise). Rule details live in
	// the step body, not here.
	RuleCount int `json:"rule_count"`
}

// ServiceDiff is the typed Before/After payload when an actions.Diff
// describes an os.systemd mutation. Spec-66 wave 4 targets os.systemd
// only; the legacy os.service handler keeps its package-local
// payload until a later wave migrates it. Consumers dispatch on the
// typed After value (not on Resource.Kind, which is shared with
// os.service via ResourceService).
type ServiceDiff struct {
	// Name is the unit filename with suffix (e.g. "myapp.service").
	Name string `json:"name,omitempty"`

	// State is the desired terminal state: "present" or "absent".
	State string `json:"state,omitempty"`

	// Path is the unit directory (default /etc/systemd/system).
	Path string `json:"path,omitempty"`

	// Sections lists the unit-file sections this step populates
	// (Unit, Service, Timer, Socket, Install). Lets consumers see
	// the shape of the unit without surfacing potentially large /
	// sensitive Exec lines themselves.
	Sections []string `json:"sections,omitempty"`

	// Enabled / Started mirror the desired lifecycle flags. nil
	// means "leave alone."
	Enabled *bool `json:"enabled,omitempty"`
	Started *bool `json:"started,omitempty"`
}

// CronDiff is the typed Before/After payload when an actions.Diff
// describes an os.cron mutation. The handler reports intent only
// (no /etc/cron.d read at plan time). Consumers dispatch on
// Resource.Attributes["kind"] == "os.cron".
type CronDiff struct {
	// Name is the cron entry's filename in /etc/cron.d (identity).
	Name string `json:"name,omitempty"`

	// State is the desired terminal state: "present" or "absent".
	State string `json:"state,omitempty"`

	// User the command runs as. Empty maps to root.
	User string `json:"user,omitempty"`

	// Schedule is the rendered cron schedule (five whitespace-
	// joined fields when the step uses minute/hour/... pieces).
	Schedule string `json:"schedule,omitempty"`

	// Command is the command line the entry will execute.
	Command string `json:"command,omitempty"`
}

// MountDiff is the typed Before/After payload when an actions.Diff
// describes an os.mount mutation. The handler reports intent only
// (no /etc/fstab or /proc/mounts read at plan time). Consumers
// dispatch on Resource.Attributes["kind"] == "os.mount".
type MountDiff struct {
	// Src is the device / spec (UUID=..., LABEL=..., tmpfs, etc.).
	Src string `json:"src,omitempty"`

	// Dest is the mount point (the idempotency identity).
	Dest string `json:"dest,omitempty"`

	// FSType is the filesystem type (ext4, xfs, tmpfs, nfs, ...).
	FSType string `json:"fstype,omitempty"`

	// State is mounted | unmounted | fstab_only | absent. Empty
	// defaults to "mounted" by handler convention.
	State string `json:"state,omitempty"`

	// Options is the mount-options list (defaults, noatime, ...).
	Options []string `json:"options,omitempty"`
}

// GitCheckoutDiff is the typed Before/After payload when an
// actions.Diff describes a git.checkout mutation. Consumers
// dispatch on Resource.Attributes["kind"] == "git.checkout".
// Before holds the observed HeadSHA when dest is a git repo at
// plan time; After holds the requested Ref. After.HeadSHA is
// empty at plan time — ref resolution happens at apply.
type GitCheckoutDiff struct {
	// Dest is the working-copy path (identity).
	Dest string `json:"dest,omitempty"`

	// Ref is the requested ref (branch / tag / sha).
	Ref string `json:"ref,omitempty"`

	// HeadSHA is the observed HEAD sha. Populated in Before when
	// dest is a git repo at plan time.
	HeadSHA string `json:"head_sha,omitempty"`
}

// GitConfigDiff is the typed Before/After payload when an
// actions.Diff describes a git.config mutation. The handler
// reports declared intent (set / unset per key) — it does not
// shell out to git at plan time to compare current values.
// Consumers dispatch on Resource.Attributes["kind"] ==
// "git.config".
type GitConfigDiff struct {
	// Scope is "local" | "global" | "system".
	Scope string `json:"scope,omitempty"`

	// Repo is the working-copy path for local scope; empty for
	// global / system.
	Repo string `json:"repo,omitempty"`

	// Entries is the per-key intent: each carries Key + Value +
	// Op ("set" | "unset"). Sorted by Key for stable plan output.
	Entries []GitConfigEntry `json:"entries,omitempty"`
}

// GitConfigEntry is one key-level intent in a GitConfigDiff.
type GitConfigEntry struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
	Op    string `json:"op,omitempty"` // "set" | "unset"
}

// RepoDiff is the typed Before/After payload when an actions.Diff
// describes a pkg.repo mutation. The handler reports intent only
// — reading the existing sources list and matching it to the
// requested shape is non-trivial across managers and out of scope
// for the cheap-Diff contract. Before stays nil. Consumers
// dispatch on Resource.Attributes["kind"] == "pkg.repo".
type RepoDiff struct {
	// Name is the repo identifier (also the on-disk filename).
	Name string `json:"name,omitempty"`

	// State is "present" or "absent". Empty maps to "present"
	// by handler convention.
	State string `json:"state,omitempty"`

	// Driver names the populated manager block: "apt" | "dnf" |
	// "brew" | "" (none).
	Driver string `json:"driver,omitempty"`
}

// TransactionDiff is the synthesized After payload for a transaction-
// parent step. Transactions have no handler, so the planner cannot
// emit a Differ.Diff — instead the plan-render call site synthesizes
// one from the parent step + its sibling children. Consumers
// dispatch on Resource.Attributes["kind"] == "transaction".
type TransactionDiff struct {
	// Name is the transaction step's name (Identifier carries the
	// same value).
	Name string `json:"name,omitempty"`

	// AllowIrreversible mirrors step.AllowIrreversible — when true,
	// non-Reverser children were accepted at plan time.
	AllowIrreversible bool `json:"allow_irreversible,omitempty"`

	// BodyChildren is the ordered list of body-child step names (the
	// `transaction:` block).
	BodyChildren []string `json:"body_children,omitempty"`

	// RollbackChildren is the ordered list of rollback-child step
	// names (the `on_rollback:` block).
	RollbackChildren []string `json:"rollback_children,omitempty"`
}

// TryDiff is the synthesized After payload for a try-parent step.
// Same shape rationale as TransactionDiff. Consumers dispatch on
// Resource.Attributes["kind"] == "try".
type TryDiff struct {
	// Name is the try step's name.
	Name string `json:"name,omitempty"`

	// TryChildren is the ordered list of body-child step names (the
	// `try:` block).
	TryChildren []string `json:"try_children,omitempty"`

	// CatchChildren is the ordered list of catch-handler step names
	// (the `catch:` block).
	CatchChildren []string `json:"catch_children,omitempty"`

	// FinallyChildren is the ordered list of finally-handler step
	// names (the `finally:` block).
	FinallyChildren []string `json:"finally_children,omitempty"`
}

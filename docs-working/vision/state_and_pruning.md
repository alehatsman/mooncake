# State and pruning — detecting orphaned resources

> Status: proposal. The runlog + Reverser pieces this builds on are
> shipped; this doc sketches the smallest extension that turns "the
> journal" into "the state file" and adds Terraform-style prune to
> plan/apply.

## The problem in one paragraph

A user renames a resource in their preset — say `mcsearch` → `dex` —
updates every reference, and re-runs `mooncake apply`. The new run
creates the `dex` resources cleanly. But the artifacts created under
the old name (binaries, symlinks, config files) are still on disk.
They are not in the current preset anymore, so plan never sees them.
They are not in the journal's "what changed this run" view either,
because nothing changed about them this run. They are invisible.

This is the same gap that drove Terraform and CDK to a state file.
Mooncake already has the data it needs — the runlog is a state file
in everything but name — what's missing is the small layer that
*derives current managed state from the journal* and *diffs it
against the desired set* at plan time.

## What's already shipped

Three pieces already do most of the work. The proposal does not
introduce new primitives; it composes existing ones.

### 1. The runlog *is* a state file

Every step record in `~/.mooncake/runs.jsonl` already carries:

- a canonical `resource` key (`file:/etc/sudoers.d/aleh-nopasswd`,
  `pkg:gnupg`, `os.group:docker`, …),
- `action` (the handler name),
- `reversible` (true/false, set by `Cost.Reversible`),
- a full `diff.before` / `diff.after` snapshot.

Reducing this by `(resource, latest ts where status != "pruned")`
yields the current *managed set* — the set of resources mooncake
has touched and believes it currently owns.

The append-only journal is the source of truth. No separate state
file is introduced. This is on purpose: separate state files in
Terraform are a frequent source of "state drift" pain, and we
already have something better — a per-step audit trail that survives
every run.

### 2. The `Reverser` ABI already declares undo semantics per handler

`internal/actions/handler_abi.go:209` — handlers opt in by
implementing:

```go
type Reverser interface {
    Reverse(ctx Context, step *config.Step, result Result) (*config.Step, error)
}
```

11 of 13 priority handlers already implement it. The kernel doc
(`kernel.md`) treats reversibility as one of the four typed
properties every step carries. Pruning is just *invoking Reverse
against a resource the current preset no longer asks for*, using the
`diff.before` snapshot stored in the runlog as the reverse context.

### 3. Transactional rollback already wires Reverse into apply

`internal/executor/transaction.go` runs a LIFO Reverse pass when a
transaction fails. `internal/apply/reverse_context.go` provides the
minimal `actions.Context` Reverse needs. These exist; today they
only fire inside a failing run. Pruning is the same path, fired
across runs instead.

## The minimal proposal

Four additive changes. None of them touches the handler ABI.

### A. A reducer: journal → managed set

New package (probably `internal/managedstate` or grow
`internal/runlog`) exposing:

```go
type ManagedResource struct {
    Resource   string             // "file:/etc/sudoers.d/aleh-nopasswd"
    Action     string             // "file.write"
    LastRunID  string             // r/01KS60N57ZZ7PDKGVCF2V65S5S
    LastSeenAt time.Time
    Reversible bool
    Snapshot   json.RawMessage    // diff.before, used as ReverseData
}

func ManagedSet(r io.Reader) (map[string]ManagedResource, error)
```

It walks `runs.jsonl` once and keeps the latest non-pruned entry per
`resource` key. Cheap, deterministic, no separate file to keep in
sync.

### B. Plan-time orphan diff

In the planner, after producing the desired-step set for the current
preset:

```
orphans = ManagedSet keys − desired resource keys
```

These become a new plan operation kind, rendered after creates and
updates:

```
+ create  file:/home/aleh/bin/dex                (new)
~ update  file:/home/aleh/.zshrc                 (1 line changed)
- prune   file:/home/aleh/bin/mcsearch           (last managed by r/01KS5… 2026-05-12, reversible)
- prune   file:/home/aleh/.config/mcsearch/      (last managed by r/01KS5… 2026-05-12, reversible)
! orphan  shell:setup-mcsearch-completions       (last ran in   r/01KS5… 2026-05-12, NOT reversible — manual cleanup)
```

Orphans surface in *every* plan, not only when `--prune` is passed.
The user sees the drift; they decide whether to act on it. This is
the part that closes the original problem — *visibility comes for
free, action is opt-in*.

### C. `apply --prune` (opt-in, never the default)

For each orphan where `reversible == true`:

1. Materialize a synthetic `*config.Step` whose `Result.ReverseData`
   is the stored `Snapshot` from the managed set.
2. Look up the registered handler by `Action`.
3. Call `Reverser.Reverse(reverseCtx, step, result)` via the existing
   path used by transaction rollback. That returns the undo-step.
4. Execute the undo-step.

For each orphan where `reversible == false`: list it, refuse to
auto-prune, point at the run id and date that produced it. The user
cleans up by hand. This matches the kernel's existing posture —
"refusals are documented as follow-ups," not silently swept under
the rug.

`--prune` is never default for the same reason `rm -rf` isn't:
mistakes are unrecoverable. Plan-time visibility is default; action
is consented.

### D. `pruned` journal entries close the loop

After a successful prune of `file:~/bin/mcsearch`, append:

```json
{"ts":"…","run_id":"r/…","resource":"file:/home/aleh/bin/mcsearch",
 "action":"file.write","status":"pruned","reverse_of":"r/01KS5…"}
```

The reducer treats `status == "pruned"` as terminal — that resource
drops out of the managed set on the next plan. No separate "tombstone
file" needed; the journal is still the source of truth.

## Worked example — the `mcsearch → dex` rename

Starting state: the journal has, from a run on 2026-05-12,

```
resource=file:/home/aleh/bin/mcsearch       action=file.write reversible=true
resource=file:/home/aleh/.config/mcsearch/  action=file.template reversible=true
resource=shell:setup-mcsearch-completions   action=shell        reversible=false
```

User edits the preset, replaces every `mcsearch` reference with
`dex`, runs `mooncake plan`:

```
Plan summary: 3 create · 0 update · 2 prune · 1 orphan-manual
+ create  file:/home/aleh/bin/dex
+ create  file:/home/aleh/.config/dex/
+ create  shell:setup-dex-completions
- prune   file:/home/aleh/bin/mcsearch        (r/01KS5… 2026-05-12, reversible)
- prune   file:/home/aleh/.config/mcsearch/   (r/01KS5… 2026-05-12, reversible)
! orphan  shell:setup-mcsearch-completions    (r/01KS5… 2026-05-12, NOT reversible)
          run was: setup-mcsearch-completions.sh — clean up by hand
```

User runs `mooncake apply --prune`. The two reversible orphans are
undone via `file` and `template` handlers' existing Reverse paths.
The shell orphan is reported as unhandled; the user sees the exact
command that produced it and removes whatever it installed (or
writes a one-off cleanup step).

## The rough edges we accept

Honesty about what this doesn't solve, because Terraform learned all
of these the hard way:

- **Pre-journal artifacts are invisible.** Anything mooncake created
  before this feature lands has no `runs.jsonl` entry. Best-effort
  going forward; the user is told this once in the docs and not
  again.
- **Non-reversible steps are surfaced, not auto-cleaned.** `shell`,
  `exec`, `pkg.upgrade`, anything else without a `Reverser`. The
  journal-recorded command is shown so the user can clean up by
  hand. We do not invent reverse semantics for actions that
  legitimately don't have them.
- **No automatic rename detection.** A rename is `destroy + create`.
  If that becomes painful for a real user workflow (it didn't for
  the `mcsearch → dex` case), add an explicit `moved:` directive
  later — Terraform's `moved {}` block is the proven shape. Not now.
- **Snapshot drift.** The reverse uses `diff.before` from the last
  apply, not the *current* on-disk state. If the file changed
  out-of-band since then, the prune may fail or restore an old
  version. The right answer is what file handlers already do:
  refuse if the live state doesn't match the snapshot, surface
  the mismatch, let the user decide. This is identical to how
  in-run Reverse already handles drift.
- **Multi-preset scenarios.** Two presets on the same host both
  managing `~/.zshrc` will fight each other over ownership. This is
  pre-existing — the journal doesn't currently scope by preset.
  Either (a) live with last-writer-wins and document it, or (b) add
  a `preset_id` field to the runlog and scope the managed set per
  preset. (b) is a follow-up, not a blocker.

## What this is not

To stay inside the kernel discipline from `kernel.md`:

- **Not a new ABI.** No handler interface changes. Pruning rides
  the existing `Reverser` contract.
- **Not a separate state file.** The journal is the state. One
  source of truth, append-only, already audit-grade.
- **Not a destroy-by-default feature.** Visibility is default;
  destruction is opt-in via `--prune`. This is the project's
  consistent posture (cf. `plan` exists, `apply` requires
  confirmation, `--dry-run` is a first-class flag).
- **Not a Terraform port.** No backends, no remote state, no locks.
  Just *"the journal already tells us what we own; let plan diff
  against it."*

## Open questions worth resolving before specing

1. **Where does the reducer live?** Extending `internal/runlog` keeps
   journal logic in one place. A new `internal/managedstate` keeps
   the reducer's API surface narrower and easier to test. Lean
   toward the latter unless the runlog package wants the read-side
   helpers anyway.
2. **How are orphans rendered in non-text plan formats?** JSON and
   the explain renderer (`internal/explain/`) both need a `prune`
   operation kind. Probably mirror the existing `Operation` enum on
   `Diff`.
3. **Does `--prune` imply `--yes`?** Today `apply` confirms once at
   the top. Pruning is higher blast-radius than create/update. Worth
   a second confirmation, especially when ≥ N resources would be
   pruned. (Borrow the threshold from `internal/agent/confirm.go`
   logic.)
4. **What's the migration story for users with months of
   `runs.jsonl`?** Probably nothing: the reducer just works against
   whatever's there. But if `resource` keys evolved over time
   (older entries with stale shapes), the reducer needs to ignore
   what it can't parse, not crash.
5. **Should `plan` annotate orphans even without `--prune`?** The
   proposal says yes (visibility is the win). Worth double-checking
   that doesn't make routine plans noisy for users who don't care.

## Why this is worth doing

The kernel doc claims four typed properties: Diff, Reverse, Cost,
Permissions. Today three of them (Diff, Cost, Permissions) are
visible at plan time. *Reverse is invisible until something fails.*
This proposal makes Reverse visible at plan time too — the user sees
what mooncake would undo if they stopped asking for it, the same
way they see what it would create or change.

The `mcsearch → dex` story is the symptom. The deeper win is that
mooncake stops being a "fire-and-forget mutation tool" and starts
being a "round-trip managed-resource tool" — without changing the
kernel, without a separate state file, and without a new ABI.

Related: [`kernel.md`](kernel.md) (Reverse property),
[`goals.md`](goals.md) (idempotency / inspection as core properties),
existing runlog implementation (`internal/runlog/runlog.go`,
`internal/apply/runlog_write.go`).

# Spec 48: Per-Host Overlays + Filter Selectors

**Epic:** Personal Fleet — see [`epics/done/epic-personal-fleet.md`](../../epics/done/epic-personal-fleet.md), sub-epic P6.
**Status:** Draft (PR 14 in progress)
**Effort:** S (~2–3 days)
**Value:** Medium — the minimum-viable "same plan, different boxes" story.
Without this, a personal fleet of mixed Linux + macOS boxes either needs
hand-conditional plans or one plan per box. With this, one plan with a
small overlay per peer covers the realistic differences.
**Depends on:** spec-43 (controller-side plan walking, peers.toml with
`tags`). Nothing in agentd changes.

> **2026-05-14 drift note.** The original draft proposed `--tag` for the
> peer-filter flag. By the time PR 14 landed, `--tag` was already taken on
> `fleet apply` by the existing step-tag-forwarder (added during fleet
> polish). To avoid overloading one flag with two different semantics
> (peer selection vs. step-tag forwarding), this spec was revised to use
> two parallel flags:
>
> - `--peer-filter <key=value>[,…]` — peer selection (this spec).
> - `--step-filter <key=value>[,…]` — step-tag forwarding (renames the
>   pre-existing `--tag`; v1 accepts only `tag=<x>`).
>
> v1 of both flags supports only `tag=` as the key. Extension to `name=`,
> `os=`, `role=`, etc. is the subject of a follow-up spec under
> [`spec-50-extended-filter-keys.md`](spec-50-extended-filter-keys.md).

---

## Problem

Your laptop is Arch. Your MacBook is Darwin. Your two desktops are Arch but
one has an NVIDIA GPU and the other doesn't. You want one `config.yml` to
drive all four with small per-box differences:

- Mac uses `brew`; Linux uses `pacman`.
- One desktop installs CUDA; the others don't.
- Hostname-specific app prefs.

Today's mooncake already supports variable files (`mooncake apply -v vars/foo.yml`)
and conditional steps (`when: …`). What's missing is **a convention** for
where per-host overlays live and **a way to filter peers** at fleet-apply
time.

This spec adds both — strictly controller-side. No daemon changes.

---

## Goals

- **G1** Define `vars/by-host/<hostname>.yml` as the conventional location
  for per-host overlays. `mooncake fleet apply` automatically loads the
  matching overlay for each peer.
- **G2** Define `vars/by-tag/<tag>.yml` as the conventional location for
  per-tag overlays. Loaded for every peer that carries that tag in
  `peers.toml`.
- **G3** Add `--peer-filter <key=value>` (repeatable) to `mooncake fleet
  apply` that filters peers before any sync happens. v1 accepts only the
  `tag=<x>` form. Multiple values within one flag = AND; repeating the
  flag = OR. Rename the existing fleet `--tag` flag to `--step-filter`
  for symmetry; v1 accepts only `tag=<x>` for it as well.
- **G4** Resolve the peer's hostname *before* sync so the right overlay is
  bundled in the sync set.

**Non-goals:**

- Group hierarchies, inventory inheritance, group_vars/host_vars — the
  Ansible model. Out of scope; flat is fine for personal fleet.
- Server-side per-host vars resolution. Everything is decided on the
  controller; the peer sees one vars file already-merged.
- Multi-value tag operators (`tag in [a,b,c]`, `tag matches *prod*`).
  v1 supports exact match only.

---

## Reuse map

**Reused:**

- Existing var-file merging in `executor.Start`'s `VarsFiles` parameter
  (later overrides earlier — see `runs_handler.go:18-20`). The runs
  submit handler already accepts `vars_files`.
- spec-43 `peers.toml` with `tags` field.
- spec-43 plan-dir walker — extended to also walk `vars/by-host/` and
  `vars/by-tag/` as part of the recursive mirror.
- spec-45 peer hostname (from `GET /v1/version.hostname`) — needed to
  resolve overlays.

**New:**

| Component | Location |
|---|---|
| Per-peer vars-file resolution | `internal/fleet/overlays.go` |
| Tag-selector flag parsing | `cmd/fleet.go` (extends apply subcommand) |

That's it. This spec is small.

---

## Convention

Inside a plan-dir, by convention:

```
config.yml                       ← top-level plan
vars/
  common.yml                     ← always loaded (operator's choice)
  by-host/
    laptop.yml                   ← loaded only when applying to peer "laptop"
    macbook.yml
  by-tag/
    darwin.yml                   ← loaded for any peer tagged "darwin"
    gpu.yml                      ← loaded for any peer tagged "gpu"
```

None of these paths are magic to mooncake-core. The convention is enforced
by `mooncake fleet apply`, which assembles the per-peer vars-file list at
submit time:

```
vars_files = [
  "vars/common.yml",            # if present
  "vars/by-tag/<each_tag>.yml", # for each tag the peer has, in tag order
  "vars/by-host/<peer>.yml",    # most specific, loaded last
]
```

Later files override earlier on key collision — same as existing
`-v a.yml -v b.yml` behavior. Files that don't exist are silently skipped.

### Resolving the peer's hostname

The hostname used for `by-host/<name>.yml` matches each peer's `name` in
`peers.toml`, NOT the peer's OS hostname. This makes the overlay
deterministic and operator-controlled:

- Rename peer in peers.toml → rename the overlay file.
- No need to query the peer's `hostname` before sync.

(Spec-41 ensures `peers.toml`'s `name` matches what the daemon advertises
where possible; for power users who want different conventions, this is
plain TOML.)

---

## Sync interaction

Spec-39 mirrors the entire plan-dir recursively. Per-host and per-tag
overlay files therefore get synced to every peer, even files for peers
that aren't being applied to. That's intentional and cheap (these files are
tiny). It also means a `--peers laptop` apply still syncs `macbook.yml`,
which the operator can ignore.

If overlay files grow large or contain sensitive secrets, the operator can:
- Use `vars/private/` outside the by-host convention and reference
  selectively in `config.yml` via includes.
- Use `--peers <name>` to limit fan-out; the unused overlay files are
  inert.

A future content-addressed sync (per the spec-43 follow-up) would
naturally skip uploading overlay files unchanged across reruns.

---

## Tag selectors

```
mooncake fleet apply config.yml --peer-filter tag=os=darwin
mooncake fleet apply config.yml --peer-filter tag=gpu --peer-filter tag=env=home
mooncake fleet apply config.yml --peer-filter tag=os=darwin,tag=workstation  # AND
mooncake fleet apply config.yml --peer-filter tag=os=darwin --peer-filter tag=os=linux  # OR
```

Semantics:

- v1 only accepts `tag=<value>` as the key. Any other key (`name=`,
  `os=`, `role=`) errors with a message pointing at the follow-up spec.
  The flag is a predicate DSL on purpose so the extension lands as a
  validator change, not a flag rename.
- `tag=<value>` matches any peer whose `tags` list in `peers.toml`
  contains the exact string `<value>`. The value can itself contain `=`
  — e.g. `tag=os=darwin` matches the literal tag string `os=darwin`.
- Multiple `tag=…` terms separated by `,` within one `--peer-filter`
  flag → AND.
- Multiple `--peer-filter` flags → OR across them.
- No `--peer-filter` → all peers (existing behavior).
- `--peers <name,...>` already exists from spec-43; it's compatible:
  the tag predicate intersects with the name-selected set.

Print the resolved set before applying (planned; PR 14 prints only an
error when 0 peers match):

```
$ mooncake fleet apply config.yml --peer-filter tag=os=darwin
selected 1 of 4 peers: macbook
sync: 12 files (0.4 MiB)…
…
```

---

## Tasks

### Task 1 — Overlay resolution

1. New `internal/fleet/overlays.go`:
   ```go
   func ResolveVarsFiles(planDir string, peer Peer) []string
   ```
   Returns the ordered list of vars-files to send for this peer.
   Stats each candidate; only present files are included.

### Task 2 — Filter selectors

1. Extend `cmd/fleet.go` apply subcommand:
   ```
   --peer-filter <expr>   # repeatable; AND within one flag, OR across flags
   --step-filter <expr>   # renames existing --tag; same expression syntax
   ```
2. Parser: `parseFilterFlags(args []string) ([][]filterTerm, error)` —
   returns a list of AND-groups, generic over keys so the follow-up spec
   can extend without re-parsing.
3. `validatePeerFilterKeys` / `extractStepFilterTags` reject anything
   other than `tag=` in v1.
4. Apply selector after loading peers.toml and after the `--peers` name
   filter (intersect, not replace). Exit 1 with a clear message when zero
   peers match.

### Task 3 — Wire overlays into submit

1. In the per-peer goroutine inside `fleet apply`:
   - Call `ResolveVarsFiles(planDir, peer)`.
   - Translate to peer-side absolute paths under
     `<state_dir>/synced/<scope>/...`.
   - Pass as `vars_files` in the `POST /v1/runs` body.

### Task 4 — `mooncake apply` (local) parity

`mooncake apply config.yml --vars-from-host <name>` for local use, so the
overlay convention also works without the fleet. Optional in this spec;
mention as a future follow-up if the convention catches on.

### Task 5 — Tests

1. Overlay resolver: given a plan-dir with various overlay files, and a
   peer with given name+tags, assert the exact ordered file list.
2. Tag selector: cover AND/OR combinations against a fixture peer list.
3. Apply integration: with two fake peers (different names + tags),
   assert each peer received the correct `vars_files` array in its run
   submit body.

---

## Open questions

1. **What if the peer's `name` contains characters illegal in a filename?**
   spec-43 restricts `name` to `[a-zA-Z0-9._-]{1,64}`; safe. If we ever
   loosen that, we need a sanitization step here.
2. **Tag namespace pollution.** `os=darwin` vs `darwin` (bare) — two
   different conventions. We don't enforce; the operator picks. Document
   both as valid; lean on examples to suggest `key=value` for OS, role,
   env, and bare tags for capabilities (`gpu`, `nvme`).
3. **Should there be a `vars/by-tag/all.yml` that always applies?** Just
   call it `vars/common.yml` (already conventional). Don't add a special
   case.
4. **Apply-local-only flag.** A `mooncake apply --host <name>` that simply
   sets `host_name` as a variable for use in `when:` predicates is the
   smallest hook into existing-action conditional logic. Worth considering
   alongside the overlay convention for symmetry.

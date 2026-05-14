# Spec 44: Per-Host Overlays + Tag Selectors

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md), sub-epic P6.
**Status:** Draft
**Effort:** S (~2–3 days)
**Value:** Medium — the minimum-viable "same plan, different boxes" story.
Without this, a personal fleet of mixed Linux + macOS boxes either needs
hand-conditional plans or one plan per box. With this, one plan with a
small overlay per peer covers the realistic differences.
**Depends on:** spec-39 (controller-side plan walking, peers.toml with
`tags`). Nothing in agentd changes.

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
- **G3** Add `--tag <key=value>` and `--tag <name>` flags to
  `mooncake fleet apply` that filter peers before any sync happens.
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
- spec-39 `peers.toml` with `tags` field.
- spec-39 plan-dir walker — extended to also walk `vars/by-host/` and
  `vars/by-tag/` as part of the recursive mirror.
- spec-41 peer hostname (from `GET /v1/version.hostname`) — needed to
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

A future content-addressed sync (per the spec-39 follow-up) would
naturally skip uploading overlay files unchanged across reruns.

---

## Tag selectors

```
mooncake fleet apply config.yml --tag os=darwin
mooncake fleet apply config.yml --tag gpu --tag env=home
mooncake fleet apply config.yml --tag os=darwin,workstation  # AND within one flag
mooncake fleet apply config.yml --tag os=darwin --tag os=linux  # OR across flags
```

Semantics:

- A bare tag name (`gpu`) matches any peer whose `tags` list contains
  exactly `gpu`.
- A `key=value` tag matches any peer with a tag of that form.
  (For v1, `key=value` is just a string convention; the parser splits on
  `=` for display, but matching is exact-string.)
- Multiple values separated by `,` within one `--tag` flag → AND.
- Multiple `--tag` flags → OR.
- No `--tag` → all peers (current spec-39 behavior).
- `--peers <name,...>` already exists from spec-39; it's compatible:
  intersect with the tag-selected set.

Print the resolved set before applying:

```
$ mooncake fleet apply config.yml --tag os=darwin
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

### Task 2 — Tag selector

1. Extend `cmd/fleet.go` apply subcommand:
   ```
   --tag <expr>   # repeatable; OR across flags, AND within a single flag
   ```
2. Parser: `parseTags(args []string) [][]string` returning a list of
   AND-groups.
3. Apply selector after loading peers.toml; print the resolved set.

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
   spec-39 restricts `name` to `[a-zA-Z0-9._-]{1,64}`; safe. If we ever
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

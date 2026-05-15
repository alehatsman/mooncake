# Spec 51: Local-Apply Overlay Parity

**Epic:** Personal Fleet — see [`epics/done/epic-personal-fleet.md`](../../epics/done/epic-personal-fleet.md).
**Status:** Draft (follow-up to spec-48 PR 14)
**Effort:** XS (~1 day)
**Value:** Small but real — closes the asymmetry where `vars/by-host/<name>.yml`
works for `fleet apply` but not for `mooncake apply` on the same machine.
Operators editing a config locally on `laptop` would expect
`mooncake apply` to pick up `vars/by-host/laptop.yml` automatically. Right
now it doesn't, which makes the overlay convention feel fleet-only.
**Depends on:** spec-48 (overlay convention defined and shipped).

---

## Problem

Spec-48 introduces a convention for overlays:

```
vars/
  common.yml
  by-tag/<tag>.yml
  by-host/<name>.yml
```

`mooncake fleet apply` walks the plan-dir, resolves each peer's overlays
via `fleet.ResolveVarsFiles(planDir, peer)`, and bundles them into the
per-peer submit. Result: `vars/by-host/macbook.yml` is loaded when applying
to peer `macbook`.

`mooncake apply` (local, same-machine) does *not* do this. The local
apply path reads `--vars-file`/`-v` flags only. So:

- An operator on `laptop` who edits `vars/by-host/laptop.yml` and runs
  `mooncake apply config.yml` gets the *unmodified* result. The overlay
  is silently inert. They have to explicitly pass `-v vars/by-host/laptop.yml`.
- The overlay convention reads as "only for fleet." Newcomers learn it
  once for fleet, then get burned the first time they edit a vars file
  and rerun locally.

The fleet path resolves the peer's *name* from `peers.toml`. The local
path doesn't have a "peer" — it just has the running machine. There are
two clean ways to plumb this; this spec picks one.

---

## Goals

- **G1** `mooncake apply config.yml` auto-loads overlays matching the
  *local* machine: `vars/common.yml`, `vars/by-host/<hostname>.yml`,
  and (deferred to G2) `vars/by-tag/<tag>.yml` if tags are declared.
- **G2** A `--host <name>` flag overrides the auto-detected hostname so
  operators can preview "what would this apply look like if I were
  `macbook`" without renaming the host. (Sibling to fleet's `--peers`.)
- **G3** An `--overlays=off` flag disables auto-loading for the rare
  case where an operator wants the un-overlaid plan (debugging, CI).
- **G4** Auto-loading is silent on a clean miss (no `by-host/<name>.yml`
  exists) and noisy on an explicit miss (`--host other` when
  `by-host/other.yml` doesn't exist).

**Non-goals:**

- Fact-driven overlays beyond hostname (e.g. `by-os/darwin.yml`). The
  fleet path uses operator-declared `peers.toml` tags; the local path
  has facts available, but introducing a *new* convention only for
  local-apply diverges from the fleet path. Defer to spec-50 follow-up
  that unifies fact-driven keys across both paths.
- Auto-loading `vars.yml` (root-level, no `by-host/` prefix). The
  spec-48 convention is intentionally namespaced; reusing it locally
  shouldn't expand the namespace.

---

## Design

### Hostname resolution

```go
// Local apply hostname source order:
//   1. --host <name> flag if set
//   2. $MOONCAKE_HOST env var if set
//   3. os.Hostname() with the first DNS label only (no .local suffix)
```

The DNS-label trim matters: macOS reports `MacBook-Air.local`; we want
`MacBook-Air` so `vars/by-host/MacBook-Air.yml` is a sensible file to
commit. Document the exact rule in `mooncake apply --help`.

### Tag resolution (G2 deferred)

A local machine has no `peers.toml` entry, so `by-tag/<tag>.yml`
overlays don't have a tag source by default. Options:

- **A** — Drop tag overlays locally. Hostname + common only. Simplest.
- **B** — Add an optional `~/.config/mooncake/local-tags.toml`
  declaring this machine's tags. Symmetric with `peers.toml`'s tags
  but adds a new config file.
- **C** — Auto-derive tags from facts (`os=darwin`, `arch=arm64`,
  `package_manager=brew`). Powerful but introduces the "facts-as-tags"
  question prematurely.

**Lean A for v1.** Hostname coverage is 90% of the value. Revisit B/C
if anyone asks.

### Auto-load order

Same as fleet (`fleet.ResolveVarsFiles`) minus by-tag, so the merge
semantics are identical:

```
1. vars/common.yml              (if present)
2. vars/by-host/<hostname>.yml  (if present)
3. --vars-file args             (in order, later overrides)
```

### Code reuse

`internal/fleet/overlays.go:ResolveVarsFiles` already does the file-stat
+ order work. Extract a smaller helper that doesn't need a `Peer`:

```go
// In a new internal/overlay/ package (or internal/fleet/ if we accept
// the dependency direction):
func ResolveLocalOverlays(planDir, hostname string) []string
```

Or simpler: add a `ResolveLocalOverlays` to `internal/fleet/overlays.go`
that calls into the same private helper. Both `mooncake apply` and
`mooncake fleet apply` use the same code path, so the overlay
convention can't drift between them.

---

## Implementation outline

1. Refactor `ResolveVarsFiles` to delegate to a `resolveOverlayCandidates`
   helper that takes (planDir, hostname, tags) — `tags` is nil for the
   local path.
2. Add `cmd/mooncake.go` flag: `--host <name>` (default: derived
   hostname) on the `apply` subcommand.
3. Add `cmd/mooncake.go` flag: `--overlays=on|off` (default: `on`).
4. In `applyAction`: call `fleet.ResolveLocalOverlays(planDir,
   resolvedHostname)` and prepend to `varsAbs`.
5. Tests:
   - Auto-load picks up `by-host/<hostname>.yml` when present.
   - `--host other` errors if `by-host/other.yml` doesn't exist.
   - `--overlays=off` disables auto-loading entirely.
   - Conflict resolution: explicit `--vars-file` still wins on key
     collision.

---

## Open questions

1. **Hostname canonicalization.** `os.Hostname()` returns
   `MacBook-Air.local` on macOS, `laptop` on most Linux, `WIN-ABC123` on
   Windows. The first-DNS-label rule trims `.local`. But should we also
   lowercase? Lean no — preserve operator intent.
2. **`MOONCAKE_HOST` vs `--host`.** Env var is useful for CI; flag is
   useful for ad-hoc overrides. Ship both; flag wins.
3. **Should `--host` accept the *peer name* convention if the
   `peers.toml` entry's name differs from the machine hostname?** No —
   `peers.toml` is a fleet concept. Keep the local path independent of
   `peers.toml`.
4. **Surfacing what was loaded.** `mooncake apply -v` already prints
   the resolved vars-files in plan output. Make sure auto-loaded
   overlays show up there too, distinct from `--vars-file` (perhaps
   prefixed `(overlay)` in the plan listing).

---

## Success criteria

After this spec lands:

1. On `laptop`, `mooncake apply config.yml` loads
   `vars/by-host/laptop.yml` automatically.
2. `mooncake apply --host macbook config.yml --dry-run` previews
   the apply as if the machine were `macbook`.
3. `mooncake apply --overlays=off config.yml` ignores the convention
   entirely.
4. The same `ResolveVarsFiles` code path serves both `mooncake apply`
   and `mooncake fleet apply` — no two-source-of-truth bug for the
   overlay convention.

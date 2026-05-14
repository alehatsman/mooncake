# Spec 50: Extended Filter Keys for `--peer-filter` / `--step-filter`

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md).
**Status:** Draft (follow-up to spec-48 PR 14)
**Effort:** S–M (~3–5 days depending on which keys land in v1)
**Value:** Medium — `--peer-filter tag=<x>` covers the common case but
forces operators to maintain redundant tags for things mooncake already
knows about (peer name, OS, version). Extended keys remove the bookkeeping
and make `--peer-filter os=darwin` Just Work without a `[[peers]] tags`
edit.
**Depends on:** spec-48 (the `parseFilterFlags` predicate DSL and the
v1 `tag=`-only validators are already in place).

---

## Problem

spec-48 ships `--peer-filter` and `--step-filter` as a generic predicate
DSL: `key=value`, AND within commas, OR across flags. v1 accepts only the
key `tag=`. That covers the spec-48 use case but bottoms out fast:

- Operators want `--peer-filter os=darwin` without having to remember to
  add `tags = ["os=darwin"]` to `peers.toml`. Operating system is already
  in `/v1/version` on each peer; mooncake should use it.
- `--peer-filter name=laptop` is the natural generalization of the
  existing `--peers laptop,desktop1` flag — and once it ships, `--peers`
  becomes a shorthand-or-deprecation candidate.
- `--step-filter name="install nvim"` would let operators target a
  single step by name without editing the plan to add a `tags:` field
  on every step.

The spec-48 parser is already generic over keys; this spec is the
validator + evaluator extension. No new flag is introduced.

---

## Goals

- **G1** `--peer-filter` accepts these keys in addition to `tag`:
  - `name=<peer_name>` — exact match against `peers.toml` `[[peers]] name`.
  - `os=<os>` — match against the peer's reported OS from
    `/v1/version`. Cached after first successful probe per peer; re-probed
    on each `fleet apply` start.
  - `role=<role>` — match against `peers.toml` `[[peers]] roles = [...]`
    (new optional field; backward-compatible with existing peer entries).
- **G2** `--step-filter` accepts these keys in addition to `tag`:
  - `name=<step_name>` — forward to daemon; daemon already supports
    `--names` on the executor.
- **G3** When a key is unknown, the error message names the spec and
  lists the accepted keys (mirrors v1 behavior). Helps users discover
  what's available without reading docs.
- **G4** Deprecate `--peers <name,...>` as a separate flag; it becomes
  a shorthand for `--peer-filter name=<n1>,name=<n2>…` internally, but
  remains accepted (no breaking change).

**Non-goals:**

- Pattern matching (`os=darwin*`, regex). The predicate DSL is exact-match
  by design. Add a separate `--peer-filter os~=…` operator if a real use
  case appears — don't extend the existing operator.
- Tag *negation* (`!tag=foo`). Open-ended; revisit if users hit it.
- Fact-driven keys beyond `os` (e.g. `arch=arm64`, `package_manager=apt`).
  These exist in mooncake's fact system but the daemon doesn't expose them
  yet over a fast probe endpoint. Punt to a later spec that adds a
  `/v1/peer-facts` probe.

---

## Reuse map

**Reused:**

- `parseFilterFlags` from spec-48 — the AND/OR group structure is the
  evaluator input.
- `cmd/fleet.go` apply Action — extends the validator + evaluator;
  no new flag wiring.
- `/v1/version` daemon endpoint — already returns `hostname` and `version`.
  This spec adds `os` to the response (`runtime.GOOS`); thin daemon
  change.

**New:**

| Component | Location |
|---|---|
| Probe-on-apply cache for `/v1/version` per peer | `internal/fleet/probe.go` |
| Extended evaluator (`os=`, `name=`, `role=`) | `cmd/fleet.go` (replaces v1 evaluator) |
| `roles` field on `Peer` (optional; backward-compat) | `internal/fleet/peers.go` |
| `os` field on `/v1/version` response | `internal/agentd/server.go` |

---

## Implementation outline

### Phase A — `name=` and `role=` (controller-side only)

These need zero daemon work — `name` is already in `peers.toml`, and
`roles` is just a new optional `[]string` field. Land first; smallest
diff.

- `peerMatchesFilters` learns three additional cases:
  - `key == "name"` → `p.Name == value`.
  - `key == "role"` → membership in `p.Roles`.
  - `key == "tag"` → unchanged.
- `validatePeerFilterKeys` allowlist becomes `{tag, name, role}`.
- `--peers` becomes a shorthand: at parse time, lift its comma-separated
  list into the equivalent `--peer-filter name=…,name=…` group OR'd with
  whatever `--peer-filter` flags the user passed. Document as
  "shorthand; `--peer-filter` is the canonical form."

### Phase B — `os=` (needs daemon change)

- Add `os string \`json:"os"\`` to `/v1/version` (just `runtime.GOOS`).
  Cheap, no auth change.
- Controller: probe each selected peer's `/v1/version` *before*
  evaluating the `os=` predicate. The Apply path already calls
  `GetVersion()` after selection — move it before, cache by peer name in
  a per-`fleet apply` map. Unreachable peers fail the `os=` predicate
  (no probe = no match); print a warning so the operator notices.
- Evaluator learns `key == "os"`.

### Phase C — `--step-filter name=`

- Daemon already accepts a `step_names` list on `POST /v1/runs` (per the
  executor's `--names` support). Wire the value through alongside `Tags`.
- Controller: `extractStepFilterTags` becomes
  `extractStepFilter(args) (tags, names []string, err error)`.

Phases can ship as three small PRs or one bundled PR; A is independent
of B and C.

---

## Open questions

1. **`os=` value vocabulary.** `runtime.GOOS` returns `darwin`, `linux`,
   `windows`, `freebsd`, etc. Match that exactly, or normalize (e.g. `mac`
   → `darwin`)? Lean exact-match; operators can write `tag=…` overrides
   if they hate `darwin`.
2. **What does `name=laptop --peer-filter tag=gpu` mean — AND or OR
   across the keys?** Today the rule is "AND within a flag, OR across."
   `name=laptop,tag=gpu` is AND. Across separate flags is OR. So both
   match: peer is `laptop` OR peer has tag `gpu`. Document explicitly;
   it's the same rule, but cross-key OR/AND has caught me out before.
3. **Should `roles` and `tags` be the same field?** `roles = ["db",
   "primary"]` vs `tags = ["role=db", "primary"]` express the same intent
   two ways. Lean to keep them separate — `roles` is semantically
   "what this peer is for"; `tags` is "free-form labels." Match
   k8s/Ansible conventions.
4. **`--peers` deprecation path.** Soft (still works, no warning) or
   loud (one-time deprecation notice on first use)? Lean soft; `--peers`
   is the muscle-memory shorthand and breaking it would be hostile.
5. **Fact-driven keys beyond `os`.** Mooncake's fact system collects
   `arch`, `package_manager`, `kernel_version`, etc. Worth a thin
   `/v1/peer-facts` endpoint that returns a frozen-at-probe-time fact
   snapshot, or do we lift facts into `/v1/version`? Probably a separate
   endpoint — facts can be large.

---

## Success criteria

After this spec lands:

1. `mooncake fleet apply config.yml --peer-filter os=darwin` works
   against a fresh peers.toml (no `tags` edits needed).
2. `mooncake fleet apply config.yml --peer-filter name=laptop` is
   equivalent to `--peers laptop`.
3. `mooncake fleet apply config.yml --step-filter name="install nvim"`
   targets one step on each selected peer.
4. Unknown key error names the keys that *are* valid: `unsupported
   --peer-filter key "arch" (valid: tag, name, os, role)`.
5. Backward-compat: existing `peers.toml` entries with no `roles` field
   still load and validate.

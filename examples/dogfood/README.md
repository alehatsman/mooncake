# Dogfood

This directory holds [`mooncake.yml`](mooncake.yml) — the parts of this
project's [`Makefile`](../../Makefile) that map cleanly to existing
mooncake actions, expressed as a mooncake config. Mooncake builds
itself through itself.

## What it is

A **credibility artifact**, not a Makefile replacement.

The Makefile still owns the canonical build. This file proves the
kernel can run its own quality gate without inventing new YAML
keywords for the dogfood's sake. Every step is typed, observable
in the runlog, idempotent in the sense each action declares its
own change semantics, and reversible where the action ships
`Reverse()`. (The `shell` action here does not — that's a current
limitation, not a stance. See "what this intentionally doesn't do"
below.)

## What it implements

| Make target  | Step(s) in this YAML                           |
|---           |---                                             |
| `task build` | `build mooncake binary`                        |
| `task test-race` | `run go test with race detector`           |
| `task lint`  | `lint with golangci-lint`                      |
| `task scan`  | `lint with golangci-lint` + `govulncheck`      |
| `task schema-check` | `schema-check (regenerate JSON schema)` |
| `task docs-check` | `docs-check (regenerate generated docs)`  |
| `task ci`    | every step above runs in sequence              |

Run it:

```bash
mooncake plan  --config examples/dogfood/mooncake.yml   # preview
mooncake apply --config examples/dogfood/mooncake.yml   # execute
```

Sequential ordering is sufficient. The schema-check and docs-check
steps invoke `./out/mooncake`; if the build step fails, the
executor stops there.

## What it intentionally doesn't do

These Make patterns can't be expressed with the keywords listed in
[`S-dogfood-mooncake-yml`](../../docs-working/vision/brainstorm/2026-05-16-stories.md#s-dogfood-mooncake-yml)
(`shell`, `file`, `on_change`, `requires`, `unless`, `transaction`)
and the action set that ships today, so they stay in the Makefile:

- **File-timestamp dependency tracking** (`make` rebuilds when
  `mooncake.go` is newer than `bin/mooncake`). Mooncake tracks
  state idempotency, not transformation timestamps. The Innovator's
  brainstorm pass flagged content-hash caching as bazel-shaped and
  pulling toward [non-goal #2](../../docs-working/vision/non_goals.md);
  it's out of scope on purpose.
- **Step-level parallelism** (Make's `-j N`). Mooncake apply is
  sequential today. This is a current limitation, not a stance — if
  a real Makefile-replacement push lands, parallel-within-host is
  the spec that comes with it.
- **Tests-as-state**. "Tests should pass" isn't converged
  filesystem state. The Makefile's `test`/`test-race` targets are
  *probes*, not mutators; expressing them as `shell:` steps here is
  honest about that — the step "succeeds when `go test` exits 0,"
  not "converges to a passing state."
- **`task install` / `task release`**. Both shell sudo / external
  side effects (`/usr/local/bin`, `goreleaser` publishing). They
  could be expressed, but their idempotency semantics aren't
  obvious enough to commit a credibility artifact to.
- **The `*-pkg` agent-DX shortcuts** (`task build-pkg`,
  `task test-pkg`, etc.) take a `PKG=` argument. Mooncake doesn't
  parameterize step bodies at apply time from CLI flags; runtime
  vars come from `vars:` and `--set`, which is fine for static
  pipelines but awkward for "give me a fast loop on one package."
  These stay in the Makefile.

### About `requires:`

The story
[`S-dogfood-mooncake-yml`](../../docs-working/vision/brainstorm/2026-05-16-stories.md#s-dogfood-mooncake-yml)
listed `requires:` as an existing keyword. It isn't — neither
`internal/config/schema.json` nor any example file uses it; the
brainstorm got that detail wrong. Step ordering in this dogfood
file uses sequential placement instead. This is exactly the kind
of mismatch the dogfood is meant to surface; if explicit
declarative dependencies turn out to be load-bearing for some
future config, that's a spec proposal, not a quiet keyword
addition.

## Status

**Release-blocker on the next minor.**

The brainstorm round-2 synthesis classified this file as a
release-blocker example for the next minor version (story
[`S-dogfood-mooncake-yml`](../../docs-working/vision/brainstorm/2026-05-16-stories.md#s-dogfood-mooncake-yml)
DoD). The repo's [`ROADMAP.md`](../../ROADMAP.md) currently
redirects to the docs site; this README carries the status inline
to avoid stale-duplicating it in two places. When the next minor
cuts, this file should be in the release notes as the dogfood
deliverable.

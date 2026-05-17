# Manual Test Findings — 2026-05-15

> **Status as of 2026-05-18: the MT-N queue is closed.** Every CRITICAL,
> HIGH, MEDIUM, and LOW finding from the original 43-item audit (and the
> later rounds that extended into the 50s/60s/70s/80s) has been fixed
> and verified in source. The only item still showing as open in the
> per-file tables is #11 (curated snapshot tool inventory — a design
> decision, not a bug). Per-finding commit refs live in each file's
> summary table; original repros are kept under `### Original report`
> sections so historical context is preserved.
>
> Folder lives under `docs-working/archive/` because the pass is complete;
> future manual-test passes should open a new `findings-<DATE>/` directory.

Filed by an LLM acting as a manual tester. Built a static binary,
ran 25 scenarios across `ubuntu:24.04`, `alpine:3.21`, and `debian:bookworm-slim`
containers. **43 numbered findings** across 6 testing rounds.

> **The headline pattern**: five of seven HIGH+ findings share the
> same shape — **silent success that's actually broken**. CI passes,
> the recap shows green, but the action did nothing useful or
> something unverified. This is the most dangerous class of bug because
> no one learns about it from feedback; they learn from production
> incidents.

## How this is organized

Findings are grouped by area, not by round. Each area file is
self-contained; severity-sorted within.

| File | Findings | Theme |
|---|---:|---|
| [`silent-success-bugs.md`](./silent-success-bugs.md) | 14 | The "green recap, broken behavior" class. **Start here.** |
| [`ssot-drift.md`](./ssot-drift.md) | 8 | Validator, schema, docs, examples not aligned |
| [`template-engine.md`](./template-engine.md) | 6 | Rendering, escaping, filter syntax, metrics-in-templates |
| [`coverage-gaps.md`](./coverage-gaps.md) | 7 | Preset gaps, tool action gaps, repo.tree, presets search |
| [`cli-and-friction.md`](./cli-and-friction.md) | 7 | Minimal-container friction, CLI nits, error-message quality |
| [`positive-keepers.md`](./positive-keepers.md) | 7 | Features to feature; "do not regress" list |
| [`verification-2026-05-15.md`](./verification-2026-05-15.md) | (status) | Fix-status after 9 MT-N fix commits landed |

## Severity rollup

| Severity | Original | After MT fixes (2026-05-15 same-session) | Final (2026-05-18) |
|---|---:|---:|---:|
| **CRITICAL** | 2 | **0** | **0** |
| **HIGH** | 7 | 5 | **0** |
| **MEDIUM** | 9 | 8 | **0** |
| **LOW** | 19 | 14 | **0** |
| **(design decision, not a bug)** | — | — | **1** (#11 curated tools list) |
| **(positive — keep)** | ≥12 | ≥14 | ≥30 (MT fixes themselves now belong here) |

Both **CRITICAL** bugs were fixed within the same session. All HIGH and
MEDIUM bugs are now fixed; see each file's summary table for per-finding
commit refs.

## Top three fixes (highest ROI per LoC)

1. **#15** — unify `creates:` / `unless:` honor across all action handlers
2. **#27 via #35** — wire validator to `mooncake schema generate`'s output (already exists, just not connected)
3. **#40** — let `tool github-release` install bare binaries (jq, hadolint, kind, etc.)

Each is a small, scoped fix that closes a big DX hole.

## Repro environment

```
Binary:     CGO_ENABLED=0 go build -ldflags='-s -w' -o out/mooncake-static ./cmd
Containers: ubuntu:24.04, alpine:3.21, debian:bookworm-slim
Mount:      -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro
            -v $PWD/presets:/work/presets:ro
            -v <playbook>:/work/mooncake.yml:ro
            -w /work
```

Test playbooks live in `/tmp/mooncake-tests/` on the test host (not
checked in).

## Commit trail

The original monolith was at
`docs-working/analysis/manual-test-findings-2026-05-15.md` (1479 lines).
History:

- `346d73d` — file 21 manual-test findings (rounds 1+2+3)
- `7cd8ae9` — round 4 (step, artifact, mcp)
- `fed91a1` — round 5 (validator drift, failed_when, 4 keepers)
- `7c66d94` — round 6 (schema/docs generate, tool action gaps)

Future rounds append to the relevant per-area file in this directory.

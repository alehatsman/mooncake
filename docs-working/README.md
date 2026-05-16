# docs-working (v2)

Working documents — strategic direction + per-stream active work.

This is the rewrite. The old `docs-working/` stays alongside until this
replaces it.

## Layout

```
vision/
  goals.md                          What Mooncake is, what "done" looks like
  non_goals.md                      What Mooncake refuses to become
  good_lessons_from_other_tools.md  What 30 years of provisioning got right
  bad_lessons_from_other_tools.md   The pathologies that killed prior systems

streams/
  core/                             Typed action vocabulary, planner, executor
    specs/                          Active specs (currently empty)
    bugs/                           Open bug analyses (file new ones here)
  fleet/                            Multi-machine: agentd, transport, multiplexer
    specs/                          Active: spec-55, spec-58
    bugs/
  dx/                               Developer experience: init / doctor / history
    specs/                          Active: (none)
    bugs/
  agent/                            AI agent safety: MCP, transactions, ABI, secrets
    specs/                          Active: spec-31
    bugs/
```

## How to use this

- **New spec?** File under `streams/<stream>/specs/`. Each stream's
  `README.md` lists what's active and what shipped.
- **New bug?** File under `streams/<stream>/bugs/`. Keep the analysis
  next to the stream that owns the code.
- **Shipped spec?** Move to `streams/<stream>/specs/done/` (or just rely
  on `git log` — neither approach is canonical yet).
- **Strategic question?** Check `vision/` before proposing a feature.
  Especially `non_goals.md`: many proposals look reasonable until you
  see the historical receipts of why they kill projects.

## Conventions

- **Status of shipped work lives in commit history**, not in a separate
  PROGRESS.md. The v1 PROGRESS.md became its own maintenance burden.
- **Stream affiliation matters more than spec number**. Spec numbers are
  monotonic for history; what you actually want is "which stream owns
  this?" The folder hierarchy answers it.
- **Vision docs are short and stable**. If you find yourself updating
  them every week, the right place is probably a stream README, not
  vision/.

## What's *not* here (from v1)

- Long-form analyses of closed GitHub issues — they were valuable when
  the bug was open; once shipped, the fix lives in `git log -S<flag>`.
- The PROGRESS.md rev-N changelog — replaced by per-stream READMEs +
  commit history.
- `clustermanagement/` brainstorms and `analysis/` audits — exploratory
  one-shot docs that served their purpose. If a finding has lasting
  weight, it gets distilled into vision/.
- `epics/` — over-organized for a project at this scale. Stream READMEs
  carry the strategic frame.

# Modules and Components

Mooncake's reuse layer is two concepts: a **component** (a reusable unit of
steps with typed `props:`) and a **module** (a Git repository that exports one
or more components via an `index.yml` manifest, pinned to an explicit tag).

The full guide — quick start, authoring a component, the `use:` reference
format, the module manifest, the lockfile, the cache, and known limitations —
lives at [`internal/modules/README.md`](internal/modules/README.md), which is
also what generates [the Modules concept page](dist/docs/concepts/modules.md).

This file used to duplicate that content; folded into one source 2026-09-08
(moongit #206) so the two stop drifting apart.

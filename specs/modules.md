---
id: modules
status: draft
owners: [aleh]
covers:
  - "internal/modules/*.go"
  - "cmd/mod/*.go"
---

# Modules (Git-Native Distribution)

## Intent
Modules are the reusable-component distribution layer: a module is a Git
repository (or a subpath of one) that exports one or more mooncake components
through an `index.yml` manifest, pinned to an explicit Git tag. The `mooncake
mod` CLI fetches a module into an on-disk cache and records an alias in the
playbook's `modules:` block; a resolver turns a `use:` reference into a concrete
component file. Fetches are content-addressed by `<host>/<owner>/<repo>@<tag>`,
clone shallowly, and cache atomically so concurrent runs and offline listings
stay correct. Path semantics are deliberate: a fetched component's own assets
resolve against its origin dir, while its outputs land in the consumer's
invocation dir.

## Behavior
- WHEN a reference `<host>/<owner>/<repo>[/<subpath>]@<version>` is parsed, the
  version SHALL be required and a reference without `@<version>`, with an empty
  segment, or with fewer than three path segments SHALL be rejected.
- WHEN a module is fetched, it SHALL be shallow-cloned (`--depth 1 --branch
  <tag>`) into a sibling temp dir and atomically renamed into
  `<cache>/<host>/<owner>/<repo>@<version>`; a populated cache dir SHALL be a
  hit that skips the clone, and a lost rename race SHALL fall back to the
  existing dir.
- WHERE `$MOONCAKE_MODULE_CACHE` is set it SHALL be the cache root; otherwise
  the root SHALL be `~/.cache/mooncake/modules`.
- WHEN a clone fails, a missing tag SHALL be reported distinctly from a
  network/auth failure, and `GIT_*` env vars SHALL be scrubbed so a parent git
  context cannot redirect the clone.
- WHERE a host appears in `Fetcher.InsecureHosts` or the comma-separated
  `$MOONCAKE_MODULE_INSECURE` list, that host SHALL clone over plain `http`
  (e.g. a local moongit on `127.0.0.1:8080`); every other host SHALL clone over
  `https`.
- WHEN `index.yml` is loaded, it SHALL require a non-empty `name` and a non-empty
  `exports` map; `ResolveExport` SHALL treat `""`/`default` as the `default`
  entry and SHALL verify the exported component file exists on disk.
- WHEN a `use:` reference resolves, an inline form containing `@` SHALL fetch
  directly (default export only), an `alias` SHALL look up the source in the
  `modules:` block, and `alias/export` SHALL select a named export.
- WHERE a reference carries a subpath, the module root SHALL be that
  subdirectory and a `..` subpath that escapes the cache dir SHALL be rejected.
- WHILE listing offline (e.g. `mooncake task`'s description resolution),
  `FetchCached`/`ResolveCached` SHALL resolve only from the local cache and
  SHALL never clone.
- WHEN `mooncake mod add <ref> [--as <alias>]` runs, it SHALL fetch the module,
  default the alias to the index `name`, and upsert `modules.<alias>: <ref>`
  into the nearest (`--playbook`, default `./mooncake.yml`) playbook.
- WHEN `mooncake mod cache list` / `cache clean` run, they SHALL print the
  cached `<host>/<owner>/<repo>@<version>` entries / remove the entire cache
  root.
- WHEN a fetched component runs, its `shell`/`cmd` cwd SHALL be the invocation
  dir, its `file.*` source paths SHALL resolve against the module origin dir,
  and its `file.*` destination paths SHALL resolve against the invocation dir.

## Non-goals
- Component execution, the planner's per-step `component_dir`/`invocation_dir`
  overlay, and action semantics — owned by the execution-engine spec.
- The `modules:`/`use:`/`props:` document grammar — owned by the config-model
  spec.
- A central registry, version-range resolution, or lockfile pinning beyond an
  explicit tag (references are exact tags only).
- Preserving comments/key order when rewriting the `modules:` block on `add`
  (yaml.v3 round-trip rewrites the file; acceptable for phase 1).

## Checklist
- [x] `Reference` parse: required `@version`, `<host>/<owner>/<repo>[/subpath]`,
  empty-segment rejection.
- [x] Atomic clone-into-temp + rename cache with race-loser fallback; cache hit
  skips clone.
- [x] `$MOONCAKE_MODULE_CACHE` override; default `~/.cache/mooncake/modules`.
- [x] Missing-tag vs network/auth error disambiguation; `GIT_*` scrub.
- [x] Insecure-host http clone via `InsecureHosts` + `$MOONCAKE_MODULE_INSECURE`.
- [x] `index.yml` load: required `name`/`exports`, default export, existence check.
- [x] Resolver: inline `@`, `alias`, `alias/export`; subpath with `..` escape guard.
- [x] Offline `FetchCached`/`ResolveCached` (no clone).
- [x] `mooncake mod add` upserts `modules:` block; `cache list`/`cache clean`.
- [x] Component shell cwd = invocation dir; `file.*` src = origin-relative,
  dest = invocation-relative (M2, #43).
- [ ] DRIFT (#50): a module component's `file.copy` `dest:` that is RELATIVE and
  contains a subdir resolves against the module cache/origin dir instead of the
  invocation dir, despite M2; absolute (`{{ invocation_dir }}/…`) and bare
  relative dests land correctly.

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

A Git tag is a *mutable* pointer: the same `@v1.2.0` can resolve to different
trees on two machines, or on the same machine a month apart. `mooncake.modules.lock`
closes that hole. It records a content hash per module reference, and a run that
finds a lockfile verifies every module it resolves against it. Absent a lockfile
nothing verifies and behavior is unchanged — the lockfile is opt-in, created by
`mooncake mod tidy`.

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

### Lockfile

- WHERE a module lockfile is written, its name SHALL be
  `mooncake.modules.lock`. It SHALL NOT be `mooncake.lock`, which is already
  owned by the `tool` action (`internal/lockfile`) for tool-install pinning;
  two unrelated lockfiles sharing one filename would silently clobber each
  other.
- WHEN a module tree is hashed, the hash SHALL be `h1:<base64(sha256(summary))>`
  where the summary is, for every regular file under the cache dir sorted by
  slash-separated relative path, the line `<sha256-hex>  <relpath>\n`. The
  `.git` directory SHALL be excluded; non-regular entries (symlinks, devices,
  sockets) SHALL be excluded.
- WHERE a reference carries a subpath, the hash SHALL cover the whole cached
  repo (`<cache>/<host>/<owner>/<repo>@<version>`), not the subpath, and the
  lock entry SHALL be keyed by `<host>/<owner>/<repo>@<version>` — one cache
  dir, one hash, one entry, however many subpaths reference it.
- WHEN the lockfile is saved, entries SHALL be sorted by ref so the file is
  byte-deterministic and diffs are reviewable, and the write SHALL be atomic
  (temp file + rename).
- WHEN a lockfile is present for a run, every module the resolver fetches SHALL
  be verified: a hash mismatch SHALL abort with both hashes named, and a
  reference absent from the lockfile SHALL abort telling the operator to run
  `mooncake mod tidy`.
- WHERE no lockfile is found, no verification SHALL occur and resolution SHALL
  behave exactly as it did before the lockfile existed.
- WHEN `mooncake mod tidy` runs, it SHALL fetch every reference in the
  playbook's `modules:` block, write their current hashes to
  `mooncake.modules.lock` next to that playbook, and drop entries no longer
  referenced.
- WHEN `mooncake mod verify` runs, it SHALL re-hash every locked module from
  the local cache without cloning, report each as ok/mismatch/missing, and exit
  non-zero if any is not ok.
- WHEN `mooncake mod list` runs, it SHALL print each alias in the playbook with
  its ref, whether it is cached, and whether it is locked; `--format json` SHALL
  emit the same as structured data.

## Non-goals
- Component execution, the planner's per-step `component_dir`/`invocation_dir`
  overlay, and action semantics — owned by the execution-engine spec.
- The `modules:`/`use:`/`props:` document grammar — owned by the config-model
  spec.
- A central registry or version-range resolution. References stay exact tags;
  the lockfile pins the *tree behind* a tag, it does not resolve ranges.
- File modes in the hash. The `h1:` summary covers path + content only, matching
  Go's `dirhash`; a module that flips only an executable bit hashes the same.
  Tracked as a known limitation, not a silent one.
- Recording the resolved commit SHA. The tree hash is the thing that matters for
  reproducibility and it works for subpath modules; a SHA would need an extra
  `git rev-parse` per fetch to buy nothing more.
- `mooncake mod init` (#46) — a new-project scaffold, gated on an external
  skeleton module publishing. Unrelated to lock verification.
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
- [x] Relative `dest:` containing a subdir resolves against the invocation dir,
  not the module cache dir (#50, fixed in `294b076f`: `ExpandStepsWithContext`
  now sets `FromComponent: true` — `internal/plan/planner.go:232,839`).
- [x] `mooncake.modules.lock`: `h1:` tree hash, sorted deterministic save,
  atomic write; name guarded against colliding with the `tool` lockfile.
- [x] Verify-on-resolve when a lockfile is present; no-op when absent; a
  present-but-corrupt lockfile fails rather than degrading to unverified.
- [x] `mooncake mod tidy` / `verify` / `list`, each with `--format json`.

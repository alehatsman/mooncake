# Spec 65: Module system — Phase 1 (loader + cache + `use:`)

**Status:** Draft
**Stream:** core
**Effort:** M (~1–2 weeks)
**Value:** High. Replaces the dropped in-tree presets directory with
Git-native distribution. Foundation for `mooncake share`, agent-
authored modules, and any future curated module org.

**Source brief:**
[GitHub issue #24](https://github.com/alehatsman/mooncake/issues/24).
**Strategic context:**
[`vision/sharing_and_modules.md`](../../../vision/sharing_and_modules.md).

---

## Problem

The in-tree `presets/` directory is dropped. The codebase still has
the loader (`internal/presets/loader.go`), the parameter validator
(`internal/presets/validator.go`), the expander
(`internal/presets/expander.go`), and the CLI
(`cmd/presets.go`) — but no preset content to load.

Users today have three options, all bad:

1. Inline every step into `mooncake.yml`. Loses reuse.
2. Maintain their own preset directory and reference it by relative
   path. Loses versioning + integrity.
3. Wait for us to ship modules. The wait is now.

The replacement primitive is **Git-native versioned modules**: a
module is a Git repo (or subpath of one) carrying a `module.yml`
manifest with typed parameters + steps. The user pins module versions
in a lockfile; the loader fetches them on demand, caches locally,
verifies checksums.

Phase 1 of the rollout is the minimum that gets one module to flow
end-to-end from `github.com/owner/repo@version` → cache → executed
steps in a user's plan. Lockfile, vendor mode, signing, and pseudo-
versions all defer to later phases.

## Goals

- **G1** `use:` step type. In a playbook,
  `use: github.com/owner/repo[/subpath]@<version>` resolves to a
  module reference. `with:` provides the parameter overrides.
- **G2** Module manifest format: `module.yml` at the module root
  with `parameters:` (same schema as today's `preset.yml`) and
  `steps:` (same shape as a plan).
- **G3** Loader: given a `use:` reference, fetch the module repo at
  the requested version into the local cache and load its `module.yml`
  through the existing preset expander.
- **G4** Cache layout at `~/.cache/mooncake/modules/<host>/<owner>/<repo>@<version>/`
  mirroring `$GOMODCACHE/cache/download/` shape.
- **G5** Parameter validation runs before any step from the module
  reaches the executor — same path the current preset validator uses.
- **G6** Plan mode (`mooncake plan`) renders module steps with their
  resolved parameters expanded, so the operator sees what will run
  before commit.
- **G7** Module-internal templates and `import:` work — a module can
  use `tasks/install.yml` exactly like today's presets do.

## Non-goals

- **`mooncake.lock` file.** Phase 2.
- **Checksum verification.** Phase 2.
- **Pseudo-versions** (`v0.0.0-<ts>-<sha>`). Phase 4.
- **Vendor mode** (`mooncake mod vendor`). Phase 4.
- **Signed tags / Sigstore.** Phase 5.
- **Cross-module dependencies.** A module's steps can use the typed
  action vocabulary; they cannot `use:` another module. Deferred
  indefinitely per `vision/sharing_and_modules.md` non-goals.
- **Private-Git auth.** Works over public HTTPS in Phase 1; SSH
  auth comes later when a real user asks.
- **A curated module index / search command.** Phase 1 assumes the
  user knows the module URL.
- **Reuse of existing `cmd/presets.go` subcommands** (`presets list`,
  `presets search`, etc.). Those operate on a local presets directory
  that no longer ships; whether they survive is a Phase 3 question.

## Design

### Module identity and resolution

A `use:` value has three parts:

```
github.com/owner/repo/subpath @ v1.2.3
└──────────────────────────┘   └────┘
        module path             version
```

- **module path** — a Git URL prefix that resolves to a clone-able
  repo plus a path inside it. The subpath is optional (omitted means
  the repo root). Mirrors Go's import-path convention.
- **version** — a Git tag. Phase 1 requires a tag; commit SHAs and
  pseudo-versions defer to Phase 4.

The loader splits the module path at the first directory after the
repo root. Concretely:

```
github.com/mooncake-modules/postgres        → repo=…/postgres,    subpath=""
github.com/mooncake-modules/all/postgres    → repo=…/all,         subpath="postgres"
```

For Phase 1 the splitter assumes "first `/<token>` after the host is
the owner, second is the repo, the rest is subpath." This matches
GitHub. Other hosts (GitLab nested groups) defer to Phase 2 — error
clearly until then.

### Manifest: `module.yml`

Same schema as today's `preset.yml`. The current Docker preset is
already module-shaped:

```yaml
name: docker
description: Install and configure Docker container runtime
version: 1.0.0

parameters:
  state:        { type: string, default: present, enum: [present, absent] }
  start_service:    { type: bool, default: true }
  install_compose:  { type: bool, default: true }

steps:
  - name: Install Docker
    import: tasks/install.yml
    when: parameters.state == "present"
  - name: Configure Docker
    import: tasks/configure.yml
    when: parameters.state == "present"
```

Phase 1 renames `preset.yml` → `module.yml` at the file-naming level
(loader accepts either for transition; emits a deprecation note when
it finds `preset.yml`). No schema change.

### `use:` step type

```yaml
modules:
  postgres: github.com/mooncake-modules/postgres@v1.3.0

steps:
  - use: postgres
    with:
      tls: true
      replication: true
```

Two-form support: in the `modules:` block, declare a short alias for
the module reference; in `steps`, `use:` takes the alias. Direct form
(`use: github.com/…@v1.3.0`) also works for one-shot usage. The alias
form parallels Go's `import` rename and reads cleanly in long
playbooks.

The planner expands a `use:` step into the module's `steps:` block,
substituting `parameters.*` references with the resolved `with:`
values. This is the existing preset-expansion path, repointed.

### Loader behaviour

```
1. Parse use:reference  →  (host, owner, repo, subpath, version)
2. cache_dir = ~/.cache/mooncake/modules/<host>/<owner>/<repo>@<version>/
3. If cache_dir exists and is non-empty → use it (skip fetch).
4. Else:
   a. Shallow git clone https://<host>/<owner>/<repo>.git → temp dir
   b. git checkout <version>
   c. Atomic rename temp dir → cache_dir
5. module_yml_path = cache_dir / subpath / module.yml
6. Load + validate + expand (existing path)
```

Concurrency: cache writes use atomic-rename, so two processes
fetching the same module-version race harmlessly.

Failure modes that need to be loud:
- module path doesn't parse → "expected `github.com/owner/repo[/path]@version`"
- version tag missing → "no such tag `v1.3.0` in `github.com/…`"
- module.yml not found at expected subpath → "module repo exists but
  has no module.yml at <subpath>"
- network unreachable during fetch + nothing cached → "module
  <reference> not cached and fetch failed: <error>"

### CLI surface (Phase 1)

```
mooncake mod download <reference>    # explicit fetch (otherwise lazy on plan/apply)
mooncake mod cache list               # show what's cached
mooncake mod cache clean              # rm -rf the cache dir
```

`mooncake mod tidy`, `mod verify`, `mod vendor`, `mod publish` —
all Phase 2+.

### Reuse map

| Existing | How Phase 1 uses it |
|---|---|
| `internal/presets/loader.go` | Loader of `module.yml` content; same code path |
| `internal/presets/validator.go` | Parameter validation; unchanged |
| `internal/presets/expander.go` | Step expansion + parameter substitution; unchanged |
| `cmd/presets.go` | Stays for `presets add <url>` UX; doesn't gain new `mod` subcommands here |

### New code

| Component | Location |
|---|---|
| `use:` step type on `Step` | `internal/config/config.go` (new struct + action tag) |
| Reference parser | `internal/modules/reference.go` |
| Git fetcher + cache | `internal/modules/fetch.go` |
| `mooncake mod ...` CLI | `cmd/mod.go` |

## Acceptance criteria

- [ ] `use: github.com/owner/repo@v1.0.0` in `mooncake.yml`
      resolves, fetches, and expands into the playbook
- [ ] Parameter overrides via `with:` validate against the module's
      declared `parameters:` schema
- [ ] Cache layout matches `~/.cache/mooncake/modules/<host>/<owner>/<repo>@<version>/`
- [ ] Re-running the same plan twice fetches once (cache hit on
      second run)
- [ ] `mooncake plan` renders module steps with parameter
      substitution before mutation
- [ ] Module-internal `import: tasks/install.yml` still resolves
      relative to the module's own root
- [ ] `mooncake mod download <ref>` is an explicit eager fetch
- [ ] Network failures during fetch surface as a clear error, not a
      generic Git error
- [ ] One real curated module (`github.com/mooncake-modules/docker@v1.0.0`,
      seeded from the dropped `presets/docker/`) flows end-to-end in
      a smoke-test playbook

## Migration

Phase 1 leaves the existing `cmd/presets.go` surface alone. Users
with local presets in `~/.mooncake/presets/` continue to work via
the registry layer. The new `use:` shape is additive.

The Phase 3 cleanup decision — whether `presets add` and `presets list`
disappear once modules are the norm — defers until module adoption is
real.

## Open questions

1. **Reuse `proxy.golang.org` or write our own fetcher?** The Go
   module proxy will serve any Git repo with a valid `go.mod` at its
   root, even if the rest is YAML. We could declare a stub `go.mod`
   in each module repo to opt into the proxy's caching for free. The
   alternative is direct `git clone` per fetch. Probably ship direct
   `git clone` first (simpler, fewer dependencies), reserve the option
   to add proxy support if rate-limiting bites.

2. **Should aliases in `modules:` allow version overrides at use
   site?** `use: postgres@v1.4.0` to temporarily try a newer version
   without editing the `modules:` block. Lean no for v1 (one source
   of truth per module ref); revisit if real workflows demand it.

3. **What's the minimum sanity check on a fetched module before
   loading?** Phase 1 = "file exists at expected path." Phase 2 adds
   checksum verification. Anything in between? Probably not.

4. **Does `use:` participate in `for_each:`?** Today, `import:` does.
   Probably yes — but check that parameter substitution composes
   correctly with iteration scoping. Worth a regression test.

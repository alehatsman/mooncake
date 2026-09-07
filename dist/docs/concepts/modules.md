---
type: concept
name: modules
source: internal/modules
---

# Modules

Modules are the reusable-component distribution layer for mooncake. A module is
a Git repository (or a subpath of one) that exports one or more components
through an `index.yml` manifest. References are pinned to an explicit Git tag —
no ranges, no lockfile, no surprises.

## Quick start

```bash
# Add a module to the nearest mooncake.yml
mooncake mod add github.com/mooncake-modules/postgres@v1.0.0

# Add with an explicit alias
mooncake mod add github.com/mooncake-modules/postgres@v1.0.0 --as pg
```

This fetches the module, reads its `index.yml` for the default alias name, and
upserts the `modules:` block in `./mooncake.yml`:

```yaml
modules:
  postgres: github.com/mooncake-modules/postgres@v1.0.0
```

Then reference the module in a component:

```yaml
steps:
  - use: postgres            # default export
  - use: postgres/backup     # named export
```

## Authoring a component

A component is a YAML file with two top-level keys:

```yaml
# components/install.yml
props:
  tls:  { type: bool,   default: false }
  port: { type: string, default: "5432" }
  state:
    type: string
    enum: [present, absent]
    default: present

steps:
  - pkg.install:
      name: postgresql

  - import: tasks/configure.yml
    when: "props.tls"
```

`props:` supports:

- `type:` — `string` · `bool` · `array` · `object`
- `required:` — defaults to `false`
- `default:` — value when the caller omits it
- `enum:` — restrict to a fixed set
- `description:` — surfaced by `mooncake validate` and the schema docs

Inside steps, reference props via `{{ props.<name> }}` (or `{{ parameters.<name> }}` —
the legacy spelling still resolves; see *Migration* below).

Components can include other YAML files relative to the component's directory
(`import: tasks/configure.yml`), but cannot themselves invoke other components
yet (no `use:` inside a component).

### Migration: `parameters:` → `props:`

Components that still use `parameters:` continue to work. Loading one emits a
one-time deprecation warning:

```
warning: components/install.yml uses `parameters:` which is deprecated — rename to `props:`
```

To migrate: rename the top-level key. Step expressions can use either
`{{ parameters.x }}` or `{{ props.x }}` — both namespaces are injected at
expansion time. Mixing `props:` and `parameters:` in the same file is an
error; pick one.

## Reducing boilerplate: default props + task shorthand

Two sugars cut the ceremony when a repo wires the same module export into many
tasks.

### Module-level default props

A `modules:` entry can be an object with `props:` instead of a bare string.
Those props are applied as **defaults** to every `use:` of that alias, so an
invariant value (a `dir:`, a `go_tags:`) is declared once, not at every call
site:

```yaml
modules:
  goq:
    source: "127.0.0.1:8080/owner/go-quality@v0.1.1"
    props:
      go_tags: "{{ GO_TAGS }}"   # templated like any per-call prop
  tq:
    source: "127.0.0.1:8080/owner/ts-quality@v0.1.0"
    props: { dir: web }

steps:
  - use: tq/lint            # runs with dir=web, no props: needed
  - use: tq/lint            # a per-call prop wins over the default:
    props: { dir: other }   #   → dir=other
```

Precedence, highest first: **per-call `props:` > module default props >
the component's own prop defaults**. The bare-string form
(`goq: ".../@v0.1.1"`) still works and carries no defaults.

A default prop is applied **only to the exports that declare it** — a default
for a prop a given component doesn't define is silently skipped, not an error.
That's what lets one binding carry, say, a `go_tags` default that only some of
a module's exports accept: `use: goq/lint` picks it up, `use: goq/budget-status`
(which has no `go_tags` prop) ignores it.

### Task shorthand: a string task value is a `use:` reference

A task value may be a `use:` reference string instead of a full
`{ steps: [...] }` map. It expands to a single-step task:

```yaml
tasks:
  ui-lint: tq/lint          # == { steps: [{ use: tq/lint }] }
  ui-build: tq/build
  lint: goq/lint
```

When a shorthand task has no `desc:`, `mooncake task` shows the referenced
**component's own `description:`** — so the listing never drifts from the
component. Local components are read directly; module aliases are resolved
from the **local module cache only** (never cloned, so the listing stays
offline). If the module isn't cached yet, the listing shows a `→ <ref>` hint
until the next run populates the cache. Need extra props?
Use the full map form: `ai-lint-all: { steps: [{ use: goq/ai-lint, props: { all: true } }] }`.

Combined with module default props, the one-liner is a complete working task.

## Reference format

```
<host>/<owner>/<repo>[@<version>]
<host>/<owner>/<repo>/<subpath>[@<version>]
```

Version is required — the `@<version>` suffix must be present and non-empty.
Examples:

```
github.com/mooncake-modules/postgres@v1.3.0
gitlab.com/myorg/infra/roles/nginx@v2.0.0
127.0.0.1:8080/aleh/mooncake-modules@main
```

The host, owner, and repo segments must all be non-empty. A `..` subpath that
would escape the module cache root is rejected.

## Module manifest (`index.yml`)

Every module repository must have an `index.yml` at its root (or subpath root):

```yaml
name: postgres
description: PostgreSQL install, configure, and backup components
exports:
  default: components/install.yml
  backup:  components/backup.yml
  replica: components/replica.yml
```

- `name` — required; becomes the default alias when added with `mod add`.
- `exports` — required map of export names to component paths (relative to the
  module root). The `default` key is the export selected by `use: alias` without
  a slash suffix.

## Using modules in a playbook

Declare the module once under `modules:`, then reference it in any `use:` step:

```yaml
modules:
  postgres: github.com/mooncake-modules/postgres@v1.3.0

steps:
  - use: postgres            # resolves to exports.default
  - use: postgres/backup     # resolves to exports.backup
```

Inline remote references skip the alias map entirely — useful for one-off
inclusions without touching the playbook:

```yaml
steps:
  - use: github.com/mooncake-modules/postgres@v1.3.0   # default export inline
```

Local paths (`./components/foo.yml`) are dispatched directly by the executor
and do not go through the module resolver.

## Path resolution inside a module component

When a fetched component runs, two directories govern path resolution:

| Path type | Resolves against |
|-----------|-----------------|
| `file.*` source paths | Module origin dir (the cached module root) |
| `file.*` destination paths | Invocation dir (caller's working directory) |
| `shell`/`cmd` cwd | Invocation dir |

This means a component can ship its own templates/files alongside its
component YAML while still writing outputs into the caller's tree.

## Cache

Modules are shallow-cloned (`--depth 1 --branch <tag>`) into an on-disk cache
and atomically renamed into place. Concurrent fetches of the same reference
race for the rename; the loser finds the directory already populated and
short-circuits.

Default cache root: `~/.cache/mooncake/modules`  
Override: `$MOONCAKE_MODULE_CACHE`

Manage the cache with the `mod cache` subcommands:

```bash
mooncake mod cache list    # show all cached <host>/<owner>/<repo>@<version> entries
mooncake mod cache clean   # remove the entire cache root
```

`mooncake task` and other read-only callers resolve components from the local
cache only — a listing never triggers a network clone.

## Environment variables

| Variable | Effect |
|----------|--------|
| `MOONCAKE_MODULE_CACHE` | Override the cache root directory |
| `MOONCAKE_MODULE_INSECURE` | Comma-separated list of `host` or `host:port` values that are allowed to clone over plain `http` instead of `https`. Use for trusted local servers (e.g. a self-hosted moongit on `127.0.0.1:8080`). |

## Lockfile: reproducible trees

`mooncake mod tidy` fetches every reference in the playbook's `modules:` block
and every export those modules pull in transitively, then writes
`mooncake.modules.lock` next to the playbook — an `h1:`-hashed tree pin, sorted
deterministic, dropping entries no longer referenced. `mooncake mod verify`
re-hashes every locked module and fails on drift; `mooncake mod list` prints
each alias with its resolved version and lock status. See `specs/modules.md`
for the full contract.

## Error messages

The loader maps failure modes to canonical messages, useful when matching in
tests or CI tooling:

| Condition                          | Message                                                           |
|------------------------------------|-------------------------------------------------------------------|
| Reference missing `@version`       | `expected <url>@<version>, e.g. github.com/owner/repo@v1.0.0`     |
| Tag not in repo                    | `no tag <v> in <host>/<owner>/<repo>`                             |
| `index.yml` missing                | `module has no index.yml at root (<path>)`                        |
| Export name unknown                | `module <name> has no export "<x>"; available: <list>`            |
| Export points at missing file      | `export "<x>" points to <rel> which does not exist`               |
| Network failure, nothing cached    | `module not cached and fetch failed: <git error>`                 |
| Local path not found               | `component not found: <path>`                                     |
| Unknown alias                      | `unknown module alias "<a>" (not declared in modules: block)`     |

## CLI reference

- [`mooncake mod add`](../cli/mod_add.md) — fetch a module and register it in
  the playbook
- [`mooncake mod tidy`](../cli/mod_tidy.md) — write/refresh `mooncake.modules.lock`
- [`mooncake mod verify`](../cli/mod_verify.md) — re-hash locked modules, fail on drift
- [`mooncake mod list`](../cli/mod_list.md) — print aliases with resolved version + lock status
- [`mooncake mod cache list`](../cli/mod_cache_list.md) — list cached entries
- [`mooncake mod cache clean`](../cli/mod_cache_clean.md) — remove the cache

## Known limitations

- No cross-module dependencies, SSH/private-Git auth, or `mooncake mod init`
  scaffold yet — see moongit #181's remaining children for status.
- `mod add` rewrites the `modules:` block via `yaml.v3`; comments and key order
  in that section are not preserved.
- A module component's `file.copy` destination that contains a subdirectory
  suffix resolves against the module origin dir rather than the invocation dir
  when it is relative (issue #50). Use `{{ invocation_dir }}/subdir/file` as a
  workaround.

<!-- Generated by mooncake docs generate; version dev -->

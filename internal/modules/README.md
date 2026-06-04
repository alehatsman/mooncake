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

## CLI reference

- [`mooncake mod add`](../cli/mod_add.md) — fetch a module and register it in
  the playbook
- [`mooncake mod cache list`](../cli/mod_cache_list.md) — list cached entries
- [`mooncake mod cache clean`](../cli/mod_cache_clean.md) — remove the cache

## Known limitations

- References are pinned to exact tags — version ranges and automatic update are
  out of scope for phase 1.
- `mod add` rewrites the `modules:` block via `yaml.v3`; comments and key order
  in that section are not preserved.
- A module component's `file.copy` destination that contains a subdirectory
  suffix resolves against the module origin dir rather than the invocation dir
  when it is relative (issue #50). Use `{{ invocation_dir }}/subdir/file` as a
  workaround.

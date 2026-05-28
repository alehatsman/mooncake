# Spec 67: Module system — revised design (supersedes spec-65)

**Status:** Draft
**Stream:** core
**Effort:** M (~1–2 weeks)
**Supersedes:** spec-65 (module-system-phase-1)

---

## Problem

Spec-65 drafted the initial module system using `module.yml` and `parameters:`. After
usability review, the design is revised on three points:

1. `index.yml` is a **manifest** (name + exports map), not a component itself.
2. Components use `props:` instead of `parameters:`.
3. Local components (`use: ./path/to/component.yml`) are in scope for phase 1.

Everything else from spec-65 carries forward: Go-module-shaped identity, Git tag
versions, `~/.cache/mooncake/modules/` cache, reuse of existing component
loader/validator/expander.

---

## Concepts

### Module

A Git repository (or subpath of one) that exports one or more components via
`index.yml`. Identified by a URL + Git tag:

```
github.com/mooncake-modules/postgres@v1.3.0
└──────────────────────────────────┘  └────┘
           module path               version
```

### Component

A reusable unit with typed `props:` and `steps:`. Stored as a `.yml` file inside a
module (or locally on disk).

```yaml
# components/install.yml
props:
  tls:  { type: bool, default: false }
  port: { type: int,  default: 5432  }
steps:
  - pkg.install:
      name: postgresql
  - import: tasks/configure.yml
    when: "props.tls"
```

### `index.yml`

The module's entry point. Declares the module's identity and maps export names to
component files. Does not contain steps or props itself.

```yaml
# index.yml
name: postgres
description: PostgreSQL database setup and management
exports:
  default: components/install.yml
  backup:  components/backup.yml
```

- `default` export resolves when the user writes `use: postgres` (no slash).
- Named exports resolve when the user writes `use: postgres/backup`.
- Files not listed in `exports:` are internal — helpers, templates, task includes.

---

## Playbook syntax

### Remote module

```yaml
# mooncake.yml
modules:
  postgres: github.com/mooncake-modules/postgres@v1.3.0

steps:
  - use: postgres          # default export → components/install.yml
    with:
      tls: true

  - use: postgres/backup   # named export → components/backup.yml
    with:
      schedule: "0 2 * * *"
```

### Local component

```yaml
steps:
  - use: ./components/setup-user.yml
    with:
      username: alice
```

No `modules:` entry required. Path resolves relative to the playbook file.

---

## CLI surface (phase 1)

```
mooncake mod add <url>@<version>          # fetch, cache, write to modules: block
mooncake mod add <url>@<version> --as <alias>  # override the alias
mooncake mod cache list                   # show cached modules
mooncake mod cache clean                  # remove all cached modules
```

### `mooncake mod add` behaviour

1. Parse the reference: `(host, owner, repo, subpath, version)`.
2. Fetch + cache the module (same as lazy fetch on plan/apply — see Loader below).
3. Read `index.yml` → extract `name:` as the default alias.
4. Write or update the `modules:` block in the nearest `mooncake.yml`.
5. Print a summary:

```
added postgres (github.com/mooncake-modules/postgres@v1.3.0)
exports: default, backup
```

Version must be explicit — `mooncake mod add github.com/…/postgres` without a tag is
an error. Use `git tag -l` or the GitHub UI to find the latest tag.

---

## Loader behaviour

```
use: postgres
  │
  ├─ look up alias "postgres" in modules: block
  │    → github.com/mooncake-modules/postgres@v1.3.0
  │
  ├─ cache_dir = ~/.cache/mooncake/modules/github.com/mooncake-modules/postgres@v1.3.0/
  │
  ├─ if cache_dir missing:
  │    a. shallow git clone https://github.com/mooncake-modules/postgres.git → temp dir
  │    b. git checkout v1.3.0
  │    c. atomic rename temp → cache_dir
  │
  ├─ read cache_dir/index.yml
  │    → resolve export "default" → components/install.yml
  │
  └─ load cache_dir/components/install.yml as a component
       → validate props against with:
       → expand steps (existing component expander, repointed)

use: ./components/setup-user.yml
  │
  └─ skip modules: lookup
       → load path relative to playbook file
       → validate props, expand steps (same path)
```

Failure modes:

| Condition | Error message |
|---|---|
| Reference missing `@version` | `expected <url>@<version>, e.g. github.com/owner/repo@v1.0.0` |
| Tag not found in repo | `no tag v1.3.0 in github.com/mooncake-modules/postgres` |
| `index.yml` missing | `module has no index.yml at root` |
| Export name not in `index.yml` | `module postgres has no export "backup"; available: default` |
| Component file missing | `export "backup" points to components/backup.yml which does not exist` |
| Network failure, nothing cached | `module not cached and fetch failed: <git error>` |
| Local path not found | `component not found: ./components/setup-user.yml` |

---

## `props:` field

`props:` replaces `parameters:` in all component files. Same schema (type, default,
required, enum, description). The existing validator and expander in
`internal/component/` are updated in place — no parallel code paths.

Inside step expressions, props are referenced as `props.<name>`:

```yaml
props:
  tls: { type: bool, default: false }
steps:
  - import: tasks/tls.yml
    when: "props.tls"
```

Backward compat: the loader emits a deprecation warning when it finds `parameters:`
in a component file and maps it to `props:` transparently. Hard-remove in phase 2.

---

## Module repo layout (convention, not enforced)

```
github.com/mooncake-modules/postgres/
├── index.yml
├── components/
│   ├── install.yml
│   └── backup.yml
├── tasks/
│   ├── configure.yml
│   └── tls.yml
└── templates/
    └── postgresql.conf.j2
```

Only `index.yml` is required. `components/`, `tasks/`, `templates/` are conventions
that `mooncake mod init` will scaffold (phase 2).

---

## New code

| Component | Location |
|---|---|
| Reference parser | `internal/modules/reference.go` |
| Git fetcher + cache | `internal/modules/fetch.go` |
| `index.yml` loader | `internal/modules/index.go` |
| Module resolver (ties reference + fetch + index) | `internal/modules/resolver.go` |
| `Use` field on `Step`, `Modules` map on `Plan` | `internal/config/config.go` |
| `mooncake mod ...` CLI | `cmd/mod.go` |

## Changed code

| Component | Change |
|---|---|
| `internal/component/loader.go` | Accept `props:` field; deprecation shim for `parameters:` |
| `internal/component/validator.go` | Validate `props:` map |
| `internal/component/expander.go` | Substitute `props.<name>` in step expressions |
| `internal/executor/executor.go` | Dispatch `use:` step via module resolver or local path loader |

---

## Acceptance criteria

- [ ] `use: github.com/owner/repo@v1.0.0` inline (no alias) resolves, fetches, expands
- [ ] `modules:` alias + `use: alias` resolves via the alias map
- [ ] `use: alias/name` resolves named export from `index.yml`
- [ ] `use: ./components/foo.yml` resolves relative to playbook file
- [ ] `props:` validates against `with:` before any steps reach the executor
- [ ] `mooncake plan` renders expanded steps with props substituted
- [ ] Cache hit on second run — no re-fetch
- [ ] `mooncake mod add <url>@<version>` fetches + writes `modules:` block
- [ ] `--as <alias>` overrides the default alias
- [ ] Missing version tag in `mod add` is a clear error
- [ ] All failure modes in the table above surface as their specified messages
- [ ] `parameters:` in a component file emits a deprecation warning and still works

## Non-goals (phase 1)

- Lockfile / checksum verification (phase 2)
- `mooncake mod tidy`, `mod verify`, `mod vendor` (phase 2+)
- `mooncake mod latest <url>` to resolve latest tag (phase 2)
- SSH / private Git auth (later)
- Cross-module dependencies
- `mooncake mod search`
- `mooncake mod init` scaffold (phase 2)

---

## Open questions

1. ~~`use:` inline without `modules:` block~~ — **included in phase 1.** `use:
   github.com/owner/repo@v1.0.0` directly in steps resolves without an alias entry.

2. **`mooncake_version:` in `index.yml`** — deferred to phase 2.

3. **`for_each:` + `use:`** — `use:` already exists on `Step` (previously pointing at
   `*PresetInvocation`). The field is repointed to the new component reference type.
   `for_each:` composes with `use:` the same way it composes with any step today —
   no special handling needed. Add a regression test to confirm props substitution
   scopes correctly inside a loop.

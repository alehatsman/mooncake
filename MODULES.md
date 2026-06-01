# Modules and Components

Mooncake's reuse layer is two concepts:

- **Component** — a reusable unit of steps with typed inputs (`props:`).
  Stored as a YAML file (`components/install.yml`, `./shared/setup.yml`, …).
- **Module** — a Git repository (or subpath of one) that exports one or more
  components via an `index.yml` manifest at its root. Identified by
  `<host>/<owner>/<repo>[/subpath]@<tag>`.

Components are the unit you consume. Modules are how components are
distributed and versioned.

---

## Quick start

```yaml
# mooncake.yml
modules:
  postgres: github.com/mooncake-modules/postgres@v1.3.0
  # or, with default props applied to every `use: <alias>` (see below):
  # postgres: { source: github.com/mooncake-modules/postgres@v1.3.0, props: { tls: true } }

steps:
  # Remote module via alias (default export)
  - use: postgres
    props:
      tls: true

  # Named export from the same module
  - use: postgres/backup
    props:
      schedule: "0 2 * * *"

  # Local component, no modules: entry needed
  - use: ./components/setup-user.yml
    props:
      username: alice
```

Add the module to your playbook with the CLI:

```
mooncake mod add github.com/mooncake-modules/postgres@v1.3.0
```

The version tag is required — pinning is the whole point.

---

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
the legacy spelling still resolves, see *Migration* below).

Components can include other YAML files relative to the component's directory
(`import: tasks/configure.yml`), but cannot themselves invoke other components
yet (no `use:` inside a component).

---

## Authoring a module

A module is a Git repository with an `index.yml` at the root:

```yaml
# index.yml
name: postgres
description: PostgreSQL database setup and management
exports:
  default: components/install.yml
  backup:  components/backup.yml
```

Conventional layout:

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

Only `index.yml` is required. Files not listed under `exports:` are
internal — helpers, templates, task includes. They are reachable from
exported components via `import:`/`template:` but not via `use:`.

Tag each release: `git tag v1.3.0 && git push --tags`. Mooncake fetches by
tag; the tag and the cache directory are the integrity boundary.

---

## How `use:` is resolved

`use:` accepts four syntactic forms; the executor classifies them and
dispatches accordingly:

| Form                                     | Resolution                                                           |
|------------------------------------------|----------------------------------------------------------------------|
| `./components/foo.yml` / `../x/y.yml`    | Local path relative to the playbook (or including) file              |
| `github.com/owner/repo@v1.0.0`           | Inline remote — fetch + load `index.yml`'s `default` export          |
| `alias`                                  | Look up `alias` in `modules:` → fetch + `default` export             |
| `alias/exportname`                       | Same as above but use the named export                               |

`props:` is always a sibling key on the step that supplies prop values.

### Resolution flow for a remote/alias use:

```
use: postgres
 │
 ├─ alias "postgres" in modules:?  → github.com/mooncake-modules/postgres@v1.3.0
 │
 ├─ cache dir = ~/.cache/mooncake/modules/github.com/mooncake-modules/postgres@v1.3.0/
 │
 ├─ if missing:
 │    1. git clone --depth 1 --branch v1.3.0 <url> <tmp>
 │    2. atomic rename <tmp> → cache dir
 │
 ├─ read index.yml → resolve "default" → components/install.yml
 │
 └─ load component, validate props against props:, expand steps
```

The cache is content-immutable at `(host, owner, repo, version)`. A second
run with the same version is a pure cache hit — no clone, no network.

---

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
**component's own `description:`** (for a local component) — so the listing
never drifts from the component. For a module alias it shows a `→ <ref>` hint
rather than resolving the module (a listing stays offline). Need extra props?
Use the full map form: `ai-lint-all: { steps: [{ use: goq/ai-lint, props: { all: true } }] }`.

Combined with module default props, the one-liner is a complete working task.

---

## The `mooncake mod` CLI

```
mooncake mod add <url>@<version>                # fetch, cache, write modules:
mooncake mod add <url>@<version> --as <alias>   # override the default alias
mooncake mod cache list                         # list cached modules
mooncake mod cache clean                        # wipe the cache
```

`mod add`:

1. Parses the reference; rejects refs missing `@<version>`.
2. Fetches and caches the module.
3. Reads `index.yml` and uses `name:` as the default alias (override with `--as`).
4. Writes/updates the `modules:` block in `./mooncake.yml` (or `--playbook <path>`).
5. Prints a summary listing the available exports.

The cache location can be overridden with `MOONCAKE_MODULE_CACHE`. Default:
`~/.cache/mooncake/modules`.

---

## Error messages

The loader maps failure modes to canonical messages, useful when matching in
tests or in CI tooling:

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

---

## Migration: `parameters:` → `props:`

Components that still use `parameters:` continue to work. Loading one emits a
one-time deprecation warning:

```
warning: components/install.yml uses `parameters:` which is deprecated — rename to `props:`
```

To migrate: rename the top-level key. Step expressions can use either
`{{ parameters.x }}` or `{{ props.x }}` — both namespaces are injected at
expansion time. Mixing `props:` and `parameters:` in the same file is an
error; pick one.

---

## What's not in phase 1

These are explicit non-goals; expect them in phase 2+:

- Lockfile and checksum verification
- `mooncake mod tidy` / `mod verify` / `mod vendor`
- `mooncake mod latest <url>` (resolve to latest tag)
- SSH / private Git authentication
- Cross-module dependencies
- `mooncake mod init` scaffold
- Comment preservation when `mod add` rewrites `mooncake.yml`

---

## See also

- [`docs-working/streams/core/specs/spec-67-module-system-revised.md`](docs-working/streams/core/specs/spec-67-module-system-revised.md)
  — design rationale and acceptance criteria.
- [`internal/modules/`](internal/modules/) — parser, fetcher, index loader, resolver.
- [`internal/presets/`](internal/presets/) — component loader, validator, expander
  (named `presets` for historical reasons; the user-facing concept is "component").

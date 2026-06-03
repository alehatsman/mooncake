---
id: config-model
status: draft
owners: [aleh]
covers:
  - "internal/config/*.go"
---

# Config Model (YAML Playbook)

## Intent
The config model is the typed, declarative surface a mooncake playbook is
written against. It defines the loadable document — a versioned root with
global vars, module bindings, an ordered step list, and named tasks — and the
universal shape of a step: exactly one action plus orthogonal modifiers
(conditionals, idempotency guards, loops, tags, privilege, environment) and
compound control-flow wrappers (transaction, try/catch/finally). It is read-only
intent; parsing and validation reject malformed documents before any plan or
mutation is built.

## Behavior
- WHEN a config file is read, the root document SHALL parse as a `RunConfig`
  with optional `version`, `vars`, `modules`, `tasks`, and an ordered `steps`
  list, OR as a bare top-level array of steps for backward compatibility.
- WHEN `mooncake apply`/`plan`/`validate` run without `-c`, the loader SHALL
  auto-discover `./mooncake.yml` or `./mooncake/main.yml`.
- WHERE input is supplied, the loader SHALL accept either YAML or JSON via the
  same auto-detect path (`DecodeAuto`).
- WHEN a step is validated, exactly one action field SHALL be set; zero or more
  than one is an error (compound wrappers count as the step's single action).
- WHERE an action key is used, it SHALL be the dot-namespaced name for its
  domain (`file.*`, `text.*`, `os.*`, `pkg`, `cmd`, `repo.*`, `artifact.*`,
  `container.*`, `git.*`, `http.request`, `observe.*`, `wait.*`) or a
  foundational flat key (`shell`, `assert`, `log`, `use`, `import`, `vars`,
  `vars.load`, `action`/`with`).
- WHEN a step sets `when`, the step SHALL run only if the expression evaluates
  truthy against the merged variable scope.
- WHEN a step or run is filtered by `--tags`/`--skip-tags`, only steps carrying
  (or lacking) a matching `tags` entry SHALL be selected for execution.
- WHEN a step declares `creates`/`unless_exists` or `unless`/`unless_command`,
  the step SHALL be skipped when the path exists or the probe command succeeds.
- WHEN a step declares `for_each` or `for_each_file`, it SHALL expand into one
  cloned step per item with the loop variable bound in scope.
- WHEN a step uses `import`, the named file SHALL be loaded and its steps
  spliced in; cyclic includes SHALL be rejected.
- WHEN a step uses `vars.load`, the referenced file's variables SHALL merge into
  the shared variable scope for subsequent steps.
- WHEN a step sets `transaction`, it SHALL carry no leaf action and its children
  apply all-or-nothing; an optional sibling `on_rollback` runs only if the
  transaction rolled back.
- WHEN a step sets `try`, optional `catch`/`finally` branches SHALL be valid only
  alongside `try`; `catch`/`finally` without `try` is an error.
- WHEN a value is tagged `!secret <provider>:<key>`, it SHALL be carried as a
  typed secret reference resolved at apply time, never inlined as plaintext.
- WHEN `modules:` binds an alias, a step's `use: <alias>` SHALL resolve against
  it, with module-level default props overridden by per-call `props`.
- WHEN validation runs, it SHALL emit `Diagnostic`s (severity, message, YAML
  path, source position, context) covering YAML syntax, JSON-schema structure,
  template syntax, the one-action invariant, and compound-shape rules.

## Non-goals
- Expression evaluation semantics for `when`/`changed_when`/`failed_when`
  (`internal/expression`) and Jinja2 template rendering (`internal/template`) —
  owned by the templating/expression spec.
- The `dscl` helper (`internal/dscl`) — a macOS directory-service shim consumed
  by `os.user`/`os.group` action handlers, not part of the document model.
- Per-action field semantics and execution (owned by per-action handler docs
  and the execution-engine spec).
- Plan compilation, diffing, applying, and rollback execution — execution-engine
  spec.

## Checklist
- [x] `RunConfig` root: `version`, `vars`, `modules`, `tasks`, `steps` (+ bare
  step-array back-compat).
- [x] Auto-discovery of `./mooncake.yml` / `./mooncake/main.yml`.
- [x] YAML or JSON input via `DecodeAuto`.
- [x] `Step` one-action invariant enforced by `Validate()`.
- [x] Dot-namespaced action keys + foundational flat keys.
- [x] `when` conditional and `tags` selection fields.
- [x] Idempotency guards: `creates`/`unless_exists`, `unless`/`unless_command`.
- [x] Loops: `for_each`, `for_each_file`.
- [x] `import` include with cycle detection; `vars.load` variable include.
- [x] `transaction` + `on_rollback` compound shape and validation rules.
- [x] `try`/`catch`/`finally` compound shape and validation rules.
- [x] `!secret <provider>:<key>` typed secret refs.
- [x] `modules:` alias bindings with default props (`use:`/`props:`).
- [x] Diagnostic-based validation (YAML, JSON-schema, template, structural).

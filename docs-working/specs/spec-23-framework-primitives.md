# Spec 23: Framework Primitives — `on_change`, `try`/`catch`/`finally`, `!secret`

**Status:** Draft
**Epic:** E9 Modern Action Surface — bucket E9.2
**Effort:** M (1–2 weeks)
**Value:** Closes three long-standing primitive gaps that every realistic
playbook hits: reactive triggers, structured error handling, and secrets
that don't leak into logs.

**Source:** `VISION_ACTIONS.md` §6.1, §6.2, §6.7.

**Note:** `transaction:` blocks (§6.8) are split into their own spec
(spec 30) because they depend on the extended ABI's `Reverse` method
(spec 22).

---

## Problem

Every non-trivial playbook today hits one of these walls:

1. **No reactive triggers.** When `file.template: nginx.conf.j2` writes a
   new config, you want the nginx service reloaded — but only if the
   template actually changed. Today you write
   `when: nginx_cfg.changed` on a separate step, which works for one
   downstream but doesn't scale and clutters the playbook. Ansible's
   `notify` solves this with a magic registry; we want a modern, explicit
   shape.
2. **No structured error recovery.** When a deploy script fails mid-way,
   you want to roll back, notify, then bail. Today: `continue_on_error`
   makes the step swallow its error but the next step has no idea
   anything went wrong, and there's no concept of "run X regardless".
3. **Secrets in YAML are values, not references.** A `file.write` step
   with a Vault token in the `content:` field bakes the secret into
   plans, run logs, MCP tool I/O, and snapshot artifacts. We want
   `content: !secret vault:secret/app#token` — the literal value never
   appears anywhere except the executor's in-memory call to the secret
   provider.

---

## Goals

- **G1** Add `on_change:` keyword on `Step`. Value is a sequence of
  Steps that run iff the parent step's `outputs.changed` is true. No
  global "handlers" namespace.
- **G2** Add `try` / `catch` / `finally` top-level Step shape. Each
  branch is a sequence of Steps. Catch runs on any error in `try`;
  finally always runs.
- **G3** Add `!secret <ref>` YAML tag. Resolved at apply time by a
  registered secret-provider; never serialized into plans, logs, run
  logs, or MCP tool output.
- **G4** Wire all three through the planner, executor, and event/runlog
  redaction.
- **G5** No legacy compatibility — keep the modern shape pure.

**Out of scope (separate specs):**

- `transaction:` blocks with reverse-on-failure (spec 30).
- Secret-provider plugins themselves (`secret.vault`, `secret.1password`)
  — those are Tier-2 plugin specs. This spec ships only the `!secret`
  tag mechanism + a built-in `env:` provider (`!secret env:DB_PASSWORD`).

---

## Design

### 1. `on_change:` — Reactive triggers

YAML shape:

```yaml
- file.template:
    src: nginx.conf.j2
    dest: /etc/nginx/nginx.conf
  on_change:
    - os.service: { name: nginx, state: reloaded }
    - log: "nginx reloaded after config change"
```

Semantics:

- `on_change` is a sequence of Steps (same shape as the top-level steps).
- Each child step runs iff the parent step's outputs reported `changed:
  true`. Otherwise skipped (with `skip_reason: "parent didn't change"`).
- Children run in declaration order. A failing child surfaces its error
  normally (and may itself have `on_change`, `try/catch`, etc. — they
  nest).
- Children inherit the parent's `as_user`, `env`, and `cwd` unless they
  override.
- Children's `outputs` are scoped to themselves; they don't leak into
  the parent.

In the Step struct (`internal/config/config.go`):

```go
OnChange []Step `yaml:"on_change" json:"on_change,omitempty"`
```

### 2. `try` / `catch` / `finally`

YAML shape:

```yaml
- name: deploy app
  try:
    - pkg.install: { name: postgresql }
    - file.template: { src: app.conf.j2, dest: /etc/app/app.conf }
    - os.service: { name: app, state: restarted }
  catch:
    - log: "deploy failed, rolling back"
    - shell: ./rollback.sh
  finally:
    - notify.slack: { message: "deploy finished (success={{ try.success }})" }
```

Semantics:

- The Step type gains `Try`, `Catch`, `Finally` fields, each a `[]Step`.
- When all three are present, the Step is a "compound" step (no other
  action allowed in the same Step).
- `try` runs sequentially. On the first error:
  - `catch` runs (if defined). `catch` steps can read `try.error` /
    `try.failed_step` via outputs.
  - If a catch step itself errors, the compound Step propagates the
    later error.
- `finally` runs whether `try` succeeded or failed.
- The compound Step's overall `changed` is `OR` of its children's
  changed.
- The compound Step's `outputs.success` is `true` iff `try` ran to
  completion without entering `catch`.

In the Step struct:

```go
Try     []Step `yaml:"try" json:"try,omitempty"`
Catch   []Step `yaml:"catch" json:"catch,omitempty"`
Finally []Step `yaml:"finally" json:"finally,omitempty"`
```

Validation: if `Try` is set, no action field may be set; if `Try` is
unset, `Catch` and `Finally` must also be unset.

### 3. `!secret <ref>` — Secret references

YAML shape:

```yaml
- file.write:
    path: /etc/app/token
    content: !secret env:APP_TOKEN

- pkg.install:
    name: ghcli
    extra:
      - --token=!secret vault:secret/github#cli-token
```

Mechanism:

- A custom YAML tag `!secret` carries a string ref (`provider:path`).
- At parse time, `!secret` values are wrapped in a `SecretRef` Go type
  (`internal/config/secret.go`):

  ```go
  type SecretRef struct{ Ref string }
  func (s SecretRef) String() string { return "!secret " + s.Ref }
  func (s SecretRef) MarshalJSON() ([]byte, error) { return []byte(`"!secret"`), nil }
  ```

  `MarshalJSON` deliberately omits the ref — only the type marker reaches
  serialized output.

- At apply time, the template engine (`internal/template/`) recognizes
  `SecretRef` and calls the registered secret provider. The resolved
  value is passed to the action; the original `SecretRef` stays in the
  Step struct so re-serialization stays redacted.

- Providers register via `internal/security/secrets.go`. Built-in
  `env:` provider ships in this spec; `vault:`, `1password:`, `age:`,
  `sops:` follow in their own Tier-2 specs.

- Redaction: the executor's event publisher and runlog writer both
  receive a SecretRef-aware redactor. Any string containing a resolved
  secret value gets the literal redacted to `***` in events / runlogs.
  (`internal/security/redact.go` already has the redaction primitive;
  we wire SecretRef-resolved values into its denylist.)

### Interactions

- `on_change` + `try/catch`: an `on_change` child can be a `try/catch`
  compound. Order: parent runs → on_change runs (which may have its own
  try/catch).
- `!secret` + `on_change`: secrets resolved in the parent are *not*
  passed to children; children re-resolve.
- `!secret` + `try/catch`: a SecretRef resolution failure in `try` goes
  to `catch` like any other error. The error message must redact the
  attempted provider/path beyond the provider prefix (i.e. `vault:` is
  fine, `vault:secret/app#token` is not).

---

## Key files

| File | Change |
|---|---|
| `internal/config/config.go` | Step gains `OnChange`, `Try`, `Catch`, `Finally` `[]Step` fields. Validation updated. |
| `internal/config/secret.go` | New file. `SecretRef` type, custom YAML tag, JSON marshal. |
| `internal/config/reader.go` | Wire `!secret` tag into YAML decoder. |
| `internal/plan/planner.go` | Expand `on_change` and `try/catch/finally` into the plan. Each compound Step becomes a small DAG of plan steps with `parent_id` linking. |
| `internal/executor/executor.go` | Execute compound shape: `try` → on error → `catch` → `finally`. Skip-or-run `on_change` based on parent's `outputs.changed`. |
| `internal/template/renderer.go` | SecretRef resolution at render time. |
| `internal/security/secrets.go` | New file. Provider registry. Built-in `env:` provider. |
| `internal/security/redact.go` | Extend denylist with SecretRef-resolved values at apply time. |
| `internal/events/event.go` | Per-step event payload gains `triggered_by` (for `on_change` children) and `compound_role` (`try`/`catch`/`finally` or empty). |
| `internal/runlog/runlog.go` | Same. |
| `internal/mcp/tools.go` | MCP plan tool surfaces compound structure. |
| `internal/config/schema.json` | Regenerate to include the new keywords. |

---

## Tasks (phased)

1. **Phase 1** — `!secret` tag + `env:` provider + redaction wiring.
   Smallest blast radius. Lands first so subsequent specs can use it.
2. **Phase 2** — `on_change` keyword. Planner expansion, executor
   conditional execution, tests.
3. **Phase 3** — `try` / `catch` / `finally`. Compound Step parsing,
   validation, planner expansion, executor compound execution, tests.
4. **Phase 4** — Cross-cutting integration tests (`on_change` ↔ try,
   secret inside try, etc.).
5. **Phase 5** — Docs. New pages: `docs-next/guide/triggers.md`,
   `docs-next/guide/error-handling.md`, `docs-next/guide/secrets.md`.
6. **Phase 6** — Schema regen, `make schema-check` + `make docs-check`
   green.

---

## Acceptance criteria

- `examples/triggers/on-change-nginx.yml` writes a templated config and
  reloads nginx only when the template content actually differs from
  what's on disk.
- `examples/error-handling/try-catch-deploy.yml` simulates a failing
  middle step; `catch` runs, `finally` runs, exit code is non-zero, run
  log shows the compound structure.
- `examples/secrets/env-secret.yml` reads `APP_TOKEN` from environment
  via `!secret env:APP_TOKEN`. `mooncake plan --format json` output
  contains `"content": "!secret"` — no token value. The actual file
  written under `--apply` contains the resolved token.
- Compound steps appear in MCP tool output as a single Step with
  `try`/`catch`/`finally` children inline.
- Build / vet / lint / test green. Schema regenerated. Docs check green.

---

## Open questions

1. **Should `on_change` see grandparents' changes?** I.e. if a parent
   has `on_change: [...]` and one of those children also has
   `on_change: [...]`, does the grandchild fire on grandparent change
   or parent change? Probably parent (each on_change scopes to its
   immediate parent). Codify in tests.
2. **Should `try` rescue from panics, not just returned errors?** Go
   tradition says no; spec-21 actions return errors, not panic. Decline
   panic handling.
3. **`!secret` resolution timing — plan vs apply?** Plan resolves to
   `***` (informational), apply resolves to real value. This keeps
   plan output safe to share. Confirm in spec.
4. **Custom YAML tag interop with the schema validator?** JSON Schema
   has no tag concept; the validator sees a string after parsing. The
   redaction is in the marshal layer. Confirm there's no leak.
5. **Naming: `!secret` vs `!ref`?** `!secret` is explicit. `!ref` is
   shorter but could collide with future non-secret refs (template
   refs, file refs). Lean `!secret`.

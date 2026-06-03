---
id: templating-and-facts
status: draft
owners: [aleh]
covers:
  - "internal/template/*.go"
  - "internal/expression/*.go"
  - "internal/facts/*.go"
  - "internal/factsfmt/*.go"
---

# Templating, Expressions & Facts

## Intent

The data layer turns YAML intent into resolved values before any action runs: it
renders templated strings, evaluates conditionals, and gathers a snapshot of the host
so playbooks can branch on the machine they target. It is read-only — it computes the
inputs handlers act on, never mutating the system itself.

## Behavior

- WHERE a string contains `{{ … }}` or `{% … %}`, it is rendered by the pongo2
  engine (Jinja2-like) against the variable scope; autoescape is disabled globally
  because mooncake renders to terminals and event payloads, not HTML
  (`internal/template/renderer.go:82`).
- WHEN a bare `{{ var }}` / `{{ var.field }}` resolves to a map or slice, it is
  auto-routed through the `tojson` filter so it renders as JSON instead of pongo2's
  `<TYPE Value>` reflect repr (`internal/template/renderer.go:411`).
- WHERE templates need custom filters, the engine registers `expanduser` (expands a
  leading `~`), `tojson`/`json`, and `strftime` (POSIX format codes; unknown codes
  fail loudly) exactly once (`internal/template/renderer.go:74`).
- WHEN rendering at plan time, `RenderPreserving` keeps `{{ expr }}` placeholders for
  any root variable not yet in scope (e.g. registered results from later steps),
  falling back to plain `Render` if the template contains `{% %}` control tags
  (`internal/template/renderer.go:370`).
- IF a user writes Jinja2-style filter args `{{ x | default('y') }}`, the render
  error is annotated with a hint pointing at pongo2's colon syntax
  `{{ x | default:'y' }}` (`internal/template/renderer.go:451`).
- WHERE a step carries a `when` / `changed_when` / `failed_when` condition, the
  string is first rendered as a template, then evaluated as an expr-lang expression
  against the same scope (`internal/plan/planner.go:1017`,
  `internal/executor/finalize.go:63`).
- WHEN evaluating, undefined variables are allowed (evaluate to nil) so conditions
  may reference results of steps that have not run or were skipped
  (`internal/expression/evaluator.go:67`).
- WHEN evaluating, `True`/`False` are bound as boolean constants (unless the caller
  already defined them) so a pongo2-rendered `True`/`False` token from
  `when: "{{ flag }}"` evaluates correctly (`internal/expression/evaluator.go:45`).
- WHERE expressions need helpers, the `AllFunctions()` library plus a legacy `has`
  are injected — string/number/array predicates, `default`, `coalesce`, `ternary`,
  and `env`/`has_env` for environment access (`internal/expression/evaluator.go:84`,
  `internal/expression/functions.go:532`).
- IF compilation or evaluation fails, the error is enhanced with the offending
  expression and a hint (syntax / undefined / type / nil / index)
  (`internal/expression/evaluator.go:110`).
- WHEN facts are first needed, `facts.Collect()` gathers OS/arch/host/user, network,
  hardware (CPU/memory/disks/GPUs), kernel, distribution, package manager,
  toolchains, optional Ollama, uptime, and service state — cached once per process
  via `sync.Once` (`internal/facts/facts.go:126`, `internal/facts/cache.go:12`).
- WHERE facts feed templates, `Facts.ToMap()` flattens them into scope keys plus
  convenience booleans — `linux`/`darwin`/`macos`/`windows`, `apt_available` and
  peers — and a `home` alias for `user_home` (`internal/facts/facts.go:186`).
- WHEN facts enter execution, the planner and executor scope merge `Facts.ToMap()`
  into the variable namespace alongside user vars and registered results
  (`internal/plan/planner.go:292`, `internal/executor/scope.go:102`).
- WHERE the operator runs `mooncake facts`, `factsfmt.DisplayFacts` renders the
  snapshot as a sectioned, human-readable terminal report
  (`internal/factsfmt/factsfmt.go:61`).

## Non-goals

- The variable scope model itself (precedence of user vars vs facts vs registered
  results, scope cloning, loop-var injection) — owned by the execution-engine /
  planner spec; this spec only describes what the data layer contributes to it.
- Action behavior and the result envelope — owned by `specs/actions.md` and the
  execution-engine spec.
- Choice of template/expression libraries as a stable public contract (pongo2 and
  expr-lang are implementation choices, not a guaranteed external API).

## Checklist

- [x] pongo2 rendering with autoescape off; `Render` / `RenderPreserving`.
- [x] Custom filters: `expanduser`, `tojson`/`json`, `strftime`.
- [x] Auto-JSON of non-scalar bare variables.
- [x] Jinja2 filter-arg misuse hint.
- [x] expr-lang evaluation for `when`/`changed_when`/`failed_when` (render-then-eval).
- [x] Undefined-variable tolerance + `True`/`False` binding.
- [x] `AllFunctions()` library incl. `env`/`has_env`; enhanced compile/runtime errors.
- [x] Host facts gathering (cross-platform + per-OS), cached once.
- [x] `Facts.ToMap()` with convenience booleans + `home` alias, merged into scope.
- [x] `factsfmt` terminal rendering of facts.
- [ ] `internal/template/renderer.go:1` package doc and several comments describe the
      engine as both "pongo2" and "Jinja2"; on-disk naming is consistent but the
      user-facing vocabulary should be pinned to one term to avoid drift.

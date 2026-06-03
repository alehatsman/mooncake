---
id: task-runner
status: draft
owners: [aleh]
covers:
  - "cmd/task/*.go"
  - "internal/config/discover_tasks.go"
---

# Task Runner (`mooncake task`)

## Intent
The task runner is the make-like, named-target front end for mooncake: a
`tasks.yml` (or a `tasks:` block in `mooncake.yml`) declares named tasks, and
`mooncake task <name>` runs one through the very same planner + executor that
`mooncake apply` uses — only the step list and the var overlay differ. Bare
`mooncake task` lists the discovered tasks with their descriptions. It is the
dev-loop surface (build/test/lint), so step stdout streams by default and the
preview path is `--plan`, never `--dry-run`.

## Behavior
- WHEN `mooncake task` is run, discovery SHALL prefer `./tasks.yml`/`./tasks.yaml`;
  otherwise it SHALL fall back to the apply-config search path
  (`mooncake.yml`, `mooncake/main.yml`) ONLY IF that file defines at least one
  task.
- WHERE a dedicated tasks file is chosen AND a `mooncake.yml` in the same dir
  also defines `tasks:`, the tasks file SHALL win and a single stderr warning
  SHALL name the shadowed apply-config.
- WHERE `--config`/`-c` is given, it SHALL be used verbatim and no discovery or
  shadow check SHALL run.
- WHEN a dedicated tasks file declares top-level `steps:` outside any task, a
  stderr warning SHALL be emitted that those steps are ignored by the task runner.
- WHEN no config is found, the runner SHALL print the no-config hint and exit
  with the validation exit code.
- WHEN `mooncake task` is run with no argument, it SHALL list every task sorted
  by name as `name — description`; more than one positional argument SHALL be
  rejected.
- WHEN a task has no explicit `desc:`, its listed description SHALL fall back to
  the `description:` of the component referenced by a single-`use:` shorthand
  task, resolved CACHE-ONLY (never cloned); if unresolvable it SHALL show
  `→ <ref>`, else `(no description)`. The field is `desc:` — `description:` is
  not a task field.
- WHEN `mooncake task <name>` runs, the planner SHALL splice the task's steps
  and vars via its `TaskName` field and execute the resulting plan through the
  in-memory apply runner, streaming step stdout/stderr regardless of
  `--log-level`.
- WHEN `--plan`/`-p` is set, the plan SHALL be built and rendered (text/json/yaml
  via `--format`, with `--diff`/`--show-origins` for text) WITHOUT executing,
  and SHALL NOT be written to a file.
- WHEN variables are layered, precedence (highest first) SHALL be `--vars` files
  (later wins on collision), then task-level `vars:`, then file-level `vars:`.
- WHEN steps are filtered, `--tags` and `--skip-tags` SHALL compose via AND over
  the task's steps.
- WHEN `--dry-run` is passed, it SHALL be rejected with a usage error steering
  the user to `--plan`.
- WHERE a tasks file declares only `tasks:` with no top-level `steps:`, it SHALL
  carry `version: "1.0"` so the reader parses it as a structured config rather
  than a bare step array.
- WHERE templates are rendered, the built-in directory vars SHALL be
  `component_dir` and `invocation_dir`; there is no built-in `{{ cwd }}` var.

## Non-goals
- The planner's task→steps splice, loop expansion, secret resolution, and
  template rendering — owned by the execution-engine / config-model specs.
- The `tasks:`/`desc:`/`steps:` document grammar — owned by the config-model spec.
- Apply-only operator UX deliberately dropped from `task`: `--from-plan`,
  `--host`/`--overlays`, `--allow-stale`, `--max-plan-age`, `--tui`, and saving
  plans to disk (that is `mooncake plan -o`).
- Module fetch/cache/resolve mechanics — owned by the modules spec.

## Checklist
- [x] Discovery: `tasks.yml`/`tasks.yaml` preferred; `mooncake.yml` fallback
  only if it defines tasks; explicit `--config` bypass.
- [x] Shadow warning when both a tasks file and a tasks-bearing apply-config exist.
- [x] Warning for top-level `steps:` in a dedicated tasks file.
- [x] No-config hint + validation exit code.
- [x] Listing sorted `name — description`; ≤1 positional arg.
- [x] `desc:` precedence with single-`use:` cache-only `description:` fallback
  (`→ <ref>` / `(no description)`).
- [x] Run via planner `TaskName` + in-memory apply runner; step stdout streams.
- [x] `--plan` render (text/json/yaml, `--diff`, `--show-origins`), no file write.
- [x] Var precedence: `--vars` > task `vars:` > file `vars:`; `--tags`/`--skip-tags` AND.
- [x] `--dry-run` rejected with a `--plan` hint.
- [x] Tasks-only file needs `version: "1.0"`; `{{ cwd }}` is not a built-in var.

# Spec 21: Modernization Cutover (Mooncake 2.0)

**Status:** Shipped — 9 commits between `c77c461` and `f0ac797` on `master`
(2026-05-12 / 2026-05-13). Build / vet / lint / 55 test packages green;
schema + docs regenerated; sibling feature branches (`agentd`, `spec-19`
tool, `container-actions`, `partition-action-exploration`) integrated.

**Epic:** E9 Modern Action Surface — see
[`epic-spec-21-followup.md`](epic-spec-21-followup.md) for the
next-iteration roadmap (specs 22–31).

**Effort delivered:** ~2 days end-to-end (extended by CI fixes + branch
integration in the same session).

**Value:** Foundational — locked in the modern action surface and
dot-namespaced YAML keys before any further expansion. Without this,
every subsequent spec would have carried the cost of mixed v1/v2
conventions forever.

**Companion docs:** `VISION.md`, `VISION_ACTIONS.md`.

**Backward compatibility:** None. Clean break. No aliases, no deprecation
period, no migrator tool. Users on existing YAML stay on `v1.x`; `v2.0`
parses only the modern surface.

---

## Problem

Mooncake's action and framework vocabulary is a mix of Ansible-derived names
(`file`, `template`, `become`, `register`, `with_items`, `include_vars`) and
ad-hoc flat names (`file_replace`, `repo_apply_patchset`,
`artifact_capture`). Two problems flow from this:

1. **Strategic positioning.** Mooncake's wedge is being the modern, AI-era
   config runtime. Inheriting 2010s vocabulary undercuts that and freezes in
   cognitive baggage forever — once the schema ships v1.x in production for
   long enough, renaming requires deprecation pain. The window to do this
   cleanly is now, before the install base grows.
2. **Scaling surface area.** As we add Tier-1 actions (`pkg.install`,
   `os.user`, `text.line`, `git.clone`, `wait.port`, etc. — see
   `VISION_ACTIONS.md` §5), the existing flat keys clash with the natural
   dot-namespaced names of the new ones. Adding `pkg.install` while keeping
   `package` is incoherent.

This spec is the single coordinated cutover that pivots the entire surface
to dot-namespaced action keys and modern framework keywords, in one release.

---

## Goals

- **G1** Rename every action YAML key to its modern form (see §6.1 mapping).
- **G2** Rename every framework keyword on `Step` to its modern form
  (see §6.2).
- **G3** Rename the Handler ABI's two existing methods (`Execute` → `Apply`,
  `DryRun` → `Plan`) and corresponding callers — see §6.3.
- **G4** Regenerate the JSON Schema (`internal/config/schema.json`) and the
  generated `mooncake.d.ts` to reflect the new surface.
- **G5** Rewrite every built-in preset (~1192 YAML files under `presets/`)
  and every example (~62 YAML files under `examples/`) to use the new
  surface.
- **G6** Update every doc page that shows YAML or references the renamed
  surface.
- **G7** Cut a `v2.0.0` release.

**Out of scope (explicit follow-up specs):**

- New Tier-1 actions (`pkg.install`, `os.user`, `text.line`, `git.clone`,
  `wait.port`, …) — each gets its own spec, all targeting the modern surface
  this spec lands.
- New ABI methods (`Diff`, `Reverse`, `Cost`, `Permissions`) — Spec 22.
- New framework primitives (`on_change`, `try`/`catch`/`finally`,
  `transaction:`, `!secret` refs) — Spec 23.
- Ansible-to-Mooncake translator CLI — out-of-band tool, not part of v2 core.

---

## Decisions locked

These are not up for re-litigation inside this spec; they were settled in
`VISION_ACTIONS.md` and during the brainstorm.

1. **No legacy aliases. Ever.** Single canonical name per action.
2. **Dot-namespaced names for actions that touch the world** (file, text,
   os, pkg, repo, artifact, …). Flat names for control flow and meta
   primitives (`shell`, `assert`, `wait`, `vars`, `import`, `use`, `log`).
3. **No backward compatibility shim** in v2. The v1.x branch keeps existing
   YAML running for users who don't migrate.
4. **One coordinated PR / release**, not a multi-step rollout. The schema
   change is atomic by design.
5. **`shell` and `assert` keep their flat keys** because they are
   foundational primitives, not domain actions. They would be silly to
   namespace.
6. **`for_each` and `for_each_file` are two separate keywords**, not one
   polymorphic keyword. The semantics differ enough (`for_each_file` walks a
   tree and exposes `item.is_dir`, `item.depth`) to deserve its own keyword.
7. **Go package directory reshuffle is OUT of scope for this spec.** Phase 4
   removed. Keep `internal/actions/file_replace/` etc. as-is; only the YAML
   key + `Metadata().Name` change. Reshuffle gets a follow-up spec if anyone
   misses the alignment.
8. **CLI is already on `plan`/`apply`** — verified in `cmd/mooncake.go`:721,
   735, 741. The recent `feat!: drop deprecated dry-run / check / run
   surface` commit already cut this. No CLI work in this spec.

---

## 5b. Audit findings (gathered 2026-05-12)

Concrete answers to gaps the first draft punted on.

### Registry & dispatch — already name-driven

`internal/actions/registry.go` keys handlers by `meta.Name` from each
handler's `Metadata()`. **The rename therefore flows from updating two
places per action**: the handler's `Metadata().Name` literal, and the YAML
tag on the corresponding `Step` field. No registration-site changes needed
elsewhere.

### Single central dispatcher

`internal/config/config.go:818-898` (`Step.DetermineActionType()`) is the
one if/else chain that returns the action-type *string* from a populated
struct field. Every rename must update both the returned literal *and* the
Go field name being checked.

### Hard-coded action-name literals to update by hand

Found by `grep -rEn '"(file|template|shell|service|copy|download|unarchive|assert|preset|print|command|include_vars|package|wait)"' internal/` (excluding handler + test files):

| File | Line | Today |
|---|---|---|
| `internal/executor/dryrun.go` | 174 | `d.logAction("unarchive", msg)` |
| `internal/logger/agent_subscriber.go` | 168 | `Action: "package"` |
| `internal/config/template_validator.go` | 89 | literal `"shell"` |
| `internal/config/template_validator.go` | 121 | literal `"include_vars"` |

Plus `internal/config/config.go:DetermineActionType` itself. Anything else
that turns up via the grep gets the same treatment.

False positives to ignore (these are *values*, not action names):
- `internal/artifacts/metadata.go:158-161` — file-extension classification.
- `internal/filetree/walker.go:47` — file-tree node state, not action name.
- `internal/config/config.go:719` — `FilePath` Origin field tag.
- `internal/config/config.go:384` — `AssertFile` sub-property.

### Schema generation — pinned

`make schema-generate` (see `Makefile:185-192`) runs:
```
./out/mooncake schema generate --format json --output internal/config/schema.json --strict
./out/mooncake schema generate --format typescript --output internal/config/schema.d
```
`make schema-check` (`Makefile:194-208`) verifies cleanliness in CI.

**Open audit item**: there is a root-level `mooncake.d.ts` whose first lines
read `Auto-generated from action metadata. Do not edit manually -
regenerate with: mooncake schema generate --format typescript`. The
Makefile only writes `internal/config/schema.d`. Determine in Phase 5
whether the root file is (a) stale and should be deleted, (b) a published
artifact that needs a second `schema generate --format typescript --output
mooncake.d.ts` invocation added to `Makefile:schema-generate`, or (c) a
symlink. Quick `diff internal/config/schema.d mooncake.d.ts` + a `git log
-- mooncake.d.ts` will tell.

### Test fixtures — minimal

Outside `presets/` (1192) and `examples/` (62), the only YAML files in the
source tree are `internal/facts/tools.yml` (a facts source, not an example
— do not rewrite) and `internal/metrics/testdata/` (metrics test data, not
step YAML — do not rewrite). All other test YAML lives as inline strings
inside `*_test.go` files; failing tests after the rename will surface them.

---

## 6. The renames (canonical mapping)

### 6.1 Action keys + Go field names

The full mapping — YAML key, Go field on `Step` (`internal/config/config.go:
656-680`), `Metadata().Name` literal, and the value returned from
`DetermineActionType()` (`config.go:818-898`). All four must update together.

**Naming convention for new Go field names:** PascalCase concatenation of
namespace + verb, no separator. `file.write` → `FileWrite`. `text.replace`
→ `TextReplace`. `os.service` → `OsService`. `vars.load` → `VarsLoad`.
Flat keys keep their idiomatic Go name (`Shell`, `Assert`, `Wait`, `Log`,
`Use`, `Import`, `Vars`).

| Today YAML | Modern YAML | Go field today → modern | Pointer type (unchanged) |
|---|---|---|---|
| `file` | `file.write` | `File` → `FileWrite` | `*File` |
| `template` | `file.template` | `Template` → `FileTemplate` | `*Template` |
| `copy` | `file.copy` | `Copy` → `FileCopy` | `*Copy` |
| `download` | `file.download` | `Download` → `FileDownload` | `*Download` |
| `unarchive` | `file.unarchive` | `Unarchive` → `FileUnarchive` | `*Unarchive` |
| `file_replace` | `text.replace` | `FileReplace` → `TextReplace` | `*FileReplace` |
| `file_insert` | `text.insert` | `FileInsert` → `TextInsert` | `*FileInsert` |
| `file_delete_range` | `text.delete_range` | `FileDeleteRange` → `TextDeleteRange` | `*FileDeleteRange` |
| `file_patch_apply` | `text.patch` | `FilePatchApply` → `TextPatch` | `*FilePatchApply` |
| `package` | `pkg` | `Package` → `Pkg` | `*Package` |
| `service` | `os.service` | `Service` → `OsService` | `*ServiceAction` |
| `command` | `cmd` | `Command` → `Cmd` | `*CommandAction` |
| `repo_search` | `repo.search` | `RepoSearch` → `RepoSearch` (unchanged) | `*RepoSearch` |
| `repo_tree` | `repo.tree` | `RepoTree` → `RepoTree` (unchanged) | `*RepoTree` |
| `repo_apply_patchset` | `repo.patch` | `RepoApplyPatchset` → `RepoPatch` | `*RepoApplyPatchset` |
| `artifact_capture` | `artifact.capture` | `ArtifactCapture` → `ArtifactCapture` (unchanged) | `*ArtifactCapture` |
| `artifact_validate` | `artifact.validate` | `ArtifactValidate` → `ArtifactValidate` (unchanged) | `*ArtifactValidate` |
| `shell` | `shell` | `Shell` (unchanged) | `*ShellAction` |
| `assert` | `assert` | `Assert` (unchanged) | `*Assert` |
| `wait` | `wait` | `Wait` (unchanged) | `*WaitAction` |
| `print` | `log` | `Print` → `Log` | `*PrintAction` |
| `preset` | `use` | `Preset` → `Use` | `*PresetInvocation` |
| `include` | `import` | `Include` → `Import` | `*string` |
| `include_vars` | `vars.load` | `IncludeVars` → `VarsLoad` | `*string` |
| `vars` | `vars` | `Vars` (unchanged) | `*map[string]interface{}` |

**Pointer-type renames are optional and out of scope.** Renaming `*File` →
`*FileWriteAction` cascades into every action package and helps nothing.
Keep pointer types as-is; only the field name and the YAML tag change.

The `meta.Name` literal in each handler's `Metadata()` updates to the new
YAML key (e.g. `Name: "file.write"`). Located via:
```
grep -rn 'Name:\s*"' internal/actions/*/handler.go
```

### 6.2 Framework keywords on `Step`

| Today | Modern | Notes |
|---|---|---|
| `with_items` | `for_each` | Aligns with Terraform / modern IaC. |
| `with_filetree` | `for_each_file` | Or fold into `for_each` with a filetree expression — TBD in Task 5. |
| `register` | `as` | Terse, modern. |
| `become: true` + `become_user: <name>` | `as_user: <name>` | Single field. `as_user: root` means root. Omitted = current user. |
| `creates` | `unless_exists` | Clearer English. |
| `unless` | `unless_command` | Disambiguates from `unless_exists`. |
| `ignore_errors` | `continue_on_error` | Modern wording. |
| `retries` + `retry_delay` | `retry: {attempts, delay, backoff}` | Single structured block. |
| `when` | `when` | Keep — universal. |
| `changed_when` | `changed_when` | Keep. |
| `failed_when` | `failed_when` | Keep. |
| `tags` | `tags` | Keep. |
| `timeout` | `timeout` | Keep. |
| `env`, `cwd` | `env`, `cwd` | Keep. |
| `name` | `name` | Keep. |

### 6.3 Handler ABI

| Today | Modern |
|---|---|
| `Execute(ctx, step) (Result, error)` | `Apply(ctx, step) (Result, error)` |
| `DryRun(ctx, step) (Result, error)` | `Plan(ctx, step) (Result, error)` |
| `Metadata()`, `Validate(step)` | unchanged |

`Diff`, `Reverse`, `Cost`, `Permissions` arrive in **Spec 22**, not here.
Keep the interface tight in this cutover.

### 6.4 YAML migrator tool design

Lives at `scripts/migrate/v1-to-v2/main.go`. **Throwaway code** — kept in
the repo after merge so external users can run it, marked unsupported,
removable any time.

**Design:**

```
mooncake-migrate-v2 [--write] [--dry-run] [--root <dir>] [<file>...]
```

Behavior:
1. Walks each file or directory argument (default: `.`), processing
   `*.yml` and `*.yaml`.
2. Parses each file with `gopkg.in/yaml.v3` into a `*yaml.Node` AST (NOT
   `Unmarshal` into a struct — that loses comments and key order).
3. Walks the AST; for every node that *is a Step object*, renames the
   action key and framework keywords using a hard-coded mapping table
   (the same tables in §6.1 and §6.2).
4. Re-serializes via `yaml.Marshal` (preserves comments, key order, line
   endings).
5. `--dry-run` prints a unified diff per file and exits non-zero if any
   changes pending. `--write` overwrites in place. Default = dry-run.

**"Is a Step object" detection** (the only subtle part):

A `yaml.Node` of kind `MappingNode` is a Step iff its parent is one of:
- The document root if the root is a `SequenceNode` (top-level list of
  steps — the common shape).
- A `SequenceNode` whose parent's preceding scalar key is `steps`.
- A `SequenceNode` whose parent's preceding scalar key is one of
  `try`, `catch`, `finally`, `on_change`, `transaction` (forward-compat —
  these don't exist in v1 but will in v2).

Crucially, a `MappingNode` whose parent's preceding scalar key is itself
an action name (`assert`, `file`, `template`, …) is **NOT** a Step — it's
the action's own property bag. Do not rewrite its keys.

**Mapping tables:**

```go
var actionRenames = map[string]string{
    "file":               "file.write",
    "template":           "file.template",
    "copy":               "file.copy",
    "download":           "file.download",
    "unarchive":          "file.unarchive",
    "file_replace":       "text.replace",
    "file_insert":        "text.insert",
    "file_delete_range": "text.delete_range",
    "file_patch_apply":  "text.patch",
    "package":            "pkg",
    "service":            "os.service",
    "command":            "cmd",
    "repo_search":        "repo.search",
    "repo_tree":          "repo.tree",
    "repo_apply_patchset": "repo.patch",
    "artifact_capture":   "artifact.capture",
    "artifact_validate":  "artifact.validate",
    "print":              "log",
    "preset":             "use",
    "include":            "import",
    "include_vars":       "vars.load",
    // shell, assert, wait, vars — unchanged
}

var frameworkRenames = map[string]string{
    "with_items":     "for_each",
    "with_filetree":  "for_each_file",
    "register":       "as",
    "creates":        "unless_exists",
    "unless":         "unless_command",
    "ignore_errors":  "continue_on_error",
    // become/become_user collapse: handled with a custom transform that
    // emits as_user: <name> or as_user: root, then removes both old keys.
    // retries + retry_delay collapse: emit retry: {attempts, delay}.
}
```

The `become`/`become_user` and `retries`/`retry_delay` collapses need
custom logic, not a simple key rename — code them as explicit transforms.

**Acceptance for the migrator:**

- Round-trips comments and key order on every file under `presets/`.
- Does not mutate any YAML file outside of valid Step keys (verified by
  diffing a hand-curated set of "should-not-change" fixtures).
- Run on `presets/` + `examples/` produces a single big commit where every
  v2 file parses with the v2 schema.

---

## 7. Key files

### Core schema and parsing

| File | Role |
|---|---|
| `internal/config/config.go` | `Step` struct fields (l.656–680) — rename all action fields. Framework field renames (l.683–706). Custom `UnmarshalYAML` may need adjusting. |
| `internal/config/schema.json` | JSON Schema — regenerate against new field names. |
| `internal/config/schema.d` (if exists) | Regenerate. |
| `mooncake.d.ts` | TypeScript types — regenerate via `schemagen`. |
| `internal/schemagen/` | Generator may need name-mapping updates if it derives YAML keys from Go field names. |

### Action handlers

Each action's `handler.go` declares its name in `Metadata()`. Update the
`Name` field on every one of these:

```
internal/actions/{artifact_capture, artifact_validate, assert, command,
copy, download, file, file_delete_range, file_insert, file_patch_apply,
file_replace, include_vars, package, preset, print, repo_apply_patchset,
repo_search, repo_tree, service, shell, template, unarchive, vars, wait}
/handler.go
```

Plus the corresponding `<action>/handler_test.go` for each.

Optional but recommended: rename the Go package directories to match the new
namespace (`internal/actions/file_replace/` → `internal/actions/text/replace/`)
to keep filesystem layout coherent with the YAML surface. Mechanical refactor.

### Registry & dispatcher

| File | Role |
|---|---|
| `internal/register/register.go` | Update all `_ "...actions/<name>"` imports if package dirs are renamed. |
| `internal/actions/registry.go` | Registry keys (`registry.Register("file", ...)`) — confirm they pull from `Metadata().Name`, not a hard-coded string. |
| `internal/actions/handler.go` | Interface — rename `Execute` → `Apply`, `DryRun` → `Plan`. |
| `internal/executor/executor.go` | Dispatch — rename calls. |
| `internal/executor/mode.go`, `internal/actions/mode.go` | If "dryrun"/"execute" strings are used as mode names, rename to "plan"/"apply". |

### Planner / loops / register

| File | Role |
|---|---|
| `internal/plan/planner.go` | `evaluateItemsExpression`, `with_items`/`with_filetree` resolution. Rename to `for_each` / `for_each_file`. |
| `internal/plan/planner.go` (loop context) | `LoopContext.Type` values (`"with_items"`, `"with_filetree"`) → `"for_each"`, `"for_each_file"`. |
| `internal/executor/` | Wherever `step.Register` / `step.WithItems` / `step.Become` are read — rename. |

### MCP server

| File | Role |
|---|---|
| `internal/mcp/tools.go` | If tool names are derived from action names, they propagate automatically. If hard-coded, update. |
| `internal/mcp/server.go` | Tool descriptions — sweep for any literal old names. |

### Agent loop

| File | Role |
|---|---|
| `internal/agent/prompt.go` | Any embedded examples in the system prompt — update to modern surface. |
| `internal/agent/sanitize.go` | If it scans for action names, update the list. |

### Built-in presets & examples

- `presets/` — 1192 YAML files. Rewrite all. Mechanical sed with a known
  mapping table is acceptable; spot-check a representative sample
  (kubectl, ollama, dev tools, complex multi-file presets) by hand.
- `examples/` — 62 YAML files. Same treatment.
- `test-repo-ops.yml` at project root.

### Documentation

- `docs-next/` — every page that shows YAML.
- `docs-next/guide/config/actions.md` — full rewrite of the action table.
- `docs-next/guide/config/reference.md` — full rewrite of the property
  reference.
- `docs-next/api/config.md`, `docs-next/api/`.
- `docs-next/presets/definitive-style-guide.md` (1390 lines) — rewrite.
- `README.md`, `LLM_GUIDE.md`, `llms.txt`, `CLAUDE.md`.
- `docs-next/about/changelog.md` — add the v2.0 entry.

---

## 8. Tasks

Phased so each phase compiles before moving on. PRs can stack but the cutover
lands as a single tag.

### Phase 1 — ABI rename (mechanical, no behavior change)

Rename `Execute` → `Apply` and `DryRun` → `Plan` across the entire codebase.

1. Edit the `Handler` interface in `internal/actions/handler.go`.
2. Rename the method on every handler under `internal/actions/*/handler.go`.
3. Rename all call sites in `internal/executor/`, `internal/plan/`,
   `internal/mcp/`, tests.
4. `go build ./...` and `make test` clean.

This compiles atomically — one PR.

### Phase 2 — `Step` struct & YAML parser rework

Rewrite `Step` (`internal/config/config.go:644-715`).

1. Rename every action field — Go field name follows new YAML key:
   `Template *Template` → `FileTemplate *FileTemplate` mapped to
   `yaml:"file.template"`, etc.
2. Keep the "exactly one action field non-nil per step" invariant in
   `ValidateOneAction` / `countActions` (`internal/config/config.go:737`).
3. Update Go field names everywhere they're read (planner, executor,
   handler dispatch, tests). The Go-side rename is mechanical;
   compiler will catch every site.
4. Verify YAML keys with dots round-trip cleanly through `gopkg.in/yaml.v3`
   (they do — `.` is unreserved in YAML keys, but write a tiny test to be
   safe).
5. Rename framework keyword fields per §6.2: `WithItems` → `ForEach`,
   `WithFileTree` → `ForEachFile`, `Register` → `As`, `Become` /
   `BecomeUser` → `AsUser` (consolidate), `Creates` → `UnlessExists`,
   `Unless` → `UnlessCommand`, `IgnoreErrors` → `ContinueOnError`,
   `Retries` + `RetryDelay` → `Retry *RetryPolicy`.

### Phase 3 — Registry, dispatch, and `Metadata().Name`

Registry is already name-driven (audit §5b) — no registry-code changes
needed. Concretely:

1. **Update each handler's `Metadata().Name`** to the new dotted name. One
   string-literal edit per handler.
2. **Update `Step.DetermineActionType()`** (`internal/config/config.go:818-898`)
   — every Go field name in the if-chain AND every returned string literal.
3. **Update the 4 hard-coded literals** found in the audit:
   - `internal/executor/dryrun.go:174` — `"unarchive"` → `"file.unarchive"`
   - `internal/logger/agent_subscriber.go:168` — `"package"` → `"pkg"`
   - `internal/config/template_validator.go:89` — `"shell"` (unchanged, but
     verify context still applies)
   - `internal/config/template_validator.go:121` — `"include_vars"` →
     `"vars.load"`
4. **Re-run** `grep -rEn '"(file|template|copy|download|unarchive|file_replace|file_insert|file_delete_range|file_patch_apply|package|service|command|repo_search|repo_tree|repo_apply_patchset|artifact_capture|artifact_validate|print|preset|include|include_vars)"' internal/ cmd/` after Phases 2 & 3 to catch any literal that slipped in between the audit and the cutover.
5. **CLI commands**: no work — `apply` and `plan` already shipped (audit §5b).
   Sweep `cmd/*.go` only for any old action-name literals in `Usage`/`Description`.

### Phase 4 — Removed

Package directory reshuffle (`internal/actions/file_replace/` →
`internal/actions/text/replace/`) is **out of scope** per Decision 7.
Filesystem layout stays. Follow-up spec if anyone misses the alignment.

### Phase 5 — Schema regeneration

1. Regenerate `internal/config/schema.json` from the renamed `Step` struct.
   Whatever generator path the project uses (`make schema`,
   `mooncake schema generate`, or `internal/schemagen/`) — run it.
2. Regenerate `mooncake.d.ts`.
3. Hand-review the new schema for shape correctness — the diff will be
   large; focus on: action keys, framework keywords, `RetryPolicy` shape,
   `AsUser` single-field replacement of `become`/`become_user`.

### Phase 6 — `for_each` and `for_each_file` rework in the planner

Per Decision 6: two keywords, not one polymorphic.

1. `internal/plan/planner.go` — rename `evaluateItemsExpression`,
   `expandWithItems`, `expandWithFileTree` to `evaluateForEachExpression`,
   `expandForEach`, `expandForEachFile`.
2. `LoopContext.Type` enum values: `"with_items"` → `"for_each"`,
   `"with_filetree"` → `"for_each_file"`.
3. Update everywhere that reads `step.WithItems` / `step.WithFileTree` to
   read `step.ForEach` / `step.ForEachFile`. Compiler catches all sites.

### Phase 7 — Built-in presets rewrite

1. Build the Go-based migrator per §6.4 design at
   `scripts/migrate/v1-to-v2/main.go`. Use `yaml.v3` node API. Hard-coded
   mapping tables from §6.1 and §6.2.
2. Tests in `scripts/migrate/v1-to-v2/main_test.go`:
   - Round-trips a hand-curated v1 fixture to expected v2 form.
   - "Should-not-change" fixture: a YAML doc that uses `file` as a property
     inside `assert:` must come through unchanged.
   - Collapses `become: true` + `become_user: deploy` to `as_user: deploy`.
   - Collapses `retries: 3` + `retry_delay: 2s` to `retry: {attempts: 3,
     delay: 2s}`.
3. Dry-run on `presets/` and `examples/`; spot-check 10–20 diffs
   (shell-heavy, file-heavy, package-heavy, preset-heavy patterns).
4. Apply with `--write`. **Single commit** with message
   `chore(v2): rewrite all presets/examples to modern surface (spec-21)`
   so `git blame` recovery is one `--follow` hop.
5. Run `mooncake plan` against every file in `examples/`. All must parse
   and produce a valid plan.

### Phase 8 — Documentation rewrite

1. `docs-next/guide/config/actions.md` — rewrite the action reference table.
2. `docs-next/guide/config/reference.md` — rewrite property reference.
3. `docs-next/presets/definitive-style-guide.md` — full sweep; every YAML
   block needs updating.
4. `README.md` — update Quick Start, the action table, all snippets.
5. `LLM_GUIDE.md` — update; also fix the stale "13 actions" count (there are
   ~24 today, will be ~24 after rename).
6. `llms.txt`, `CLAUDE.md` — update.
7. `docs-next/about/changelog.md` — write the v2.0 entry with the full
   rename mapping table as a reference for users migrating manually.

### Phase 9 — Tests

1. Existing unit tests pick up the rename via the compiler. The bulk of
   work is regenerating YAML fixtures used by integration tests. Locate via
   `grep -rn '\.yml' internal/` and update.
2. Add a new test `internal/config/config_test.go::TestModernKeysParse` that
   round-trips one example YAML per action with the new keys.
3. Add a `internal/config/config_test.go::TestLegacyKeysRejected` that
   confirms an old YAML (e.g. `with_items:`, `register:`, `package:`,
   `file:`) **fails to parse** with a clear error: "unknown key 'X' — see
   migration guide". This is the v1→v2 trip-wire.

### Phase 10 — Release

1. Update `LLM_GUIDE.md` action count and `Status:` to "v2.0 modern surface".
2. Bump module version, tag `v2.0.0`, push.
3. The GoReleaser flow handles the rest (see `docs/development/releasing.md`).
4. Announcement: short README banner pointing to `changelog.md` for the
   mapping table.

---

## 9. Acceptance criteria

- `go build ./...`, `go vet ./...`, `make lint`, `make test-race` all clean.
- `mooncake plan` and `mooncake apply` (renamed from `run`/`dry-run` in the
  recent breaking-change commits) work against every file under `examples/`
  and a sample of 20 presets across the major preset categories.
- Loading any v1-era YAML (`file:`, `template:`, `package:`, `with_items:`,
  `register:`, `become:`) fails parse with a clear error pointing at the
  changelog migration table.
- JSON Schema (`internal/config/schema.json`) has zero references to old
  action keys or old framework keywords.
- `mooncake.d.ts` regenerated, hand-spot-checked for the new shape.
- `internal/mcp/tools.go` exposes tools by the modern names; an MCP client
  introspecting the server sees only modern names.
- The agent loop's system prompt (`internal/agent/prompt.go`) shows modern
  YAML in any embedded examples.
- README's Quick Start runs end-to-end on a fresh machine with `v2.0.0`.
- `docs-next/about/changelog.md` v2.0 entry contains the full mapping table.

---

## 10. Sequencing & risk notes

### Recommended landing order

```
Phase 1 (ABI rename)        ─┐
Phase 2 (Step struct)        ├─ compiles after each phase
Phase 3 (registry / names)   ─┘   → one PR, mechanical, reviewable

Phase 6 (for_each planner)    ── after Phase 2 (one small PR)

Phase 5 (schema regen)        ── after Phases 2+3+6 land (schema diff
                                  reviewed in isolation)

Phase 7 (presets / examples)  ── after Phase 5 (parser must accept new)
Phase 8 (docs)                ── parallel with Phase 7
Phase 9 (tests)               ── parallel with Phase 7

Phase 10 (release)            ── final
```

Phase 4 was removed — see Decision 7.

Phases 1–3 can land as one PR (mechanical rename, compiles). Phase 5 as a
separate PR (schema diff is large; easier to review in isolation). Phases
6–9 as a third PR (preset / doc rewrite is the bulk).

### Risks

1. **Hard-coded action names.** Audit needed: any string literal `"file"`,
   `"template"`, `"package"`, `"shell"` in non-Go places (test fixtures,
   docs, MCP descriptions, agent prompts) must be found by grep. A single
   `grep -rn '"file"\|"template"\|"package"'` across the repo will surface
   them. Budget half a day for this sweep.
2. **The preset rewrite tool's correctness.** A naive sed pass will break
   nested action contexts (e.g. `assert: { file: ... }` where `file` is a
   sub-property, not an action key). The Go-based YAML rewriter is worth
   the half-day to write properly. Throwaway code, lives under `scripts/`.
3. **`mooncake.d.ts` regeneration drift.** If the TS generator emits in a
   different order or shape after the rename, hand-review carefully — TS
   consumers of the schema (the agent SDK in `mooncake.d.ts`) may break in
   unexpected places.
4. **Loss of git blame on preset files.** A mass rewrite kills `git blame`
   utility for 1192 files. Mitigation: do the rewrite in **one** commit with
   message `chore(v2): rewrite all presets to modern surface (see spec-21)`
   so `git log --follow` plus `git blame -L` past that commit still works
   cleanly.
5. **External preset consumers.** Users with their own preset repos will be
   broken at v2.0. Acceptable per the "no backward compatibility" decision,
   but the changelog must be prominent and include the mapping table and a
   pointer to the throwaway migrator script (kept buildable in
   `scripts/migrate/` even after merge, marked unsupported).

### What this spec does NOT decide

- Whether new framework primitives (`on_change`, `try/catch/finally`,
  `transaction:`) land in v2.0 or v2.1. Lean: v2.1 (separate spec). v2.0
  ships as a pure rename so the diff is reviewable.
- Whether the agent SDK's tool schema format changes. Lean: no — keep the
  MCP tool shape stable, only the names change.
- Naming for the eventual `pkg.install` vs `pkg` debate (the Tier-1 new
  actions). That belongs in the per-action specs (22, 23, …).

---

## 11. Follow-up specs that depend on this landing

- **Spec 22** — Extended Handler ABI (`Diff`, `Reverse`, `Cost`,
  `Permissions`). Foundational for the agent-safety pitch and for §6.8
  transactional groups.
- **Spec 23** — New framework primitives: `on_change` reactive triggers,
  `try`/`catch`/`finally`, `transaction:` blocks, `!secret` references.
- **Spec 24** — Tier-1 action: `pkg.install` (proper one, with `pkg.repo`).
- **Spec 25** — Tier-1 action: `os.user` + `os.ssh_key`.
- **Spec 26** — Tier-1 action: `git.clone` + `git.checkout`.
- **Spec 27** — Tier-1 action: `text.line`, `text.patch.json`.
- **Spec 28** — Tier-1 action: `wait.port` / `wait.http` / `wait.command`.
- **Spec 29** — Tier-1 action: `os.cron`, `os.systemd`.
- **Spec 30** — Optional `mooncake translate ansible <path>` CLI tool
  (separate binary or subcommand — TBD).

Each of these targets the modern surface this spec lands. None of them
should land before Spec 21 merges, to avoid dual-surface confusion.

---

*This spec is the cutover that unblocks the rest of the
`VISION_ACTIONS.md` roadmap. Treat it as a single atomic project — partial
landings produce a worse state than either the v1 surface or the v2 surface
alone.*

---

## Appendix A — Agent handoff: first-probe commands

Before changing any code, run these to confirm the audit findings still
hold. If any returns unexpected output, **stop and ask** — the codebase
shifted since this spec was written.

```bash
# 1. Registry uses Metadata().Name (not hard-coded strings).
#    Expect to see: r.handlers[meta.Name] = handler
grep -n 'handlers\[' internal/actions/registry.go

# 2. DetermineActionType is the dispatch chain to update.
#    Expect ~24 if-blocks ending at line ~898.
sed -n '818,898p' internal/config/config.go | head -5

# 3. Step struct action fields (yaml tags) are the rename targets.
#    Expect lines 656-680 of config.go showing Template, File, Shell, etc.
sed -n '656,680p' internal/config/config.go

# 4. Hard-coded literals that the rename must touch.
grep -rEn '"(file|template|copy|download|unarchive|file_replace|file_insert|file_delete_range|file_patch_apply|package|service|command|repo_search|repo_tree|repo_apply_patchset|artifact_capture|artifact_validate|print|preset|include|include_vars)"' internal/ cmd/ | grep -v _test.go | grep -v handler.go

# 5. Each handler's Metadata().Name to update.
grep 'Name:\s*"' internal/actions/*/handler.go

# 6. Schema regen path is pinned.
grep -A2 'schema-generate:' Makefile

# 7. CLI subcommands already on plan/apply.
grep -nE 'Name:\s*"(plan|apply|run|dry-run)"' cmd/mooncake.go

# 8. The Step framework keyword fields to rename.
sed -n '680,710p' internal/config/config.go
```

## Appendix B — Phase-1-only handoff (if executing in stages)

The minimum useful first PR is Phases 1+2+3 as one mechanical rename
landing. The subagent prompt should:

1. Read this spec end-to-end first.
2. Run Appendix A probe commands; report any drift.
3. Execute Phase 1 (Handler ABI rename) — `Execute → Apply`, `DryRun → Plan`
   across the entire codebase. Verify `go build ./...` clean.
4. Execute Phase 2 (Step struct rework) — all field renames per §6.1 table,
   all framework keyword renames per §6.2. Update `DetermineActionType()`.
   Verify `go build ./...` clean.
5. Execute Phase 3 (handler `Metadata().Name` + 4 hard-coded literals).
   Verify `go build ./...`, `go vet ./...`, `make lint`, `make test-race`
   clean.
6. **Stop and report.** Do NOT run `make schema-generate` yet (Phase 5),
   do NOT rewrite presets (Phase 7). Human reviews the diff and decides
   whether to continue.

Tests will fail at this point because the YAML fixtures inside `*_test.go`
files still use v1 keys. That is expected — leave them failing; Phase 9
fixes them after the schema regenerates.


# Spec 17: Batched Package Installs + Templated `names`

**Effort:** M (1–2 days)
**Value:** High — makes the `package` action usable for real-world large package lists

---

## Problem

The `package` action has two related shortcomings that make it impractical for
large package lists (e.g. a dotfiles repo declaring ~140 packages on Arch):

1. **Per-package invocation.** When a step lists `names: [a, b, c, …]`,
   `installPackages` (`internal/actions/package/handler.go:355`) loops over each
   name, runs `<manager> -Q <pkg>` to check installed status, and then runs
   `<manager> install <pkg>` **separately** for each missing package. For ~140
   packages this is ~140 pacman invocations vs the canonical single
   `pacman -S --needed pkg1 … pkgN`. Each pacman invocation cold-loads its
   DB, so the slowdown is large. The log is also one line per package.

2. **No way to pass a templated list into `names`.** `pkg.Names` is `[]string`,
   so YAML parsing requires a literal list at parse time. Pongo2
   (`internal/template/renderer.go`) only renders `string → string`, so
   `names: "{{ pacman_packages }}"` does not expand to the list provided by
   `include_vars`. Today users either inline the list under the step (losing
   data/action separation) or use a `with_items: "{{ pacman_packages }}"` loop
   (one plan step per item — even slower).

Reference: a dotfiles repo at `/home/alehatsman/dotfiles/` just migrated from
`shell: pacman -S --needed {{ pacman_packages | join: " " }}` to
`package: { names: [...inline 140 entries...] }`. Both problems showed up
immediately.

---

## Goals

- **G1** Batch installs/removes per manager into a single subprocess call.
- **G2** Accept `name`/`names` as template expressions that resolve to a
  string, a YAML list, or a JSON-encoded list.
- **G3** Preserve current YAML schema (no breaking changes).
- **G4** Keep idempotency: do not re-invoke the manager when every package is
  already in the desired state.

Non-goals: AUR (`yay`/`paru`), `flatpak`, `snap`, `nix`. Follow-ups.

---

## Key files

| File | Role |
|---|---|
| `internal/actions/package/handler.go` | Handler — modify `Execute`, `installPackages` (l.355), `removePackages` (l.398), `buildInstallCommand` (l.497), `buildRemoveCommand` (l.535). |
| `internal/actions/package/handler_test.go` | Tests — see `TestHandler_BuildInstallCommand` (l.~530), `installPackages` test (l.1318). |
| `internal/config/config.go` | `Package` struct — `Names []string` must accept a templated string too. |
| `internal/config/schema.json` | JSON Schema — widen `names`. |
| `internal/template/renderer.go` | Pongo2 wrapper. Add a helper to parse a rendered string into a list. |
| `internal/plan/planner.go` | `evaluateItemsExpression` / `convertToSlice` already solve the same problem for `with_items` — extract into a shared resolver. |

---

## Task 1 — Batch installs (G1, G4)

In `installPackages` (`handler.go:355`), replace the per-package loop with two
phases:

1. **Check phase.** Loop calling `isPackageInstalled`, partition into
   `existingPkgs` and `toInstall`. Keep the existing reporting.
2. **Install phase.** If `len(toInstall) > 0`, build a single command with all
   packages via new helper:
   ```go
   buildBatchInstallCommand(manager string, pkgs []string, upgrade bool, extra []string) []string
   ```
   Call `runCmd` once. If empty, `result.SetChanged(false)` and return.

Manager-specific batched commands (append the full list at the end):

| Manager | Batched install |
|---|---|
| apt | `apt-get install -y <pkgs...>` |
| dnf | `dnf install -y <pkgs...>` |
| yum | `yum install -y <pkgs...>` |
| pacman | `pacman -S --noconfirm --needed <pkgs...>` (**add `--needed`**) |
| zypper | `zypper install -y <pkgs...>` |
| apk | `apk add <pkgs...>` |
| brew | `brew install <pkgs...>` |
| port | `port install <pkgs...>` |
| choco | `choco install -y <pkgs...>` |
| scoop | `scoop install <pkgs...>` |

Same change for `removePackages` (`handler.go:398`) with
`buildBatchRemoveCommand`. Pacman batched remove: `pacman -R --noconfirm <pkgs...>`.

**`extra` argument ordering:** today `extra` is between manager+verb and the
package name (`apt-get install -y --no-install-recommends vim`). In the batched
form, append `extra` **before** the package list:
`apt-get install -y --no-install-recommends vim git curl`. Keep existing apt
test (`TestHandler_BuildInstallCommand` "apt with extra arguments") passing.

**Event emission:** `EventPackageManaged` (`handler.go:389`) — one emit per
call, listing all `Installed`/`AlreadyPresent`. Same shape.

**Logging:** `ec.Logger.Infof("  Installing packages: %s", strings.Join(toInstall, ", "))`
instead of one line per package. Keep debug-level per-package check logs.

The `upgrade` parameter on `buildInstallCommand` is currently
`//nolint:unparam` (l.497). Preserve that no-op parity in the batched form.

---

## Task 2 — Templated `name` / `names` (G2, G3)

### Schema/struct change — `internal/config/config.go`

Add a custom `UnmarshalYAML` on `Package`. Accept either:

- a YAML sequence (`names: [a, b]`) → store directly into `Names []string`
- a YAML scalar string (`names: "{{ pacman_packages }}"`) → stash in a new
  `NamesExpr string` field for late evaluation

(Same treatment for `Name` is optional — current scalar template rendering
already covers single-package templating.)

### Resolution at execute time — `handler.go:Execute`

After determining the manager, if `pkg.NamesExpr != ""`:

1. Render via `ctx.GetTemplate().Render(pkg.NamesExpr, vars)`.
2. The rendered output may be:
   - YAML/JSON list literal (`"[a, b, c]"` or `'["a","b","c"]'`) — parse with
     `yaml.Unmarshal` into `[]string`.
   - Pongo2-stringified Go slice (`"[a b c]"`) — strip brackets, split on whitespace.
   - Whitespace- or comma-separated string (`"a b c"`) — split.
3. Reuse logic from `internal/plan/planner.go:evaluateItemsExpression` (already
   accepts `[]interface{}`, `[]string`, and dot-notation expressions).

**Strongly preferred:** extract a shared `internal/template/listresolver.go`
so `with_items` and `package.names` share one resolution path. Same problem,
two places.

After resolution, `packages = resolvedList` and proceed unchanged.

### Schema doc

Update `internal/config/schema.json` so `names` is:
```json
"oneOf": [
  { "type": "array", "items": { "type": "string" } },
  { "type": "string" }
]
```
Regenerate `internal/config/schema.d` (via `mooncake schema generate` or the
project's `make schema` target — check `Makefile`).

### Validation

`Validate` (`handler.go:71`) — if both `Name` and `Names`/`NamesExpr` are unset
and `Upgrade` is false, error message stays the same.

---

## Task 3 — Tests

Add to `internal/actions/package/handler_test.go`:

1. **Batched build-command tests** — extend `TestHandler_BuildInstallCommand`
   to assert `buildBatchInstallCommand("pacman", []string{"a","b","c"}, false, nil)`
   returns `["pacman", "-S", "--noconfirm", "--needed", "a", "b", "c"]`. Cover
   all managers.
2. **Batch idempotency** — fake `isPackageInstalled` so packages `["a","b","c"]`
   returns `(true, false, true)`; assert exactly one install subprocess spawned
   with args ending in `b`. (Likely requires a thin runner interface on
   `Handler` so tests can mock — see `runCmd` at l.342. If too much for one PR,
   introduce a per-test override hook.)
3. **Templated `names`** — feed `names: "{{ packages }}"` with
   `packages = []string{"a", "b"}` in variables; assert the resolver returns
   `[]string{"a", "b"}`. Repeat with:
   - `names: "a b c"` (whitespace string)
   - `names: "[a, b, c]"` (Pongo2-stringified list)
   - `names: '["a","b","c"]'` (JSON list)
4. **Schema round-trip** — load YAML with `names: "{{ x }}"` through
   `config.LoadConfig`; assert no validation error.
5. **Integration** — add `examples/actions/package_templated_names.yml` using
   `include_vars` + a `package:` step. Wire into existing examples test layout.

---

## Task 4 — Docs

1. Update `docs-next/api/config.md` and the action reference to document
   `names` accepting a templated expression. Canonical pattern:
   ```yaml
   - include_vars: ./packages.yml
   - package:
       manager: pacman
       state: present
       names: "{{ pacman_packages }}"
   ```
2. Add to `docs-next/about/changelog.md`: "package action: batched installs per
   manager; `names` now accepts a templated expression."
3. Regenerate auto-generated docs: `mooncake docs generate --section action-properties`.

---

## Acceptance criteria

- `go test ./internal/actions/package/... ./internal/config/... ./internal/plan/...`
  passes (including new tests).
- `go vet ./...` and `make lint` clean.
- Running this YAML on Arch installs all packages with **exactly one**
  `pacman -S --noconfirm --needed …` invocation:
  ```yaml
  - include_vars: ./packages.yml
  - package:
      manager: pacman
      state: present
      names: "{{ pacman_packages }}"
    become: true
  ```
- Inline literal `names:` continues to work (regression test).
- `mooncake plan` output shows the resolved list (existing TODO at
  `handler.go:116` notes plan-phase expansion is desirable — bonus if cheap).
- Dotfiles repo at `/home/alehatsman/dotfiles/` can revert `arch/packages.yml`
  to a vars file (`pacman_packages: [...]`) with a single `package` step in
  `arch/main.yml`:
  ```yaml
  - include_vars: ./packages.yml
  - package:
      names: "{{ pacman_packages }}"
      state: present
      manager: pacman
    become: true
  ```
  `./run.sh --tags packages --dry-run` resolves the list and matches the
  inline form.

---

## Sequencing / risk notes

- **Task 1 first** — mechanical, no schema change, high value.
- **Task 2 second** — touches YAML parsing and may interact with
  `with_items` resolution. Extract the shared resolver early.
- Event payload shape unchanged — only the count of emissions changes.
- Optional later optimization for `--needed`-aware managers (pacman, brew,
  apk): skip the per-package check phase and let the manager handle
  idempotency. Out of scope for this PR — keep the explicit check for uniform
  behavior across managers.

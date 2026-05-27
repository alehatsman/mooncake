# Proposal 07: Migrate `mooncake presets` references to `mooncake mod`

**Status:** Draft — ready for execution
**Effort:** S–M (~½ day of mechanical doc rewrites + 5 small code edits)
**Value:** Closes the loop on the presets-CLI retirement that landed in
`2b7eee8e` (cmd-side delete) and `4db53ad6` (orphan packages delete).
Every remaining `mooncake presets …` reference in the tree currently
points users at a command that exits with "command 'presets' not
found" — a broken first-impression for anyone reading docs, scaffold
templates, or doctor output.

---

## Context

Three commits already landed:

- **`2b7eee8e`** — deleted `cmd/presets.go` (1517 LOC) + 32 preset
  tests + the `presets` entry in `createApp`.
- **`9645bd3c`** — extracted kernel white-box tests and dropped
  test-helper duplicates.
- **`4db53ad6`** — deleted `internal/recommend/`,
  `internal/presets/registry/`, and `docs-next/presets/`.

What still exists and is *not* dead:

- `internal/presets/` — the component loader. spec-67 modules, the
  `preset` action handler, the planner, the doctor preset-paths check,
  and MCP's `list_presets` tool all still call it. Keep.
- `presets/` (the 388-component catalog in the repo root). Components
  are still loaded via `use:` from playbooks and modules.

What's broken in user-facing prose: every CLI example, scaffold hint,
doctor fix message, first-run welcome, and MCP comment that still
spells `mooncake presets …`. The closest live successors are
`mooncake mod` (fetch+register modules) and `mooncake actions list`
(browse the action vocabulary). Some references have no replacement —
those need a judgment call (drop, or rephrase around
`internal/presets`'s in-process API).

---

## Inventory of remaining references

Generated from `grep -rnE "mooncake presets" --include="*.md"
--include="*.txt" --include="*.ts" --include="*.yml" --include="*.go"`,
excluding `docs-working/` (which is in-flight thinking — readers expect
historical references there) and `.tmp/`.

### Tier 1: whole files about the retired CLI — delete

| File | LOC | Why delete |
|---|---:|---|
| `docs-next/guide/presets.md` | 758 | Entirely describes `mooncake presets install ollama` and parameter collection. Every command in it returns "command not found." |
| `docs-next/guide/preset-lifecycle.md` | 417 | Documents `presets add / install / status / uninstall / update / info / trust-key` — none of which exist. |
| `docs-next/guide/preset-authoring.md` | 849 | "How to author a preset *for the presets CLI*." Components are still authorable but for **modules** — that needs a different guide, not this one re-edited. |

**Action:** `git rm` all three. The modules surface gets its own
`docs-next/guide/modules.md` (out of scope for this proposal — track
as follow-up). `MODULES.md` at repo root already covers the
authoring/distribution model; reuse that as the seed.

### Tier 2: isolated CLI mentions inside otherwise-still-relevant docs — rewrite

| File | Lines | Current | Proposed |
|---|---|---|---|
| `docs-next/guide/faq.md` | 267 | `mooncake presets list` | `mooncake actions list` (browse the action vocabulary, which is what the surrounding FAQ entry is actually about) |
| `docs-next/guide/quick-reference.md` | 66–80 | full "Presets" section with `list/install/status/uninstall` | Replace the whole section with a "Modules" section: `mooncake mod add <url>@<v>`, `mooncake mod cache list`, `mooncake mod cache clean`. |
| `docs-next/guide/troubleshooting.md` | 441, 463 | `mooncake presets list`, `mooncake presets status docker` | These troubleshooting recipes assume the preset CLI exists; reword to `mooncake actions list` and `mooncake mod cache list` respectively, or drop the recipes if they no longer make sense in context. |
| `docs-next/development/roadmap.md` | 105–119 | uses `presets install ollama` as an example | The roadmap section is illustrative; either substitute a `mooncake apply` example with a `use: ollama-module@v1` step in the playbook, or remove the example entirely if it's stale. |
| `examples/README.md` | 68–70 | "330+ ready-made workflows" via `presets list`/`recommend` | Components still exist (used through `use:`). Reword: "Use components from the catalog under `presets/` via `use:` in a playbook; for discovery browse the directory or, in MCP-equipped clients, use the `list_presets` tool." |
| `testing-next/README.md` | 133–134 | smoke-test commands invoking `presets list` and `presets install docker` | Replace with `mooncake mod add github.com/mooncake-modules/docker@v1` (if such a module exists) or substitute different smoke commands (`mooncake facts`, `mooncake actions list`). |

### Tier 3: in-binary user-facing strings — code edits

| File | Line | Current | Proposed |
|---|---|---|---|
| `internal/doctor/checks_presets.go` | 39 | `r.Fix = "install the mooncake-presets package, or `mooncake presets update` to fetch from the remote registry"` | The check still tests preset *search paths* (loader-side), which are still valid — keep the check, change the fix string to `"populate at least one of the preset search paths — clone github.com/mooncake/presets or vendor components under ./presets/"`. |
| `internal/doctor/registry.go` | 30 | `checkTool{name: "fzf", usedBy: []string{"mooncake presets (interactive selector)"}}` | The interactive selector is gone. **Delete the fzf entry** unless some future feature needs it. If kept, set `usedBy` to a literal `[]string{}` and add a `TODO: drop or re-purpose` comment. |
| `internal/logger/first_run_subscriber.go` | 30 | `"  Browse 330+ built-in workflows with `mooncake presets list`.\n"` | Replace with `"  Browse 330+ built-in components under ./presets/ (load with `use:`).\n"` OR drop the second line entirely (the first line "Try `mooncake plan` …" is the load-bearing hint). |
| `internal/scaffold/scaffold.go` | 345 | `"  mooncake presets list  # browse 330+ built-in workflows"` | Drop the line, or replace with `"  mooncake actions list  # browse the action vocabulary"`. |

### Tier 4: scaffold template playbook hints — rewrite

These are user-facing because `mooncake init` writes them into a new
project, where a developer reads them on day 1.

| File | Lines | Notes |
|---|---|---|
| `internal/scaffold/templates/dotfiles/mooncake.yml` | 14–16 | Three `mooncake presets info zsh/neovim/tmux` echoes. Each component still exists under `presets/`. Either rephrase as "see ./presets/zsh/" etc., or drop the hint block (the YAML steps are themselves the documentation). |
| `internal/scaffold/templates/dotfiles/README.md` | 18 | Same `presets list` hint as `first_run_subscriber.go` — same fix. |
| `internal/scaffold/templates/empty/mooncake.yml` | 5 | Single comment: `# or `mooncake presets list` for ready-made workflows.` Replace with `# or browse ./presets/ for ready-made components to `use:`.` |
| `internal/scaffold/templates/server/mooncake.yml` | 20–22 | Three `mooncake presets info docker/ufw/sshd-hardening` echoes. Same fix as dotfiles template. |

### Tier 5: code comments referencing the retired CLI — light rewrite

These don't affect users; they're maintainer-facing and can be edited
opportunistically.

| File | Lines | Notes |
|---|---|---|
| `internal/mcp/discovery.go` | 23, 171–172 | Comments compare the MCP `list_presets` tool to the CLI `presets list`. The tool itself still works (calls `presets.DiscoverAllPresets`). Just reword the comparison: drop the `≈ mooncake presets list` parenthetical or substitute `≈ presets.DiscoverAllPresets()`. |

---

## Execution order suggested for the next agent

1. **Tier 1 deletions** — fastest LOC win, no semantic decisions.
   `git rm` the three `docs-next/guide/preset*.md` files. Update
   `mkdocs.yml` if it lists them in the nav.

2. **Tier 3 code edits** — small, mechanical, build-affecting. Run
   `go build ./... && go vet ./...` after each.

3. **Tier 4 scaffold templates** — affects new-project onboarding;
   verify by running `mooncake init` into a tmpdir and reading the
   output.

4. **Tier 2 docs rewrites** — needs the most judgment per line; do
   last when the patterns from earlier tiers are settled.

5. **Tier 5 comments** — optional; can land in a separate housekeeping
   commit.

## Open questions for the executing agent

- **Does a `docker` / `ollama` module exist on `github.com/mooncake-modules`?**
  Several of the proposed replacements assume so. If not, the example
  commands need to point at the literal `./presets/<name>/` directory or
  defer until the module ecosystem catches up.

- **Is there a planned `mooncake mod search` / `mooncake mod list`?**
  Several of the original CLI verbs (`presets list`, `presets search`,
  `presets info`) have no current mod counterpart. The replacement
  strings assume operators discover modules by browsing
  `github.com/mooncake-modules/*` directly. If the project plans to add
  these verbs, the doc rewrites should preview them.

- **`docs-next/development/roadmap.md`** — does the roadmap entry that
  uses `presets install ollama` still describe a feature on the
  roadmap, or is it itself stale and worth deleting wholesale?

## Out of scope

- Renaming `internal/presets/` to `internal/components/`. Mentioned in
  `MODULES.md`:238 as "named `presets` for historical reasons." Worth
  doing eventually, but it's a code-side rename with cascading import
  updates, not part of this docs-and-strings migration.
- Building `docs-next/guide/modules.md` to replace the deleted
  `presets.md`. Separate doc-authoring task.

## Verification checklist for the executing agent

- [ ] `grep -rnE "mooncake presets" --include="*.md" --include="*.txt"
  --include="*.ts" --include="*.yml" --include="*.go"` returns only
  hits under `docs-working/` (historical).
- [ ] `go build ./... && go vet ./... && go test ./... -short` passes.
- [ ] `mooncake init` into a tmpdir produces a project whose hints all
  point at live commands or static directories.
- [ ] `mooncake doctor --format text` no longer prints
  `mooncake presets update` as a fix.
- [ ] First-run hint (after `mooncake apply` on an empty config)
  doesn't suggest `mooncake presets list`.

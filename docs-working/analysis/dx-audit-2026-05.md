# DX Audit — May 2026

Working doc. Stream 4 (Developer Experience) has no specs yet
(`docs-working/README.md:36`); this audit is the input that should generate
them.

> **Thesis**: Mooncake's kernel (ring 1 in `VISION.md`) is production-quality,
> but the front door is missing. A first-time user hits a wall before they
> ever see what the engine can do. `mooncake init` is the single highest-ROI
> fix.

---

## TL;DR — the four biggest wins

| # | Fix | Why it matters | Effort |
|---|---|---|---|
| 1 | Add `mooncake init` | No project scaffold today; users hand-write YAML from copy-paste | **S** |
| 2 | Default config discovery (`mooncake.yml` / `./mooncake/main.yml`) | Every command demands `--config`; removes 100% of repetitive typing | **XS** |
| 3 | Fix README + hello-world drift (`run` → `apply`, fake `--dry-run`, broken example paths) | Quickstart copy-paste fails on line 1 | **XS** |
| 4 | Promote `plan` as the dry-run; add `--dry-run` alias on `apply` | The safety story is real but invisible | **XS** |

Everything else in this doc is downstream of those four.

---

## 1. Current state — what works

These are not problems; documenting them so we don't regress.

- **Single-binary install** via `install.sh` (POSIX sh, OS+arch detect,
  curl-or-wget fallback). Clean.
- **Typed errors** with context (`internal/executor/errors.go`, `RenderError`,
  `CommandError`, `FileOperationError`, `StepValidationError`,
  `AssertionError`). `errors.Is/As` works.
- **Config validation** has actionable messages: enum violations explain
  allowed values, mode patterns explain octal format
  (`internal/config/error_messages.go`).
- **Run history** lives at `~/.mooncake/runs.jsonl` automatically; `mooncake
  last` surfaces the most recent run without setup.
- **`plan` command** is a real dry-run with `--diff` and `--show-origins`. It
  just isn't sold as the dry-run.
- **Help text on individual commands is fine** once you find them
  (`mooncake plan --help` is a model — aliases shown, defaults shown, layering
  semantics for `--vars` explained).
- **MCP server + agent loop ship today**. The "AI substrate" pitch is real
  code, not slideware.

---

## 2. Friction points (ranked by user impact)

### 2.1 No `mooncake init` — the canonical first command is missing

Vision document explicitly calls for it twice (`VISION.md:101`, `VISION.md:323`)
but it isn't built. Every comparable tool ships one:

| Tool | First command |
|---|---|
| `terraform init` | scaffolds `.terraform/`, downloads providers |
| `npm init` | interactive questionnaire → `package.json` |
| `cargo new` | scaffolds `Cargo.toml`, `src/`, `.gitignore` |
| `git init` | creates `.git/`, prints next step |
| **`mooncake init`** | **does not exist** |

Today, a fresh install requires the user to **hand-write YAML** before
mooncake does anything useful. This is the single biggest funnel leak.

See §4 for a proposed design.

### 2.2 README/CLI drift will fail every quickstart copy-paste

The README's Quick Start block (`README.md:53-56`):

```bash
mooncake run --config config.yml --dry-run    # ← BROKEN
mooncake run --config config.yml              # ← deprecated alias
```

Reality:

- `run` is an undocumented deprecated alias of `apply` (`cmd/mooncake.go:66`,
  comment only — flag set is shared but `run` is not registered as a CLI
  command in `createApp`).
- **There is no `--dry-run` flag anywhere** — the dry-run is the separate
  `plan` subcommand. The README's comparison table even claims "Dry-run:
  Native" while the documented command to invoke it doesn't exist.
- `examples/01-hello-world/` (`README.md:124`) — directory is actually
  `examples/hello-world/`.
- `examples/hello-world/README.md:14` itself uses `mooncake run --config
  config.yml` and at line 68 points to a non-existent
  `02-variables-and-facts/` directory.

A user who follows the README literally fails before mooncake's engine runs.

### 2.3 `--config` is required but not enforced at flag level on `apply`

`apply` declares `--config` as optional (`cmd/mooncake.go:71-75`) because it
*can* be omitted when `--from-plan` is supplied. But when neither is given,
the executor fails with `"config file path is empty"`
(`internal/executor/executor.go:678` per the explore audit — generic, no
remediation hint).

Two clean fixes:

- Make `--config` required *unless* `--from-plan` is set (CLI-level mutual
  exclusion).
- **Better:** default `--config` to `./mooncake.yml` (or
  `./.mooncake/main.yml`) if neither flag is provided. Then `mooncake apply`
  with no args Just Works in a scaffolded project.

### 2.4 No convention-over-configuration for the project file

There is no canonical filename and no auto-discovery. Every `apply` / `plan` /
`validate` invocation needs `-c <path>`. Compare:

- `terraform apply` → finds `*.tf` in `.`
- `docker compose up` → finds `compose.yaml` in `.`
- `make build` → finds `Makefile` in `.`
- `mooncake apply` → fails

Pick **one** default filename and document it (proposal: `mooncake.yml`, with
`./mooncake/main.yml` as the multi-file convention). Search order:
`./mooncake.yml` → `./mooncake/main.yml` → error with a helpful message that
suggests `mooncake init`.

### 2.5 `plan` is the dry-run but isn't marketed as one

`mooncake plan` does exactly what `terraform plan` does — inspects current
state, shows `↑ ✓ - ?` symbols (`cmd/mooncake.go:496-507`), optionally shows
unified diffs with `--diff`. It's good.

But nothing in the README, hello-world, or `apply --help` ever uses the word
"dry-run" near it. Fix:

- Add `--dry-run` as an alias on `apply` that delegates to the plan path.
- Update README comparison table cell to "Dry-run: `mooncake plan` or
  `mooncake apply --dry-run`".
- Add a single sentence to `apply --help`: *"To preview without changes, see
  `mooncake plan`."*

### 2.6 Examples directory is overwhelming and unindexed

30 entries, no order, no roadmap. Mixed conventions:
- Some are folders with READMEs (`hello-world/`, `loops/`)
- Some are bare `.yml` files at root (`wait-example.yml`,
  `repo-apply-patchset-example.yml`)
- Numeric prefixes (`01-…`) referenced in docs don't match the actual names

Minimum fix: add `examples/README.md` with a *short* curated path:

```
1. hello-world/         — shell + facts
2. variables-and-facts/ — vars, facts, templates
3. conditionals/        — when:
4. loops/               — with_items, with_filetree
5. real-world/          — dotfiles, dev box
```

Move loose `*-example.yml` files into folders or a `recipes/` subdir.

### 2.7 Preset discoverability — 330 in haystack

`mooncake presets` (no args) drops into an `fzf` selector. If `fzf` isn't
installed, the fallback dumps a list with no categorisation. `presets search`
requires a prior `presets update` to populate the remote registry cache —
nothing tells the user that on first run.

Lighter touches:

- On first `presets` invocation without `fzf`, suggest installing it OR offer
  a short hand-curated "top 20" by category (dev-tools, languages,
  containers, services).
- `presets search` with empty cache should auto-run `presets update` once or
  print a clear "run `mooncake presets update` first".
- Add `mooncake presets recommend` — filter by detected facts (`os`,
  `package_manager`, `apt_available`) to suggest 5–10 likely presets.

### 2.8 `mooncake doctor` is missing

Vision §9 (`VISION.md:326`) calls for it. Today there's no command that
sanity-checks the install, prints which presets are available where, whether
`fzf` is on PATH, whether the agentd socket exists, whether
`~/.mooncake/runs.jsonl` is writable, etc. A health check is the standard
"my tool isn't working, what now" escape hatch.

### 2.9 No `mooncake.yml` next-steps when the run succeeds

After a successful `apply`, mooncake prints the RECAP line and exits. A
first-run user has no idea what to do next: re-run? edit? commit? share? A
single trailing tip ("Run `mooncake plan` to preview changes before each
`apply`. See `mooncake presets list` for built-in workflows.") would carry a
new user across the cold-start gap.

### 2.10 No telemetry-free analytics for *which* docs work

Out of scope for this audit, but worth flagging: the project ships with no
way to know if §2.2's broken example is actually trapping users. Consider
adding `mooncake feedback` (opt-in, one-shot) before scaling marketing.

---

## 3. Recommendations — ordered by ROI

| # | Action | Files touched | Risk |
|---|---|---|---|
| R1 | Ship `mooncake init` (§4) | new `cmd/init.go`, embedded templates | low |
| R2 | Default `--config` to `./mooncake.yml`, then `./mooncake/main.yml` | `cmd/mooncake.go` (~10 lines) | low |
| R3 | Fix `README.md` quickstart + comparison table | `README.md`, `examples/hello-world/README.md` | none |
| R4 | Add `--dry-run` flag on `apply` that delegates to plan path | `cmd/mooncake.go` (~20 lines) | low |
| R5 | Make `apply` reject missing config with a helpful error that mentions `mooncake init` | `cmd/mooncake.go` (~5 lines) | none |
| R6 | Add `examples/README.md` with curated learning path | docs only | none |
| R7 | Promote/rename `last` to `mooncake history` and add `mooncake history show <run-id>` | `cmd/last.go` | low |
| R8 | Add `mooncake doctor` | new `cmd/doctor.go` | medium |
| R9 | Add `mooncake presets recommend` | `cmd/presets.go` | medium |
| R10 | Add trailing next-step tip after first successful `apply` (one-time, gated on `~/.mooncake/.first-run`) | small | none |

R1–R5 are a single cohesive PR and the entire wedge that closes the first-run
gap. They should ship together.

---

## 4. Spotlight — `mooncake init` design

The smallest valuable `init` does five things:

```
$ mooncake init
What are you setting up?
  1) Dotfiles / dev box        (recommended for solo devs)
  2) Server (Linux service host)
  3) Empty playbook
  4) Agent sandbox (no shell, only typed actions)
> 1

Created:
  ./mooncake.yml         — main playbook
  ./.mooncake/           — local state (gitignored)
  ./README.md            — quickstart for this project
  ./.gitignore           — sensible defaults
  Vars file: ./mooncake.vars.yml (optional, empty)

Detected facts:
  os: linux  distribution: arch  package_manager: pacman  apt_available: false

Suggested presets:
  - zsh           (shell + plugins)
  - neovim
  - git
  - tmux

Next:
  mooncake plan          # preview what will run
  mooncake apply         # run it
  mooncake presets list  # browse 330+ workflows
```

### 4.1 Required behavior

1. **Refuse to overwrite** an existing `mooncake.yml` without `--force`.
2. **Detect facts** (`internal/facts/`) and embed sensible defaults in the
   generated playbook (correct package manager, right OS gates).
3. **Generate a runnable file**, not a placeholder. After `init`, `mooncake
   plan` must produce a non-empty plan that does something safe (e.g. ensures
   `~/.config/` exists, prints a hello). Empty scaffolds bounce people back
   to the docs.
4. **Init a git-friendly layout** — `.gitignore` excluding `.mooncake/` and
   plan artifacts.
5. **Be non-interactive** under `--non-interactive` with `--template
   <name>` (the four options above as template names: `dotfiles`,
   `server`, `empty`, `agent-sandbox`).

### 4.2 Templates live in `presets/`

Don't invent a separate template system. The four `init` templates *are*
presets — they just have a flag that wires the loader to copy files into the
working directory rather than execute them.

This means:
- Community can add templates via the existing preset mechanism.
- `mooncake init --template foo/bar` can pull from the registry once the
  marketplace exists (`VISION.md:5.1`).
- One thing to maintain, not two.

### 4.3 Out of scope for v1

- Cloning remote dotfile repos (Vision §9 "Borrow this")
- Multi-machine sync setup
- Linking into a hub / agentd

These belong to follow-on specs. Ship the local-scaffold story first.

---

## 5. Suggested phasing → specs

Convert this audit into Stream 4 specs:

- **spec-dx-1: project scaffolding (`mooncake init`)** — R1, R3, R6.
- **spec-dx-2: convention-over-config defaults** — R2, R4, R5, R10.
- **spec-dx-3: `mooncake doctor`** — R8.
- **spec-dx-4: preset recommendation + onboarding** — R7, R9.

Land 1+2 together; they're the user-visible "Mooncake feels like a real tool
now" moment. 3 and 4 are independent and can ship later.

---

## 6. Open questions

1. **Filename**: `mooncake.yml` (terraform/cargo style — single file) or
   `./mooncake/` (directory — supports multi-file natively from day 1)? Lean
   single-file for the init scaffold; the directory form is what users grow
   into.
2. **Vars file naming**: `mooncake.vars.yml` vs `vars.yml` vs
   `.mooncake/vars.yml`? Whichever we pick, document it as the *only* default
   and don't auto-load others — surprise auto-loading bites you later.
3. **Should `init` add a git remote / commit?** No. The user owns git.
4. **Should `init` print the generated `mooncake.yml`?** Yes — show, don't
   hide. Lets the user immediately understand and edit.
5. **Does `init` enable agentd / MCP?** No. Default is the simple CLI loop;
   AI features are opt-in via separate `mooncake mcp` / `mooncake agentd`
   subcommands that already exist.

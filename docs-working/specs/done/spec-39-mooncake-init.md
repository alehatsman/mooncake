# Spec 39: `mooncake init` — Project Scaffolding

**Status:** Draft
**Epic:** Stream 4 — Developer Experience (first spec in stream)
**Effort:** S (1–2 days)
**Value:** High — closes the largest single gap in the first-run funnel.
A fresh install today requires hand-writing YAML before the engine does
anything useful. Every comparable tool (`terraform init`, `npm init`,
`cargo new`, `git init`) ships this command.

**Background:** `docs-working/analysis/dx-audit-2026-05.md` (§2.1, §4).
**Vision pointer:** `VISION.md:101`, `VISION.md:323`.

---

## Problem

A user who just ran `curl … | sh` from the README has nowhere to go:

1. No project file exists in the cwd.
2. No command scaffolds one.
3. `mooncake apply` fails with `"config file path is empty"`
   (`internal/executor/executor.go`).
4. The README's quickstart is broken (`mooncake run --config X --dry-run`
   references a deprecated command and a non-existent flag), so even
   copy-paste fails.
5. The 30-entry `examples/` directory has no index, and 330+ presets
   are invisible until the user already knows what they want.

The result: a tool whose kernel is production-quality reads as
"undocumented research code" on first contact.

---

## Goals

- **G1** Ship `mooncake init` that scaffolds a runnable project in the
  current working directory in under a second.
- **G2** Cover the four most common starting points as templates:
  `dotfiles`, `server`, `empty`, `agent-sandbox`.
- **G3** Use detected facts (`internal/facts/`) so the generated
  playbook picks the right package manager and OS gates out of the
  box — no manual edits needed before `mooncake plan` works.
- **G4** Generate a project that runs cleanly on first try: `mooncake
  plan` after `mooncake init` must produce a non-empty plan with no
  errors and no destructive steps.
- **G5** Be non-interactive under `--non-interactive` with `--template`
  for CI / agent use.
- **G6** Refuse to overwrite existing files without `--force`. Never
  silently clobber user work.

**Non-goals (deferred to follow-on specs):**

- Cloning remote dotfile repos (Vision §9 "Borrow this").
- Multi-machine sync setup.
- Linking into agentd or a hub.
- A template registry / marketplace integration. Templates are
  embedded in the binary for v1.

---

## User experience

### Default (interactive) run

```
$ mooncake init
What are you setting up?
  1) Dotfiles / dev box        (recommended for solo devs)
  2) Server                    (Linux service host)
  3) Empty playbook            (start from scratch)
  4) Agent sandbox             (no shell — only typed actions)
> 1

Detected:
  os: linux  distribution: arch  package_manager: pacman  apt_available: false

Created:
  mooncake.yml         — main playbook
  mooncake.vars.yml    — variables (empty, optional)
  .mooncake/           — local state (gitignored)
  .gitignore           — sensible defaults

Next:
  mooncake plan          # preview what will run
  mooncake apply         # run it
  mooncake presets list  # browse 330+ built-in workflows
```

### Non-interactive

```
$ mooncake init --template dotfiles --non-interactive
Created mooncake.yml, mooncake.vars.yml, .mooncake/, .gitignore
```

### Re-run safety

```
$ mooncake init
mooncake.yml already exists. Use --force to overwrite or `mooncake init
--template empty --dir ./new-project` to scaffold elsewhere.
```

### Flags

| Flag | Purpose |
|---|---|
| `--template <name>` | One of `dotfiles`, `server`, `empty`, `agent-sandbox`. Required with `--non-interactive`. |
| `--non-interactive` | Skip prompts; fail if required input missing. |
| `--force` | Overwrite existing `mooncake.yml` / `.gitignore`. |
| `--dir <path>` | Scaffold into a directory other than cwd. Creates if missing. |
| `--no-vars` | Skip `mooncake.vars.yml` for templates that don't need it. |
| `--list-templates` | Print the four template names + one-line descriptions, exit 0. |

---

## Templates

Templates are embedded in the binary as `go:embed` directories. Each is
a `presets/`-style folder shipped under `internal/init/templates/`.

```
internal/init/templates/
├── dotfiles/
│   ├── mooncake.yml
│   ├── mooncake.vars.yml
│   ├── gitignore        (renamed to .gitignore on write)
│   └── README.md        (one-pager for this scaffolded project)
├── server/
│   └── …
├── empty/
│   └── …
└── agent-sandbox/
    └── …
```

Each template file is rendered through the existing template engine
(`internal/template/`) with detected facts as variables. This lets a
single source file generate the right output for any OS without
maintaining per-OS copies.

### Template: `dotfiles` (the default suggestion)

Runs three safe, idempotent steps so `plan` shows green / would-change
appropriately:

```yaml
# mooncake.yml
- name: Ensure ~/.config exists
  file:
    path: "{{ home }}/.config"
    state: directory

- name: Print system summary
  shell: echo "mooncake managing {{ os }}/{{ arch }} on {{ distribution }}"

- name: Suggest a starter preset
  shell: |
    echo "Try: mooncake presets info zsh"
    echo "Or:  mooncake presets info neovim"
```

### Template: `server`

Same shape, but the suggested presets are server-flavored
(`docker`, `ufw`, `sshd-hardening`) and `become:` is wired up so the
user immediately sees the sudo password flow.

### Template: `empty`

The lowest-noise template: a single `print` step with a comment block
linking to `mooncake presets list` and the docs. Used by agents that
will overwrite the file anyway.

### Template: `agent-sandbox`

Sets `meta:` flags that disable the `shell` action (per Stream 2
direction), and the file shows only typed-action examples (`file:`,
`template:`, `pkg.install:`). Forward-compatible with the safe-agent
runtime when it lands.

---

## Generated files

### `mooncake.yml`

Always rendered from the chosen template. Top of file is a comment
block:

```yaml
# Generated by `mooncake init --template dotfiles`
# Edit freely. Preview changes with `mooncake plan`. Apply with `mooncake apply`.
```

### `mooncake.vars.yml`

Empty (`{}`) or a single example variable, commented out:

```yaml
# Variables available to mooncake.yml as {{ name }}.
# Example:
#   editor: nvim
```

Skipped when `--no-vars` is passed.

### `.gitignore`

```
# Mooncake local state
.mooncake/

# Plan artifacts
*.plan.json
*.plan.yaml
```

If `.gitignore` already exists, mooncake **appends** the missing lines
under a `# Mooncake` header rather than overwriting. Idempotent — if the
section already exists, no change.

### `.mooncake/`

Created empty. Holds run artifacts, lockfiles, and (eventually)
local-only state. Already used by `internal/lockfile/` and
`internal/artifacts/`; this just makes its existence explicit in the
project tree.

### README.md

Optional. Created only by the `dotfiles` template (skipped by others
to avoid clobbering existing READMEs in the cwd). A 20-line quickstart
specific to the chosen template.

---

## Implementation

### Package layout

```
cmd/init.go                    # new — CLI wiring
internal/init/                 # new package
├── init.go                    # Scaffold(opts) entry point
├── templates.go               # template loading, embed.FS
├── templates/                 # embedded template directories
│   ├── dotfiles/
│   ├── server/
│   ├── empty/
│   └── agent-sandbox/
└── prompt.go                  # interactive prompts (only used when -i)
```

### `cmd/init.go`

```go
func initCommand() *cli.Command {
    return &cli.Command{
        Name:  "init",
        Usage: "Scaffold a new mooncake project in the current directory",
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "template", Aliases: []string{"t"}},
            &cli.BoolFlag{Name: "non-interactive", Aliases: []string{"n"}},
            &cli.BoolFlag{Name: "force", Aliases: []string{"f"}},
            &cli.StringFlag{Name: "dir", Value: "."},
            &cli.BoolFlag{Name: "no-vars"},
            &cli.BoolFlag{Name: "list-templates"},
        },
        Action: func(ctx *cli.Context) error {
            if ctx.Bool("list-templates") {
                return initpkg.ListTemplates(os.Stdout)
            }
            return initpkg.Scaffold(initpkg.Options{
                Template:        ctx.String("template"),
                NonInteractive:  ctx.Bool("non-interactive"),
                Force:           ctx.Bool("force"),
                Dir:             ctx.String("dir"),
                NoVars:          ctx.Bool("no-vars"),
                Stdin:           os.Stdin,
                Stdout:          os.Stdout,
                Stderr:          os.Stderr,
            })
        },
    }
}
```

Register in `cmd/mooncake.go` `createApp().Commands` list.

### `internal/init/init.go`

```go
package init

import (
    "embed"
    "github.com/alehatsman/mooncake/internal/facts"
    "github.com/alehatsman/mooncake/internal/template"
)

//go:embed templates/*
var templatesFS embed.FS

type Options struct {
    Template       string
    NonInteractive bool
    Force          bool
    Dir            string
    NoVars         bool
    Stdin          io.Reader
    Stdout         io.Writer
    Stderr         io.Writer
}

func Scaffold(opts Options) error {
    // 1. Resolve template (interactive prompt if blank and !NonInteractive).
    // 2. Reject if Template not in {dotfiles, server, empty, agent-sandbox}.
    // 3. Collect facts via facts.Collect().
    // 4. For each file in templates/<name>/:
    //      - Skip if NoVars && file == "mooncake.vars.yml"
    //      - Render via template engine with facts as vars.
    //      - Refuse write if exists && !Force (except .gitignore — append).
    //      - Write atomically (tmp + rename).
    // 5. Create .mooncake/ directory.
    // 6. Print "Created:" summary and next steps.
}
```

Rendering uses the existing `internal/template/` engine — same Jinja2-
like syntax users will see in their playbooks. This is deliberate:
templates are the user's first encounter with mooncake's templating,
so they should be in the same language.

### Idempotency / refuse-to-clobber

For each target file:

1. `os.Stat` first.
2. If exists and `--force` not set:
   - For `mooncake.yml` / `mooncake.vars.yml` / `README.md`: abort with
     a helpful error naming the file.
   - For `.gitignore`: append-or-noop (read existing, check for
     `# Mooncake` section, append if missing).
   - For `.mooncake/`: noop (just `os.MkdirAll`).
3. If `--force` set: overwrite atomically (tmp + rename) for the
   `mooncake.yml` family; never wholesale-replace `.gitignore`.

### Interactive prompt

Single survey: "What are you setting up?" with the four numbered
options. Implementation can be a 20-line read-from-stdin loop — no
external prompt library needed.

If stdin isn't a TTY and `--non-interactive` wasn't passed, abort with:
*"`mooncake init` needs a TTY for interactive use. Run with
`--non-interactive --template <name>` instead."*

---

## Acceptance criteria

1. `mooncake init --list-templates` prints exactly four templates with
   one-line descriptions and exits 0.
2. `mooncake init --template empty --non-interactive --dir /tmp/foo`
   creates `/tmp/foo/{mooncake.yml,mooncake.vars.yml,.gitignore,.mooncake/}`.
3. After (2), `mooncake plan -c /tmp/foo/mooncake.yml` exits 0 and
   prints a non-empty plan with zero errors.
4. After (2), `mooncake apply -c /tmp/foo/mooncake.yml` exits 0,
   makes no destructive changes, and a second `apply` reports
   `changed=0` (idempotent).
5. `mooncake init` in a directory that already contains `mooncake.yml`
   refuses with a clear message naming the file and the `--force` /
   `--dir` escape hatches.
6. `mooncake init --force` in (5) overwrites `mooncake.yml`
   atomically (no partial-write window).
7. `mooncake init` in a directory with an existing `.gitignore`
   appends the mooncake section without removing existing lines; a
   second `init` is a no-op on `.gitignore`.
8. Generated `mooncake.yml` references the detected `package_manager`
   on Linux (Arch → pacman, Debian → apt) and `homebrew` on macOS.
9. `mooncake init --template dotfiles --non-interactive` works inside
   a non-TTY shell.
10. `mooncake init` without `--non-interactive` and without a TTY
    fails fast with a clear remediation message.
11. All templates pass `mooncake validate` cleanly post-scaffold.
12. Total wall time for `mooncake init --template empty
    --non-interactive` is under 500ms on a warm cache.

---

## Open questions

1. **Filename**: `mooncake.yml` (single-file, terraform/cargo style)
   vs `mooncake/main.yml` (directory). Spec proposes single-file for
   the scaffold; users grow into the directory form. Default-config
   discovery (spec-dx-2) must accept both.
2. **Vars file naming**: `mooncake.vars.yml` reads well next to
   `mooncake.yml`. Alternatives considered and rejected: `vars.yml`
   (too generic in a multi-tool repo), `.mooncake/vars.yml` (hidden,
   surprising). Confirm before implementation.
3. **Should `dotfiles` template create a `./dotfiles/` directory?**
   Lean no for v1 — keep the scaffold flat. Users add it when they
   have actual dotfiles to manage. Reconsider if user feedback flags
   it.
4. **Should `agent-sandbox` template auto-enable agentd or MCP?**
   No. Both stay opt-in via their existing subcommands. The template
   only models the *content* shape, not the runtime.
5. **Where do templates live long-term?** Embedded in the binary for
   v1. Once the preset marketplace ships (Vision §5.1), templates
   become a preset category and `mooncake init --template
   user/repo` works. Document this trajectory in a code comment so
   the embed isn't load-bearing forever.

---

## Dependencies / sequencing

- **None upstream.** This spec can land standalone.
- **Pairs with spec-dx-2** (default config discovery). Land together
  for the cohesive "Mooncake feels like a real tool" moment: `init`
  scaffolds `mooncake.yml`, then `mooncake plan` and `mooncake apply`
  Just Work without `-c`.
- **Unblocks** spec-dx-3 (`mooncake doctor`) and spec-dx-4 (preset
  recommendation) — both want a scaffolded project to operate on.

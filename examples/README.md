# Mooncake Examples

A short, ordered path from "hello world" to real workflows. Read the
top entries first; everything below the divider is reference material
to browse when you hit a relevant problem.

## Learning path

1. **[hello-world/](hello-world/)** — shell commands + global facts
   (`{{ os }}`, `{{ arch }}`).
2. **[variables-and-facts/](variables-and-facts/)** — custom variables
   and the full system-facts reference.
3. **[conditionals/](conditionals/)** — `when:` expressions and OS
   gating with facts like `apt_available`.
4. **[files-and-directories/](files-and-directories/)** — `file.write`,
   directory creation, mode handling.
5. **[loops/](loops/)** — `with_items`, `with_filetree`, register +
   loop patterns.
6. **[templates/](templates/)** — Jinja2-style rendering with
   `file.template`.
7. **[real-world/](real-world/)** — dotfiles, dev-box bootstrap, and
   other end-to-end scenarios.

## Quick starts

```bash
# Run the simplest example
mooncake apply --config hello-world/config.yml

# Preview without running
mooncake apply --config hello-world/config.yml --dry-run

# Scaffold your own project (uses these examples' patterns)
mooncake init --template dotfiles
```

---

## Reference: browse when relevant

**Topic-specific folders**
- `containers/` — container actions
- `execution-control/` — tags, register, retries
- `macos-services/` — launchd integration
- `ollama/` — Ollama installation/management
- `register/` — capturing step output for later steps
- `sudo/` — `become:` and password handling
- `tags/` — filtering steps with `--tags`
- `advanced-file-operations/` — `text.*` and `file.*` actions
- `multi-file-configs/` — splitting playbooks across files
- `scenarios/` — end-to-end realistic configs

**Single-file recipes**
- `artifact-capture-example.yml`, `artifact-plan-embedding-example.yml`,
  `ARTIFACTS_README.md` — `artifact.capture` action
- `assert-enhanced-example.yml` — `assert:` action
- `file-delete-range-example.yml`, `file-insert-example.yml`,
  `file-patch-apply-example.yml`, `file-replace-example.yml` —
  `text.*` actions on existing files
- `global-variables-example.yml` — global variable scoping
- `json-output-example.md` — structured JSON CLI output
- `llm-agent-workflow.yml` — agent-loop pattern
- `repo-apply-patchset-example.yml` — `repo.patch` workflow
- `wait-example.yml` — `wait.*` actions

## See also

- Browse `./presets/` — 330+ ready-made components (zsh, neovim,
  docker, …) you include in a playbook with `use:`
- `mooncake actions list` — full typed-action surface
- `mooncake mod add github.com/mooncake-modules/<name>@<v>` —
  fetch a remote module into your project
- `mooncake init --list-templates` — starter scaffolds
- [Full documentation](https://mooncake.alehatsman.com)

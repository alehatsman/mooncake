# Stream: dx

Developer experience. The outer CLI surface that turns "kernel + YAML"
into something a solo developer adopts on Friday evening.

If it's the first thing a new user sees, or the first thing they reach
for when something is wrong, it lives here.

## Scope

| In | Out |
|---|---|
| `mooncake init` (project scaffolding, template selection) | Action handlers (see [core](../core/README.md)) |
| `mooncake doctor` (~16 health checks across install/system/state/presets/tools/project/services) | Multi-peer fan-out of doctor (see [fleet](../fleet/README.md) spec-55) |
| `mooncake history` + run audit consumption | Agent-specific UX (see [agent](../agent/README.md)) |
| `mooncake presets recommend` + preset registry | |
| Default config discovery (`./mooncake.yml`, `./mooncake/main.yml`) | |
| `--dry-run` alias on `apply` | |
| Structured error messages + suggested fixes | |
| First-run tips and the friction-reduction layer in general | |

## State

**Shipped.** The gap from "kernel-only, hand-write YAML" to "Mooncake
feels like a real tool" is closed.

Recent shipped specs (see commit history for the full receipts):

- spec-39 — `mooncake init` with template selection
- spec-40 — default config discovery + `--dry-run` alias
- spec-41 — `mooncake doctor` (16 health checks)
- spec-42 — examples index + `history` + `presets recommend`
- agent-dx — `AGENT.md` cheat sheet + Makefile targets for sub-second
  per-package feedback (`build-pkg`, `test-pkg`, `test-fn`, `lint-pkg`,
  `check-pkg`), gopls structural lookups (`sym`, `doc`, `refs`,
  `callers`, `impl`), and `budget-status` for soft-cap monitoring.

## Active specs

None.

## Open gaps

These came out of the DX audit and live as informal follow-ups, not
specs:

- **`mooncake share <preset>` / marketplace.** The "borrow this
  preset" loop is Stream-ecosystem territory; no spec drafted.
- **"Import existing dotfiles" command.** Would lower the migration
  cost for users with existing Ansible / Chef / shell-script dotfiles.
  No spec drafted.
- **macOS preset coverage smaller than Linux.** Bridge-stream issue
  that surfaces here because new macOS users notice it first.
- **First-run tip / "what now?" affordances** after `mooncake apply`
  completes — DX-audit items R7–R10 are partly done.

If any of these become real user asks, draft a spec under
`streams/dx/specs/`. Until then, they live in `git log` and as items
that surface on real use.

## Cross-stream dependencies

- [core](../core/README.md) — DX exposes the same plan/apply
  primitives Core ships. New error categories from Core get DX
  treatment (suggested fixes).
- [fleet](../fleet/README.md) — `mooncake fleet init` mirrors
  `mooncake init`'s pattern; sharing the onboarding shape is
  intentional.
- [agent](../agent/README.md) — DX cares about the human user; Agent
  cares about the LLM caller. Both consume Core. The two surfaces stay
  separate by design (LLM-shaped output ≠ human-shaped output).

# Mooncake — Action Design Principles

> Extracted from the retired `VISION_ACTIONS.md`. These are the standing rules
> for any new action — Tier-1 or otherwise. Check all three sections before
> writing code.

---

## 1. Design principles (read before adding *any* action)

Non-negotiable for Tier-1 actions. Plugins can relax these but must declare
which they're relaxing.

1. **Modern naming.** Dot-namespaced (`pkg.install`, `text.line`, `git.clone`).
   Domain-first. No legacy compat names — AI can translate from Ansible/Chef
   at author time, or a separate translator tool can do it later.
2. **Idempotent or explicitly not.** Every action declares
   `idempotent: true | false | conditional` in metadata. Default `true`.
   `shell` is `false` unless `creates`/`unless` constrains it.
3. **Plannable.** Every action returns a structured plan/diff with zero side
   effects when invoked in plan mode. No "we'll just run it and tell you what
   happened" hacks.
4. **Snapshot-aware.** Every action declares the resource it touches (path,
   service name, k8s object, DNS record) so the snapshot subsystem records
   before/after.
5. **Reversible by default, irreversible by exception.** Every action either
   (a) produces a `Reverse` action automatically, or (b) declares
   `reversible: false` with a reason. This is what makes "agent did something
   dumb, undo it" actually work.
6. **Typed with JSON Schema.** No untyped maps. Schemas are what the MCP
   server and agent SDK expose as tool definitions — **the schema is part of
   the product, not implementation detail**.
7. **Single responsibility.** `file.write` writes files, it does not also
   template. `file.template` renders, it does not also fetch. Composition
   over conflation.
8. **Secure by default.** Anything taking a password / token / key redacts in
   logs, events, and the run log. No exceptions.
9. **Cross-platform unless meaningfully OS-specific.** If it can be unified
   (`pkg.install` across apt/dnf/brew), it must be. If not
   (`os.launchd_plist`), the namespace marks it OS-specific.
10. **Composable through `outputs`.** Every action publishes typed outputs
    for downstream steps. No magic stdout coupling.
11. **Stable error taxonomy.** Each action's failure modes map to typed
    errors so policy / retry / agent code can branch on them.

---

## 2. Ansible migration stance

Not a feature in the core. Handled out-of-band:

- **AI translation at author time.** Drop an Ansible playbook into Claude /
  Cursor / Codex, ask for the Mooncake equivalent. LLMs handle YAML-to-YAML
  rewrites extremely well, and the dot-namespaced names are *more* learnable
  than Ansible's flat namespace anyway.
- **Optional separate translator CLI later.** If demand warrants, ship
  `mooncake translate ansible playbook.yml` as its own binary. Keeps the
  core surface untouched.
- **No compat aliases in the core.** Once you accept one (`package` →
  `pkg.install`), you accept all of them, forever. Don't start.

---

## 3. Decisions to lock in before writing any new action

- **Extended ABI** (`Diff` / `Reverse` / `Cost` / `Permissions`): are these
  mandatory or optional methods on the handler interface? Lock before the first
  new action ships. See `specs/action-surface/spec-22-extended-handler-abi.md`.
- **Plugin model**: defer the WASM decision by shipping in-tree Go for the
  first year; do *not* paint into a corner that prevents WASM later.
  See `specs/ecosystem/spec-31-tier2-plugin-model.md`.
- **`on_change` framework primitive**: land *before* shipping any
  `os.service`-adjacent actions; retrofitting later is ugly.
  See `specs/safe-agent-runtime/spec-23-framework-primitives.md`.
- **Outputs schema**: every action's outputs must be in its JSON schema before
  the action ships. No "we'll document it later".
- **No legacy aliases. Ever.** Closed for debate per design directive.

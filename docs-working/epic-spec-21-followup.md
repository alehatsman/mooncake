# Epic: Post Spec-21 — Modern Action Surface Buildout

> Roadmap for the iterations that follow spec-21's v2 cutover. Working doc —
> iterate here before formalizing individual specs.

**North star (from [VISION_ACTIONS.md](../VISION_ACTIONS.md)):** Mooncake's
action surface IS the product. Spec-21 locked in the modern surface
(dot-namespaced action keys, modern framework keywords, `ForEachField`,
`AsUser`, `Retry`). Everything after is buildout on top of that surface.

**Constraint:** stay tight. ~30 Tier-1 actions total — not 200. Each action
must satisfy the 11 design principles in `VISION_ACTIONS.md` §3
(idempotent, plannable, snapshot-aware, reversible, typed, secure-by-default,
…).

---

## Where we are post-spec-21

Tier-1 actions shipped today (24, all dot-namespaced):

```
shell  cmd  assert  wait  vars  vars.load  log  use  import
file.write  file.template  file.copy  file.download  file.unarchive
text.replace  text.insert  text.delete_range  text.patch
pkg  os.service
repo.search  repo.tree  repo.patch
artifact.capture  artifact.validate
container  container.image
tool   (spec-19)
```

Holes from `VISION_ACTIONS.md` §2 still wide open:
- No `pkg.install` / `pkg.remove` / `pkg.repo` (the *batched* `pkg` exists,
  but the imperative surface for adding repos isn't there)
- No `os.user` / `os.group` / `os.ssh_key` / `os.cron` / `os.systemd` /
  `os.firewall` / `os.mount` / `os.sysctl`
- No `git.clone` / `git.checkout`
- No `text.line` (lineinfile-equivalent) or `text.patch.json|yaml|ini`
- No `wait.port` / `wait.http` / `wait.file` (current `wait` action handles
  one polymorphic form; we want a clean per-domain split)
- No reactive `on_change` triggers
- No `try`/`catch`/`finally`
- No `transaction:` group with reverse-on-failure
- No secret refs (`!secret vault:...`)
- No Tier-2 plugin model (notifications, k8s, db, etc.)
- Extended Handler ABI (`Diff`, `Reverse`, `Cost`, `Permissions`) not built

---

## Sequencing strategy

Five buckets, ordered by dependency:

```
E9.1  Extended Handler ABI            ← unblocks E9.4 (transactions)
E9.2  Framework primitives             ← independent value
E9.3  Tier-1 action buildout           ← parallel; depends on E9.1 for
                                          some safety features
E9.4  Transactional groups + Reverse   ← needs E9.1 Reverse + handler coverage
E9.5  Tier-2 plugin model              ← independent; proves marketplace path
```

E9.1 and E9.2 land in parallel. E9.3 (the user-visible bulk) can begin as
soon as E9.1's `Diff` + `Reverse` interfaces are defined — actions can
implement them as they're built rather than retrofitting later. E9.4 is the
"agent safety pitch" demo; depends on enough actions implementing `Reverse`
to make a meaningful transaction. E9.5 is the proof-of-concept for the
plugin model — picks a low-risk domain (notifications) so distribution
mechanics can be sorted independently of the actions themselves.

---

## Spec map

| Spec | Title | Bucket | Effort |
|------|---|---|---|
| [22](spec-22-extended-handler-abi.md) | Extended Handler ABI (Diff / Reverse / Cost / Permissions) | E9.1 | M |
| [23](spec-23-framework-primitives.md) | Framework primitives (on_change, try/catch/finally, !secret refs) | E9.2 | M |
| [24](spec-24-pkg-surface.md) | `pkg.install` / `pkg.remove` / `pkg.repo` (full pkg.* surface) | E9.3 | M |
| [25](spec-25-text-surface.md) | `text.line` + `text.patch.{json,yaml,ini}` | E9.3 | M |
| [26](spec-26-git-actions.md) | `git.clone` / `git.checkout` / `git.config` | E9.3 | S |
| [27](spec-27-os-identity.md) | `os.user` + `os.group` + `os.ssh_key` | E9.3 | M |
| [28](spec-28-os-scheduling.md) | `os.cron` + `os.systemd` + `os.firewall` + `os.mount` + `os.sysctl` | E9.3 | L |
| [29](spec-29-wait-primitives.md) | `wait.port` / `wait.http` / `wait.file` / `wait.command` | E9.3 | S |
| [30](spec-30-transactions.md) | `transaction:` blocks with reverse-on-failure | E9.4 | L |
| [31](spec-31-tier2-plugin-model.md) | Tier-2 plugin model + `notify.*` proof | E9.5 | L |

Other follow-ups deferred (don't have specs yet):
- AI-translation CLI for Ansible playbooks (per `VISION.md` §5 / §8 of
  `VISION_ACTIONS.md`)
- `mooncake hub` — control plane (separate epic; see `VISION.md` §8)
- `mooncake guard` — agent sandbox runtime (separate epic; `VISION.md` §7)
- Action versioning (per-action `@v` pins; `VISION_ACTIONS.md` §9 Q2)

---

## Cross-cutting decisions to lock before any of the above ships

Picked by spec-21 already, restated here so each downstream spec doesn't
re-litigate them:

1. **Modern names only.** No legacy/Ansible aliases. AI does Ansible→v2
   translation at author time; if it ever becomes a real ask, we ship a
   `mooncake translate` CLI as a separate binary.
2. **Dot-namespaced action keys** for domain actions. Flat keys for
   foundational primitives (`shell`, `assert`, `wait`, `vars`).
3. **Single canonical schema source.** `internal/config/schema.json` is
   generated from Go struct tags by `mooncake schema generate`; never
   edited by hand. Same for `internal/config/schema.d` and root
   `mooncake.d.ts`.
4. **Test fixtures live alongside Go test files**, not in `testdata/` —
   inline YAML strings in `_test.go` is the project convention.
5. **`make schema-check` and `make docs-check` are CI-blocking.** Every
   spec that changes the surface must regen both.

---

## Open epic-level questions

1. **WASM plugin runtime — when?** Spec-31 (Tier-2 plugins) starts with
   in-tree Go plugins. WASM unlocks the long-tail community marketplace
   but is a major scope item. Defer decision until first 5 official
   Tier-2 plugins are alive; if patterns repeat enough, WASM becomes the
   next epic.
2. **Policy DSL for action gating** (per `VISION.md` §10.7) — needed
   before "Mooncake as agent sandbox" ships, but not on the critical
   path of the action buildout. Likely E10.
3. **Drift remediation loop** — once `Diff` exists (spec-22), do we
   ship a `mooncake remediate` command that auto-generates a Step list
   from the diff? Probably an epic of its own.
4. **Versioning of actions** — should `pkg@v1` exist or do we just
   pretend everything's v1 forever and break on edges? Defer until first
   ABI churn after spec-22.

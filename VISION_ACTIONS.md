# Mooncake — Action Surface Expansion (Modern)

> Companion to `VISION.md` §6.1. Focused brainstorm on **what actions to add,
> what design principles to hold, and what plumbing has to come with them**.
>
> **Design directive: Modern only.** No Ansible/Chef/Puppet vocabulary in the
> core. Migration from legacy tools is handled by AI translation (LLMs are
> excellent at YAML-to-YAML rewrites) or, if needed, a separate translator
> CLI shipped later. The core surface stays clean.

---

## 1. Why this matters more than the rest of the vision

The action surface *is* the product. Everything else — daemon, hub, agent SDK,
marketplace — is plumbing around it. If actions are weak, the rest is a fancy
wrapper around `shell:`.

Two threats if we get this wrong:

1. **Action sprawl** — 200 half-baked actions, none idempotent, none typed.
   (Ansible's failure mode.)
2. **Action poverty** — agents fall back to `shell:` for everything, defeating
   the whole "typed funnel" pitch.

The path between them is **few, deep, opinionated** core actions + a plugin
system for the long tail.

---

## 2. Where we are today

13 actions: `print`, `vars`, `shell`, `command`, `include_vars`, `file`,
`template`, `copy`, `download`, `unarchive`, `assert`, `preset`, `service`.

**Coverage today**: filesystem (good), templating (good), command execution
(good), services (basic), HTTP downloads (basic), assertions (good).

**Holes that bite agents and ops users immediately**:

- No first-class package management (`shell: apt install …` everywhere).
- No idempotent user / SSH-key / sudoers management.
- No declarative package-repo or APT/Yum source management.
- No structured text editing beyond `template` (no JSON/YAML/INI/line edits).
- No `git` action — agents shell out to clone, lose idempotency.
- No cron / timer actions.
- No firewall / network primitives.
- No secrets-store integration (Vault, 1Password, age, sops).
- No cloud-API actions (DNS, certs, S3, k8s).
- No wait/poll primitive.
- No reactive "if X changed, do Y" pattern.

---

## 3. Design principles (read before adding *any* action)

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

## 4. The extended action ABI

Today: `Metadata`, `Validate`, `Execute`, `DryRun`.

Modern target (Terraform/Pulumi-influenced vocabulary):

```go
type Handler interface {
    // identity & validation
    Metadata() ActionMetadata
    Validate(step Step) error

    // execution
    Plan(ctx Context, step Step) (Plan, error)        // was: DryRun
    Apply(ctx Context, step Step) (Result, error)     // was: Execute

    // safety primitives
    Diff(ctx Context, step Step) (Diff, error)        // structured delta
    Reverse(ctx Context, step Step, r Result) (Step, error)  // rollback step
    Cost(ctx Context, step Step) (CostEstimate, error)
    Permissions(step Step) PermissionSet
}
```

- **`Plan` / `Apply`** — terminology aligns with how the rest of modern IaC
  thinks. "Dry-run" is dated language; "plan" is what users actually want.
- **`Diff`** beats `Plan` for UIs and for the agent SDK ("show me what
  changes, structurally").
- **`Reverse`** is the rollback story. Most actions synthesize this from the
  snapshot diff; some (drop a DB table) can't and say so.
- **`Cost`** powers the risk classifier — agents see "412-file change, risk
  7/10" before committing.
- **`Permissions`** lets the daemon and policy engine gate before execution.

These are additive — existing handlers keep working with defaults during the
ABI transition.

---

## 5. The tiered action surface

Don't ship 100 first-party actions. Ship a tight Tier 1, curated Tier 2, and
a marketplace for Tier 3.

### Tier 1 — Kernel (in main binary, always present)

The 80% of system config + what an AI agent needs to be productive without
`shell:`. Target: ~30 actions, dot-namespaced by domain.

**Already shipped** (will be renamed under namespaces in a 2.0):

| Today | Modern |
|---|---|
| `file` | `file.write` |
| `template` | `file.template` |
| `copy` | `file.copy` |
| `download` | `file.download` |
| `unarchive` | `file.unarchive` |
| `shell` | `shell` (keep — it's already short and domain-neutral) |
| `command` | `cmd` |
| `service` | `os.service` |
| `assert` | `assert` (becomes a namespace: `assert.command`, `assert.file`, …) |
| `vars`, `include_vars` | become declarative `vars:` / `imports:` blocks, not "actions" |
| `print` | `log` |
| `preset` | `use` (modern term: "use a preset", like `use` in CSS / many DSLs) |

**To add in Tier 1** (priority order):

| Action | Why Tier 1 |
|---|---|
| `pkg.install` / `pkg.remove` | Unified apt/dnf/brew/pacman/apk/zypper. Today everyone reaches for `shell`. |
| `pkg.repo` | Apt sources, Yum repos, Homebrew taps. Pairs with `pkg`. |
| `os.user` | Idempotent user creation. |
| `os.group` | Idempotent group management. |
| `os.ssh_key` | Authorized keys, idempotent. |
| `os.cron` | Idempotent scheduled jobs. |
| `os.systemd` | Write + reload unit files with validation. |
| `os.mount` | `/etc/fstab` + actual mount. |
| `os.sysctl` | Kernel parameter management. |
| `os.firewall` | Abstracted ufw / firewalld / pf / nftables. |
| `git.clone` / `git.checkout` | Agents touch git constantly; idempotent. |
| `text.line` | "Ensure this line is present/absent in this file." |
| `text.replace` | In-place regex replace with diff. |
| `text.patch.json` | Structural JSON edits (`set`, `delete`, `merge`). |
| `text.patch.yaml` | Same, for YAML. |
| `text.patch.ini` | Section/key management for INI. |
| `wait.port` / `wait.http` / `wait.file` | Agents need this to chain steps reliably. |
| `file.archive` | Inverse of `file.unarchive`. |

That gets us to ~30 actions covering essentially all of "configure a Linux
host" without shelling out.

### Tier 2 — Official plugins (shipped, opt-in load)

Bigger blast radius, narrower audience. Separate Go modules registered on demand.

- **Containers**: `container.run`, `container.image`, `container.compose`
- **Kubernetes**: `k8s.apply`, `k8s.helm`, `k8s.kustomize`
- **Databases**: `db.postgres.user`, `db.postgres.query`, `db.sql.migration`,
  `db.redis.set`
- **Cloud DNS**: `dns.record` with provider drivers (Cloudflare, Route53,
  Hetzner, …)
- **Certs**: `cert.acme` (Let's Encrypt etc.)
- **Cloud storage**: `cloud.s3.object` (works with any S3-compatible)
- **Secrets**: `secret.get` from Vault / 1Password / AWS SM / age / sops
- **VCS hosts**: `git.repo` (settings), `git.release` (download / publish)
- **Notifications**: `notify.slack`, `notify.webhook`, `notify.email`,
  `notify.pagerduty`
- **macOS-specific**: `os.launchd_plist`, `os.defaults`, `pkg.mas`
- **Windows-specific**: `os.registry`, `os.scheduled_task`, `os.feature`

### Tier 3 — Community marketplace (WASM or Go plugins)

Anything else. Signed, versioned, optionally rated. Authors can charge or
open-source; Mooncake takes a marketplace cut eventually.

---

## 6. Framework primitives that *come with* richer actions

Adding rich actions forces new framework concepts. Modern paradigms only —
not Ansible direct ports.

### 6.1 Reactive `on_change` triggers (not "handlers")

Modern reactive style — each step has typed `outputs`; downstream steps react
to changes explicitly. No magic global handler registry.

```yaml
steps:
  - file.template:
      src: nginx.conf.j2
      dest: /etc/nginx/nginx.conf
    as: nginx_cfg

  - os.service:
      name: nginx
      state: reloaded
    when: nginx_cfg.changed
```

Or sugared with a dedicated `on_change:` keyword if the common case warrants:

```yaml
  - file.template:
      src: nginx.conf.j2
      dest: /etc/nginx/nginx.conf
    on_change:
      - os.service: { name: nginx, state: reloaded }
```

No "notify a handler by string name across the playbook" magic.

### 6.2 `try` / `catch` / `finally`

Modern error handling, not `block`/`rescue`/`always`.

```yaml
- try:
    - file.write: ...
    - shell: risky.sh
  catch:
    - shell: rollback.sh
  finally:
    - notify.slack: { message: "deploy finished" }
```

Names everyone knows from every modern language.

### 6.3 `for_each` (not `with_items`)

Aligns with Terraform/modern-IaC. Both list and map forms.

```yaml
- pkg.install:
    name: "{{ item }}"
  for_each: [neovim, ripgrep, fzf, tmux]

- os.user:
    name: "{{ item.key }}"
    shell: "{{ item.value.shell }}"
  for_each: "{{ users }}"
```

Many actions should also accept arrays natively for efficiency
(`pkg.install: { names: [a, b, c] }`) rather than looping N times.

### 6.4 `outputs` (not `register`)

Every action publishes typed outputs to a namespace. Downstream `when:` and
`for_each:` reference them.

```yaml
- shell: ./scan.sh
  as: scan

- text.line:
    path: /etc/motd
    line: "Last scan: {{ scan.stdout }}"
  when: scan.changed
```

Universal `.changed`, `.stdout`, `.stderr`, `.failed`, plus action-specific
outputs documented in each action's schema.

### 6.5 `as:` user (not just `become: root`)

Run as any user, not just sudo-to-root.

```yaml
- git.clone:
    repo: ...
    dest: /opt/app
  as_user: deploy
```

### 6.6 Action-level retry / backoff

Universal across every action via top-level `retry:` block, not action-specific.

```yaml
- wait.port:
    port: 5432
  retry: { attempts: 30, delay: 2s, backoff: linear }
```

### 6.7 Secret references in YAML (not values)

```yaml
- file.write:
    path: /etc/app/token
    content: !secret vault:secret/app#token
```

Pairs with the `secret.*` plugin. The literal value never appears in plans,
logs, runlogs, or the MCP tool I/O — only the reference does.

### 6.8 Transactional groups

A step group that either fully applies or fully reverses via the `Reverse`
ABI method.

```yaml
- transaction:
    - pkg.install: { name: postgresql }
    - db.postgres.user: { name: app }
    - db.postgres.db: { name: app, owner: app }
  # if any step fails, prior steps' Reverse() runs in LIFO order
```

This is the killer demo for the AI safety pitch. Hard to build, worth it.

---

## 7. The first 10 to actually build (opinion)

Ranked by ROI over the next 8–12 weeks:

1. **`pkg.install` / `pkg.remove`** — kills 60% of `shell: apt/brew/dnf` use.
2. **`pkg.repo`** — pairs with `pkg`; can't ship `pkg` without it.
3. **`text.line`** — kills most "edit /etc/something" shell-outs.
4. **`git.clone`** — agents touch git constantly; idempotent clone is huge.
5. **`os.user` + `os.ssh_key`** — pair them; covers "set up a server".
6. **`os.cron`** — easy win, requested often.
7. **`wait.port` + `wait.http`** — agents need this to chain steps reliably.
8. **`text.patch.json`** — agents *love* JSON; structural edits beat template
   regen.
9. **`os.systemd`** — pairs with existing `service`; closes "create a service
   from scratch".
10. **`notify.slack` / `notify.webhook`** — first multi-driver plugin, proves
    the plugin model.

These 10 alone move Mooncake from "shell wrapper with idempotency" to
"genuinely covers hand-managed server config" — without inheriting a single
Ansible name.

---

## 8. Migration from Ansible (for the record)

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

## 9. Open questions

1. **Plugin distribution.** Three options:
   - Static-linked Go modules (build-time, slow iteration).
   - Out-of-process subprocess (HashiCorp-style, friction in distribution).
   - WASM (clean sandbox, ecosystem still maturing for system-config).
   Probably: in-tree Go for Tier 1 & 2 for year one, then WASM for the
   community marketplace once that ecosystem hardens.
2. **Action versioning.** If `pkg.install` v2 changes its schema, how does a
   plan declare which version it targets? `mooncake: ">=2"` at plan level?
   Per-action `pkg.install@1`? Probably both: plan-level minimum, per-action
   pin only when needed.
3. **Cloud-action granularity.** `dns.record` with a provider field, or
   `dns.cloudflare.record` / `dns.route53.record` separately? Probably the
   former for the obvious cases, latter when provider quirks leak.
4. **Compound actions.** A Tier-1 action that internally composes others
   (e.g. `app.deploy` = git.clone + pkg.install + os.systemd + os.service)?
   Tempting but slippery slope. Probably keep composition strictly in
   preset-land.
5. **How aggressive on Reverse / rollback?** Most differentiated pitch for AI
   safety, but hardest to implement well. Ship Tier 1 without it, retrofit
   action-by-action as snapshot coverage matures. The transactional groups
   (§6.8) only need it for the actions used inside them.
6. **The `2.0` rename.** Existing actions (`file`, `template`, `service`)
   need renaming to namespaced form. Breaking change. Probably bundle into a
   single coordinated `mooncake 2` release with an auto-upgrade tool.

---

## 10. Decisions to lock in before writing code

Before any new action lands:

- **Extended ABI (§4):** mandatory vs optional methods. Lock now.
- **Plugin model (§9 Q1):** defer the WASM decision by shipping in-tree Go
  for the first year; do *not* paint into a corner that prevents WASM later.
- **`on_change` framework primitive (§6.1):** land *before* shipping
  `os.systemd` / `os.service`-adjacent actions; retrofitting later is ugly.
- **Outputs schema (§6.4):** every action's outputs must be in its JSON
  schema before the action ships. No "we'll document it later".
- **No legacy aliases. Ever.** Closed for debate per design directive.

---

*Cross-references: `VISION.md` §6.1 (capability brainstorm), §7 (Safe Agent
Runtime — depends critically on these actions existing), §11 Phase A
(timeline).*

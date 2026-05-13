# Spec 26: `git.*` Actions — clone, checkout, config

**Status:** Draft
**Epic:** E9 Modern Action Surface — bucket E9.3
**Effort:** S (3–5 days)
**Value:** High. Every AI-agent playbook touches git within the first
few steps. Today: shell-out to git, lose idempotency, fight with
credentials. First-class git actions fix this for the common 80%.

**Design principles:** `docs-working/action-design-principles.md`

---

## Problem

Agents and humans both reach for git constantly. Today's playbook for
"clone repo at a specific ref" is:

```yaml
- shell: |
    if [ ! -d /opt/app/.git ]; then
      git clone https://github.com/example/app /opt/app
    fi
    cd /opt/app && git fetch origin && git checkout v1.2.3
```

Problems:
- Not idempotent (running twice may produce different state if
  upstream advanced and the working tree had local commits).
- No diff in plan mode ("we'd be at commit X, currently at Y").
- Credentials are opaque — shell-exposed env vars leak into runlogs.
- No "shallow clone" / "depth" knob.
- Submodule init/update is yet more shelling.

---

## Goals

- **G1** `git.clone` — clone-or-pull, idempotent, supports depth,
  recurse-submodules, branch.
- **G2** `git.checkout` — switch to a ref (branch, tag, commit) within
  an existing repo.
- **G3** `git.config` — manage `git config` keys (local/global/system).
- **G4** All three implement `Diff`, `Reverse` (where reasonable),
  `Permissions`.
- **G5** Credentials via `!secret` refs (spec 23) — never inline.

**Out of scope:**

- `git.commit` / `git.push` — Mooncake's domain is *configuring*
  systems, not authoring git history. Defer indefinitely.
- `git.repo` (managing GitHub/GitLab repo settings) — Tier-2 (`vcs.*`
  plugin domain).
- Git LFS — separate optional flag once a real use case appears.

---

## Design

### `git.clone`

```yaml
- git.clone:
    repo: https://github.com/example/app
    dest: /opt/app
    ref: v1.2.3              # branch, tag, or commit SHA
    depth: 1                 # shallow clone
    recurse_submodules: true
    update: true             # if dest exists: fetch + checkout ref
    force: false             # discard local changes when updating
    credentials:
      username: deploy
      password: !secret env:GIT_TOKEN
```

Semantics:
- If `dest` doesn't exist or isn't a git repo: clone.
- If `dest` exists and is a git repo:
  - If `update: false`: noop.
  - If `update: true`: fetch, then checkout `ref`. Local changes:
    - `force: false` + dirty working tree → error.
    - `force: true` → discard local changes via `git reset --hard`.
- `ref` is resolved via `git rev-parse` after fetch; sha lock is what's
  asserted.

Outputs:
```yaml
outputs:
  sha:           "abc1234"
  ref_resolved:  v1.2.3
  changed:       true            # true on initial clone OR sha change
```

### `git.checkout`

```yaml
- git.checkout:
    dest: /opt/app
    ref: v1.2.3
    force: false
```

For when the repo already exists (e.g. a separate clone step earlier in
the playbook). Idempotent: noop if HEAD already at the resolved sha.

### `git.config`

```yaml
- git.config:
    scope: global              # local | global | system
    repo: /opt/app             # required iff scope=local
    set:
      "user.email": dev@example.com
      "core.autocrlf": "false"
    unset:
      - "credential.helper"
```

Wraps `git config --<scope> [--get|--unset]` calls. Idempotent: skips
keys already at the requested value.

### Cross-cutting

- **Permissions:** `git.clone` declares `Network: true`,
  `RequiredBinaries: [git]`, and `Sudo: true` iff `dest` is in a
  privileged location. `git.checkout` no `Network`. `git.config`
  no `Network`.
- **Cost:** `git.clone` `Risk: 4`, `Resources: 1` (one tree). Shallow
  vs deep affects `Bytes`. `git.checkout` `Risk: 3`. `git.config`
  `Risk: 2`.
- **Diff:** `git.clone` reports `Operation: create` initially,
  `update` on subsequent runs with from-sha → to-sha. `git.config`
  reports per-key before/after.
- **Reverse:** `git.config` reverses by restoring the previous value
  (snapshot-based). `git.clone` is declared `reversible: false` (no
  rmtree-on-rollback to avoid foot-guns; users who need this build
  it via `try/catch/finally` + `file.write` cleanup).

### Credentials handling

Credentials are passed via `!secret` (spec 23). Implementation:
- HTTPS: temporary `GIT_ASKPASS` script writes the credential to fd 0
  when git asks. Script is in `os.TempDir()`, mode 0600, removed on
  exit.
- SSH: handled via `GIT_SSH_COMMAND` if the user supplies a private key
  via `!secret`. Key written to tmpfile, mode 0600.

Credential values never appear in `git`'s stdout/stderr capture (we
mark the process output as `redact: true` so the runlog gets `[REDACTED
KEYS]` placeholders).

---

## Key files

| File | Change |
|---|---|
| `internal/actions/git_clone/`, `internal/actions/git_checkout/`, `internal/actions/git_config/` | New handler packages. |
| `internal/config/config.go` | New Step fields: `GitClone`, `GitCheckout`, `GitConfig` + their action structs. |
| `internal/register/register.go` | Register three new actions. |
| `internal/config/schema.json` etc. | Regenerate. |
| `examples/git/clone-and-checkout.yml` | Worked example. |

---

## Tasks (phased)

1. **Phase 1** — `git.clone` core: clone + update + ref-checkout +
   sha output. No submodules, no credentials.
2. **Phase 2** — Credentials via `!secret`. Both HTTPS and SSH paths.
   Redaction tests against stdout capture.
3. **Phase 3** — `git.checkout` and `git.config`.
4. **Phase 4** — Submodules: `recurse_submodules: true`.
5. **Phase 5** — Implement spec-22 hooks (`Diff`, `Reverse`,
   `Permissions`).
6. **Phase 6** — Docs + examples.

---

## Acceptance criteria

- `git.clone` of a public HTTPS repo to a fresh dest succeeds; second
  run reports `changed: false`.
- `git.clone` with `ref: v1.2.3` then later `ref: v1.3.0` reports
  `changed: true` and `sha` matches `git rev-parse v1.3.0`.
- `git.clone` with `!secret env:GIT_TOKEN` succeeds against a private
  repo; the token never appears in runlogs or events.
- `git.config set: user.email=...` is idempotent on second run.
- All three implement spec-22 hooks (or explicitly mark
  `reversible: false`).
- Build / vet / lint / test green.

---

## Open questions

1. **What happens if `ref` is a moving branch (e.g. `main`) and
   upstream advanced?** Default policy: `update: true` always fetches
   and fast-forwards. If user wants pinning, they specify a sha or
   tag.
2. **How to handle `safe.directory` config rules?** When `git.clone`
   runs as root into `/opt/app` and then a non-root user uses the
   repo, git refuses. Probably set the safe.directory automatically
   via `git.config` when `as_user` differs from clone-time user.
3. **Submodule auth — same `!secret`?** Probably yes; document.
4. **Should `git.clone` support `branch:` separately from `ref:`?**
   Lean: just `ref:`. Less confusion; `ref:` accepts anything git
   understands.

# Spec 24: `pkg.*` Surface — install / remove / repo

**Status:** Draft
**Epic:** E9 Modern Action Surface — bucket E9.3
**Effort:** M (1–2 weeks)
**Value:** High. The existing `pkg` action is declarative
(`state: present|absent`) and works well for the common case; what we're
missing is the rest of the package-management story: adding/removing
repositories, listing installed packages for inventory, and tightening
the imperative cases (forced reinstall, holds, autoremove).

**Source:** `VISION_ACTIONS.md` §5 (Tier-1 priorities §7.1, §7.2).

---

## Problem

Today, `pkg` handles:

```yaml
- pkg:
    state: present
    names: [nginx, curl]
```

It works across apt/dnf/yum/pacman/zypper/apk/brew/port/choco/scoop with
batched installs (spec-17). But the realistic playbook needs more:

1. **Adding repos.** APT requires `add-apt-repository` or writing
   `/etc/apt/sources.list.d/*.list` plus a GPG key. Yum/dnf has
   `dnf config-manager`. Brew has `brew tap`. Today every preset
   reaches for `shell:` to do this.
2. **Holds / pins.** "Install postgres 15 specifically; don't let
   `unattended-upgrades` move it." Today: shell.
3. **Force reinstall.** "I broke this package; reinstall it." Today:
   shell.
4. **Autoremove.** "Clean up orphaned dependencies after a remove."
   Today: shell.
5. **Inventory.** "List installed packages and versions." Useful for
   facts, snapshots, and the agent SDK — but no first-class API.

The clean fix isn't to bolt these onto `pkg` as flags; it's to expose
the namespaced surface `pkg.repo`, `pkg.hold`, etc. — and rename the
current single-action `pkg` to `pkg.install` (with `state: absent`
becoming the natural way to express `pkg.remove`, or providing both as
sugar).

---

## Goals

- **G1** Rename today's `pkg` action to `pkg.install` (action key
  `pkg.install`). `pkg.remove` is sugar for `pkg.install` with `state:
  absent`.
- **G2** Add `pkg.repo` — declarative repository management with key
  trust, GPG validation, source-list entries.
- **G3** Add `pkg.hold` — mark/unmark packages as held.
- **G4** Add `pkg.upgrade` — upgrade specific packages or all of them
  (replaces `state: latest`).
- **G5** Add `pkg.list` — return installed packages as an output (no
  side effects). Action of kind "query" — useful inside `outputs:` for
  downstream conditionals.
- **G6** All actions are batched where the manager supports it.
- **G7** All actions implement `Diff`, `Permissions`, and `Reverse`
  (spec 22).

**Out of scope:**

- AUR (`yay`, `paru`), `flatpak`, `snap`, `nix` — Tier-2 plugin specs.
- `pkg.search` — separate read-only query action; defer.
- Package-manager-specific knobs that don't generalize (e.g. apt's
  `--no-install-recommends`) stay on `extra:` like today.

---

## Design

### Action surface

| Action | Purpose | Idempotent | Reversible |
|---|---|---|---|
| `pkg.install` | Ensure packages present (or absent via `state:`) | yes | yes (install↔remove) |
| `pkg.remove` | Sugar — equivalent to `pkg.install: { state: absent }` | yes | yes |
| `pkg.repo` | Add/remove an apt/yum/brew tap repository | yes | yes |
| `pkg.hold` | Mark/unmark a package as held / pinned | yes | yes (hold↔unhold) |
| `pkg.upgrade` | Upgrade named packages or all | partially (depends on manager state) | no (irreversible) |
| `pkg.list` | Return installed packages — read-only query | n/a | n/a |

### `pkg.install` (modernized rename of today's `pkg`)

```yaml
- pkg.install:
    name: nginx
    state: present
    manager: apt          # auto-detected if empty
    update_cache: true
    extra: [--no-install-recommends]
```

Multi-package:
```yaml
- pkg.install:
    names: [nginx, curl, jq]
```

`state: absent` is equivalent to `pkg.remove`.

Outputs:
```yaml
outputs:
  installed:      [nginx, curl]   # newly installed this run
  already_present: [jq]
  removed:        []
  changed:        true
```

### `pkg.repo`

```yaml
- pkg.repo:
    name: nodesource             # human-readable label
    state: present
    apt:
      uri: https://deb.nodesource.com/node_20.x
      suites: [nodistro]
      components: [main]
      gpg_key_url: https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key
      gpg_key_fingerprint: "9FD3..."
    # Or for dnf/yum:
    dnf:
      baseurl: https://rpm.nodesource.com/pub_20.x/nodistro/nodejs/x86_64/
      gpg_key_url: ...
      gpg_check: true
    # Or for brew:
    brew:
      tap: homebrew/cask-fonts
```

The action picks the relevant config block based on the active manager.
Trust model: GPG fingerprint is required when `gpg_check: true` (default);
without it the repo addition fails with a clear error.

`pkg.repo` writes to:
- apt: `/etc/apt/sources.list.d/<name>.sources` (DEB822 format) +
  `/etc/apt/keyrings/<name>.gpg`. Triggers `apt-get update` for the
  specific source via `apt-cache policy` parity.
- dnf/yum: `/etc/yum.repos.d/<name>.repo`.
- brew: `brew tap <repo>`.

Reverse: removes the source-list file and (if no other repo uses it)
the keyring file. Brew untaps.

### `pkg.hold`

```yaml
- pkg.hold:
    name: postgresql-15
    state: held         # or "unheld"
```

Manager-specific implementation:
- apt: `apt-mark hold` / `unhold`
- dnf: `dnf versionlock add` / `delete`
- pacman: append to `IgnorePkg` in `pacman.conf`
- brew: `brew pin` / `unpin`

### `pkg.upgrade`

```yaml
- pkg.upgrade:
    names: [nginx, curl]   # or omit names to upgrade all
    autoremove: true       # apt autoremove / dnf autoremove after
```

Outputs: list of upgraded packages with from→to versions.

### `pkg.list`

```yaml
- pkg.list:
    manager: apt           # default: auto
  as: installed_pkgs

# Now: installed_pkgs.packages is a list of {name, version, manager}
```

No side effects. Implementation: parse `dpkg -l` / `rpm -qa` / `pacman
-Q` / `brew list --versions`.

### Cross-cutting

- **Permissions:** `pkg.install`/`pkg.remove`/`pkg.repo`/`pkg.hold`/
  `pkg.upgrade` all declare `Sudo: true` and `Network: true`. `pkg.list`
  is read-only, no perms required.
- **Cost:** install/remove → `Resources: len(packages)`,
  `Risk: 5` (routine). `pkg.upgrade` → `Risk: 6`. `pkg.repo` →
  `Risk: 6` (adds a software source — explicit trust).
- **Diff:** install → `{Operation: create, After: [pkgs]}`; remove →
  delete; repo → create/delete of the source file with full content
  diff.
- **Reverse:** install ↔ remove; repo add ↔ repo remove; hold ↔ unhold;
  upgrade is declared `reversible: false` (can't dependency-resolve
  back to old versions reliably).

---

## Key files

| File | Change |
|---|---|
| `internal/actions/package/` | Rename package directory? No — keep `internal/actions/package/` for backward Go-package-name compat. Add `handler_install.go`, `handler_repo.go`, `handler_hold.go`, `handler_upgrade.go`, `handler_list.go`. |
| `internal/actions/package/handler.go` | Today's single Handler split into per-action Handlers. Existing logic moves to `handler_install.go`. |
| `internal/config/config.go` | `Pkg` field renamed to `PkgInstall` (yaml `pkg.install`); new fields `PkgRepo`, `PkgHold`, `PkgUpgrade`, `PkgList`. `pkg` (yaml) becomes alias for `pkg.install` for one release to soften the change. (Actually — per "no legacy aliases ever", no alias. Document the rename in v2.1 changelog.) |
| `internal/register/register.go` | Register all 5 new actions. |
| `internal/config/schema.json` | Regenerate. |
| Examples & presets | Many presets use `pkg:` today — bulk-migrate to `pkg.install:`. Reuse the v1→v2 migrator pattern (one-shot Python script under `scripts/migrate/v2.0-to-v2.1/`). |
| Tests | Per-action handler tests; integration test against a Docker apt/dnf/pacman matrix (already exists in `testing-next/`). |

---

## Tasks (phased)

1. **Phase 1** — Rename `pkg` → `pkg.install`. Single-action rename via
   the migrator pattern. Build + tests green. (Equivalent to a mini
   spec-21 cutover for one action.)
2. **Phase 2** — Add `pkg.repo`. Per-manager driver. Apt first
   (DEB822 sources + keyring). Tests on the Docker matrix.
3. **Phase 3** — Add `pkg.hold`. Smaller scope.
4. **Phase 4** — Add `pkg.upgrade`. Replace `state: latest` on
   `pkg.install` with a deprecation hint.
5. **Phase 5** — Add `pkg.list`. Pure query action.
6. **Phase 6** — Implement `Permissions()`, `Cost()`, `Reverse()`,
   `Diff()` on all five actions (requires spec 22 landed).
7. **Phase 7** — Docs + examples + migrator + changelog.

---

## Acceptance criteria

- `pkg.install` works identically to today's `pkg` for the common
  `present`/`absent` case across all supported managers (apt/dnf/yum/
  pacman/zypper/apk/brew/port/choco/scoop).
- `pkg.repo` adds a nodesource apt repo + GPG key end-to-end on Ubuntu;
  follow-up `pkg.install: nodejs` succeeds.
- `pkg.hold` prevents subsequent `pkg.upgrade` from moving a held
  package.
- `pkg.list` returns at least 50 packages on a freshly-installed
  Ubuntu image.
- All five actions report `Diff`, `Cost`, `Permissions` via spec-22
  interfaces.
- Idempotency: `pkg.repo` applied twice → changed=true once, then
  changed=false. Reverse restores prior state byte-identical.
- `make schema-check` clean; `make docs-check` clean.

---

## Open questions

1. **Do we keep `pkg:` as an alias for `pkg.install:`?** Per spec-21
   "no legacy aliases ever", probably no. But this is a v2.0→v2.1
   rename, not a v1→v2; the "no aliases" rule was framed against v1.
   Decide before implementing.
2. **Does `pkg.repo` validate the GPG key against a known set, or just
   trust whatever fingerprint the user supplies?** Lean: trust the
   user's fingerprint, document the security model clearly.
3. **What does `pkg.upgrade` do on managers that don't support
   per-package upgrade (e.g. pacman's `-Syu`)?** Probably error out
   with a clear message rather than silently upgrade everything.
4. **Should `pkg.list` cache its output in facts?** Maybe — could fold
   into `facts` subsystem so subsequent steps can use it without
   re-shelling out. Defer to a follow-up.

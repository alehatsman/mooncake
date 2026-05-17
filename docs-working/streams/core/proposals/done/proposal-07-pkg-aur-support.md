# Request — `pkg.install`: AUR / yay support on Arch

**Status**: Shipped 2026-05-17 (commit `7eba594e`, merged `f58cc89c`)
**Filed**: 2026-05-16 by aleh (from x1, while migrating dotfiles shell→actions)
**Related**: [`spec-24-pkg-surface`](../../../../docs-working/specs/done/spec-24-pkg-surface.md) — slots into G1 (`pkg.install` manager set) and G2 (`pkg.repo`, sort of — AUR is closer to a parallel package manager than a repo).

---

## The user-facing ask, in one sentence

> Let me write `pkg.install: manager: yay names: [spotifyd, ...]` instead
> of a 20-line `shell: yay -S --noconfirm --needed \ ...` block in my
> dotfiles.

## Why it matters today

Today's `pkg.install` auto-detects from a closed set: apt, dnf, yum,
pacman, zypper, apk, brew, port, choco, scoop, winget. On Arch, AUR is
a parallel ecosystem — `yay` (or `paru`) wraps pacman for AUR packages
and is the de-facto standard. Without first-class support, every Arch
dotfiles repo reaches for `shell: yay -S ...`, which:

- Renders as `would run: yay -S --noconfirm --needed git-delta tealdeer
  nwg-dock-hyprland ...` in plan mode — opaque, can't diff against
  installed state, no idempotency report ("3 packages would be
  installed, 14 already present").
- Has to encode `--needed` manually, gets retry/backoff boilerplate
  wrong, and silently ignores `state: absent`.
- Breaks the audit trail: plan diffs in CI can't tell "added one
  package" from "ran an opaque shell command".

## Concrete current usage (motivating example)

```yaml
# platforms/arch/packages.yml — today
- name: Install AUR packages
  shell: |
    yay -S --noconfirm --needed \
      git-delta \
      tealdeer \
      nwg-dock-hyprland \
      bruno-bin \
      shfmt \
      yq \
      code-minimap \
      zoxide \
      tree-sitter-cli \
      google-chrome \
      1password \
      1password-cli \
      globalprotect-openconnect \
      bluetuith \
      wlogout
  timeout: 15m
  retry:
    attempts: 2
    delay: 10s
  tags: [arch, system, packages]
```

What it should look like:

```yaml
- name: Install AUR packages
  pkg.install:
    manager: yay
    state: present
    names:
      - git-delta
      - tealdeer
      - nwg-dock-hyprland
      - bruno-bin
      ...
  timeout: 15m
  tags: [arch, system, packages]
```

## Design notes / open questions

- **Detection.** `yay` is itself an AUR package; it has a bootstrap
  problem (cloned + built from `aur.archlinux.org/yay.git`). The dotfiles
  step `Build yay AUR helper` handles this. `pkg.install: manager: yay`
  should refuse with a clear error if `yay` is missing rather than try to
  install it.
- **paru parity.** Same UX, different binary. Two options:
  (a) one manager `aur` that picks yay-or-paru in that order,
  (b) explicit `manager: yay` / `manager: paru`. (b) is more honest;
  Arch users have strong opinions.
- **Mixed pacman + AUR.** Today a single `pkg.install:` with `manager:
  pacman` and `names: [...]` works fine for repo packages. AUR is a
  *parallel* set. Two separate steps is the natural model — no need to
  conflate.
- **`pkg.list` extension.** `manager: yay` for `pkg.list` would let
  inventory facts include AUR packages with their installed version,
  useful for snapshots/drift.
- **Reverse-capture.** `pkg.install` should capture pre-state and emit
  Reverse the same way; yay supports `-R` so this is mechanical.

## Effort estimate

Small. yay is `pacman -S` with two wrapper flags (`-Sua` for AUR
upgrades, `--needed` for idempotency). The bulk of work is detection +
clean error when missing + plan-mode rendering ("3 of 14 already
present"). spec-22 ABI hooks should auto-flow if the handler is built
on top of the existing pacman driver.

## Won't fix unless

- The author decides AUR is too distro-specific to be in core (in which
  case this becomes a tier-2 plugin under spec-31).
- Or yay's CLI shape diverges from pacman's so much that the abstraction
  leaks too badly to be useful.

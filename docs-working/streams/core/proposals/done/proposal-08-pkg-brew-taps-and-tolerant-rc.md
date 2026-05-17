# Request — `pkg.install`: brew taps + tolerant exit codes for "already-tapped"

**Status**: Shipped 2026-05-17 (commit `7409ac3c`, merged `cd0aebcc`)
**Filed**: 2026-05-16 by aleh
**Related**: [`spec-24-pkg-surface`](../../../../docs-working/specs/done/spec-24-pkg-surface.md) — fits G2 (`pkg.repo`); taps are macOS's analogue of an APT sources.list entry.

---

## The user-facing ask, in one sentence

> Let me declare `brew tap homebrew/cask-fonts` as data, not as
> `shell: brew tap homebrew/cask-fonts || true`.

## Why it matters today

Brew "taps" are formula sources — pulling a third-party tap is a
prerequisite for installing fonts (`homebrew/cask-fonts`), drivers, or
unmaintained packages. The natural mooncake home for them is `pkg.repo`
(spec-24 G2), but `pkg.repo` was designed around APT/yum sources +
GPG keys and may not have a brew driver yet.

The secondary issue this surfaces: brew exits **non-zero on no-op
re-tap** (`brew tap homebrew/cask-fonts` returns rc=1 if already
tapped). Dotfiles currently work around it with:

```yaml
shell: brew tap homebrew/cask-fonts 2>&1 | grep -v "already tapped" || true
failed_when: rc not in [0, 1]
```

That's enough idempotency pain to make the shell step un-portable.

## Concrete current usage

```yaml
# platforms/macos/packages.yml — today
- name: Tap homebrew/cask-fonts
  shell: brew tap homebrew/cask-fonts 2>&1 | grep -v "already tapped" || true
  failed_when: rc not in [0, 1]
  tags: [macos, system, packages]

- name: Install brew apps
  shell: |
    brew install --cask \
      visual-studio-code \
      docker \
      ...
  failed_when: rc not in [0, 1]
  retry:
    attempts: 2
    delay: 10s
  tags: [macos, system, packages]
```

What it should look like:

```yaml
- name: Tap fonts
  pkg.repo:
    manager: brew
    name: homebrew/cask-fonts
    state: present
  tags: [macos, system, packages]

- name: Install brew casks
  pkg.install:
    manager: brew
    kind: cask
    state: present
    names: [visual-studio-code, docker, ...]
  tags: [macos, system, packages]
```

## Design notes

- **Tap-as-repo.** `pkg.repo: manager: brew` only needs `name:`
  (e.g. `homebrew/cask-fonts` or a full git URL for custom taps). No
  GPG-key concept exists for brew — taps are signed by being trusted git
  remotes. Schema is naturally a subset of the APT/yum `pkg.repo` shape.
- **Cask vs formula.** `pkg.install: manager: brew` already exists, but
  doesn't (?) distinguish formulae from casks. A `kind: cask` (or
  `kind: formula`, default) field would lift `brew install --cask` out
  of shell.
- **Tolerant idempotency.** This is the broader issue: many package
  managers return non-zero on idempotent no-ops (apt `update` when no
  packages changed, brew `tap` when already tapped, pacman `-Syu` on
  no-op). `pkg.install` already absorbs this; `pkg.repo` should too,
  ideally without per-call `failed_when` lists.
- **`brew --no-update`.** Brew prepends an auto-update by default;
  shipping a `pkg.install: refresh: false` (analogous to apt's
  `update_cache:`) saves a slow update on every play.

## Effort estimate

Small if pkg.repo's APT/yum driver framework is generic enough to
plug brew taps in. Medium if the cask-vs-formula distinction grows into
a wider "package kind" concept (also applies to pacman groups,
choco/scoop packages-vs-tools, etc.).

# F049 — pkg handler's Permissions() declares Sudo: true regardless of manager

**Filed**: 2026-05-17 by aleh (from main_pc / WSL, while migrating dotfiles `platforms/arch/packages.yml` AUR shell)
**Severity**: Medium (blocks a real, declared, supported user path)
**Component**: `internal/actions/package/handler.go`

---

## Summary

`pkg` action's `Permissions()` unconditionally returns `Sudo: true` for
every step regardless of the resolved package manager. The preflight
in `executor/preflight.go` then rejects any non-elevated invocation
that doesn't carry `as_user: root`. That's correct for
apt/dnf/pacman/choco — and **wrong** for `yay`/`paru`/`brew`, which
must run as a non-root user (yay/paru refuse root explicitly; brew
will install into the user's prefix and reject root).

Proposal-07 shipped `manager: yay`/`paru` batch templates, but the
permission preflight makes the path unusable without an as_user
workaround that contradicts the manager's own contract.

## Reproduction

```yaml
- pkg:
    manager: yay
    state: present
    names: [git-delta, tealdeer]
```

```
$ mooncake plan -c arch.yml
failed to inspect plan: step "Install AUR packages" requires
elevated privileges (Sudo: true) but mooncake is not running as
root and the step has no as_user; add as_user: root or run
mooncake with sudo
```

Adding `as_user: root` makes the preflight pass but breaks the
action at runtime — yay aborts when invoked as root.

## Root cause

`internal/actions/package/handler.go`:

```go
func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
    return actions.PermissionSet{
        Sudo:    true,
        Network: true,
    }
}
```

The method doesn't read `step.Pkg.Manager`. Compare with
`internal/actions/pkg_repo/handler.go`, which already does the
right thing — it switches on the populated nested block
(`apt`/`dnf`/`brew`) and returns `Sudo: false` for brew.

## Proposed fix

Make `pkg.Permissions()` manager-aware. The decision matrix:

| Manager                  | Sudo | Why                                              |
| ------------------------ | ---- | ------------------------------------------------ |
| apt / dnf / yum / pacman | true  | System package manager, writes /var/lib + /usr |
| zypper / apk / port      | true  | Same                                             |
| choco / scoop            | (Windows-specific, preflight already disabled on Windows) |
| yay / paru               | false | AUR wrappers refuse root by design               |
| brew                     | false | User-prefix install; brew refuses root           |

```go
func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
    ps := actions.PermissionSet{Network: true}
    manager := ""
    if step != nil && step.Pkg != nil {
        manager = step.Pkg.Manager
    }
    switch manager {
    case "yay", "paru", "brew":
        ps.Sudo = false
    default:
        ps.Sudo = true
    }
    return ps
}
```

Edge case: when `manager` is empty (auto-detect), default to
`Sudo: true`. The auto-detect path runs at apply time after
preflight, so we can't know yet — and Sudo: true is the
safer-for-most-targets default. Operators on Arch who want yay
can be explicit (`manager: yay`).

## Sites unblocked (alehatsman/dotfiles)

1 shell in `platforms/arch/packages.yml`:

- Install AUR packages (currently `shell: yay -S --noconfirm --needed ...`)

The corresponding pacman block in the same file already uses
`pkg:` cleanly because `as_user: root` is correct for pacman.

## Adjacency

This is the mirror of what `pkg_repo` already does. The fact that
`pkg_repo` handles per-driver permissions correctly while `pkg`
doesn't suggests this is a copy-paste-and-extend opportunity:
share a helper (`packageManagerNeedsSudo(manager string) bool`)
between the two handlers.

# Spec 36 — Windows support

**Status**: In progress
**Branch**: `worktree-windows-support`
**Driver**: User is about to provision new Windows 11 PCs and wants the path to flow through mooncake end-to-end.

---

## Problem

Mooncake's "Windows support" today is partial and broken in places:

- `package` action knows `choco` and `scoop` but not `winget` — the default on Windows 11.
- `shell` action defaults to `pwsh` (`internal/actions/shell/handler.go:226`) but pwsh isn't installed on a fresh Windows. The `cmd` interpreter would invoke `cmd -c "..."` but cmd.exe needs `/c`. The schema enum rejects `powershell`. Net: `shell:` is broken by default on Windows.
- `copy`, `file`, `command`, `service`, `assert` test suites contain `if runtime.GOOS == "windows" { t.Skip(...) }` — behaviour on Windows is largely unverified (28 skips).
- No way to express Windows host setup (firewall rules, scheduled tasks, registry, WSL setup) in mooncake YAML.

The downstream effect lives in `dotfiles/platforms/windows/bootstrap.ps1`: a PowerShell script that runs *before* mooncake to set up WSL2, firewall, and a scheduled task. Mooncake then provisions the Linux guest. The Windows host is never managed by mooncake.

Goal: bring the Windows host into mooncake's domain so a fresh Win11 PC can be provisioned with `mooncake.exe apply -c bootstrap.yml` from an Administrator PowerShell, then `mooncake apply -c main.yml` inside WSL.

---

## Decisions

1. **Run model**: native `mooncake.exe` on the Windows host. One entry point — no thin installer wrapper. The user invokes `mooncake.exe apply -c bootstrap.yml` from Administrator PowerShell.
2. **No separate PowerShell action**: extend the existing `shell` action with platform-routed implementations (`exec_unix.go` / `exec_windows.go`), matching mooncake's existing pattern (`package` detects apt/brew/winget; `service` routes systemd/launchd/sc). Optional PS-specific fields (`run_as_admin`, `error_action`) live on `ShellAction`, ignored on non-Windows.
3. **Testing**: manual, on the user's Windows box (SSH'd). Mooncake.exe is cross-compiled from Linux and shipped over for each iteration. CI for Windows is out of scope.
4. **High-level Windows actions** (`windows_firewall`, `windows_scheduled_task`, `windows_registry`): out of scope. Bootstrap is done via `shell:` with PowerShell commands.
5. **No backwards compatibility**: this is a pre-release sideproject (see `LLM_GUIDE.md` and `feedback-no-backwards-compat` memory). Change defaults, rename fields, drop the enum — no shims.

---

## Phases

### Phase 0 — Smoke test ✅ done

Cross-compiled `mooncake.exe` and ran a smoke YAML on the Windows box. Findings:

| Action | Status | Notes |
|---|---|---|
| `file.write` (dir + file) | ✅ | Works |
| `file.copy` | ✅ | Works |
| `file.template` | ✅ | Pongo2 renders fine |
| `cmd` (argv exec) | ✅ | Works |
| `shell` | ❌ | Three interlocking bugs — covered in Phase 1 below |

### Phase 1 — Shell action OS routing

Split the shell handler so platform-specific exec construction lives in dedicated files:

```
internal/actions/shell/
  handler.go          # orchestration: Validate, retry, timeout, become semantics
  exec_unix.go        # //go:build !windows — builds exec.Cmd for bash/sh
  exec_windows.go     # //go:build windows  — builds exec.Cmd for powershell/cmd/pwsh
```

Concrete behaviour:

- **Windows default interpreter: `powershell`** (Windows PowerShell 5.1, always present). Was `pwsh` which isn't installed by default. Direct change, no fallback.
- **Interpreter dispatch on Windows**:
  - `powershell` → `powershell.exe -NoProfile -NonInteractive -Command <cmd>`
  - `pwsh` → `pwsh.exe -NoProfile -NonInteractive -Command <cmd>` (errors if pwsh not on PATH)
  - `cmd` → `cmd.exe /c <cmd>`
- **Unix dispatch unchanged**: `bash -c <cmd>` / `sh -c <cmd>`.
- **Drop the schema enum** for `shell.interpreter` (`internal/schemagen/enums.go:16` and regenerate `schema.json`). The interpreter is just a binary name — enum prevents legitimate values like `powershell`, `zsh`, `fish`, `nu`, etc.
- **New optional fields on `ShellAction`** (Windows-only; no-op on Unix):
  - `run_as_admin: bool` — asserts the current process is elevated; errors out otherwise. Does NOT attempt UAC elevation.
  - `error_action: string` — sets `$ErrorActionPreference` before user script (default `Stop`).
- **EncodedCommand for safety**: on Windows, pass the user script via `-EncodedCommand <base64>` to avoid quoting hell. Implementation detail of `exec_windows.go`.

Test coverage on Linux side: shell handler is split but `exec_unix.go` keeps behaving exactly as today, so existing tests pass unchanged.

### Phase 2 — `winget` in the package action

`internal/actions/package/handler.go`:

- Add `pmWinget = "winget"` constant.
- `detectWindowsPackageManager` returns `winget` first if present, then choco, then scoop.
- Install: `winget install --exact --silent --accept-package-agreements --accept-source-agreements --id <pkg>`
- Remove: `winget uninstall --exact --silent --id <pkg>`
- Upgrade: `winget upgrade --exact --silent --accept-package-agreements --accept-source-agreements --id <pkg>`
- Check installed: `winget list --exact --id <pkg>` (non-zero exit ⇒ not installed)
- Unit tests like the existing choco/scoop ones.

### Phase 3 — Cross-platform audit

Sweep these test files, run on Windows via SSH, decide per skip:

| File | Skipped tests |
|---|---|
| `internal/actions/assert/handler_test.go` | 2 |
| `internal/actions/command/handler_test.go` | 8 |
| `internal/actions/copy/handler_test.go` | 3 |
| `internal/actions/file/handler_test.go` | 6 |
| `internal/actions/package/handler_test.go` | 2 |
| `internal/actions/service/handler_test.go` | 1 |
| `internal/actions/shell/handler_test.go` | 6 |

For each: either (a) make the test pass on Windows by fixing the production code, or (b) replace the blanket skip with a comment justifying why it's POSIX-only (e.g. testing `chmod 0700`).

Expected fix areas:
- Path separators in test expectations
- Line-ending assumptions (`\n` vs `\r\n` in template output — pick LF, document it)
- `become`: no-op on Windows (UAC is process-wide, not per-command). Surface this explicitly in `package`/`shell` handlers.

### Phase 4 — `bootstrap.yml` in dotfiles

Replace `dotfiles/platforms/windows/bootstrap.ps1` with `dotfiles/platforms/windows/bootstrap.yml`:

```yaml
- name: Set WSL default version to 2
  shell:
    cmd: wsl --set-default-version 2
    run_as_admin: true

- name: Install Ubuntu-24.04 in WSL
  shell:
    cmd: |
      $distros = wsl --list --quiet 2>$null
      if ($distros -notmatch "Ubuntu-24.04") {
          wsl --install -d Ubuntu-24.04
      }
    run_as_admin: true

- name: Deploy .wslconfig
  file.template:
    src: ./templates/wslconfig.j2
    dest: "{{ env.USERPROFILE }}/.wslconfig"

- name: Add firewall rule for WSL SSH
  shell:
    cmd: |
      if (-not (Get-NetFirewallRule -DisplayName "WSL2 SSH" -ErrorAction SilentlyContinue)) {
          New-NetFirewallRule -DisplayName "WSL2 SSH" -Direction Inbound `
              -Protocol TCP -LocalPort {{ wsl_ssh_port }} -Action Allow -Profile Any
      }
    run_as_admin: true

- name: Register WSL SSH autostart scheduled task
  shell:
    cmd: |
      # …Register-ScheduledTask block from the old bootstrap.ps1…
    run_as_admin: true
```

The two-phase user flow becomes:

```
PS Admin> mooncake.exe apply -c platforms/windows/bootstrap.yml
PS> wsl --shutdown
PS> wsl
WSL>  cd ~/dotfiles && mooncake apply -c main.yml -v variables.yml
```

`dotfiles/main.yml` gets an `is_windows_host` branch separate from the existing `is_wsl` branch.

---

## Files to change (mooncake repo)

| File | Phase | Change |
|---|---|---|
| `internal/actions/shell/handler.go` | 1 | Trim to orchestration; remove `runtime.GOOS == "windows"` branch |
| `internal/actions/shell/exec_unix.go` | 1 | New — POSIX `exec.Cmd` builder |
| `internal/actions/shell/exec_windows.go` | 1 | New — Windows builder; powershell/cmd/pwsh dispatch + EncodedCommand |
| `internal/config/config.go` | 1 | Add `RunAsAdmin`, `ErrorAction` to `ShellAction` |
| `internal/schemagen/enums.go` | 1 | Remove `shell.interpreter` enum |
| `internal/config/schema.json` | 1 | Regenerate without the enum; include new fields |
| `internal/actions/shell/handler_test.go` | 1 | Cover the new fields + Windows dispatch (`_test_windows.go` if needed) |
| `internal/actions/package/handler.go` | 2 | winget constant + detection + install/remove/check |
| `internal/actions/package/handler_test.go` | 2 | winget tests |
| `internal/actions/*/handler_test.go` | 3 | Resolve Windows skips |
| `internal/security/become_*.go` | 3 | Document/enforce no-op on Windows |

## Files to change (dotfiles repo)

| File | Phase | Change |
|---|---|---|
| `platforms/windows/bootstrap.yml` | 4 | New |
| `platforms/windows/bootstrap.ps1` | 4 | Delete after parity proven |
| `main.yml` | 4 | Branch on `is_windows_host` |
| `variables.yml` | 4 | Add `is_windows_host` default |

---

## Out of scope

- Windows-native presets (user explicitly dropped).
- High-level Windows actions (`windows_firewall`, `windows_scheduled_task`, `windows_registry`). Revisit if PowerShell scripts in dotfiles start repeating.
- Windows CI runner. Manual testing only for this spec.
- `gsudo`/programmatic UAC elevation. User runs Administrator PowerShell.
- Symlink support on Windows (requires developer-mode or admin; the `file` action's symlink path is POSIX-only by design).

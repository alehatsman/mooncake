# Spec 56: `fleet bootstrap` for Windows targets

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md).
**Status:** Draft
**Effort:** M (~5 days — most cost is in the scheduled-task XML template + idempotent re-apply path)
**Value:** Medium-high — removes the "Windows host setup lives in dotfiles
PowerShell, gets it wrong, requires a follow-up reboot" failure mode that
bit us in May 2026 (see Motivation below).
**Depends on:** spec-44 (SSH bootstrap), spec-49 (agentd on Windows binary),
spec-36 (Windows-as-target for `apply`).

---

## Motivation

When debugging an unreachable peer in May 2026, the symptom was both halves
of a main_pc fleet (WSL agentd on :7878, Windows agentd on :7879) silently
gone. Two distinct root causes lived in two separate dotfiles assets:

1. **WSL side**: no `mooncake-agentd.service` unit was ever installed in
   the WSL distro. Dotfiles open the firewall ports for :7878 but never
   actually bootstrap agentd inside the distro. The whole controller-side
   `mooncake fleet bootstrap` works perfectly for any other Linux box —
   but nobody had run it against the WSL distro, because the existing
   dotfiles flow didn't make it part of "provision this machine."

2. **Windows side**: dotfiles' `Mooncake-Agentd-Autostart` scheduled task
   used `LogonType=Interactive` + `-AtLogOn` trigger. Agentd runs inside
   the user's interactive Windows session, and gets killed the moment that
   session signs out. SSH-only access (the normal remote-management state)
   doesn't keep an interactive session alive. So every reboot put the
   Windows-side agentd in "Ready but never running" state until the user
   physically signed in at the console.

Both failure modes are *bootstrap correctness* bugs in the dotfiles layer.
But they didn't fail loudly: `mooncake fleet status` reported
"unreachable" — the user discovered the underlying mistakes only by
shelling in over OpenSSH and walking through `Get-ScheduledTaskInfo` and
`systemctl status` by hand.

Spec-36 §191 left high-level Windows actions deferred until "PowerShell
scripts in dotfiles start repeating." That threshold has been crossed:
the current `dotfiles/platforms/windows/bootstrap.yml` has **eight**
PowerShell blocks doing the moral equivalent of an installer (firewall
rules × 2, Hyper-V firewall rules × 2, scheduled task registration × 2,
kick-start, token print). The dotfiles bootstrap is *re-implementing
spec-44 §88* — badly, without the idempotency guarantees, version
detection, or token round-trip the canonical bootstrap provides.

The cleanest fix is to extend `mooncake fleet bootstrap` so a Windows
target is provisioned end-to-end by mooncake itself, exactly the way
Linux and macOS targets already are.

---

## Problem

Today `mooncake fleet bootstrap user@host` understands two target families:

```go
// internal/fleet/bootstrap.go (existing)
switch osName {
case "linux":
    // systemd unit at /etc/systemd/system/mooncake-agentd.service
case "darwin":
    // launchd plist at /Library/LaunchDaemons/com.mooncake.agentd.plist
}
```

Windows is unhandled — `internal/fleet/installer.go` returns
`unsupported os` from `Render()` for anything other than `linux`/`darwin`.
The Windows-side daemon binary itself works (spec-49 shipped it); what's
missing is a "register an autostart wrapper around it, idempotently"
story analogous to systemd's unit-file install.

Without spec-56, dotfiles will keep carrying the autostart logic, and
every consumer of mooncake-on-Windows ends up writing their own
`Register-ScheduledTask` block — with the same bug we just hit.

---

## Goals

- **G1** `mooncake fleet bootstrap aleh@win-box` succeeds against a fresh
  Windows 11 host that has only OpenSSH server enabled. After return:
  - `mooncake.exe` is installed at `%LOCALAPPDATA%\Mooncake\bin\mooncake.exe`.
  - `agentd.token` is generated and persisted at
    `%LOCALAPPDATA%\Mooncake\agentd.token`.
  - A scheduled task `Mooncake-Agentd-Autostart` runs at boot under
    `LogonType=S4U` so the daemon survives signout / SSH-only sessions.
  - Windows firewall (and, when `mirrored` networking is detected,
    Hyper-V firewall) is open for the agentd bind port.
  - `peers.toml` on the controller gets a fresh `[[peers]]` entry with
    the new token.

- **G2** Re-running the same command is a no-op when the existing
  install matches the controller's version (matching spec-44 §88 step 3
  short-circuit).

- **G3** `mooncake fleet upgrade` (existing — spec-44 §340) extends to
  Windows targets transparently, replacing the binary atomically and
  re-execing agentd without re-creating the scheduled task.

- **G4** Bootstrap output names the autostart artifact precisely so a
  user looking at "what did mooncake just install on my box?" can find
  it (e.g. `installed scheduled task "Mooncake-Agentd-Autostart" with
  S4U principal, AtStartup trigger`).

**Non-goals:**

- Windows-host bootstrap *before* OpenSSH is enabled. Users still need
  Administrator PowerShell once to enable OpenSSH server, set
  `administrators_authorized_keys`, and reboot. That one-time prep
  stays in `dotfiles/platforms/windows/bootstrap.yml` (or equivalent).
- Domain-joined / AD-managed Windows. We target standalone /
  workgroup-style Windows 11 boxes; AD comes with its own GPO machinery
  that we don't want to fight.
- Group Managed Service Accounts. The S4U principal covers the
  "survive logout, run under the configured user's profile" need
  without requiring AD infra.

---

## Decisions

1. **Service abstraction grows a `windows` branch.**
   `internal/fleet/installer.go`'s `Installer` struct gains
   `case "windows"` in `UnitPath()`, `UnitName()`, `Render()`,
   `EnableStartCmd()`, `StopDisableCmd()`, and `IsActiveCmd()`. New
   embedded template `init/mooncake-agentd-windows.xml` is the scheduled
   task definition (XML, not PowerShell — it's the input format
   `Register-ScheduledTask -Xml` accepts).

2. **Scheduled task principal: `LogonType=S4U`, `RunLevel=Highest`,
   `AtStartup` trigger.** Survives logout. Does not require a stored
   password. Runs at boot. `%LOCALAPPDATA%` still resolves to the
   configured user's profile so the `bin\`, `agentd.token`,
   `agentd\runs\`, and `agentd\synced\` paths stay where spec-49
   put them.

3. **Binary install location: `%LOCALAPPDATA%\Mooncake\bin\mooncake.exe`.**
   Matches spec-49 exactly. No `Program Files` install — that needs
   admin rights at every upgrade; `%LOCALAPPDATA%` doesn't.

4. **Token at `%LOCALAPPDATA%\Mooncake\agentd.token`** with `Hidden`
   attribute. ACL inherits the user profile ACL (current-user-only by
   default). Avoids the `/etc/mooncake/agentd.token` mode-600
   equivalent we use on Unix.

5. **SSH transport changes are minimal.** spec-44's
   `transport.Session` already runs commands over OpenSSH — Windows
   OpenSSH's default shell is configurable, and dotfiles already sets
   it to `powershell.exe`. spec-56 assumes the same; if the default
   shell is `cmd.exe`, bootstrap detects it via
   `$env:COMSPEC`-equivalent probe and emits a clear error rather than
   silently producing broken commands.

6. **Firewall handling is inline, not abstracted.** spec-36 §191
   explicitly defers `windows_firewall_rule` as a high-level action.
   We don't ship that here either. Firewall rules are added via direct
   PowerShell calls from `bootstrap.go`'s Windows branch — the same
   `if (-not (Get-NetFirewallRule …)) { New-NetFirewallRule … }`
   idiom dotfiles already uses, just owned by mooncake.

7. **Hyper-V firewall is detected, not assumed.** When
   `wsl.exe --status` reports `networkingMode: mirrored` (or
   `Get-NetFirewallHyperVVMCreator` returns the WSL VMCreatorId), we
   additionally register a `New-NetFirewallHyperVRule`. Otherwise we
   skip it. Detection avoids polluting non-WSL Windows boxes with
   Hyper-V rules they don't need.

8. **No PowerShell action dependency.** Spec-36 added `shell:` with
   `interpreter: powershell`; spec-56 uses the SSH transport directly
   (already running on the controller side), not the action subsystem.
   The action subsystem is for declarative configs the user writes;
   bootstrap is imperative installer code.

9. **`fleet doctor`'s SSH fallback (spec-NN, post-spec-55) keeps
   working unchanged.** The diagnostic command for Windows
   (`Get-ScheduledTaskInfo -TaskName 'Mooncake-Agentd-Autostart'`)
   stays correct because spec-56 installs the same task name.

---

## Phases

### Phase 1 — Installer.Windows branch (M)

Add to `internal/fleet/installer.go`:

```go
//go:embed init/mooncake-agentd-windows.xml
var windowsTaskTemplate []byte

func (i Installer) UnitPath() string {
    switch i.OS {
    // ...existing linux + darwin cases...
    case "windows":
        return `%LOCALAPPDATA%\Mooncake\agentd-task.xml` // staged before registration
    }
}

func (i Installer) UnitName() string {
    // ...
    case "windows":
        return "Mooncake-Agentd-Autostart"
}

func (i Installer) EnableStartCmd() string {
    // ...
    case "windows":
        return `powershell -NoProfile -Command "` +
            `Register-ScheduledTask -Xml (Get-Content -Raw '%LOCALAPPDATA%\Mooncake\agentd-task.xml') ` +
            `-TaskName 'Mooncake-Agentd-Autostart' -Force | Out-Null; ` +
            `Start-ScheduledTask -TaskName 'Mooncake-Agentd-Autostart'"`
}
```

The XML template at `init/mooncake-agentd-windows.xml`:

```xml
<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers>
    <BootTrigger>
      <Enabled>true</Enabled>
    </BootTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>{{USER}}</UserId>
      <LogonType>S4U</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>9999</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>{{BIN_PATH}}</Command>
      <Arguments>agentd --bind 0.0.0.0:{{PORT}} --token-file "{{TOKEN_PATH}}"</Arguments>
    </Exec>
  </Actions>
</Task>
```

Render substitutes `{{USER}}`, `{{BIN_PATH}}`, `{{PORT}}`, `{{TOKEN_PATH}}`
the same way the existing systemd template substitutes `{{PORT}}`.

### Phase 2 — Bootstrap.go Windows branch (M)

`internal/fleet/bootstrap.go`'s eight-step sequence (per spec-44 §88)
gets a third platform branch:

| Step | Linux/macOS | Windows |
|---|---|---|
| 1. Connect + auth | unchanged | unchanged (OpenSSH client → Windows OpenSSH server) |
| 2. Detect platform | `uname -s/-m` | `cmd /c "echo %PROCESSOR_ARCHITECTURE%"` + `ver` |
| 3. Check existing install | `mooncake --version` via `/usr/local/bin/mooncake` | `%LOCALAPPDATA%\Mooncake\bin\mooncake.exe --version` |
| 4. Upload binary | SFTP → `/tmp/...` → `sudo mv` | SFTP → temp → `Move-Item -Force` into `%LOCALAPPDATA%\Mooncake\bin\` |
| 5. Install service unit | render systemd / launchd → `/etc/...` | render scheduled-task XML → temp → `Register-ScheduledTask -Xml` |
| 6. Start service + verify | `systemctl enable --now` | `Start-ScheduledTask` |
| 7. Read bearer token | `sudo cat /etc/mooncake/agentd.token` | `Get-Content "%LOCALAPPDATA%\Mooncake\agentd.token"` |
| 8. Update peers.toml | unchanged | unchanged |

No `sudo` on Windows — the task registers under the calling user (who
must be an admin already, since `administrators_authorized_keys` was
the SSH key path). Per-step elevation isn't needed because
`Register-ScheduledTask` for the calling user's tasks doesn't require
UAC.

Firewall rules are step 5b (after task install, before start):

```powershell
if (-not (Get-NetFirewallRule -DisplayName 'Mooncake Agentd' -ErrorAction SilentlyContinue)) {
    New-NetFirewallRule -DisplayName 'Mooncake Agentd' -Direction Inbound `
        -Protocol TCP -LocalPort {{PORT}} -Action Allow -Profile Any
}
```

Hyper-V detection is step 5c, gated by `Get-NetFirewallHyperVVMCreator`
returning a WSL entry.

### Phase 3 — `fleet upgrade` for Windows (S)

`internal/fleet/upgrade.go` (or wherever spec-44's upgrade path lives)
gains Windows handling:

- Stop the scheduled task: `Stop-ScheduledTask -TaskName 'Mooncake-Agentd-Autostart'`.
- Replace the binary: `Move-Item -Force` from the staged path.
- Restart: `Start-ScheduledTask -TaskName 'Mooncake-Agentd-Autostart'`.
- Poll `/v1/version` until the new version answers.

The task definition itself doesn't get re-registered on upgrade —
binary path is stable, no change needed.

### Phase 4 — Dotfiles migration (separate PR, dotfiles repo)

After spec-56 lands in mooncake:

- Delete the eight PowerShell blocks in `dotfiles/platforms/windows/bootstrap.yml`
  that hand-rolled firewall + scheduled task + token round-trip.
- Replace with a single step: `mooncake.exe fleet bootstrap localhost`
  (or the user runs it from x1 instead). The Windows-host bootstrap.yml
  shrinks to "enable OpenSSH server + Hyper-V WSL setup", which is the
  irreducible pre-mooncake prep.

---

## Files to change (mooncake repo)

| File | Phase | Change |
|---|---|---|
| `internal/fleet/init/mooncake-agentd-windows.xml` | 1 | New — task template |
| `internal/fleet/installer.go` | 1 | Windows branch in five methods |
| `internal/fleet/installer_test.go` | 1 | Render → substitution cases for windows |
| `internal/fleet/bootstrap.go` | 2 | Platform-dispatched step 4/5/6 |
| `internal/fleet/bootstrap_windows_test.go` | 2 | Mocked-SSH integration test for the new branch |
| `internal/fleet/transport/ssh.go` | 2 | `DetectPlatform` recognises `windows` (currently errors) |
| `internal/fleet/upgrade.go` | 3 | Windows path for atomic binary replace |
| `cmd/fleet.go` | 2 | `--bin-os` flag for cross-compile-from-Linux uploads |

---

## Out of scope

- **PowerShell `windows_*` actions** (`windows_firewall_rule`,
  `windows_scheduled_task`, `windows_registry`). spec-36 §191 deferred
  these explicitly. Bootstrap doing them inline doesn't change the
  deferral — declarative apply still uses `shell:` with PowerShell.
  Re-evaluate after spec-56 lands whether the inline PowerShell in
  `bootstrap.go` should graduate into actions.
- **Domain-joined Windows.** Add a follow-up spec when first needed.
- **gMSA principal for the scheduled task.** S4U is enough for the
  fleet-of-personal-machines use case; gMSA is enterprise territory.
- **A separate Windows CI runner.** Bootstrap is exercised by manual
  test from x1 against the actual main_pc, matching spec-36 §28.
- **Unprivileged install** (no admin SSH key). Workgroup Windows needs
  the install user to be an admin to register a scheduled task at all;
  least-privilege is a separate, deeper rework.

---

## Open questions

1. **Self-upgrade semantics on Windows.** The Unix self-upgrade
   (spec-49 +) uses `exec` to swap the running process. Windows has
   no `exec` — the daemon must stop and restart. Phase 3 punts to
   "scheduled-task restart" but a cleaner alternative is the daemon
   spawning a child with the new binary and exiting; spec-49's
   `self_upgrade_windows.go` may already model this.

2. **`Restart=on-failure` vs `Restart=always` analogue.** The XML
   template above uses `RestartOnFailure` with a 1-minute interval and
   9999-count cap, which is the Windows scheduled-task analogue of
   systemd's `Restart=always` + `RestartSec=5s`. If a future spec adds
   stricter back-off (StartLimitInterval-equivalent) the XML can grow
   another knob; today the rough analogue is fine.

3. **WSL VM lifecycle interaction.** On the user's main_pc, the WSL VM
   auto-shuts down after ~8 seconds of idle. The Windows-side
   scheduled task is unaffected (host-level), but the WSL-side agentd
   stops with the VM. A `WSL2-Agentd-Keepalive` task — analogous to
   the existing `WSL2-SSH-Keepalive` — may be needed. That's a
   dotfiles concern, not a mooncake concern, but spec-56 should
   surface it so the migration PR doesn't miss it.

4. **Renaming conflict with spec-55.** `fleet doctor` is taken (the
   "fan kernel doctor across peers" spec). The 2026-May connectivity
   probe ladder built ad-hoc during the main_pc debug session is a
   *different* tool; if it's reused, it should be `fleet ping` /
   `fleet diagnose` / `fleet probe`. Not blocking spec-56 but worth
   flagging here since both tools care about "can the controller
   reach this peer."

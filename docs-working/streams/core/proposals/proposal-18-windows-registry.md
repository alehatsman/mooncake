# Request — `windows.registry`: declarative HKLM/HKCU value management

**Status**: Draft proposal
**Filed**: 2026-05-17 by aleh (from main_pc, dotfiles `platforms/windows/bootstrap.yml`)
**Related**: existing `windows.firewall_rule`, `windows.scheduled_task` — same surface family

---

## The user-facing ask, in one sentence

> Let me write `windows.registry: path: HKLM:\... name: HiberbootEnabled value: 0 type: dword` instead of a 12-line PowerShell shell with Get-ItemProperty / Set-ItemProperty boilerplate.

## Why it matters today

`platforms/windows/bootstrap.yml` already uses
`windows.scheduled_task:` and `windows.firewall_rule:` for those
two surfaces. Registry is the third leg of the Windows host
provisioning stool and is the *only* one still done via raw
PowerShell shells. Two specific edits today, both following the
exact same shape:

```yaml
- name: Disable Windows Fast Startup (HiberbootEnabled=0) for stable WSL boot
  shell:
    interpreter: powershell
    run_as_admin: true
    cmd: |
      $key = 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Power'
      $cur = (Get-ItemProperty -Path $key -Name HiberbootEnabled -ErrorAction SilentlyContinue).HiberbootEnabled
      if ($cur -eq 0) {
        Write-Host "Fast Startup already disabled"
      } else {
        Set-ItemProperty -Path $key -Name HiberbootEnabled -Value 0 -Type DWord
      }

- name: Set Windows OpenSSH default shell to PowerShell
  shell:
    interpreter: powershell
    run_as_admin: true
    cmd: |
      $key = 'HKLM:\SOFTWARE\OpenSSH'
      $want = 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe'
      if (-not (Test-Path $key)) {
        Write-Host "OpenSSH registry key missing — skipping"
      } else {
        $cur = (Get-ItemProperty -Path $key -Name DefaultShell -ErrorAction SilentlyContinue).DefaultShell
        if ($cur -eq $want) { ... } else { New-ItemProperty ... -Force }
      }
```

Both reduce to the same three-line idempotent declarative shape.

## Proposed shape

```yaml
- windows.registry:
    path: 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Power'
    name: HiberbootEnabled
    value: 0
    type: dword                 # string|dword|qword|binary|multistring|expandstring
    state: present              # present|absent (default: present)

- windows.registry:
    path: 'HKLM:\SOFTWARE\OpenSSH'
    name: DefaultShell
    value: 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe'
    type: string
    state: present
    # Optional: skip silently when the parent key doesn't exist,
    # instead of erroring. Maps to the existing `Test-Path` guard
    # in the OpenSSH-DefaultShell shell above.
    create_key: false
```

Idempotency: read current value, compare, write only on drift.
Permissions: `Sudo` doesn't quite apply on Windows; the existing
`run_as_admin` mechanism on shell steps is the closest analogue —
make this `Permissions: { RunAsAdmin: true }` for HKLM paths,
unset for HKCU.

## Implementation notes

- Use `Get-ItemProperty` for read + `Set-ItemProperty` for write
  via the existing PowerShell shell-out machinery, exactly like
  `windows.firewall_rule` invokes the firewall cmdlets. No new
  external dep.
- Type-validate `type:` against the PowerShell `RegistryValueKind`
  enum at parse time so a typo (`dwrod`) fails the planner, not
  apply.
- `value:` is a string in YAML; coerce per `type:` (dword/qword:
  parse as int, string/expandstring: literal, multistring: list).

## Sites unblocked (alehatsman/dotfiles)

2 shells in `platforms/windows/bootstrap.yml`:

- Disable Windows Fast Startup (HiberbootEnabled=0)
- Set OpenSSH DefaultShell to PowerShell

Plus future Windows-host registry tweaks (UAC behavior, RDP
auth-level, Win+L lock-screen settings, etc.) that are currently
all shaped as PowerShell shells.

# Request — `windows.hyperv_firewall_rule`: WSL2 mirrored-mode firewall

**Status**: Draft proposal
**Filed**: 2026-05-17 by aleh
**Related**: existing `windows.firewall_rule` — same shape, different
Windows subsystem; both are currently needed in `bootstrap.yml` to
expose WSL ports

---

## The user-facing ask, in one sentence

> Give me `windows.hyperv_firewall_rule:` that mirrors the existing
> `windows.firewall_rule:` schema but targets `New-NetFirewallHyperVRule`
> instead of `New-NetFirewallRule`, so my WSL bootstrap stops being
> two near-identical 30-line PowerShell shells.

## Why it matters today

WSL2 with `networkingMode=mirrored` (the modern default that
`platforms/windows/bootstrap.yml` deploys via `.wslconfig`) puts the
WSL VM behind the **Hyper-V Firewall**, which is a separate stack
from the regular Windows Firewall. A regular firewall rule (which
`windows.firewall_rule:` already handles) opens the host's NICs.
Inbound traffic still gets dropped at the VM boundary unless a
matching `New-NetFirewallHyperVRule` exists with the WSL VMCreatorId.

So bootstrap.yml has TWO declarations per port — one in the
regular firewall (native action) + one in the Hyper-V firewall
(PowerShell shell):

```yaml
- name: Add firewall rule for WSL SSH
  windows.firewall_rule:                  # native action ✓
    name: "WSL2 SSH"
    direction: inbound
    protocol: tcp
    local_port: ["{{ wsl_ssh_port }}"]
    profile: [domain, private]

- name: Add Hyper-V firewall rule for WSL SSH (if not already present)
  shell:                                   # PowerShell shell ✗
    interpreter: powershell
    run_as_admin: true
    cmd: |
      $ruleName = "WSL2 SSH"
      $wslId = (Get-NetFirewallHyperVVMCreator -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match 'WSL' } |
        Select-Object -First 1 -ExpandProperty VMCreatorId)
      if (-not $wslId) {
        $wslId = '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}'
      }
      $existing = Get-NetFirewallHyperVRule -VMCreatorId $wslId -ErrorAction SilentlyContinue |
        Where-Object { $_.DisplayName -eq $ruleName }
      if (-not $existing) {
        New-NetFirewallHyperVRule `
          -DisplayName $ruleName `
          -VMCreatorId $wslId `
          -Direction Inbound -Protocol TCP `
          -LocalPorts {{ wsl_ssh_port }} -Action Allow
      }
```

The same shell shape repeats for the agentd port. Total: 2
matching shells, ~60 lines, both pure PowerShell-with-idempotency-
boilerplate.

## Proposed shape

Mirror `windows.firewall_rule:` exactly, plus a `vm_creator_id`
field with an auto-discovery default:

```yaml
- windows.hyperv_firewall_rule:
    name: "WSL2 SSH"
    description: "WSL guest SSH (mirrored networking)"
    direction: inbound
    protocol: tcp
    local_port: ["{{ wsl_ssh_port }}"]
    action: allow
    vm_creator_id: "auto"          # auto = Get-NetFirewallHyperVVMCreator | match 'WSL'
                                   # fallback constant baked in for fresh boxes
```

For `vm_creator_id: auto`, the action's resolver runs the same
`Get-NetFirewallHyperVVMCreator | Where-Object Name -match 'WSL'`
pipeline the shell uses today, with the documented fallback to
`{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}` when WSL hasn't registered
its VMCreator yet (fresh boxes pre-first-VM-boot).

## Implementation notes

- All idempotency logic is parallel to `windows.firewall_rule`'s:
  read current rules via `Get-NetFirewallHyperVRule`, compare by
  `DisplayName`, write only on drift.
- The shell-out shape is the same too (PowerShell). Likely a tiny
  delta on top of the existing firewall handler — same code path,
  different cmdlets.

## Sites unblocked (alehatsman/dotfiles)

2 PowerShell shells in `platforms/windows/bootstrap.yml`:

- WSL2 SSH Hyper-V rule (port `{{ wsl_ssh_port }}`)
- WSL2 Mooncake Agentd Hyper-V rule (port `{{ wsl_agentd_port }}`)

Plus future WSL exposed ports (other services that bootstrap.yml
might want — postgres, jupyter, etc.) which today would each
require a fresh PowerShell block.

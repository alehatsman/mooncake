# Bug — `windows.firewall_rule` action corrupts non-ASCII description via ConvertTo-Json

**Surfaced:** 2026-05-15 during the spec-56 / spec-57 redeploy session
against `main_pc-win`.

**Repro:** Apply any `windows.firewall_rule` step whose `description`
field contains a non-ASCII character (e.g. `→`, `·`, em-dash). The
*first* apply (rule creation) succeeds — `New-NetFirewallRule
-Description` accepts UTF-8 input correctly from the `-EncodedCommand`
payload. The *second* apply (the drift-detection query) fails:

```
windows.firewall_rule: query current rule: decode rule json:
  invalid character '\x1a' in string literal
  (body: "{...\"Description\":\"WSL guest SSH (mirrored networking
   \x1a Windows host NIC)\",...}")
```

`0x1a` is ASCII SUB — the substitution character emitted when a
codepage can't represent the source byte.

---

## Root cause

Three layers, one chained problem:

1. **`Get-NetFirewallRule | ConvertTo-Json` output encoding.**
   On default Windows installs, PowerShell's default output encoding
   is the OEM codepage (usually 437 or 850 on en-US machines), *not*
   UTF-8. Non-ASCII bytes in the rule's Description field get
   re-encoded to that codepage's substitution character on the way
   out of `ConvertTo-Json`.

2. **`powershell.exe -EncodedCommand` is invariant to this.** The
   command itself goes in as UTF-16LE-base64, which doesn't help —
   `[Console]::Out` for the *child* process is still locale-dependent.
   We're inputting correctly and reading back corrupted.

3. **`internal/actions/windows_firewall_rule/handler.go:realPSRun`**
   captures `stdout` as a Go `bytes.Buffer` and passes it to
   `json.Unmarshal`. Go's JSON decoder treats `0x1a` as an invalid
   control character in a string literal (per RFC 8259), so the
   decode bombs.

The same problem would hit any future Windows action that reads back
non-ASCII data from a cmdlet — `windows.scheduled_task`'s
Export-ScheduledTask roundtrip is the next obvious risk surface.

---

## Fix

Three options, in increasing invasiveness:

### Option A — force UTF-8 output (recommended)

Inject the encoding-set line at the top of every PowerShell payload
we ship through `realPSRun`:

```powershell
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
$OutputEncoding = [System.Text.Encoding]::UTF8;
```

Both are needed — the first governs Win32 console writes, the second
governs the value PowerShell uses when piping to native EXEs (like
`ConvertTo-Json`, which doesn't pipe but follows the same convention
for its `-AsUTF8` behaviour gap).

This change goes in `internal/winutil/` (one helper, one new prelude
constant) and gets called from both action handlers'
`realPSRun`. Existing tests stay green since the prelude is appended
to existing scripts, not replacing them.

### Option B — use `pwsh.exe` (PowerShell 7+) if available

PS7 defaults to UTF-8 output globally. We could detect `pwsh.exe` on
PATH and prefer it. But it's not on a fresh Windows install, and we
shouldn't make our actions depend on a side-loaded tool.

### Option C — decode the captured bytes as Windows-1252 / OEM

Hacky and wrong: the source data was UTF-8; we'd be papering over
the layer-1 problem.

**Recommendation: Option A.** Single line change to the runner;
covers every Windows-action user (current and future); matches what
`mooncake fleet bootstrap`'s Windows branch already does inline
(see `internal/fleet/bootstrap_windows_target.go:psWrap` — the SSH
session doesn't have the OEM-codepage issue because the controller
captures raw bytes that the SSH transport tags as UTF-8 by default).

---

## Test gap

The handler test `handler_test.go` decodes a fabricated JSON body
that already has valid UTF-8 escapes — it never exercises the
codepage round-trip. Add an integration-style test (Windows-tagged,
runs in CI later) that:

1. Creates a firewall rule with a non-ASCII description via raw
   `New-NetFirewallRule`.
2. Calls `windows_firewall_rule.queryRule()`.
3. Asserts the returned `observedRule.Description` matches the
   input verbatim.

Until a Windows CI runner exists, this test runs manually against
`main_pc-win` (per spec-36 §28).

---

## Workaround in the field

Don't put non-ASCII characters in `description:` fields for
`windows.firewall_rule` (or `windows.scheduled_task` once the same
issue lands there). Stick to ASCII until the prelude fix lands.

Documented in the dotfiles commit that fixed the immediate apply
failure: [`dotfiles@2760b4c`](https://github.com/alehatsman/dotfiles/commit/2760b4c).

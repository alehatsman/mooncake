---
id: F032
title: template + download handlers' legacy Execute path builds sudo shell commands without shell-quoting → shell injection on user-supplied dest paths
severity: risk
package: internal/actions
files:
  - internal/actions/template/handler.go (lines 339-357)
  - internal/actions/download/handler.go (lines 444-461, 482-494)
status: done
resolved: 2026-05-16 — Option B (shell-quote at the call sites) applied. Exported `effects.ShellQuote` (kept `shellQuote` as in-package alias so the 13 existing call sites in `default.go` don't need touching). Replaced `fmt.Sprintf("mv %s %s && chmod %s %s", tmpPath, destPath, mode, destPath)` in `template/handler.go::executeSudoFileOperation` and `fmt.Sprintf("mv %q %q", tmpPath, dest)` in `download/handler.go::downloadFile` to single-quote-wrap every interpolated path. Option A (delete the dead Execute/DryRun + helpers) blocked behind the XL F011 work: download/handler_test.go still has ~24 direct `h.Execute()` calls. Regression test `TestF032_ExecuteSudoFileOperation_QuotesDestPath` covers `; touch`, `$(id)`, backticks, and embedded newline payloads, plus `TestF032_ExecuteSudoFileOperation_EmbeddedSingleQuote` for the `'\''` escape idiom.
verified: 2026-05-16 — template/handler.go:343-353 now shell-quotes via effects.ShellQuote on tmpPath/destPath. download/handler.go also patched
---

## What

Both `template` and `download` handlers' legacy `Execute` path
construct a `sudo sh -c <cmd>` invocation where the command is
built via `fmt.Sprintf` on user-supplied paths. Neither version
uses safe shell quoting.

### `template/handler.go:339-357`

```go
func (h *Handler) executeSudoFileOperation(tmpPath, destPath string, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext) error {
    cmd := fmt.Sprintf("mv %s %s && chmod %s %s", tmpPath, destPath, h.formatMode(mode), destPath)
    return h.executeSudoCommand(cmd, step, ec)
}

func (h *Handler) executeSudoCommand(command string, _ *config.Step, ec *executor.ExecutionContext) error {
    cmd := exec.Command("sudo", "-S", "sh", "-c", command)
    // ...
}
```

**No quoting at all.** `destPath` flows in from `step.FileTemplate.Dest`
after `ExpandPath` rendering. A user-supplied dest like

```yaml
file.template:
  src: foo.tmpl
  dest: "/tmp/x; touch /etc/owned"
```

generates the shell command:

```sh
mv /tmp/mooncake-template-12345 /tmp/x; touch /etc/owned && chmod 0644 /tmp/x; touch /etc/owned
```

which `sh -c` runs verbatim. The `; touch /etc/owned` runs with
root privileges via sudo.

### `download/handler.go:444-461`

```go
cmd := fmt.Sprintf("mv %q %q", tmpPath, dest)
if err := h.executeSudoCommand(cmd, step, ec); err != nil { ... }
```

Uses Go's `%q` verb. That's **almost** safe — but `%q` is Go-string
quoting, not POSIX shell quoting. The differences are exploitable:

- `dest = "/tmp/with\"quote.txt"` → `%q` outputs `"/tmp/with\"quote.txt"`.
  Bash inside `""` treats `\"` as backslash + double-quote (the
  backslash inside `""` is literal except before `$`, `` ` ``,
  `"`, `\`, or newline). The result mis-parses; mostly harmless
  here but not what the developer meant.
- **`dest = "/tmp/$(touch /etc/owned)/foo"`** → `%q` outputs
  `"/tmp/$(touch /etc/owned)/foo"`. Bash inside `""` **does**
  perform command substitution on `$(...)`. **Code execution.**
- `dest = "/tmp/`echo pwned`/foo"` → backticks inside `%q` are
  preserved literally; bash performs command substitution.
  Same exploit.

So `%q` is a false-sense-of-safety. The right quoting for POSIX
shells is single-quote wrapping with `'\''` for embedded
single-quotes (see `internal/effects/default.go:593` `shellQuote`).

## Why this is currently unreachable (but still a finding)

`internal/executor/DispatchStepAction` (executor.go:421-453) now
goes through `dispatchRunner` for **all** registered handlers
(comment at line 445: "Spec 16: all handlers implement Runner.
dispatchRunner is the single dispatch path"). So today's apply
path only calls `Run()`, never `Execute()`.

For `template.Run()` (line 364+) and `download.Run()`, the sudo
path goes through `ctx.Effects().WriteFile`, which uses
`internal/effects/default.go:160-161`'s **properly shell-quoted**
`mv … && chmod …`:

```go
cmd := fmt.Sprintf("mv %s %s && chmod %s %s",
    shellQuote(tmpPath), shellQuote(path), formatMode(mode), shellQuote(path))
```

So the modern Run path is safe. The vulnerable code in `Execute`
is **dead** — but only because the executor doesn't dispatch it,
not because callers can't.

External callers that can reach `Execute` today:

- Anyone embedding `actions.Handler` directly (the `Execute`
  method is part of the public interface).
- Tests that call `h.Execute(...)`.
- A future SDK that exposes the handler directly.
- If `DispatchStepAction` ever regresses (e.g. a fallback path
  added for non-Runner handlers), the vulnerable code becomes
  live.

## Why it's `risk` not `bug`

Not currently reachable via the documented apply path — so no
operational exploit today. But:

- The code still exists and compiles.
- It's a **security hazard with a clear trigger** if dispatch
  changes.
- The cost to fix is small (use `shellQuote` or delete the dead
  method).
- F011's prescription is to delete `Execute` from all 24
  handlers. This finding makes that work *security-relevant*
  rather than purely architectural.

If F011's mass deletion lands first, F032 closes for free.

## Suggested fix

### Option A (preferred) — delete the dead legacy code

The `Execute`, `DryRun`, `createFileWithBecome`,
`executeSudoFileOperation`, and `executeSudoCommand` methods in
both files are unreferenced by the executor. F011 already
proposes deleting them. Bundle that work with this finding's
remediation: deletion closes both findings simultaneously.

### Option B (interim) — shell-quote both call sites

In `template/handler.go:340`:

```go
import effects "github.com/alehatsman/mooncake/internal/effects"

// Inside executeSudoFileOperation:
cmd := fmt.Sprintf("mv %s %s && chmod %s %s",
    effects.ShellQuote(tmpPath),
    effects.ShellQuote(destPath),
    h.formatMode(mode),
    effects.ShellQuote(destPath))
```

(Requires `shellQuote` in `internal/effects/default.go:593` to be
exported, or moved to `internal/security`/`internal/shellquote`.)

In `download/handler.go:450`:

```go
cmd := fmt.Sprintf("mv %s %s",
    effects.ShellQuote(tmpPath),
    effects.ShellQuote(dest))
```

Replace `%q` with the exported `ShellQuote` helper.

### Adjacent — F005 cross-cutting helper

This finding overlaps F005 ("sudo -S shell-out helper
cross-package"). When F005's `BecomeRunner.Command(program string,
args ...string)` helper lands, neither `template` nor `download`
needs to shell-construct a `mv … && chmod …` command at all —
they can issue `sudo mv tmp dest` + `sudo chmod 0644 dest` as
two separate exec.Command calls with proper argv. That's the
most defensive shape: no shell involvement, no quoting concerns.

## Verification

After Option A or B:

- Add a test that builds a `step.FileTemplate.Dest =
  "/tmp/test; touch /tmp/PWNED"` and runs `h.Execute(...)` (if
  the method exists) or `h.Run(...)`. Verify `/tmp/PWNED` does
  NOT exist afterwards. Today's `Execute` path on this input
  creates it.
- `grep -rn 'fmt\.Sprintf.*sh' internal/actions/` — no hits on
  shell-command construction outside `effects/default.go`.

## References

- F005 — cross-cutting sudo-helper consolidation; the right
  long-run fix.
- F011 — Spec-16 legacy-method deletion; the right "delete the
  vulnerable code" shape.
- `internal/effects/default.go:593` — `shellQuote` does it
  correctly.
- The `%q` verb is documented in `fmt` as "double-quoted string,
  safely escaped with Go syntax" — explicitly NOT shell-safe.

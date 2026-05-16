# Bug — `fleet exec --timeout` escaped by shell-compound commands

**Surfaced:** 2026-05-15 while continuing the fleet-command test
sweep against the freshly-synced master.

## Repro

```sh
$ time mooncake fleet exec --peer-filter os=linux --timeout 2s 'sleep 30'
[main_pc] ✗ fleet-exec: command failed with exit code -1
[main_pc] ✔ run complete failed: 0/1 changed, 1 failed, 0 skipped (2001ms)
real    0m2.0s   ✓ killed at 2s as expected

$ time mooncake fleet exec --peer-filter os=linux --timeout 2s 'sleep 30; echo done'
[main_pc] ✗ fleet-exec: command failed with exit code -1
[main_pc] ✔ run complete failed: 0/1 changed, 1 failed, 0 skipped (30004ms)
real    0m28.0s ✗ ran the full 30s
```

The kernel marks the step "failed (exit -1)" promptly when the
timeout fires, but the controller's wall-clock proves the underlying
`sleep 30` actually ran to completion. The plan output is misleading
in the wrong direction — operators see "killed at 2s" but the box
spent the full 30s.

## Root cause

The kernel's shell action runs the command via `bash -c '<cmd>'`
(or `powershell -Command` on Windows). When the action's timeout
fires, the kernel calls `cmd.Process.Kill()` which sends SIGKILL to
**just the bash process** — not the process group.

For a single command like `sleep 30`, the immediate process *is*
sleep (bash optimises away the fork+exec when the command is the
last thing in the script). SIGKILL goes straight to sleep, sleep
dies, wall-clock = 2s.

For a compound `sleep 30; echo done`, bash has to fork+exec sleep,
wait for it, then exec echo. SIGKILL goes to bash, bash gets killed
mid-wait, but already-forked sleep keeps running with init as its
new parent. Bash's exit propagates back to the kernel's
`cmd.Wait()` after only a few ms — so the *step* reports `2001ms
failed` — but the orphaned sleep continues for the remaining 28s
of its sleep window.

The same issue affects any "fork-and-wait" shell pattern: subshells
`(...)`, pipelines `a | b`, backgrounded `a &` (even worse:
permanently orphans), command substitution `$(slow)`, etc.

## Fix

Set up a new process group for the spawned shell, then kill the
*group* on timeout. On Linux/macOS this is one `SysProcAttr`
addition plus a `syscall.Kill(-pid, SIGKILL)` on the timeout path:

```go
cmd := exec.Command("bash", "-c", script)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
// ...
// on timeout:
_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
// the negative pid → kill the process group
```

On Windows the equivalent is `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` —
the action's exec_windows.go can put the spawned cmd.exe/powershell
into a Job Object and close it on timeout. Slightly more code but
the same idea.

## Workaround

Avoid shell-compound commands when using `--timeout`. Single
commands work correctly:

```sh
mooncake fleet exec --timeout 5s 'long-running-binary'    # OK
mooncake fleet exec --timeout 5s 'cmd1 && cmd2'           # broken
```

For real plans, prefer multi-step apply with per-step timeouts —
each step is its own shell invocation, so the bug surface is small.

## Test gap

The kernel's shell-action test suite covers the single-command
timeout case (because that's the easy one to write a test for).
The compound-command case is missing. Add a test that runs
`sh -c 'sleep 30; echo done'` with a 1s timeout and asserts
wall-clock ≤ 2s.

## Tracking

Filed as [#16](https://github.com/alehatsman/mooncake/issues/16).

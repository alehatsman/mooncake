---
id: F052
title: cmd/kernel/validate.go calls os.Exit on three paths — hostile to embedded callers, breaks test isolation (F020 shape)
severity: smell
package: cmd/kernel
file: cmd/kernel/validate.go
lines: 58, 79, 98
status: done
verified: 2026-05-27 — three direct `os.Exit` calls on master @ 4db53ad6: `os.Exit(exitCodeRuntimeError)` on config-read failure (line 58) and JSON-encode failure (line 79), `os.Exit(exitCodeValidationError)` on validation-error tail (line 98). The urfave/cli action returns `nil` after the tail-exit so the error never reaches the framework's exit-code handler.
fixed: 2026-05-27 — commit `fa05dbd7 fix(cmd/kernel): F052 — validate.go returns cli.ExitCoder instead of os.Exit`. All three `os.Exit` calls replaced with `cli.Exit(msg, code)` returns. `os.Stdout` reference retained so the `os` import stays. Operator UX (stderr message + exit code) unchanged because urfave/cli reads the `ExitCoder` chain and exits the CLI process with the encoded code.
post-fix verified: 2026-05-27 on master @ fa05dbd7 — new `cmd/kernel/validate_test.go` pins all three paths: `TestValidate_HasErrors_ReturnsValidationExitCode` (validation errors → exitCodeValidationError=2), `TestValidate_InvalidYAML_ReturnsRuntimeExitCode` (parse failure → exitCodeRuntimeError=3), `TestValidate_Clean_ReturnsNil` (success path unchanged). All three would have killed the test binary before the fix because of the `os.Exit` calls. `mooncake task ci` green.
related: F020 (same shape, fixed in `apply.Runner` by routing the exit code through `runWithSignalCtx`)
---

## What

`validateAction` (`cmd/kernel/validate.go:44`) is the
`Action` of the `validate` cli.Command. It calls `os.Exit` from
three separate paths:

```go
// cmd/kernel/validate.go
func validateAction(c *cli.Context) error {
    configPath, err := cmdutil.ResolveConfigPath(c)
    // ...
    _, diagnostics, err := config.ReadConfigWithValidation(configPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
        os.Exit(exitCodeRuntimeError)        // ← (1) line 58
    }
    // ...
    if format == outputFormatJSON {
        // ...
        if err := encoder.Encode(result); err != nil {
            fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
            os.Exit(exitCodeRuntimeError)    // ← (2) line 79
        }
    }
    // ...
    if hasErrors {
        os.Exit(exitCodeValidationError)     // ← (3) line 98
    }
    return nil
}
```

This is the same shape as **F020** — `apply.Runner.installSignalHandler`
calling `os.Exit` and breaking embedded callers (agentd, MCP).
F020 was fixed by routing signal-driven exits through
`cmd/kernel/apply.go:runWithSignalCtx`; the kernel surface no
longer hard-exits.

`validate` doesn't have the F020 signal-race problem (it's
synchronous, no goroutines), but it shares the two underlying
defects:

1. **Hostile to embedded callers.** If MCP / SDK / a future
   `mooncake.Validate(...)` library function wants to wrap the
   validate verb (e.g. the agent loop calling validate before
   apply), the `os.Exit` kills the host process. The kernel
   framing in `docs-working/vision/kernel.md` calls out exactly
   this anti-pattern: cmd/kernel/* should be a thin shim over
   internal/* and never `os.Exit` directly.
2. **Breaks test isolation.** `TestValidate_*` cannot easily
   assert on the validation-failed case without spawning a
   subprocess — `os.Exit` in the test binary kills the whole
   test run. None of the other kernel commands in `cmd/kernel/`
   have this pattern (apply / plan / facts / explain / shell /
   metrics / pilot / actions all `return` errors).

## Why it isn't already fixed

`validate` was wired before the F020 cleanup defined the
"kernel verbs return errors" convention. The three `os.Exit`
calls match the verb's two failure modes (runtime error, validation
error) and the JSON-encode failure (which is itself a runtime
error). The CLI framework already differentiates these via the
exit code of the returned `cli.ExitCoder` — the conversion is
mechanical.

## Fix sketch

Replace the three `os.Exit` calls with `cli.Exit` returns. For
the validation-error tail-exit, the diagnostics are already
printed before the exit, so a silent-message `cli.Exit("", N)` is
appropriate:

```go
if err != nil {
    return cli.Exit(fmt.Sprintf("Error reading config: %v", err), exitCodeRuntimeError)
}
// ...
if err := encoder.Encode(result); err != nil {
    return cli.Exit(fmt.Sprintf("Error encoding JSON: %v", err), exitCodeRuntimeError)
}
// ...
if hasErrors {
    return cli.Exit("", exitCodeValidationError)   // diagnostics already printed above
}
return nil
```

The behavior visible to a CLI operator is identical (same stderr
message, same exit code). The behaviour visible to an embedded
caller changes from "host process dies" to "error returned" —
which is the convention every other kernel verb already follows.

Worth pairing the fix with one test
(`cmd/kernel/validate_test.go::TestValidate_HasErrors_ReturnsValidationExitCode`)
that asserts the returned `cli.ExitCoder` carries
`exitCodeValidationError` — none exists today because the
`os.Exit` pattern made it impossible to write.

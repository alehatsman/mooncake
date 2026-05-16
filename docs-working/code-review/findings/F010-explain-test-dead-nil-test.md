---
id: F010
title: explain: TestDisplayFacts_NilFacts is a dead test — defers panic-recover but never calls anything
severity: smell
package: internal/explain
file: internal/explain/explain_test.go
lines: 1066-1078
status: done
resolved_by: worktree-fix-f010
verified: 2026-05-16 — regression test passes (go test green)
---

## What

```go
func TestDisplayFacts_NilFacts(t *testing.T) {
    // Test behavior with nil Facts pointer
    // This should not panic but may produce minimal output
    defer func() {
        if r := recover(); r != nil {
            t.Errorf("DisplayFacts should not panic with nil pointer, got: %v", r)
        }
    }()

    // This will panic due to nil pointer dereference, which is expected
    // The function doesn't handle nil input, so we just verify it panics predictably
    // In production, callers should never pass nil
}
```

The test body has a `defer recover()` that errors if a panic
happens — and then **never actually calls anything**. The
function body is the recover defer plus a multi-line comment.
The test always passes vacuously.

Comment says **"should not panic"** and **"this will panic"** in
the same block, then **"we just verify it panics predictably"**
but doesn't verify anything because no call is made.

## Why it's a smell (not a bug)

It doesn't break the build. But it carries a false signal:
`TestDisplayFacts_NilFacts PASS` reads as "we cover the nil
case." The reader is misled.

It's also evidence of unresolved design intent: do we want
`DisplayFacts(nil)` to panic, or to print an empty card, or
to return early with a log line? The test says all three at once.

## Suggested fix

Pick a behavior, document it, write the matching test:

**Option A — panic is the policy:**

```go
func TestDisplayFacts_NilFacts(t *testing.T) {
    defer func() {
        if r := recover(); r == nil {
            t.Error("DisplayFacts(nil) should panic — callers must never pass nil")
        }
    }()
    DisplayFacts(nil)
}
```

Pair with a `// DisplayFacts panics on nil; this is the documented
contract.` doc-comment on the function.

**Option B — defensive nil-check:**

```go
// In explain.go:
func DisplayFacts(f *facts.Facts) {
    if f == nil {
        fmt.Println("(no facts collected)")
        return
    }
    // ... rest
}

// In the test:
func TestDisplayFacts_NilFacts(t *testing.T) {
    output := captureOutput(func() { DisplayFacts(nil) })
    if !strings.Contains(output, "no facts") {
        t.Error("nil Facts should produce empty-state message")
    }
}
```

Option A is the lighter weight: `DisplayFacts` is called from
exactly one place (the `mooncake doctor` / explain command, in
`cmd/`), and the caller has the facts object. There's no scenario
where nil is reachable. Documenting it as panic-on-nil saves a
nil check on every field access for everyone's mental model.

## Verification

- `go test -run TestDisplayFacts_NilFacts ./internal/explain/...`
- Manual: stare at the test, confirm it now exercises a path.

## References

- F009 (parent — DisplayFacts split + adjacent issues). F010 is
  pulled out separately because the fix here is a behavior
  decision, not a refactor.

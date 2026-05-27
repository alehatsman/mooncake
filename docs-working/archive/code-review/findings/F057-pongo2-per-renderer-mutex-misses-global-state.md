---
id: F057
title: template.Pongo2Renderer's per-renderer mutex doesn't guard pongo2's global TemplateSet — concurrent renderers from different goroutines race on shared state
severity: smell
package: internal/template
file: internal/template/renderer.go
lines: 406-408
status: open
discovered: 2026-05-27 — cold-read of internal/template/. `Pongo2Renderer.Render` (line 390) acquires the receiver's `r.mu` before calling `pongo2.FromString`. The acquisition comment makes the threat explicit:

```go
// internal/template/renderer.go:404-408
// Lock to prevent race condition in pongo2.FromString
// The pongo2 TemplateSet is not thread-safe for concurrent FromString calls
r.mu.Lock()
pongoTemplate, err := pongo2.FromString(template)
r.mu.Unlock()
```

The author's mental model: serialize FromString within one renderer. The bug: `pongo2.FromString` reads from the package-level default TemplateSet (`pongo2.DefaultSet`), which is shared across **every** Pongo2Renderer instance in the same process. A per-receiver mutex doesn't serialize accesses from a different receiver. Two renderers calling Render concurrently can both enter their own `r.mu.Lock()` simultaneously and race inside `pongo2.FromString`.

The render package constructs renderers at four production sites today:

```
$ grep -rn "NewPongo2Renderer" --include="*.go" | grep -v _test.go
cmd/step/step.go:104           # one-shot CLI: single renderer
internal/plan/planner.go:136   # planner: one renderer per BuildPlan
internal/apply/reverse_context.go:41  # rollback: one per Reverse() chain
internal/executor/executor.go:989,1178  # apply: two per Start (sequential)
```

Today **none of these run concurrently** — `executor.Start` is the long-lived call, agentd serializes runs through a single Worker goroutine (`internal/agentd/worker.go:14` comment: "single-goroutine FIFO runner"), and the F016 stage-1(a) cancellation model assumes one in-flight apply per worker. So the race is **latent**, not actively reproducible.

The risk surface is "anyone who later adds parallelism." Examples that would activate the race today if introduced:

- agentd grows a multi-worker pool to absorb queue depth.
- `mooncake fleet apply` switches from "fan out to peers" (each peer is a separate process; no shared pongo2) to "fan out to in-process workers" (shared pongo2).
- A future MCP tool that exposes `render_template` to LLM clients accepts concurrent requests.
- Test helpers that build multiple plans in parallel for table-driven coverage.

Each of these is a plausible 6-month-out direction. The race is invisible until then; once activated, it surfaces as nondeterministic template errors with no clear correlation to the parallelism that caused them.
related: F018 (`bufio.Scanner` global-buffer race shape — same "shared mutable state, per-receiver mutex doesn't help" pattern). F012 family of "anti-patterns the project's discipline calls out elsewhere but missed in one spot."
---

## What

`pongo2.FromString` (in `github.com/flosch/pongo2/v6/template_loader.go`)
delegates to the package-level `DefaultSet.FromString`, mutating
DefaultSet's `tplCache` and `bannedTags`/`bannedFilters` maps during
parse. Two goroutines calling `FromString` concurrently — from
distinct `Pongo2Renderer` instances — write to those maps without
synchronization. Outcomes range from spurious "template not found"
errors to map-write panics (which Go's runtime escalates to fatal
errors via `concurrent map writes` detection).

The `r.mu` mutex is per-receiver. Holding it in
`(*Pongo2Renderer).Render` serializes calls THROUGH ONE
`*Pongo2Renderer`. It does not serialize calls through two distinct
`*Pongo2Renderer` instances against pongo2's shared package-level
state.

## Why "smell" not "risk"

No production call graph reaches `pongo2.FromString` from two
goroutines today. The worker model documents this explicitly
(`internal/agentd/worker.go:15-17`):

> v1 makes no attempt at concurrency: concurrent applies of the same
> plan or of different plans touching the same paths/services can
> clobber each other.

Until parallelism is introduced, the bug is unreachable. But the
mutex's mere presence is misleading — it implies the renderer is
thread-safe, which is false at the global-state level. A
maintainer adding parallelism would look at the lock, conclude
"already handled," and ship a race.

## Proposed fix

Two options:

**(A) Promote the mutex to package-level.** Replace `r.mu` with a
`var fromStringMu sync.Mutex` shared across all renderers. Tiny
change, preserves the existing per-call serialization,
correctly guards the global state:

```go
// internal/template/renderer.go
var fromStringMu sync.Mutex // guards pongo2.DefaultSet (package-global)

func (r *Pongo2Renderer) Render(template string, ...) (string, error) {
    // ... autoJSONNonScalars ...
    fromStringMu.Lock()
    pongoTemplate, err := pongo2.FromString(template)
    fromStringMu.Unlock()
    // ...
}
```

Drop the unused `mu` field on `Pongo2Renderer` or keep it for any
future per-renderer state (e.g. a custom TemplateSet — see option B).

**(B) Give each renderer its own pongo2 TemplateSet.** pongo2's
public API supports per-instance sets:

```go
type Pongo2Renderer struct {
    set *pongo2.TemplateSet
}

func NewPongo2Renderer() (*Pongo2Renderer, error) {
    return &Pongo2Renderer{set: pongo2.NewSet("mooncake", pongo2.MustNewLocalFileSystemLoader(""))}, nil
}

func (r *Pongo2Renderer) Render(t string, vars ...) (string, error) {
    tpl, err := r.set.FromString(t)
    // ...
}
```

This is the cleaner long-term fix — different renderers' template
caches don't pollute each other, and filter registration becomes
per-set instead of global. But the conversion is bigger: every
`pongo2.RegisterFilter` call (lines 68/77/80/89) becomes
`set.Filters().Register(...)` and the `sync.Once` filter-registration
gate needs revisiting.

**Recommendation: ship (A) now** (one-line fix, closes the race,
ready to merge as soon as a parallel-render use case appears).
Track (B) as a follow-up when the project actually has multiple
TemplateSets to manage.

## Pre-fix smoke test (when the race becomes reachable)

```go
// internal/template/renderer_race_test.go
func TestPongo2RendererConcurrentFromString(t *testing.T) {
    r1, _ := NewPongo2Renderer()
    r2, _ := NewPongo2Renderer()
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(2)
        go func() { defer wg.Done(); _, _ = r1.Render(`{{ x }}`, map[string]interface{}{"x": "a"}) }()
        go func() { defer wg.Done(); _, _ = r2.Render(`{{ y }}`, map[string]interface{}{"y": "b"}) }()
    }
    wg.Wait()
}
```

Run with `-race`. Pre-fix: data race in pongo2's internal maps (or a
fatal `concurrent map writes` panic). Post-fix (option A or B):
clean.

The test would have to be guarded behind a build tag or `-race`-only
flag — the race is invisible without the detector, and the test
otherwise just looks like a noisy benchmark.

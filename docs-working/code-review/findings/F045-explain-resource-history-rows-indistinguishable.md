---
id: F045
title: `explain <resource>` history rows are visually indistinguishable when one run touches the same resource multiple times
severity: smell
package: internal/explain
file: internal/explain/resolve_w2.go, internal/explain/explain.go, cmd/mooncake.go
lines: resolve_w2.go:68-83, explain.go:210-218, mooncake.go:556-571
status: fixed
---

## What

When a single apply touches the same resource handle in more than
one step, `mooncake explain <resource>` emits a row per step but
with no per-step disambiguation — same timestamp (the *run* TS,
not the step TS), same `run_id`, same `action`, same `result`.

Repro:

```yaml
# multi.yml — three writes to the same path in one apply
- name: write 1
  file.write: {path: /tmp/multi-target.txt, content: "first"}
- name: write 2
  file.write: {path: /tmp/multi-target.txt, content: "second"}
- name: write 3
  file.write: {path: /tmp/multi-target.txt, content: "third"}
```

```
$ mooncake apply -c multi.yml
RECAP  ok=0  changed=3  skipped=0  failed=0

$ mooncake explain file:/tmp/multi-target.txt
resource: file:/tmp/multi-target.txt

history (newest first):
  2026-05-16T21:59:27Z  file.write           changed  run=r/01KRSCT9A265Q1ZVFX1PVK63XM (reversible)
  2026-05-16T21:59:27Z  file.write           changed  run=r/01KRSCT9A265Q1ZVFX1PVK63XM (reversible)
  2026-05-16T21:59:27Z  file.write           changed  run=r/01KRSCT9A265Q1ZVFX1PVK63XM (reversible)
```

Three identical lines. The reader can count them and infer "three
touches in one run", but can't tell which one came first or what
each step was named.

## Why it's worth fixing

This pattern (multiple writes to one path in one apply) shows up
in three real flows:

1. **Templated paths that collide** — `path: /etc/foo.conf` rendered
   the same way for two `with_items` iterations.
2. **Refactors mid-run** — a chmod step after a write step. Same
   resource handle (`file:/path`), two distinct actions in flight,
   but if both are `file.write` the rows look identical.
3. **Rollback storyboarding** — when explain becomes a "what
   happened to this file?" tool, the operator's first instinct is
   to walk the history rows and tie each back to a named step.
   Identical rows make that impossible without going to
   `mooncake history show` and cross-referencing.

The underlying data is already there. `runlog.Entry.Steps[]` carries
`Index` (1-based) and the framework knows each step's `Name` — but
the explain resolver drops both on the floor.

## Where the data is dropped

`internal/explain/resolve_w2.go:68-83`:

```go
var history []ResourceEvent
for _, e := range entries {
    for _, s := range e.Steps {
        if s.Resource != noun {
            continue
        }
        history = append(history, ResourceEvent{
            RunID:      e.RunID,
            OpID:       e.OpID,
            TS:         e.TS,           // run TS, not step TS — every row in
                                        // the same run gets the same value
            Action:     s.Action,
            Result:     s.Result,
            Reversible: s.Reversible,
            // s.Index dropped here
            // step name not present in runlog.Entry.Steps[] today
        })
    }
}
```

`internal/explain/explain.go:210-218` (`ResourceEvent`) has no
`Index` or `Name` field, so the wire shape can't carry it even if
the resolver started passing it.

`cmd/mooncake.go:556-571` (`renderExplainResourceText`) emits
`TS  action  result  run=ID` with no slot for step ordering.

## Suggested fix (smallest viable change)

Two parts: widen the wire shape with `Index`, then surface it in
text.

1. Extend `ResourceEvent`:

   ```go
   type ResourceEvent struct {
       RunID      string    `json:"run_id"                 yaml:"run_id"`
       OpID       string    `json:"op_id,omitempty"        yaml:"op_id,omitempty"`
       TS         time.Time `json:"ts"                     yaml:"ts"`
       Index      int       `json:"step_index,omitempty"   yaml:"step_index,omitempty"`
       Action     string    `json:"action"                 yaml:"action"`
       Result     string    `json:"result"                 yaml:"result"`
       Reversible bool      `json:"reversible,omitempty"   yaml:"reversible,omitempty"`
   }
   ```

2. Populate it in `resolveResource`:

   ```go
   history = append(history, ResourceEvent{
       RunID:      e.RunID,
       OpID:       e.OpID,
       TS:         e.TS,
       Index:      s.Index,
       Action:     s.Action,
       Result:     s.Result,
       Reversible: s.Reversible,
   })
   ```

3. Render it in `renderExplainResourceText`:

   ```go
   for _, h := range p.History {
       rev := ""
       if h.Reversible {
           rev = " (reversible)"
       }
       step := ""
       if h.Index > 0 {
           step = fmt.Sprintf(" step=%d", h.Index)
       }
       fmt.Fprintf(w, "  %s  %-20s %-7s  run=%s%s%s\n",
           h.TS.Format(time.RFC3339), h.Action, h.Result, h.RunID, step, rev)
   }
   ```

Output after the fix:

```
history (newest first):
  2026-05-16T21:59:27Z  file.write           changed  run=r/01KRSCT9A2... step=3 (reversible)
  2026-05-16T21:59:27Z  file.write           changed  run=r/01KRSCT9A2... step=2 (reversible)
  2026-05-16T21:59:27Z  file.write           changed  run=r/01KRSCT9A2... step=1 (reversible)
```

The omitempty in JSON/YAML means pre-spec-68 runs (without `Index`)
keep round-tripping cleanly.

## Out of scope, but worth a separate ticket

- **Per-step TS.** The current wire shape carries the *run* TS, not
  the *step* TS. For single-step-per-run resources this doesn't
  matter; for multi-step the index is sufficient ordering. A real
  per-step TS would need plumbing through `runlog.Entry.Steps[]`
  → `apply.writeEnrichedRunlog`.
- **Step name.** `runlog.Entry.Steps[]` doesn't carry step `Name`
  today — only `Index`, `Action`, `Resource`, `Result`,
  `DurationMs`, `Reversible`. Adding `Name` would make the text
  output much more readable (`step=3 "write config tail"`), but
  it's a runlog wire change and deserves its own finding /
  spec amendment.
- **History pagination.** A long-lived resource (months of
  applies) will produce many rows. `mooncake explain <resource>`
  has no `--limit` flag today. Out of scope here; flagged for a
  follow-up.

## Verification

Unit test in `internal/explain/resolve_test.go` (or wherever the
w2 resolver tests live):

```go
func TestResolveResource_PreservesStepIndex(t *testing.T) {
    // Synthesize a runlog with two steps on the same resource,
    // call resolveResource, assert history[0].Index == 2 and
    // history[1].Index == 1 (newest-first).
}
```

Manual: repeat the multi.yml repro above against the fixed binary
and confirm step= shows up in each row.

## References

- F045 (this finding) — surfaced during the 2026-05-17 tester pass
  against master @ 24201c00.
- Spec-68 §"The noun set" §3 (resource) — defines the
  history-of-touches contract; doesn't speak to per-step
  disambiguation, so adding `Index` is a compatible extension.
- `internal/runlog/runlog.go` — owns `Entry.Steps[].Index`;
  source of truth for the value we're forwarding.

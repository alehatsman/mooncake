# Spec 08: Run History — Log and Last-Run Summary

**Epic:** E5 Run History & Audit (S5.1 + S5.2)  
**Effort:** S (2–4h)  
**Value:** High — agents and users can check what happened last without re-running

---

## Problem

There is no persistent record of runs. After a run completes:
- You can't ask "did it succeed last time?"
- You can't check what changed
- An AI agent picking up a session has no baseline

---

## Goal

Every run appends a compact record to `~/.mooncake/runs.jsonl`. A new `mooncake last`
command prints the most recent run's recap.

---

## Run Log Format

`~/.mooncake/runs.jsonl` — append-only, one JSON object per line:

```jsonl
{"ts":"2026-05-12T10:30:00Z","config":"main.yml","changed":12,"ok":61,"skipped":8,"failed":0,"duration_ms":274000}
{"ts":"2026-05-12T14:22:10Z","config":"arch.yml","changed":0,"ok=74","skipped":3,"failed":0,"duration_ms":18200}
```

Fields:
- `ts` — RFC3339 UTC timestamp
- `config` — basename of the config file that was run
- `changed`, `ok`, `skipped`, `failed` — step counts
- `duration_ms` — total wall time

---

## `mooncake last` Command

Prints a human-readable summary of the most recent run:

```
Last run: 2026-05-12 10:30 UTC  (config: main.yml)
  changed=12  ok=61  skipped=8  failed=0  duration=4m34s
```

If no log exists:
```
No run history found (~/.mooncake/runs.jsonl)
```

`--format json` flag returns the raw last-run JSON object.

---

## Implementation

### `internal/runlog/runlog.go` (new package)

```go
package runlog

type Entry struct {
    TS         time.Time `json:"ts"`
    Config     string    `json:"config"`
    Changed    int       `json:"changed"`
    Ok         int       `json:"ok"`
    Skipped    int       `json:"skipped"`
    Failed     int       `json:"failed"`
    DurationMs int64     `json:"duration_ms"`
}

// Append writes one entry to ~/.mooncake/runs.jsonl, creating the file if needed.
func Append(e Entry) error

// Last returns the most recent entry, or ErrNoHistory if the log is empty/missing.
var ErrNoHistory = errors.New("no run history")
func Last() (Entry, error)
```

`Append` creates `~/.mooncake/` if it doesn't exist. Uses `os.OpenFile` with
`O_APPEND|O_CREATE|O_WRONLY`.

### Write on run completion

In `cmd/mooncake.go` (or a new `RunHistorySubscriber`), after `EventRunCompleted`,
call `runlog.Append(...)`. The event already carries all needed fields.

A dedicated subscriber is cleaner than coupling it to cmd:

`internal/logger/runlog_subscriber.go`

```go
type RunLogSubscriber struct {
    config string // config file basename
}

func (r *RunLogSubscriber) Handle(e events.Event) {
    rc, ok := e.(events.EventRunCompleted)
    if !ok {
        return
    }
    _ = runlog.Append(runlog.Entry{
        TS:         time.Now().UTC(),
        Config:     r.config,
        Changed:    rc.Changed,
        Ok:         rc.Ok,
        Skipped:    rc.Skipped,
        Failed:     rc.Failed,
        DurationMs: rc.DurationMs,
    })
}
```

### `cmd/last.go` (new file)

```go
func lastCommand() *cli.Command {
    return &cli.Command{
        Name:  "last",
        Usage: "print the most recent run summary",
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
        },
        Action: func(ctx *cli.Context) error {
            entry, err := runlog.Last()
            if errors.Is(err, runlog.ErrNoHistory) {
                fmt.Println("No run history found (~/.mooncake/runs.jsonl)")
                return nil
            }
            if err != nil {
                return err
            }
            if ctx.String("format") == "json" {
                enc := json.NewEncoder(os.Stdout)
                enc.SetIndent("", "  ")
                return enc.Encode(entry)
            }
            // text format
            fmt.Printf("Last run: %s  (config: %s)\n",
                entry.TS.Format("2006-01-02 15:04 UTC"), entry.Config)
            fmt.Printf("  changed=%d  ok=%d  skipped=%d  failed=%d  duration=%s\n",
                entry.Changed, entry.Ok, entry.Skipped, entry.Failed,
                fmtDuration(entry.DurationMs))
            return nil
        },
    }
}
```

Register in `cmd/mooncake.go` Commands list.

---

## Acceptance Criteria

1. After `mooncake run`, a new line is appended to `~/.mooncake/runs.jsonl`.
2. `mooncake last` prints a human-readable summary of the most recent run.
3. `mooncake last --format json` prints the raw JSON entry.
4. `mooncake last` when no log exists prints a clear message and exits 0.
5. `~/.mooncake/` directory is created automatically if missing.
6. Append is best-effort: a write failure does not fail the run itself.

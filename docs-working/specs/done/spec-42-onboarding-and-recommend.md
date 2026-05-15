# Spec 42: Examples Index, `mooncake history`, and `presets recommend`

**Status:** Draft
**Epic:** Stream 4 — Developer Experience (closes the second wave)
**Effort:** M (2–3 days)
**Value:** Medium. Polishes the post-init UX: helps new users find the
right next thing to read (examples), the right next thing to install
(presets), and review what they've already run (history).

**Background:** `docs-working/analysis/dx-audit-2026-05.md` recommendations
R6, R7, R9. Closes the audit's remaining recommendations.

**Depends on:** wave 1 (specs 39/40/41) already shipped.

---

## Problem

After `mooncake init` lands a user with a runnable project, three friction
points remain:

1. **Examples directory is unindexed.** 30 entries with mixed conventions
   (folders vs loose `*-example.yml`), no curated path. Users either
   read randomly or bounce back to the website.

2. **`mooncake last` is a one-shot peephole.** It prints the most recent
   run, but there's no way to see the run *before* that, or query a
   specific run by anything. Power users learn to `cat ~/.mooncake/
   runs.jsonl | jq ...`; everyone else is stuck on "last".

3. **330+ presets are invisible.** `presets search` only works after
   `presets update`. `presets list` dumps everything alphabetically.
   Nothing answers "what should *I* install?"

---

## Goals

- **G1** Ship `examples/README.md` with a curated 5–7 step learning path.
  Linked from the main README and from each example's local README.
- **G2** Promote `mooncake last` to `mooncake history` with subcommands:
  `history` (bare, prints most recent — preserves the `last` behaviour),
  `history list [--limit N]`, `history show <index>`.
- **G3** Add `mooncake presets recommend` — read facts, match a curated
  table of "presets that make sense for this profile", print 5–10
  candidates.
- **G4** No new external dependencies.

**Non-goals:**

- No run-ID schema change. `history show <index>` uses 1-based
  newest-first indexing (1 = most recent). Adding stable run IDs is a
  separate, larger change.
- No marketplace integration in `presets recommend`. The recommendation
  table is hand-curated and embedded; remote-registry expansion is future
  work.
- No interactive UI (TUI) for any of these. Plain text + JSON only.
- No renaming `last` to `history` *and* keeping `last` as an alias. Break
  cleanly (per project memory: no backwards compat).

---

## Deliverables

### G1 — `examples/README.md`

A short curated index ordered by learning curve, not alphabetically:

```markdown
# Mooncake Examples

A short, ordered path from "hello world" to real workflows.

1. [hello-world/](hello-world/)            — shell + global facts
2. [variables-and-facts/](variables-and-facts/) — custom vars + system facts
3. [conditionals/](conditionals/)           — `when:` expressions
4. [files-and-directories/](files-and-directories/) — file.write + state
5. [loops/](loops/)                          — with_items, with_filetree
6. [templates/](templates/)                  — Jinja2-style rendering
7. [real-world/](real-world/)                — dotfiles, dev box scenarios

Specialised examples (browse when relevant):
- containers/, macos-services/, ollama/, sudo/, register/, tags/
- artifact-*-example.yml, file-*-example.yml, repo-apply-patchset-example.yml

For ready-made workflows, see `mooncake presets list` (330+ presets).
```

Update `examples/hello-world/README.md` to link to this index in its
"Next steps" section.

### G2 — `mooncake history`

Replace `cmd/last.go` with `cmd/history.go`. Three subcommands:

```bash
# bare: most recent run — exact replacement for `mooncake last`
$ mooncake history
Last run: 2026-05-14 15:30 UTC  (config: mooncake.yml)
  changed=2  ok=5  skipped=0  failed=0  duration=0.3s

# list: recent N (default 10, oldest first so newest is bottom-of-screen)
$ mooncake history list --limit 5
#5  2026-05-12 09:10 UTC  config=mooncake.yml  changed=0  ok=7  failed=0  0.2s
#4  2026-05-13 11:22 UTC  config=mooncake.yml  changed=3  ok=4  failed=0  4.1s
#3  2026-05-13 11:34 UTC  config=mooncake.yml  changed=0  ok=7  failed=0  0.2s
#2  2026-05-14 15:29 UTC  config=mooncake.yml  changed=2  ok=5  failed=0  0.3s
#1  2026-05-14 15:30 UTC  config=mooncake.yml  changed=2  ok=5  failed=0  0.3s

# show: full entry for a specific index (1 = newest)
$ mooncake history show 2
Run #2 of 47
  timestamp: 2026-05-14 15:29:18 UTC
  config:    mooncake.yml
  changed=2  ok=5  skipped=0  failed=0
  duration:  0.3s
```

All three honour `--format json` for tooling.

**Index semantics:** 1-based, newest-first. `history show 1` is identical
to bare `history`. `history show 0` and `history show 999999` error
clearly. Indexes are stable *only for the run that's reading them* — a
new run shifts indexes by one. Acceptable: users who want stable
identifiers can pass `--format json` and read the `ts` field.

**Backwards compatibility:** `mooncake last` is removed. Per memory
`feedback_no_backwards_compat`, no shim.

### G3 — `mooncake presets recommend`

```bash
$ mooncake presets recommend
Detected: linux/amd64  arch  pacman

Recommended presets for your profile:
  zsh             Shell + plugins; widely useful on Linux/macOS
  neovim          Modern Vim, configured
  tmux            Terminal multiplexer
  fzf             Fuzzy finder — speeds up `mooncake presets`
  docker          Container runtime (pacman: docker)
  ripgrep         Fast grep replacement

Install with:  mooncake presets info <name>
                mooncake presets install <name>
```

Implementation: a single embedded YAML/Go map of `(profile-key → preset
list)`. Profile keys derived from facts:

```
linux + apt        → apt-flavoured set
linux + pacman     → pacman-flavoured set
linux + dnf        → fedora set
darwin             → homebrew set
windows            → windows set
```

Plus a small "always useful" base list (e.g. `git`, `tmux`).

Output is filtered against the locally-known preset list (from
`internal/presets.PresetSearchPaths`) so we never recommend a preset
the user can't install. A preset in the table but missing locally is
silently dropped with a debug log.

`--format json` for tooling. `--limit N` to cap output (default 8).

---

## Design

### Package layout

```
cmd/history.go            # new; replaces cmd/last.go
internal/runlog/
├── runlog.go             # existing; add Recent(n) and At(index)
└── runlog_test.go        # extended
internal/recommend/
├── recommend.go          # Recommend(facts) → []string
├── recommend_test.go
└── catalogue.go          # embedded curated table
cmd/presets.go            # adds `recommend` subcommand
examples/README.md        # new
```

### `internal/runlog/` extensions

Add two functions to the existing package:

```go
// Recent returns up to n most-recent entries, ordered newest-first.
// A missing or empty log returns nil, ErrNoHistory.
func Recent(n int) ([]Entry, error)

// At returns the entry at 1-based newest-first index. Returns
// ErrIndexOutOfRange for index < 1 or > total entries.
func At(index int) (Entry, error)

var ErrIndexOutOfRange = errors.New("history index out of range")
```

Implementation reads the JSONL file once into a slice; doctor-sized
budget (no large datasets in `runs.jsonl` in practice — a daily user
accumulates maybe 10k entries over years and the file is line-oriented).

### `cmd/history.go`

```go
func historyCommand() *cli.Command {
    return &cli.Command{
        Name:  "history",
        Usage: "Inspect past mooncake runs",
        Action: historyDefaultAction,  // mirrors `last` behaviour
        Subcommands: []*cli.Command{
            {Name: "list",  Action: historyListAction,  Flags: []cli.Flag{
                &cli.IntFlag{Name: "limit", Value: 10},
                &cli.StringFlag{Name: "format", Value: "text"},
            }},
            {Name: "show",  Action: historyShowAction,  Flags: []cli.Flag{
                &cli.StringFlag{Name: "format", Value: "text"},
            }, ArgsUsage: "<index>"},
        },
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "format", Value: "text"},
        },
    }
}
```

Delete `cmd/last.go`. Update `createApp()` to register `historyCommand`
instead of `lastCommand`. Update the test in `cmd_test.go` that asserts
the command list.

### `internal/recommend/`

```go
// Profile is the minimal subset of facts used to pick a recommendation
// list. Kept as a struct (not a string key) so we can extend without
// breaking the catalogue's matching code.
type Profile struct {
    OS             string // "linux", "darwin", "windows"
    PackageManager string // "apt", "pacman", "dnf", "brew", ...
}

func ProfileFrom(f *facts.Facts) Profile { … }

// Recommend returns a deduplicated list of preset names that are both
// (a) in the curated catalogue for this Profile, and (b) discoverable
// in the local preset search paths. Order is curated, not alphabetical.
func Recommend(p Profile, knownPresets map[string]bool, limit int) []string
```

The catalogue lives in `catalogue.go` as a Go map:

```go
var catalogue = []entry{
    // base: useful on every profile
    {name: "git",     base: true},
    {name: "tmux",    base: true},
    {name: "fzf",     base: true},
    {name: "ripgrep", base: true},
    {name: "zsh",     base: true},
    {name: "neovim",  base: true},

    // linux + apt
    {name: "docker",  profile: Profile{OS: "linux", PackageManager: "apt"}},
    // linux + pacman
    {name: "docker",  profile: Profile{OS: "linux", PackageManager: "pacman"}},
    // ...
}
```

A profile-aware match function returns base entries first, then
profile-specific entries, deduplicated.

### `presets recommend` action

```go
func recommendPresetsAction(c *cli.Context) error {
    f := facts.Collect()
    p := recommend.ProfileFrom(f)
    known := loadKnownPresetNames() // from presets.PresetSearchPaths
    names := recommend.Recommend(p, known, c.Int("limit"))

    if c.String("format") == "json" {
        return json.NewEncoder(os.Stdout).Encode(map[string]any{
            "profile":     p,
            "recommended": names,
        })
    }
    printRecommendText(os.Stdout, p, names)
    return nil
}
```

---

## Acceptance criteria

### G1 (examples index)
1. `examples/README.md` exists, lists 5–7 curated examples in order, and
   links each to its directory.
2. Main `README.md`'s "Local Examples" section links to
   `examples/README.md`.
3. `examples/hello-world/README.md` "Next steps" links to
   `examples/README.md`.

### G2 (history)
4. `mooncake history` (no args) produces the same output as the
   previous `mooncake last`.
5. `mooncake history list` prints recent runs newest-at-bottom by
   default (10 lines), oldest-first within the window.
6. `mooncake history list --limit 3` caps to 3 entries.
7. `mooncake history show 1` matches bare `history` output.
8. `mooncake history show 2` prints the second-most-recent entry.
9. `mooncake history show 0` and `show 99999` print a clear error and
   exit non-zero.
10. `mooncake history` with no run log yet prints the same "no history"
    message the old `last` did and exits 0.
11. `mooncake last` is no longer a registered command (removed cleanly).
12. All three subcommands honour `--format json`.

### G3 (presets recommend)
13. `mooncake presets recommend` returns at least one entry on a host
    with default presets installed.
14. `--limit 3` caps the output.
15. `--format json` produces well-formed JSON with `profile` and
    `recommended` fields.
16. A preset that's in the catalogue but missing from local search
    paths is silently dropped from the output.
17. On a host with no facts (theoretical — shouldn't happen with
    `facts.Collect`), `recommend` falls back to the base list only.

### Cross-cutting
18. All packages pass `go test -race`.
19. Build remains clean (`go build ./...` produces no warnings beyond
    pre-existing style nits).

---

## Open questions

1. **Should `history show` accept a timestamp instead of/in addition to
   an index?** `mooncake history show 2026-05-14T15:30:00Z`. More
   stable, but matches a substring is ambiguous. Punt to v2; for now,
   1-based index is enough.
2. **Should `presets recommend` filter by what's already installed?**
   E.g. don't recommend `git` if `git --version` succeeds. Tempting,
   but adds N shell-outs to a fast command. Lean no.
3. **How should the recommendation catalogue stay fresh?** It's
   embedded in the binary today. Once the preset marketplace exists,
   move it to a fetched-and-cached YAML (same channel as
   `presets update`).
4. **Should we add a `--profile <name>` override to `recommend`?**
   Useful for testing and CI; defer.
5. **What about Windows for `recommend`?** Stub returns base-only.
   Acceptable for v1.

---

## Dependencies / sequencing

- **None upstream.** Lands standalone on top of wave 1.
- **G1 (docs)** can ship as a tiny separate PR if useful.
- **G2 (history)** is breaking for any scripts that call `mooncake last`.
  Per project policy: acceptable, no shim.
- **G3 (recommend)** is purely additive; new subcommand, no behavioural
  change to anything else.
- **Future spec**: `mooncake history --since <duration>` and stable
  run IDs once we have a use case for them.

# Proposal 03: `mooncake watch` — hot-reload iteration loop

**Status:** Draft proposal
**Effort:** S (~2–3 days, single PR)
**Value:** High for active development; transforms the
edit→test cycle from ~5–10s to ~1–2s.

---

## Problem

The current iteration loop for a YAML author looks like this:

```
$ vim mooncake.yml          # edit
$ mooncake apply -c mooncake.yml   # ~1s test playbook startup
$ vim mooncake.yml          # tweak
$ mooncake apply -c mooncake.yml   # again
$ vim mooncake.yml
$ mooncake apply -c mooncake.yml
```

Two friction points compound:
1. **Repeated startup cost** — facts collection, schema load, plan
   compile (~1s per invocation, even for a 3-step playbook)
2. **Manual trigger** — user has to context-switch from editor to
   terminal every iteration

During the 2026-05-15 audit I ran `mooncake apply` ~250 times across
59 iterations. A watcher would have saved ~10–15 minutes (each
invocation was ~3–5s wall including docker setup, ~250 × ~3s = 12
min). For a project author iterating on a 50-step site.yml, the
savings compound.

This is the gap between "config management tool" and "developer
tool". `tsc --watch`, `cargo watch`, `pytest --looponfail` —
every other ecosystem has this.

## Proposal

A new subcommand:

```
$ mooncake watch
```

Behavior:
1. Resolves the same config file as `mooncake apply` (default
   discovery: `./mooncake.yml` → `./mooncake/main.yml`)
2. Inspects the plan's `input_files` (root + every `import:`)
3. Runs `mooncake apply --dry-run` once on startup, prints the plan
4. Watches every input_file for changes (fsnotify)
5. On change:
   - Re-parse the plan
   - If valid: print the diff vs. the previous plan
   - Wait for user confirmation? Or auto-apply?
6. Trap Ctrl-C cleanly (also fixes #87 for this surface)

Two flavors (one flag):

```bash
mooncake watch                # plan-on-change (default; safe)
mooncake watch --apply        # apply-on-change (live)
```

`--apply` is the "live" mode every interactive tool offers but
config management has historically refused (writing to production
on save = scary). With `--dry-run`-on-change as default, the
keystroke-to-feedback is fast AND nobody nukes a server.

## Output sketch

Initial run:
```
$ mooncake watch
Watching: ./mooncake.yml (3 input files)
Press 'a' to apply current plan, 'r' to re-plan, 'q' to quit, Ctrl-C to exit.

Plan: ./mooncake.yml
↑ install jq           would install
↑ write config         would write file (78 bytes)
PLAN SUMMARY  would-change=2  ok=0
```

After saving the file (jq removed):
```
~ change detected: ./mooncake.yml (12:34:56)
- 1 step removed:
  ↑ install jq           would install
+ 0 steps added
PLAN SUMMARY  would-change=1  ok=0
```

On `a` (apply):
```
Applying...
~ write config
RECAP  ok=0  changed=1  skipped=0  failed=0  150ms
```

## API

| Subcommand | Behavior |
|---|---|
| `mooncake watch` | Plan on change, print diff. No mutation. |
| `mooncake watch --apply` | Apply on change. |
| `mooncake watch --apply --debounce 500ms` | Coalesce rapid saves. |
| `mooncake watch -c <path>` | Specific config file (same as apply). |
| `mooncake watch --tags X` | Filter steps. |

Hot-keys in the watch session (no flag needed):
- `a` — apply current plan now
- `r` — re-plan (force, even if no file change)
- `d` — toggle diff view on/off
- `q` / Ctrl-C — quit cleanly

## Receipts

In my 59-iteration audit, the pattern was identical every time:
1. Write YAML
2. `docker run --rm -v ... mooncake apply -c /work/cfg.yml` (~3s)
3. Read output
4. Edit YAML
5. Repeat

If `mooncake watch` had existed, my workflow would have been:
1. `docker run -d ... mooncake watch -c /work/cfg.yml --apply`
2. Edit YAML in real time
3. Hot reload on save
4. Move on

A handful of findings (#80 text.patch broken hunks, #84 text.insert
idempotency) needed me to iterate 5-10 times on the same playbook
to characterize. That's where watch mode pays for itself.

## Implementation sketch

`internal/watch/`:
- `fsnotify`-backed watcher on `Plan.InputFiles`
- A debouncer (default 200ms, configurable)
- A small TTY UI: print + read single keypress without ncurses
- Reuses `internal/plan/` and `internal/executor/` directly — no
  new core logic, just a wrapper loop

Out of scope for v1:
- Watching included tasks/files when `import:` is template-resolved
  (`import: "{{ x }}.yml"`) — would need full re-plan to discover
  the new input set. Acceptable: re-discover on every change.
- Network FS / containerd shared mounts — fsnotify behavior varies;
  document the limitation, fall back to polling if needed
  (`--poll 1s`).

## What this doesn't address

- **No remote watch** (`mooncake fleet watch` already exists for a
  different purpose: streaming events from peers). The names
  collide; v1 of this proposal could be `mooncake apply --watch`
  to avoid the namespace clash. Editorial decision.
- **No "go back to a previous plan"**. If the user wants to revert
  to a saved plan, they use `--from-plan` separately.

## Companion: dry-run as the implicit default

This proposal pairs naturally with promoting `apply --dry-run` to
a first-class verb (`mooncake check` or `mooncake preview`). The
watcher's default mode IS "continuous dry-run"; making the verb
visible elsewhere makes the safety story easier to pitch.

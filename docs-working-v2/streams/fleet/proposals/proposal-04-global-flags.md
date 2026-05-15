# Proposal 04: Hoist `--no-color`, `--json`, `--peers-file`, `--parallel`, `--timeout` to global fleet flags

**Status:** Draft proposal
**Effort:** XS (~1 day; mostly mechanical refactor + tests)
**Value:** Medium — every fleet subcommand re-declares the same 4–5
flags. Hoisting cuts ~80 lines of help text noise and gives users
one mental model instead of N copies.

---

## Problem

Look at every fleet subcommand's `--help`. The same flags appear
on each:

| Flag | apply | exec | observe | ps | watch | status | doctor | logs | facts | upgrade |
|---|---|---|---|---|---|---|---|---|---|---|
| `--peers` | ✓ | ✓ | ? | ✓ | ✓ | ✓ | ✗ | ✓ | ✓ | ✓ |
| `--peers-file` | ✓ | ✓ | ? | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `--peer-filter` | ✓ | ✓ | ? | ✓ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| `--parallel` | ✓ | ✓ | ? | ✓ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ |
| `--no-color` | ✓ | ✓ | ? | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | ✓ |
| `--json` | ✗ | ✓ | ? | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ | ✗ |
| `--timeout` | ✗ | ✓ | ? | ✓ | ✗ | ✓ | ✓ | ✗ | ✗ | ✓ |

Most are repeated across most subcommands. Each repetition is
~3 lines of help text. The user has to learn the same flags
several times because they're scoped per subcommand.

Plus the inconsistencies that pop out of this table:
- `--no-color` missing on `fleet facts`
- `--json` missing on `fleet apply`, `fleet logs`, `fleet facts`
- `--parallel` missing on `fleet watch`, `fleet logs`, `fleet facts`,
  `fleet upgrade` — sometimes intentional, sometimes not
- `--peer-filter` only on the fan-out commands; `fleet doctor` /
  `logs` / `facts` are single-peer so no filter (acceptable)

Result: writing a CI script that targets multiple peers requires
remembering which flags work on which subcommand.

## Proposal

Promote five flags to the **`mooncake fleet`** parent command,
inherited by every subcommand:

| Flag | Type | Default | Behavior |
|---|---|---|---|
| `--peer` (repeatable, see proposal-01) | string | (all) | Peer selector. Subcommand-aware: takes effect only on multi-peer subcommands. |
| `--peers-file` | path | `~/.config/mooncake/peers.toml` | Where to read peers from |
| `--parallel` | int | 0 (unbounded) | Max concurrent peer operations |
| `--timeout` | duration | 30s (per-subcommand override) | Default per-peer deadline |
| `--no-color` | bool | (auto-detect TTY) | Disable ANSI colors; respects `NO_COLOR` env |
| `--json` | bool | false | Emit JSONL instead of formatted text |

```bash
# All three forms work; --no-color and --json apply to whatever follows
mooncake fleet --no-color exec "cmd"
mooncake fleet exec "cmd" --no-color    # subcommand-level still allowed
mooncake fleet --json status

# Compose multiple global flags before subcommand
mooncake fleet --parallel 4 --timeout 30s --peer tag=prod apply site.yml
```

Subcommands that don't accept one of these flags ignore it
silently (or warn under `-v`). E.g., `fleet facts <peer>` ignores
`--peer` (positional wins).

## Where each subcommand keeps its specifics

- `fleet apply` keeps: `--max-sync-size`, `--plan-dir`, `--vars-file`,
  `--step-filter`
- `fleet exec` keeps: `--env`, `--cwd`, `--become`, `--shell`
- `fleet ps` keeps: `--status`, `--all`, `--limit`, `--sort`, `--short`
- `fleet watch` keeps: `--poll-interval`
- `fleet upgrade` keeps: `--binary`, `--force`, `--include-os`
- `fleet bootstrap` keeps: `--port`, `--agentd-port`, `--name`, `--tag`
- `fleet pair` keeps: `--name`, `--tag`, `--token-via`
- `fleet init` keeps: `--no-mdns`, `--no-ssh-config`, `--mdns-timeout`,
  `--dry-run`, `--accept-all`
- `fleet discover` keeps: `--ssh-config`, `--no-mdns`, `--mdns-timeout`,
  `--no-probe`, `--probe-timeout`

## Help text shape

Before (every subcommand):
```
OPTIONS:
   --peers value                 Comma-separated peer names
   --peers-file value            Override the peers.toml path
   --peer-filter key=value [...] Filter by key=value (...)
   --parallel value              Max peers in flight
   --timeout value               Per-peer probe timeout (default: 3s)
   --no-color                    Disable ANSI colors
   --json                        Emit JSONL
   <subcommand-specific flags>
```

After:
```
OPTIONS:
   <subcommand-specific flags only>

GLOBAL FLEET OPTIONS:
   See `mooncake fleet --help` for --peer, --peers-file, --parallel, --timeout, --no-color, --json.
```

Subcommands' `--help` becomes shorter and more focused.

## Inheritance semantics

urfave/cli supports persistent flags on a parent command. The
implementation pattern:

```go
// cmd/fleet.go
func fleetCmd() *cli.Command {
    return &cli.Command{
        Name: "fleet",
        Flags: globalFleetFlags(),   // <-- persistent
        Subcommands: []*cli.Command{
            applyCmd(), execCmd(), psCmd(), ...
        },
    }
}

func globalFleetFlags() []cli.Flag {
    return []cli.Flag{
        &cli.StringSliceFlag{Name: "peer", ...},
        &cli.StringFlag{Name: "peers-file", ...},
        // ...
    }
}
```

Subcommand handlers read via `ctx.String("peer")` exactly as today —
just inherited from the parent context.

## Receipts

- Wrote ~15 fleet test scenarios during the audit. Each required
  remembering whether `--no-color` was on this subcommand or that
  one.
- `--peers-file` appears on every fleet subcommand. Even
  `--token-via` (`fleet pair`) is the same pattern but more
  consistent — token sourcing is specific, peers-file is universal.
- Fleet README listed flags per command — would be much shorter
  with hoisted globals.

## Pairs naturally with

- **proposal-01** (peer targeting unify) — both touch the same
  surface
- **DX proposal-02** (output middle ground) — `--json` becoming
  global means `--format json` and `--json` can converge

## Risks

- **Subcommand-specific defaults**: `fleet doctor --timeout 3s` is
  smaller than `fleet apply --timeout 5m` for good reason. Allow
  subcommands to override the inherited default:
  ```go
  applyCmd().Flag("timeout").DefaultText("5m  // overrides fleet --timeout default")
  ```
  Document the override in the subcommand's help.

- **Order of flag parsing**: `mooncake fleet apply --json site.yml`
  vs `mooncake fleet --json apply site.yml` — both should work.
  urfave/cli handles this naturally.

- **Subcommand-specific flag names that conflict**: `fleet upgrade`'s
  `--force` is upgrade-specific; `fleet kill --force` (proposal-02)
  would be kill-specific. Both fine; no collision because they're
  on different subcommands.

## What this doesn't address

- **Top-level mooncake flags** (e.g. `--log-level`, `--config`) —
  separate concern; they're already global from the apply level
- **Env-var equivalents** (e.g. `MOONCAKE_FLEET_NO_COLOR=1`) — nice
  to have; defer
- **A config file for default flags** (e.g.
  `~/.config/mooncake/fleet.toml`) — could let users persist
  `--parallel 8` once and forget. Defer to user demand.

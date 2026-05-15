# Personal Fleet — Implementation Order

> A PR-sized breakdown of the six specs under this directory. Sequenced so
> each PR is independently reviewable AND every two or three PRs end with
> a demoable capability. Sizes are rough; aim for ≤ 600 LOC of meaningful
> change per PR (excluding tests).

---

## Three phases

| Phase | Goal | Demoable end state |
|---|---|---|
| **A: One peer, one apply** (PR 1–5) | Get a single peer working end-to-end | `fleet apply config.yml` syncs + applies + streams logs from one configured peer |
| **B: Real fleet** (PR 6–11) | Multi-peer, inspection, easy onboarding | `fleet apply` across N peers with interleaved logs; `fleet status` shows health; `fleet bootstrap` adds a new box |
| **C: Polish** (PR 12–14) | Discovery + per-host overlays | `fleet init` discovers boxes on LAN; per-host vars overlays land naturally |

If we stop after Phase A, we have a usable but boring personal-fleet (manual
peers.toml, one apply at a time). If we stop after Phase B, we have the
five-bullet "Friday-evening demo" success criteria from the epic. Phase C is
the wow factor.

---

## Progress snapshot (2026-05-15)

**13 of 14 PRs shipped end-to-end.** Phase A, Phase B, and Phase C (2/3) are complete.

| Status | PRs |
|---|---|
| ✅ Shipped | 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11 (full — auto-promoted when PR 10 landed), 12 (mDNS — `70476f6`/`beb495e`), 14 |
| ⏳ Not started | 13 (`fleet init` interactive flow) |

Sidecar specs that shipped alongside:

- **spec-49 (agentd on Windows)** → `bdcc396`. TCP-only mode + Windows config defaults + two SSE race fixes. Not part of the 14-PR plan but unblocked Windows peers.
- **Fleet polish (`bd4695a`)** → empty-stdout rendering, peer-filter typo warning, Windows config path, SSE race regression tests.
- **Tag-filter UX (`ed2cee9`)** → flags-after-positional reorder, opt-in tag-filter semantics.
- **Fleet rendering + git.clone fix (`afa1d57`)** → render `step.failed` `error_message`; refuse divergent fast-forward.

PR11 history: shipped as a "lite" cut first (system `ssh`/`scp` shell-out, `nohup` for the daemon) and auto-promoted to the full surface (native SSH driver + embedded systemd/launchd templates + 8-step orchestration) once PR 9 + PR 10 landed.

### Post-PR-14 follow-up specs (drafted from real-world use, not yet sequenced)

| Spec | Topic | Effort | Notes |
|---|---|---|---|
| 50 | Extended filter keys (`os=`, `name=`, `role=`) for `--peer-filter` / `--step-filter` | S–M (~3–5 days) | Generalises spec-48's `tag=`-only DSL. Built on the existing `parseFilterFlags` predicate parser; no new flag. |
| 51 | Local-apply overlay parity — `mooncake apply` auto-loads `vars/by-host/<hostname>.yml` | XS (~1 day) | Closes asymmetry where overlays only fired via `fleet apply`. Cheapest of all the open work. |

These are loose drafts in `specs/personal-fleet/`. Sequence them after PR 12/13 or pull them forward if a real user trips on the bookkeeping.

---

## PR table

| # | Title | Spec | Status | Scope (one line) | Depends on | Demoable? | Est. LOC |
|---|---|---|---|---|---|---|---|
| **Phase A — one peer end-to-end** |||||||
| 1 | agentd TCP + bearer auth + token + /v1/version extension | 43 | ✅ | `Config` fields, second listener, bearer middleware, token gen, add `hostname` + `synced_root` to `/v1/version` | — | `curl :7878/v1/version` with bearer | ~250 |
| 2 | `PUT /v1/files` + `HEAD /v1/files` with sandbox | 43 | ✅ | Path sandbox helper, streaming write, sha256 check, 100 MiB cap | PR1 | `curl -X PUT … --data-binary @foo.yml` lands in `<state_dir>/synced/<scope>/foo.yml` | ~350 |
| 3 | Fleet CLI scaffold + peers.toml + controller_id | 43 | ✅ | `cmd/fleet.go`, `peers.toml` loader+writer, `EnsureControllerID()`, `fleet apply` skeleton (no-op) | PR1 (for /v1/version probe) | `mooncake fleet apply --help` works; peers.toml round-trip | ~300 |
| 4 | Peer transport client (HTTP + SSE) | 43 | ✅ | `Head`/`Put`/`Submit`/`Stream` methods, bearer header, SSE parser | PR2, PR3 | Run an integration test against a single agentd | ~400 |
| 5 | Sync loop + apply orchestration (single peer) | 43 | ✅ | Plan-dir walker, HEAD-skip+PUT loop, submit, stream events, interleaved output (degenerate with one peer) | PR4 | **🎯 `mooncake fleet apply config.yml` works end-to-end against one configured peer** | ~500 |
| **Phase B — real fleet** |||||||
| 6 | Multiplexer for N peers + `^C` banner | 43 | ✅ | `internal/fleet/multiplex.go`, parallel orchestration, padded prefix, color, NO_COLOR, ^C handling | PR5 | **🎯 Apply across multiple peers with interleaved `[host]` lines** | ~400 |
| 7 | `fleet status` | 46 | ✅ | Per-peer parallel probe, table renderer, `--json` | PR4 | `mooncake fleet status` shows all peers' health | ~350 |
| 8 | `fleet logs` + `fleet facts` | 46 | ✅ | Latest-run resolution, reattach via SSE, `--all`, fact pretty-print, `--query` fan-out | PR4, PR6 | `mooncake fleet logs --all` reattaches to in-flight runs | ~300 |
| 9 | SSH driver + platform detection | 44 | ✅ | `internal/fleet/transport/ssh.go` (agent+key auth, known_hosts), `DetectPlatform()` | — (parallel to others) | Driver test against alpine+sshd container | ~400 |
| 10 | Bootstrap orchestration + installer templates | 44 | ✅ | Embedded systemd unit + launchd plist, 8-step install sequence | PR9 | `internal/fleet/bootstrap.go` callable from test | ~450 |
| 11 | `fleet bootstrap` + `fleet pair` CLI | 47 | ✅ | CLI wrappers, peers.toml upsert with diff, three token-source paths for pair | PR3, PR10 | **🎯 `mooncake fleet bootstrap aleh@new-box` adds a peer in one command** | ~300 |
| **Phase C — polish** |||||||
| 12 | mDNS advertise (daemon) + query (controller) + SSH config parser | 45 | ✅ | zeroconf wrapper, agentd advertise goroutine, ssh_config parser | PR1, PR4 | `mooncake fleet discover` finds `_mooncake._tcp.local` responders on LAN | ~450 |
| 13 | `fleet init` interactive flow | 45 | ⏳ | Aggregator, prompt loop, token paste / `--ssh-fetch` paths | PR11, PR12 | **🎯 `mooncake fleet init` walks through adding 4 boxes** | ~350 |
| 14 | Per-host overlays + tag selectors | 48 | ✅ | `internal/fleet/overlays.go`, `--peer-filter`/`--step-filter` parsing, wire vars_files into submit | PR5 | `vars/by-host/macbook.yml` is applied when targeting macbook | ~250 |
| **Follow-up specs (not in original plan)** |||||||
| 15 | Extended filter keys (`os=`, `name=`, `role=`) | 50 | ✅ | Validator + evaluator extension to spec-48's predicate DSL — no new flags | PR14 | `mooncake fleet apply --peer-filter os=darwin` filters peers from `/v1/version` | ~250 |
| 16 | Local-apply overlay parity | 51 | ✅ | `mooncake apply` auto-loads `vars/by-host/<hostname>.yml` + `vars/common.yml` | PR14 | Edit `vars/by-host/laptop.yml`, run `mooncake apply` on laptop, overlay applies | ~120 |

**Totals:** ~5,200 LOC across 14 plan PRs + ~370 LOC across 2 follow-up PRs.

---

## Sequencing notes

- **PR1 is daemon-only.** Land it first so the controller PRs have a real
  endpoint to integrate against without mocks.
- **PR9 (SSH driver) is parallelizable** with PRs 6–8. It has no
  dependencies inside Phase B and could be picked up by a second
  contributor (or as cooldown work between bigger PRs).
- **PR12 (mDNS + ssh_config) is parallelizable** with PR11. Both feed PR13.
- **PR14 (overlays + tags)** depends only on PR5 — it's controller-side only
  and could ship between PR5 and PR6 if the user wants overlay support
  before multi-peer. Listed last because the demo value of multi-peer (PR6)
  is higher.

---

## Definition of done per PR

Every PR should land green on these:

1. **Build** — `go build ./...` passes on Linux and macOS.
2. **Lint** — existing `make lint` / `golangci-lint` baseline maintained.
3. **Tests** — new tests added in the same PR for new public behavior.
   No "I'll add tests later" PRs.
4. **No flag debt** — every new CLI flag has help text and at least one test
   exercising it.
5. **Demo-able when marked 🎯** — if the row's "Demoable?" column has a
   target, the PR description includes the exact command(s) to demo it
   manually.
6. **Spec drift annotated** — if implementation deviates from the spec,
   note the deviation in the PR description AND update the spec in the same
   PR (don't let spec and code drift).

---

## Risk register

| Risk | Likely PR | Mitigation |
|---|---|---|
| Path sandbox bypass in `PUT /v1/files` | PR2 | Explicit symlink rejection, `filepath.Clean` + prefix check, dedicated security test (try `../../etc/passwd`, symlink to `/etc/`) |
| SSE multiplexer races under load | PR6 | Goroutine-leak tests with `goleak`; deterministic tests using a fake clock |
| `fleet apply` partial-failure UX | PR5/PR6 | Continue-others-by-default per the epic; print a final summary line with per-peer result counts |
| Bootstrap leaves partial state on failure | PR10 | Document the per-step rollback table from spec-44; tests for at least the upload-failed and start-failed paths |
| zeroconf instability on macOS | PR12 | Smoke-test on actual macOS hardware before merge; have hashicorp/mdns as a documented fallback |
| TOML comment preservation in peers.toml | PR11 | Pick `pelletier/go-toml/v2` from the start; don't roll our own writer |

---

## What ships first if we're impatient

If we want the smallest possible "personal fleet works" before any polish:

**Minimum viable:** PR 1 → 2 → 3 → 4 → 5 (~1,800 LOC, single-peer working).
Skip PR6's multiplexer briefly — the single-peer code path is enough for the
"can I apply to my MacBook from my laptop" demo. Multi-peer becomes a quick
follow-up via PR6.

**Solid first impression:** add PR 7 (status) and PR 11 (bootstrap)
afterwards. That's PRs 1–7 + 9–11 = ~3,500 LOC for a fleet that you can
add boxes to, apply against, and inspect — without discovery or overlays
yet.

**Full epic complete:** all 14 PRs.

## What's still open (as of 2026-05-15)

Phase A, Phase B, and Phase C (2/3) are complete. Only one item remains:

1. **PR 13** — `fleet init` interactive flow. Builds on PR 12 (mDNS ✅) + PR 11 (bootstrap/pair ✅). Pure UX polish; no capability gap behind it.

PRs 15 (spec-50 ✅) and 16 (spec-51 ✅) both shipped. mDNS (PR 12) also shipped (`70476f6`/`beb495e`).

`fleet init` is the only remaining Phase C item. Defer until a real user reports friction with hand-editing peers.toml — or pull it forward as a small cooldown task.

---

## Where this doc fits

- Specs (`spec-43` through `spec-48`) own the *what* and *how*.
- This doc owns the *in what order*.
- The epic (`epic-personal-fleet.md`) owns the *why*.

If you change a spec, update its task list. If you change the order, update
this doc. Don't let one drift from the other.

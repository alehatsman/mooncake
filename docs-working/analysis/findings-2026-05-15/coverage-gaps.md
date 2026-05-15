# Coverage Gaps — Presets, Tool Action, repo.tree

Functionality that's advertised or implied to work but has missing
branches, hardcoded assumptions, or unhelpful defaults.

---

## #3 — `jq` preset only handles apt + brew, README claims four package managers — MEDIUM (broken promise)

**Repro**: apply `jq` preset on `alpine:3.21`:
```
docker run --rm -v $PWD/out/mooncake-static:/usr/local/bin/mooncake \
  -v $PWD/presets:/work/presets \
  -v /tmp/mooncake-tests/preset-jq/mooncake.yml:/work/mooncake.yml \
  -w /work alpine:3.21 sh -c 'apk add --no-cache sudo bash && mooncake apply'
```

Output:
```
- Install jq (macOS) [when: ... brew_available ...]
- Install jq (Linux) [when: ... apt_available ...]
▶ Verify installation with assert
✗ Verify installation with assert
RECAP  ok=0  changed=1  skipped=2  failed=2  70ms
```

**Why MEDIUM**: `presets/jq/README.md` lists "Linux (apt, dnf, yum,
pacman)" as supported. Actual `tasks/install.yml` only has `apt` and
`brew` branches — apk/dnf/yum/pacman silently skip, then verify
asserts → fails. Worst failure shape: install appears to succeed
(no package errors), then assert kicks in with confusing message.

**Fix**: add `apk_available`, `dnf_available`, `yum_available`,
`pacman_available` facts (or use existing capability detection), and
corresponding branches. Or downscope README to "apt, brew" and file
a backlog issue.

**Wider issue**: almost certainly not specific to jq. Every preset
authored against this pattern has the same risk. Worth a one-time
audit across `presets/*/tasks/install.yml`.

---

## #23 — `repo.tree` reports zero files with default settings — MEDIUM

**Repro**:
```
$ mkdir -p /tmp/walk/sub && echo a > /tmp/walk/a.txt && echo b > /tmp/walk/sub/b.txt
$ mooncake apply -c <step that does: repo.tree: { path: /tmp/walk }>
```

JSON event payload from `step.completed`:
```json
"result": {
  "total_dirs": 1,
  "total_files": 0,
  "tree": {"name":"walk","type":"directory","path":""}
}
```

`max_depth: 2` and `max_depth: 10` produce the same empty result via
`mooncake step`.

**Why MEDIUM**: `repo.tree` is supposed to walk a directory. By
default returns nothing. Either default `max_depth: 0` doesn't
descend OR action is silently broken. Either way, a no-args
`repo.tree: { path: ... }` should show immediate children.

---

## #37 — `mooncake presets search <local-preset-name>` doesn't find local presets — MEDIUM (UX)

**Repro**:
```
$ mooncake presets list | grep docker
docker                Install and configure Docker container runtime (v1.0.0)

$ mooncake presets search docker
Warning: could not load index for registry 'official': ... TLS error
No presets found matching "docker".
```

`presets list` finds local presets (via search-path resolution).
`presets search` ONLY consults the remote registry.

**Fix**: `presets search` should consult local search paths first,
then the remote registry. Or rename to `presets remote-search` if
remote-only is intentional.

---

## #38 — `presets list --format json` is unsupported — LOW

```
$ mooncake presets list --format json
Incorrect Usage: flag provided but not defined: -format
```

Other commands (`metrics`, `snapshot`, `history`, `apply`) support
`--format json` / `--output-format json`. `presets list` doesn't.
For programmatic preset discovery this is a gap.

---

## #39 — `tool github-release` defaults to `v{version}` tag — breaks jq-style tag prefixes — MEDIUM

**Repro**:
```
$ mooncake step "tool: { name: jq, backend: github-release, repo: jqlang/jq, version: \"1.7.1\", asset: jq-linux-amd64 }"
{"error": "http GET https://github.com/jqlang/jq/releases/download/v1.7.1/jq-linux-amd64: status 404"}
```

URL is constructed as `/releases/download/v{version}/{asset}`. jq is
tagged `jq-1.7.1`, not `v1.7.1`, so URL 404s. Works with `tag:
"jq-1.7.1"` but that's not in any example.

**Fix**: when `tag:` unset, try `v{version}` first, then `{version}`,
then error suggesting `tag:`. Or: require `tag:` when the version
isn't prefixed with `v`.

---

## #33 — `git.clone` param is `repo:` not `url:` — LOW (DX surprise)

**Repro**:
```
$ mooncake step "git.clone: { url: ..., dest: ... }"
{"error": "validation failed for git.clone action: git.clone: repo is required"}
```

Works with `repo:` instead. Ansible-consistent — but most other
Mooncake actions use `url:` (`file.download`), and `git clone <URL>`
is universally called a URL.

**Fix**: accept `url:` as alias, or document the choice loudly.

(Confirmed working: `git.clone` is properly idempotent on second
call — `changed: false` when dest already exists.)

---

## Companion finding — #40 also lives here but is severe enough for `silent-success-bugs.md`

See [`silent-success-bugs.md`](./silent-success-bugs.md#40) for
`tool github-release` always extracting as archive (bare-binary
support missing).

---

## Summary table

| # | Sev | Area | Fix |
|---|---|---|---|
| 3 | MEDIUM | jq preset (and probably others) | audit `presets/*/tasks/install.yml` |
| 23 | MEDIUM | `repo.tree` default | descend by default |
| 37 | MEDIUM | `presets search` | consult local first |
| 38 | LOW | `presets list --format json` | add JSON output mode |
| 39 | MEDIUM | `tool github-release` tag prefix | try multiple prefixes |
| 33 | LOW | `git.clone` `repo:` vs `url:` | accept alias |
| 40 | HIGH | `tool github-release` bare binaries | see silent-success-bugs.md |

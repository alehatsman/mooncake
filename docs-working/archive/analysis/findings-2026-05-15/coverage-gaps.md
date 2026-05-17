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

**Status rollup as of 2026-05-17: all coverage-gap findings ✅ FIXED or
out-of-scope (presets/ directory removed).**

| # | Sev | Status | Area | Fix |
|---|---|---|---|---|
| 3 | MEDIUM | n/a (obsolete) | jq preset cross-platform branches | top-level `presets/` directory was removed; per-preset cross-platform coverage is now an external concern |
| 23 | MEDIUM | ✅ FIXED | `repo.tree` default | `include_files` defaults to true `17642f81` |
| 37 | MEDIUM | ✅ FIXED | `presets search` | consults local search paths first `7e1554e3` |
| 38 | LOW | ✅ FIXED | `presets list --format json` | JSON output added |
| 39 | MEDIUM | ✅ FIXED | `tool github-release` tag prefix | resolves tag when unset, doesn't hardcode v-prefix `e63851d7` |
| 33 | LOW | ✅ FIXED | `git.clone` `repo:` vs `url:` | accepts `url:` alias `f3feca10` |
| 40 | HIGH | ✅ FIXED | `tool github-release` bare binaries | landed `cb6b21bb` — see silent-success-bugs.md |
| 50 | LOW | ✅ FIXED | `file.copy` directory error | names concrete alternative `f1f2f7f8` |
| 51 | LOW | ✅ FIXED | `file.copy` symlink follow | `follow_symlinks:` parameter, default true `5a6d4c9d` |
| 52 | LOW | ✅ FIXED | `git.*` path param parity | `git.config` accepts `dest:` alias `7defabe2` |
| 60 | LOW | ✅ FIXED | `tool which` when `bin:` unset | auto-resolve single executable `2e1e5840` |
| 63 | MEDIUM | ✅ FIXED | `container: state: stopped` no-op when absent | `269246a3` |
| 64 | HIGH | ✅ FIXED | `container:` inspect template fields | uses `.Config.Image` not `.ImageName` `8d477ddd` |

---

## #50 — `file.copy` on a directory errors with vague "use recursive copy action" — LOW

**Repro**:
```
$ mooncake step "file.copy: { src: /tmp/srcdir, dest: /tmp/dstdir }"
{
  "error": "src is a directory, use recursive copy action instead",
  "failed": true
}
```

The error tells you what NOT to use, but not what TO use. There's no
`file.copy_recursive`, no `file.copy_tree`, no `file.dir` in the schema.
Closest is `shell: cp -r ...` but that's not what the error implies.

**Fix**: either name the action explicitly in the error (`use shell: cp -r ...`)
or add a `recursive:` flag on `file.copy` (simplest).

---

## #51 — `file.copy` silently follows symlinks — LOW (semantic)

**Repro**:
```
$ ln -s /tmp/srcdir/a.txt /tmp/sym
$ mooncake step "file.copy: { src: /tmp/sym, dest: /tmp/syml-dst }"
{"changed": true}
$ ls -la /tmp/syml-dst
-rw-r--r-- ... /tmp/syml-dst   ← regular file, not a symlink
```

The destination is a regular file with the symlink's *target* content.
The link itself was not preserved. For dotfile/`/usr/local/bin/foo`
patterns this is probably wrong; users may want
`follow_symlinks: false` to preserve link structure.

**Fix**: add `follow_symlinks:` parameter (default true for back-compat).

---

## #60 — `tool which <name>` returns the version dir when `bin:` is unset — LOW

**Repro 1** (`bin:` unset — pre-MT-40 hint when bare-binary support
landed):
```yaml
- tool:
    name: jq
    backend: github-release
    version: "1.7.1"
    tag: jq-1.7.1
    asset: jq-linux-amd64
    # no bin:
```
```
$ mooncake tool which jq
/root/.local/share/mooncake/tools/jq/1.7.1
$ /root/.local/share/mooncake/tools/jq/1.7.1 --version
bash: ...: Is a directory
```

**Repro 2** (`bin:` set):
```yaml
- tool:
    name: yq
    backend: archive-url
    ...
    bin: yq_linux_amd64
```
```
$ mooncake tool which yq
/root/.local/share/mooncake/tools/yq/4.44.3/yq_linux_amd64    ← correct
```

**Why LOW**: with `bin:` set, the resolution works perfectly. But the
common case for github-release bare-binary is that `asset:` and `bin:`
have the same value — and forgetting `bin:` is a quiet pit. The
action should default `bin: asset` when asset is unambiguous (single
file, not an archive).

**Fix**: when `bin:` is unset and the install dir contains exactly
one executable file, auto-resolve to that.

---

## ★ `tool: backend: archive-url` works end-to-end — positive

Don't forget to record this works:
```yaml
- tool:
    name: yq
    backend: archive-url
    version: "4.44.3"
    url: https://github.com/mikefarah/yq/releases/download/v4.44.3/yq_linux_amd64.tar.gz
    checksum: "sha256:a347…"
    bin: yq_linux_amd64
```
- Verifies checksum (rejects mismatches, file does NOT land)
- Extracts to versioned dir
- Run 2: `already installed at ...`, idempotent
- `tool which yq` → binary path; `yq --version` → `4.44.3`

This is the right pattern for installing tools that don't have
github-release pages. Keep.

---

## #63 — `container: state: stopped` starts container even when absent — MEDIUM (semantic)

**Repro**:
```
$ docker ps -a | grep mc-test-nginx
(nothing)

$ mooncake step "container: { name: mc-test-nginx, image: nginx:alpine, state: stopped }"
{"changed": true, "duration_ms": 4923}

$ docker ps -a | grep mc-test-nginx
mc-test-nginx Exited (0) Less than a second ago
```

`state: stopped` against a *missing* container does:
1. Pull image
2. Create container
3. Start it
4. Stop it

Expected: ensure-stopped from existing — should be a no-op if nothing's
there (or maybe `state: stopped` shouldn't be reachable without `state:
running` having happened earlier).

**Fix**: `state: stopped` should be no-op when container doesn't exist;
add a separate `state: created_stopped` if "create and leave stopped"
is needed.

---

## #64 — `container: state: absent` leaks a Go-template parse error — MEDIUM

**Repro**:
```
$ docker ps -a | grep mc-test-nginx        # exists, stopped
mc-test-nginx ... Exited

$ mooncake step "container: { name: mc-test-nginx, image: nginx:alpine, state: absent }"
{
  "changed": false,
  "error": "container: inspect mc-test-nginx: docker container inspect mc-test-nginx failed: exit status 1 (output: template parsing error: template: :1:20: executing \"\" at <.ImageName>: map has no entry for key \"ImageName\")",
  "failed": false   ← per #61
}
```

The internal Go template referenced `.ImageName` but the docker
container struct has `.Config.Image` (or similar). The template error
leaks through as the user-facing error. And the container wasn't
removed (still exists).

**Fix**: correct the template field path, add a unit test for the
container-inspect parsing.

**Update**: this bug is broader than `state: absent`. Any second
invocation of `container:` against an existing container hits the
template error because the idempotency check calls `docker container
inspect` to compare desired-vs-actual state. Repro:
```
$ mooncake step "container: { name: mc, image: alpine:3.21, command: [sleep,30], state: running }"
{"changed": true}     ← first run, no inspect needed
$ mooncake step "container: { name: mc, image: alpine:3.21, command: [sleep,30], state: running }"
{"changed": false, "error": "template: :1:20: at <.ImageName>: map has no entry for key \"ImageName\""}
```

So `container:` is **effectively single-use** today. Any second
apply (the entire point of an idempotent config-mgmt tool) fails the
inspect path. Bumping severity in my own head from MEDIUM → HIGH but
keeping the number at #64 since the fix is the same one.

---

## #52 — `git.*` family has inconsistent path parameter names — LOW (DX surprise)

Each action uses a different param name for "the local repo":
- `git.clone`: `dest:` (with `url:` or `repo:` for source — closed in MT-33)
- `git.checkout`: `dest:`
- `git.config`: `repo:`

So `git.clone → git.config → git.checkout` requires renaming the path
param twice in adjacent steps. Either standardize on `dest:` everywhere
or alias all variants. (`git.checkout` accepts `dest:`; `git.config`
silently accepts a `dest:` field but doesn't use it — per #44 — then
errors saying `repo is required`.)

**Fix**: pick one name (`dest:` is simplest), alias the others, and
update the schema's `additionalProperties: false` enforcement once #44 lands.

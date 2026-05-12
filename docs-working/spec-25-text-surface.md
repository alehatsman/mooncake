# Spec 25: `text.*` Surface — `text.line`, structural patches

**Status:** Draft
**Epic:** E9 Modern Action Surface — bucket E9.3
**Effort:** M (1 week)
**Value:** High. Closes the "ensure this line / this JSON key / this
INI section" gap that today forces users into shell pipelines or
template-the-whole-file. `text.line` alone replaces 30% of `shell: sed`
in the wild.

**Source:** `VISION_ACTIONS.md` §5 (Tier-1 priorities §7.3, §7.8).

---

## Problem

Today's `text.*` surface covers:

- `text.replace` — regex replace
- `text.insert` — anchor-based insert
- `text.delete_range` — anchor-based deletion
- `text.patch` — unified-diff patch apply

Missing — and frequently asked for:

1. **`text.line`** — "ensure this line is present / absent in this file,
   anchored on a regex." The Ansible-equivalent (`lineinfile`) is the
   single most-used module in the wild. Without it, users write
   `text.replace` regexes that mishandle "line not yet present" cases.
2. **`text.patch.json`** — structural JSON edits (`set key.path = value`,
   `delete key.path`, `merge`). Today users template the whole JSON
   file (lossy for files with comments/order) or shell out to `jq`.
3. **`text.patch.yaml`** — same for YAML. Especially valuable because
   Mooncake itself is YAML-heavy; lots of presets edit other YAML.
4. **`text.patch.ini`** — section/key edits for INI-style files
   (systemd unit files, php.ini, ssh_config, …).

These four sit alongside the existing `text.*` actions. No replacements;
new additions.

---

## Goals

- **G1** `text.line`: ensure a line is present/absent, optionally
  anchored.
- **G2** `text.patch.json`: structural JSON edits with order/format
  preservation.
- **G3** `text.patch.yaml`: structural YAML edits with
  comment/order preservation (ruamel.yaml-equivalent in Go).
- **G4** `text.patch.ini`: section/key edits.
- **G5** All implement `Diff`, `Reverse`, `Permissions` (spec 22).
- **G6** All preserve file metadata (mode, owner, group) unless
  explicitly overridden.

**Out of scope:**

- TOML / XML / Cue — defer to Tier-2 plugins.
- Multi-file batch operations — that's `for_each` over single-file
  steps.
- Block-style edits ("ensure this block of N lines is present") — fold
  into `text.line` with multi-line support or defer.

---

## Design

### `text.line`

```yaml
- text.line:
    path: /etc/ssh/sshd_config
    line: "PermitRootLogin no"
    state: present              # or "absent"
    regexp: "^PermitRootLogin"  # if present-line not found, replace this line
    insert_after: "^Port "      # if neither matches, insert after this anchor
    backrefs: false             # allow regex backrefs in `line`
```

Semantics — present:
- If a line matching `line` exactly already exists: noop.
- Else if a line matching `regexp` exists: replace it with `line`.
- Else if `insert_after` matches: insert `line` after that match.
- Else if `insert_before` matches: insert `line` before that match.
- Else: append `line` at end of file.

Semantics — absent:
- All lines matching `line` (exact) OR `regexp` are removed.

Edge cases handled:
- File doesn't exist → create with `line` (unless `state: absent`, no-op).
- `line` contains newlines → reject at validate time.
- Idempotency: second run is a no-op (byte-identical file).

### `text.patch.json`

```yaml
- text.patch.json:
    path: /etc/foo/config.json
    set:
      "service.port": 8080
      "service.name": foo
    delete:
      - "deprecated.field"
    merge:
      "service.tags": [v2]      # array merge: append-unique
      "service.env": { LOG: info }  # object merge: deep-set, no overwrite of present keys
```

- Keys use RFC 9535 JSONPath-lite (just `a.b.c` and `a[0]` index forms;
  no wildcards or filters — keep the surface small).
- Order-preserving: emit minimal diff. Use `encoding/json/decoder` with
  raw-message holes; reassemble.
- Pretty-print: detect indentation from input (2/4 space / tab); preserve.

### `text.patch.yaml`

Same shape as `text.patch.json` but for YAML files.

Order + comment preservation: there's no perfect-fidelity YAML library
in Go (no ruamel.yaml equivalent), but `gopkg.in/yaml.v3` preserves
key order via `yaml.Node`. We can preserve comments around
node-level edits and accept that deeply-restructured comments may
drift; document the limitation.

### `text.patch.ini`

```yaml
- text.patch.ini:
    path: /etc/php/8.2/cli/php.ini
    set:
      "PHP.memory_limit": "256M"
      "PHP.max_execution_time": "30"
    delete:
      - "PHP.disable_functions"
    backup: true
```

INI parsing tolerates the ssh_config variant (no `[section]` headers).
For sectionless files, omit the section prefix in keys: `"memory_limit":
"256M"`.

### Cross-cutting

- **Permissions:** all four imply `FilesystemWrite: [path]`. Sudo iff
  the path is in a privileged location (`/etc/**`, `/usr/**`, etc.) —
  computed at `Permissions()` time.
- **Diff:** unified-diff style for `text.line`. Structural diff for
  the patch.* actions (showing JSON/YAML/INI keys added/changed/
  removed; with line-level diff as a secondary view).
- **Reverse:** all four reverse via snapshot — pre-apply file content
  is restored.
- **Cost:** all `Risk: 4` (routine config write).

---

## Key files

| File | Change |
|---|---|
| `internal/actions/text_line/` | New action handler. |
| `internal/actions/text_patch_json/` | New. Uses `encoding/json` + custom decoder. |
| `internal/actions/text_patch_yaml/` | New. Uses `gopkg.in/yaml.v3` node-level edits. |
| `internal/actions/text_patch_ini/` | New. Lightweight parser (no go-ini dependency unless needed). |
| `internal/config/config.go` | New Step fields: `TextLine`, `TextPatchJSON`, `TextPatchYAML`, `TextPatchINI`. |
| `internal/register/register.go` | Register all four. |
| `internal/config/schema.json` | Regenerate. |
| `internal/config/schema.d`, `mooncake.d.ts` | Regenerate. |
| Examples | One worked example per action under `examples/actions/`. |

---

## Tasks (phased)

1. **Phase 1** — `text.line`. Highest demand, simplest implementation.
   Tests covering present/absent/regexp/insert_after/insert_before/
   idempotency/EOF newline handling.
2. **Phase 2** — `text.patch.ini`. Simpler parser than JSON/YAML.
3. **Phase 3** — `text.patch.json`. Order-preserving emit. Snapshot
   tests against fixtures with mixed-indent / nested arrays.
4. **Phase 4** — `text.patch.yaml`. Node-API edits via `yaml.v3`.
   Document the comment-preservation limits.
5. **Phase 5** — Implement `Diff`, `Reverse`, `Permissions` on all
   four (requires spec 22).
6. **Phase 6** — Docs + examples.

---

## Acceptance criteria

- `text.line` against `/etc/ssh/sshd_config` setting
  `PermitRootLogin no` is byte-identical on second run.
- `text.patch.json` setting `service.port = 8080` in a 200-line JSON
  file preserves field order, 2-space indent, and trailing-newline
  exactly.
- `text.patch.yaml` setting a deep key in a YAML file with comments
  preserves comments adjacent to unchanged keys.
- `text.patch.ini` setting `memory_limit` in `php.ini` preserves
  section ordering and comment lines.
- All four implement and pass `Diff` + `Reverse` round-trip tests.
- Build / vet / lint / test green; schema + docs regenerated.

---

## Open questions

1. **Should `text.line` support `block:` for multi-line blocks?**
   Tempting (Ansible's `blockinfile`). Probably split into a `text.block`
   action later rather than overloading `text.line`.
2. **JSONPath syntax — full RFC 9535 or minimal subset?** Lean
   minimal subset (`a.b.c`, `a[0]`); document.
3. **YAML key order preservation — what guarantees can we make?**
   `gopkg.in/yaml.v3` preserves order on `*yaml.Node` round-trips but
   not always perfectly for deeply-nested edits. Document known cases.
4. **For INI — handle Windows CRLF preservation?** Yes; detect input
   line ending and emit the same.
5. **What's the merge semantic for `text.patch.json.merge` on arrays?**
   Default "append unique" feels right for tag-style lists but wrong
   for ordered config. Probably make it explicit:
   `merge_strategy: append|replace|unique`.

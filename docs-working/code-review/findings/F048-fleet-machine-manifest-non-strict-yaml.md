---
id: F048
title: fleet machine manifest (`fleet.yml`) parses with non-strict YAML — unknown fields silently ignored
severity: bug
package: internal/fleet
file: internal/fleet/machine.go
lines: 82-104
status: open
---

## What

`LoadMachineManifest` reads `<plan-dir>/machines/<name>/fleet.yml` and
parses it with plain `yaml.Unmarshal`:

```go
// internal/fleet/machine.go:88
if err := yaml.Unmarshal(data, &m); err != nil {
    return nil, fmt.Errorf("parse %s: %w", path, err)
}
```

`yaml.Unmarshal` (yaml.v3) does **not** reject unknown fields. The
rest of mooncake's YAML surface does: the main plan reader at
`internal/config/reader.go:255-256` runs a strict pass with
`dec.KnownFields(true)` and surfaces typo'd / unknown step fields as
hard diagnostics (this is the MT-4 path that fixed the F044-adjacent
silent-success class).

The fleet manifest is the one exception. Any field that doesn't
match `Name` / `Peer` / `Plan` / `Vars` / `Tags` silently parses as
zero-value.

## Repro

```yaml
# machines/box1/fleet.yml
phases:
  - name: windows-host
    peer: box1-windows
    plan: ./win.yml
    vrs:                     # typo: should be `vars`
      - ./shared-vars.yml
    tags:
      - windows
```

Run:

```
$ mooncake fleet apply box1
phase 1/1 — windows-host
...
```

`Phases[0].Vars` is `nil`. The shared-vars file the operator
intended to load is silently ignored. The recap shows green; the
phase ran with the wrong vars stack.

Other typo classes that pass silently:

- `peers:` (instead of `peer:`) — caught by Validate's `peer is
  empty` (zero-value of `Peer`). ✅ caught.
- `vars:` typo'd to `varz:` / `vrs:` — silently empty. ❌ silent.
- `tags:` typo'd to `tag:` — silently empty (and the operator only
  notices when their `--tags` filter excludes every step). ❌ silent.
- A whole new field a future version adds (e.g. `timeout:`,
  `retries:`) — silently ignored on older binaries. ❌ silent on
  version skew.

## Why this matters

Same shape as the manual-test class "silent success that's actually
broken" — recap is green, action did the wrong thing. The cost of
fixing is one line; the cost of *not* fixing is "operator's vars
stack silently emptied for the duration of a fleet apply across N
peers."

The main-plan loader already pays this discipline. The fleet
manifest is the odd one out.

## Suggested fix

Switch to a strict decoder:

```go
// internal/fleet/machine.go:82-93
func LoadMachineManifest(path string) (*MachineManifest, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read %s: %w", path, err)
    }
    var m MachineManifest
    dec := yaml.NewDecoder(bytes.NewReader(data))
    dec.KnownFields(true)
    if err := dec.Decode(&m); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, err)
    }
    if err := m.Validate(); err != nil {
        return nil, fmt.Errorf("%s: %w", path, err)
    }
    // ... rest unchanged
}
```

Two trade-offs to acknowledge:

1. **Version-skew brittleness.** A future binary that adds an
   optional field (e.g. `timeout:`) would have its manifests
   rejected by old binaries. The main config loader accepts that
   trade-off because the alternative (silent-ignore on old
   binaries) is worse for trust. Fleet manifests get the same
   treatment.

2. **Comment fields.** Operators occasionally add `# notes:` style
   comments inside the YAML body. YAML comments (`#`) are stripped
   by the parser regardless of strict mode, so this is a non-issue
   — but if anyone has invented an in-band `_comments:` convention
   they'd need to migrate.

Neither blocks the fix.

## How to verify

1. Build the binary.
2. Create a `machines/box1/fleet.yml` with a typo'd top-level field
   (e.g. `vrs:` instead of `vars:`).
3. `mooncake fleet apply box1` should now refuse to load with a
   message naming the unknown field, instead of silently parsing it
   as zero-value.
4. New unit test in `internal/fleet/machine_test.go`:
   `TestLoadMachineManifest_RejectsUnknownField` — feed a manifest
   with a `vrs:` typo and assert `Decode` returns an error mentioning
   the field name.

## Related

- **F044** — same family (silent unknown-field acceptance), already
  fixed for the MCP `explain` tool's `examples_limit`.
- **F033** — silent ignore of validation failures; same trust-erosion
  class.
- **`config/reader.go:255-256`** — the existing reference
  implementation of `KnownFields(true)` for the main plan YAML.
- **Manual-test finding #44** — the original "schema accepts unknown
  fields silently" finding that drove F044's fix; same shape applies
  at the manifest level here.

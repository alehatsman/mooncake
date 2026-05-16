---
id: F019
title: secrets.Resolve does not recurse into step.Vars (*map[string]interface{}) — !secret silently doesn't resolve there
severity: bug
package: internal/secrets/resolver
file: internal/secrets/resolver/resolve.go
lines: 69-115
status: open
---

## What

`walkAndResolveSecrets` handles a closed set of field kinds:

| Kind | Recurses? |
|---|---|
| `reflect.String` | yes — `resolveStringInPlace` |
| `reflect.Pointer` → `String` | yes |
| `reflect.Pointer` → `Struct` | yes — recursive call |
| `reflect.Pointer` → `Map` | **no** — falls through |
| `reflect.Pointer` → `Slice` | **no** — falls through |
| `reflect.Slice` of `String` | yes |
| `reflect.Slice` of `Struct` | **no** — `continue` if elem != String |
| `reflect.Map` of `String → String` | yes |
| `reflect.Map` of `String → interface{}` | **no** — `continue` if elem != String |

The YAML pre-pass (`internal/config/secret_tag.go:substituteSecretTags`)
rewrites **every** `!secret`-tagged scalar in the parsed tree to a
sentinel marker, regardless of where it sits. So a `!secret` inside
any nested structure is encoded as a sentinel string in the
resulting Go value.

The resolver then walks the struct and replaces markers with
resolved values — **but only in the kinds above**. Markers inside
unhandled kinds stay in.

## Concrete miss: `step.Vars`

`config.Step.Vars` is declared as `*map[string]interface{}`
(`config.go:1410`). A user-authored:

```yaml
- name: define-api-key
  vars:
    api_key: !secret env:MOONCAKE_API_KEY
```

flows like this:

1. YAML pre-pass: rewrites the `!secret` scalar to
   `"@@SENTINEL@@env:MOONCAKE_API_KEY"` (whatever
   `security.SentinelPrefix` is). The value now sits in
   `step.Vars["api_key"]` as a string.
2. Plan compile: passes through untouched (Vars is a free-form
   map).
3. Executor calls `resolver.Resolve(&step, redactor)`:
   - Walks `Step`. Hits `Vars` field (kind `Pointer`).
   - `fv.Type().Elem().Kind()` is `Map`. The Pointer switch
     (line 73-82) handles only `String` and `Struct`. Falls
     through.
4. Marker remains in `step.Vars["api_key"]`. Downstream:
   - Template renders that read `{{ api_key }}` produce the
     sentinel string verbatim (not the env-var value).
   - Subsequent steps that consume `api_key` (e.g. a `shell`
     step that uses it in a command) see the sentinel.
   - Redactor never learns the real value, so any unrelated path
     that does leak the env var (e.g. an unrelated `$MOONCAKE_API_KEY`
     in a shell command's output) is not redacted.

The user gets neither the secret nor an error. **Silent failure.**

## Same shape, additional sites

A scan of `config.Step` and the action structs for unhandled
kinds:

- **`Step.Vars` (*map[string]interface{})** — confirmed miss.
- **`[]Step` fields** (`OnChange`, `Transaction`, `Try`, `Catch`,
  `Finally`, `OnRollback`) — the resolver processes each step
  individually as the executor iterates the plan, so compound-step
  children get their own `Resolve()` call. **No miss here**
  (different control flow saves it).
- **`map[string]interface{}` in action data fields** — e.g.
  `events.PrintData.Data map[string]interface{}` (Print action),
  any action with a free-form Data field. Need to audit per-action.
- **`*config.Tool.Env map[string]string`** — handled (string→string).
- **`config.ServiceUnit.Content string`** — handled (string).

I haven't done the per-action audit; F019's claim is **at minimum
Vars is broken**.

## Why it's a bug (not a smell)

Reproducible:

```go
func TestResolver_StepVarsContainsMarker(t *testing.T) {
    t.Setenv("MC_TEST_VARS_SECRET", "leaked")
    m := map[string]interface{}{
        "api_key": security.SentinelPrefix + "env:MC_TEST_VARS_SECRET",
    }
    step := &config.Step{Vars: &m}

    redactor := security.NewRedactor()
    if err := Resolve(step, redactor); err != nil {
        t.Fatalf("resolve: %v", err)
    }
    got := (*step.Vars)["api_key"].(string)
    if got != "leaked" {
        t.Errorf("step.Vars marker not resolved: got %q", got)
    }
}
```

This test will fail today.

## Suggested fix

Extend `walkAndResolveSecrets` to handle two more cases:

```go
case reflect.Pointer:
    if fv.IsNil() {
        continue
    }
    switch fv.Type().Elem().Kind() {
    case reflect.String:
        if err := resolveStringInPlace(fv.Elem(), sf.Name, redactor); err != nil {
            return err
        }
    case reflect.Struct:
        if err := walkAndResolveSecrets(fv.Elem(), redactor); err != nil {
            return err
        }
    case reflect.Map: // NEW
        if err := resolveMapInPlace(fv.Elem(), sf.Name, redactor); err != nil {
            return err
        }
    }

// And update the Map case to handle `map[string]interface{}`:
case reflect.Map:
    if fv.Type().Key().Kind() != reflect.String {
        continue
    }
    if err := resolveMapInPlace(fv, sf.Name, redactor); err != nil {
        return err
    }

// resolveMapInPlace handles both map[string]string and
// map[string]interface{}, rewriting any string value that's a
// marker.
func resolveMapInPlace(fv reflect.Value, fieldName string, redactor *security.Redactor) error {
    for _, k := range fv.MapKeys() {
        v := fv.MapIndex(k)
        // For interface{} values, unwrap.
        if v.Kind() == reflect.Interface {
            v = v.Elem()
        }
        if v.Kind() != reflect.String {
            continue
        }
        cur := v.String()
        if !security.IsMarker(cur) {
            continue
        }
        val, _, err := security.ResolveMarker(cur)
        if err != nil {
            return fmt.Errorf("%s[%s]: %w", fieldName, k.String(), err)
        }
        if redactor != nil {
            redactor.AddSensitive(val)
        }
        fv.SetMapIndex(k, reflect.ValueOf(val))
    }
    return nil
}
```

This covers the Vars case and also `map[string]interface{}` in
any other action struct. Slice-of-Struct is a separate question
— defer until F019 lands and someone needs it.

## Open question

Should an unresolved marker at the end of `Resolve()` be a
**hard error** instead of silently passing through? A
verification pass:

```go
// At the end of Resolve(), walk again and check for any
// remaining markers. If found, return an error so the caller
// knows resolution was incomplete.
```

This would have caught F019 the day it shipped. Cost: a second
walk. Benefit: defense-in-depth — a future field added in a
shape the walker doesn't handle gets surfaced loudly instead of
silently.

## Verification

- Add the test above. Should pass after the fix.
- Run the full test suite — make sure no existing test relied on
  Vars-marker-pass-through (unlikely; it was a silent bug).

## References

- `internal/config/secret_tag.go` — the YAML pre-pass that
  guarantees markers reach all string scalars regardless of
  nesting depth.
- `internal/security.Redactor` — the denylist that's not
  populated when this resolver misses a value.

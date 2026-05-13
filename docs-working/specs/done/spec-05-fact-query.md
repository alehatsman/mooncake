# Spec 05: Fact Query

## Problem

`mooncake facts` dumps the full facts JSON — 50+ fields, verbose, expensive to
parse. An agent or script asking "is rust installed?" or "what go version?" has
to deserialize the whole blob and navigate it.

## Goal

Dot-path query into the facts tree. Returns a single scalar value.

```bash
mooncake facts --query go.version        → 1.26.3
mooncake facts --query rust.installed    → true
mooncake facts --query os                → linux
mooncake facts --query memory.free_mb   → 14540
mooncake facts --query package_manager  → pacman
```

Exit code 0 if key found and non-empty, 1 if not found or empty.
Empty output (not an error) if the value is null/empty.

## Path mapping

Facts are stored as a flat `map[string]interface{}` via `Facts.ToMap()`.
The query uses the same key names already in that map.

Examples of existing keys:
```
os                     → "linux"
arch                   → "x86_64"
distribution           → "arch"
hostname               → "thinkpad"
memory_total_mb        → 32768
memory_free_mb         → 14540
cpu_cores              → 16
cpu_model              → "Intel i7-1365U"
package_manager        → "pacman"
git_version            → "2.49.0"
go_version             → "1.26.3"
docker_version         → "27.4.0"
python_version         → "3.13.3"
```

After snapshot (spec-04) extends toolchains, new keys added there are
automatically queryable here.

Nested paths (e.g. `network.interfaces.0.name`) are out of scope for v1 —
flat keys only. If a key holds an array or object, return JSON encoding of it.

## CLI interface

```bash
# Query single value
mooncake facts --query go.version

# Multiple queries (print key=value pairs)
mooncake facts --query go.version --query rust.version --query os

# Existing full dump still works unchanged
mooncake facts
mooncake facts --format json
mooncake facts --output /tmp/facts.json
```

## Scripting usage

```bash
# Conditional in shell
if [ "$(mooncake facts --query rust.installed)" = "true" ]; then
  echo "rust is here"
fi

# In a mooncake when: expression (future — not v1)
when: "{{ shell('mooncake facts --query rust.installed') }} == 'true'"
```

## Implementation

`cmd/mooncake.go` `factsCommand` — add `--query` / `-q` flag (repeatable).

When `--query` is set:
1. Collect facts as usual (uses existing cache)
2. Call `facts.ToMap()`
3. For each query key, look up in map
4. Print value with `fmt.Println`; if multiple queries, print `key=value` per line
5. Exit 1 if any key not found

Key normalization: `go.version` → `go_version` (replace `.` with `_`).
This maps naturally to the existing ToMap() keys without needing a new namespace.

No new structs needed. ~20 lines of code in `factsCommand`.

## Out of scope

- Nested dot-path traversal into arrays/objects
- JSONPath or jq-style expressions
- Setting facts / overriding facts
- Querying registered step results (those live in variables, not facts)

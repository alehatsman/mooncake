---
id: F009
title: explain.DisplayFacts — gocyclo 44, only remaining over-cap function; split into per-section helpers
severity: smell
package: internal/explain
file: internal/explain/explain.go
lines: 55-213
status: done
resolved: 2026-05-16 — pure extraction. `DisplayFacts` now dispatches to 9 per-section helpers (`printFactsHeader`, `printSystem`, `printCPU`, `printMemory`, `printSoftware`, `printOllamaModels`, `printGPUs`, `printStorage`, `printNetwork`, `printNetworkInterfaces`) in declaration order. Each helper early-returns when its section is empty so trailing blank-line behavior matches the monolithic version. `//nolint:gocyclo` removed; `make budget-status` now reports "✓ all functions under 35". Existing 28 tests in `explain_test.go` pass unchanged because each helper still writes to stdout. Two micro-issues called out in the finding — (a) `filterRelevantCPUFlags` prefix-vs-substring drift and (b) `en/eth/wlan` interface-name allowlist hiding modern systemd names like `wlp*`/`enp*` and all Windows names — left as follow-ups; both are behavior changes deserving their own findings. The interface-allowlist has an inline TODO-style comment now so the next reader trips over it.
---

## What

`DisplayFacts` is the only non-test function over the gocyclo 35
cap today (`make budget-status` lists it as gocyclo 44). It's
annotated `//nolint:gocyclo` (line 54), so the linter doesn't
complain, but the soft-cap policy says the next touch should
refactor.

The function is **8 distinct print sections**:

| Section | Lines | What it prints |
|---|---|---|
| Header / system | 56-68 | OS, arch, hostname, kernel |
| CPU | 71-81 | cores, model, relevant flags |
| Memory | 84-92 | total/free/swap |
| Software & dev tools | 95-116 | optional per-tool versions |
| Ollama models | 119-131 | model list if installed |
| GPUs | 134-152 | vendor, model, memory, driver, CUDA |
| Storage | 155-168 | disk table |
| Network + interfaces | 171-212 | gateway, DNS, interfaces |

Each section is independent — no state shared, no early returns,
no flag combinations. The gocyclo number is bottom-up: every
`if f.X != ""` adds a branch.

Two helpers already exist (`filterRelevantCPUFlags`,
`storageTableWidths`), introduced in `27e9ade`. Extending the
same shape across the remaining sections is the residual work.

## Suggested fix

Pure extraction. No behavior change. Pattern:

```go
func DisplayFacts(f *facts.Facts) {
    printHeader()
    printSystem(f)
    printCPU(f)
    printMemory(f)
    if printSoftware(f) {
        // (printSoftware returns true if any field was non-empty,
        //  so we don't emit a trailing blank line for an absent section)
    }
    printOllamaModels(f)
    printGPUs(f)
    printStorage(f)
    printNetwork(f)
    printNetworkInterfaces(f)
}

func printCPU(f *facts.Facts) {
    fmt.Println("CPU:")
    fmt.Printf("  Cores:    %d\n", f.CPUCores)
    if f.CPUModel != "" {
        fmt.Printf("  Model:    %s\n", f.CPUModel)
    }
    if relevant := filterRelevantCPUFlags(f.CPUFlags); len(relevant) > 0 {
        fmt.Printf("  Flags:    %s\n", strings.Join(relevant, " "))
    }
    fmt.Println()
}
// ... etc.
```

Each helper is gocyclo < 10. `DisplayFacts` becomes
gocyclo ~3 (one call per section). The `//nolint:gocyclo` comment
goes away.

## Two adjacent micro-issues found while reading

These are cheap to fix as part of the same touch:

### (a) `filterRelevantCPUFlags` over-matches (line 22)

```go
if strings.HasPrefix(lower, prefix) || strings.Contains(lower, prefix) {
```

`strings.Contains` is a superset of `strings.HasPrefix`. The `||`
is redundant — `Contains` alone covers both. The function comment
on line 16 says **"case-insensitive prefix/contains match"** but
the variable name (`relevantCPUFlagPrefixes`) says prefix. They
disagree.

A real flag like `avx512_vnni` should match — both do today. A
flag like `non_avx_thing` would match `Contains` but not
`HasPrefix`. If the intent is **prefix**, drop the Contains; if
the intent is **substring**, drop the HasPrefix and rename the
variable to `relevantCPUFlagSubstrings`.

The substring intent is probably correct (`avx512_vnni` is the
sort of flag we want), so the rename is the right move.

### (b) Network-interface name allowlist is hardcoded (line 192-196)

```go
if !strings.HasPrefix(iface.Name, "en") &&
    !strings.HasPrefix(iface.Name, "eth") &&
    !strings.HasPrefix(iface.Name, "wlan") {
    continue
}
```

This hides interfaces named `wlp*` (modern systemd predictable
names — common on most Linux distros), `wlx*` (USB Wi-Fi), `usb*`
(Android tethering), `tun*` / `tap*` / `wg*` (VPN), and *all*
Windows interface names (`Ethernet0`, `Wi-Fi`, etc.).

The intent ("hide loopback, docker bridges, virtual interfaces")
is reasonable, but the implementation is **a denylist masquerading
as an allowlist**. Options:

- Replace with a denylist: skip names matching `lo`, `docker*`,
  `br-*`, `veth*`, `virbr*`, `vmnet*`, `tailscale*` (if user wants
  it hidden — debatable).
- Replace with: hide any interface with no IPv4 address (the
  `len(iface.Addresses) > 0` filter at line 183 already partly does
  this).
- Replace with: show all "up & has addresses" interfaces; let
  users filter via grep.

The third option is the simplest and most user-respecting; the
current behavior silently hides modern Wi-Fi interfaces on common
Linux distros.

## Expected payoff

- **gocyclo on `DisplayFacts` drops below 35** without the nolint.
- **The hardcoded `en/eth/wlan` allowlist becomes visible** as a
  single-purpose helper, making future audits easy.
- **Section-tests become possible** — currently the whole function
  is "print to stdout" so testing is awkward; per-section helpers
  can write to an `io.Writer` and unit-test the output.

## Tests

`internal/explain/explain_test.go` exists with 28 test functions
covering most sections. They use `os.Pipe` to capture stdout and
assert on substrings. Two notes on the existing tests:

- They will keep passing after the per-section split (each helper
  still prints to stdout) — no test churn needed.
- `TestDisplayFacts_NilFacts` (line 1066) is a dead test: it has
  the `defer recover()` setup but never calls `DisplayFacts(nil)`.
  The comment ends with "we just verify it panics predictably"
  but the actual panic call is absent. Either delete the test or
  add `DisplayFacts(nil)` (and decide what the function should do
  with `nil` — today it would nil-deref on the first `f.X` read).
  See F010.

## Verification

- `make budget-status` — `explain.DisplayFacts` falls off the
  gocyclo list.
- `gocyclo -over 35 internal/` — zero hits.
- Visual diff of `mooncake doctor` output before/after.

## References

- `27e9ade` introduced `filterRelevantCPUFlags` and
  `storageTableWidths` — the established pattern.

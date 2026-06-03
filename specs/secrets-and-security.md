---
id: secrets-and-security
status: draft
owners: [aleh]
covers:
  - "internal/config/secret_tag.go"
  - "internal/secrets/resolver/**"
  - "internal/security/**"
  - "internal/executor/policy.go"
---
# Secrets and Security

## Intent

Secrets and the security layer are how mooncake keeps the typed-funnel safe:
sensitive values reach the host without ever materializing where they can leak,
privilege escalation runs through one auditable primitive, and a per-run policy
replaces the host permission wall the actor gives up when execution moves into
the kernel. A `!secret <provider>:<path>` reference flows through plan
compilation as an opaque marker and is resolved to its real value only at apply
time, immediately added to the run's redactor denylist so it cannot surface in
plans, diffs, events, logs, or step output. Escalation is probed once per run
and applied through a single `sudo` wrapper rather than six hand-rolled call
sites. This spec applies the constitution's "secrets never leak" and "policy
replaces the permission wall" principles to concrete behavior.

## Behavior

- WHEN the YAML decoder parses a scalar tagged `!secret`, it rewrites the node
  to a sentinel-marker string (`SentinelPrefix + "<provider>:<path>"`) tagged as
  a plain string, so the secret ref flows into any action string field without
  action structs needing a typed secret union.
- IF a `!secret` scalar already carries the sentinel prefix, or its value is
  empty/non-scalar, the pre-pass leaves it untouched so re-parsing is idempotent
  and the schema validator surfaces the misuse.
- WHILE a plan is compiled, the marker is never resolved; it survives compilation
  unchanged so the in-memory plan and `mooncake plan --format json` re-render it
  as `"!secret <ref>"` and never carry the real value.
- WHEN a plan is applied (not in plan/dry-run mode), the resolver walks each
  step's typed action fields, replaces every marker via the security registry,
  and appends each resolved value to the supplied Redactor's denylist.
- WHERE the run is in plan/dry-run mode, the resolver is not invoked; markers
  stay as markers and are redacted at JSON-output time.
- WHERE a secret ref is `env:<NAME>`, the value comes from the controller's
  process environment, and an unset or empty variable is an error (an empty
  value is never propagated as a secret).
- WHERE a secret ref is `file:<path>`, the value is the file's contents with one
  trailing newline stripped (k8s Secret-mount convention); a leading `~/` expands
  to the user's home, but no globbing or env expansion is applied.
- WHERE a secret ref is `stdin:<key>`, the value is prompted interactively on the
  controlling TTY and cached per-process per-key; a non-TTY environment is
  refused rather than hanging on a blocked read.
- IF a provider fails to resolve a ref, the error names only the provider prefix
  (`vault: ...`) and never quotes the path or value, so partial-secret data does
  not leak into shared logs.
- WHEN a Redactor redacts text, it replaces every registered sensitive value
  (matched longest-first to avoid partial-substring gaps) and every registered
  regex pattern with `[REDACTED]`, and walks maps/slices to redact string leaves
  deeply while leaving map keys intact.
- WHEN a run starts, escalation availability is probed exactly once into an
  `EscalationReport` (already-root, password-supplied, passwordless NOPASSWD, or
  a typed blocked reason — NoNewPrivs, sudo-missing, insecure-sudoers,
  probe-failed) carrying an operator-facing remediation hint.
- WHERE a step needs root, escalation runs through one primitive
  (`ctx.Privileged()` → `PrivilegedRunner` → `BecomeRunner`): already-root execs
  directly, otherwise it wraps `sudo -S` (password on stdin) or `sudo -n`
  (passwordless), and become-on-an-unsupported-platform or become-with-no-
  password yields a single diagnosable error class rather than a hung prompt.
- WHERE a run carries a Policy, the executor enforces it at preflight before any
  step's side effects, in order denylist → allowlist → deny-network → max-risk,
  and the first violation stops the run with an error naming the rule and action.
- WHILE a Policy is nil or zero-valued, nothing is enforced and every step is
  allowed, so existing CLI/fleet/agent/test run paths are unchanged unless a
  caller opts in.

## Non-goals

- **An expressive policy DSL.** Policy is a flat allow/deny/network/risk struct,
  not an OPA/Rego-style language; richer agent-intent gating belongs to the
  agent-safety spec.
- **A secrets manager / vault.** Mooncake resolves refs from existing providers
  (env/file/stdin); it does not store, rotate, encrypt-at-rest, or distribute
  secrets.
- **Validating the operator's sudo password.** The probe trusts a supplied
  password rather than testing it against sudoers (that has side effects).
- **Resolving markers outside step action fields.** Plan-time variables,
  metadata, and template inputs are resolved by their own subsystems, not here.

## Checklist

- [x] `!secret` YAML pre-pass rewrites tagged scalars to sentinel markers, idempotently
- [x] Markers survive plan compilation; plan JSON re-renders `!secret <ref>`, never the value
- [x] Apply-time resolver replaces markers and adds resolved values to the Redactor denylist
- [x] Plan/dry-run mode never resolves markers
- [x] env / file / stdin providers with their documented value semantics
- [x] Provider errors redact the path; resolved values never quoted in errors
- [x] Redactor: longest-first substring + regex patterns + deep value walk, keys preserved
- [x] Once-per-run escalation probe with typed reasons + remediation hints
- [x] Single escalation primitive (`ctx.Privileged()`); `sudo -S`/`-n`; single error class
- [x] Per-run Policy gate at preflight: deny → allow → network → risk; first violation stops
- [ ] Vault / age / 1Password secret providers (only env/file/stdin ship today)

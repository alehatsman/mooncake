# Proposal 05: Error recipes — every error returns a "fix:" line, like doctor does

**Status:** Draft proposal
**Effort:** M (~5 days, cross-cutting; one PR per error category)
**Value:** High — `mooncake doctor`'s "fix: <action> — <URL>"
pattern is the project's best DX win. Most errors elsewhere don't
follow it.

---

## Problem

`mooncake doctor` already nails the pattern:

```
Tools
  ⚠ git not on PATH
       fix: install git — https://git-scm.com/downloads
       used by: git.* actions
  ⚠ sudo not on PATH
       fix: install sudo, or run as the target user directly
       used by: as_user:, become:, package installs
```

But error messages elsewhere often stop at "what":

```
command failed with exit code 1
overlay file not found for host "nonexistent": /work/vars/by-host/nonexistent.yml does not exist
failed to install packages [jq]: exec: "sudo": executable file not found in $PATH
inappropriate ioctl for device
```

The user knows *what* failed; the next question — *what should I
do?* — is unanswered. Some examples have the doctor template
already; many don't.

Receipts from audit:

| Error | Has fix: line? |
|---|---|
| #1 `as_user:root` no sudo | No |
| #14 sha256 mismatch (post-MT-14) | Has the comparison (`declared X, actual Y`); no fix line |
| #44 unknown field | Yes! Points at docs-next/guide/config/actions.md (MT-77 win) |
| #50 file.copy on directory | "use recursive copy action instead" — names nothing actionable |
| #66 bad `--max-plan-age` value | No fix line; help-dump precedes the error |
| #78 peers.toml dotted-table | No; "toml: cannot store a table in a slice" verbatim |
| #81 `as_user: alice` missing sudo | Generic "command failed" |
| #85 `--ask-become-pass` no TTY | Generic "inappropriate ioctl" |

Two-thirds lack the fix line. Many also lack the "WHY" line (why is
this the answer, not just "do X").

## Proposal

Adopt three lines per non-trivial error, doctor-style:

```
<error description; what failed>
  → why: <one-line cause; the model the system has of what went wrong>
  → fix: <the action to take; with URL if external>
```

Example transformations:

**Before (#78):**
```
parse /root/.config/mooncake/peers.toml: toml: cannot store a table in a slice
```

**After:**
```
peers.toml: cannot store a table in a slice
  → why: `[peers.local]` is dotted-table form; `[[peers]]` array-of-tables is expected.
  → fix: change to:
         [[peers]]
         name = "local"
         addr = "127.0.0.1:7878"
         token = "..."
         (see docs-next/guide/fleet/peers.md, or run `mooncake fleet init`)
```

**Before (#1 originally; MT-1 fixed it):**
```
failed to install packages [jq]: exec: "sudo": executable file not found in $PATH
```

**After (the MT-1 fix already removed this case for root):**
```
sudo: executable file not found in $PATH
  → why: as_user:<non-root-user> needs sudo to switch users.
  → fix: install sudo on the host, or run mooncake as the target user directly.
         If the playbook only changes things the current user owns,
         drop `as_user:` from the step.
```

**Before (#85):**
```
sudo password setup failed: failed to resolve password:
  failed to read password: inappropriate ioctl for device
```

**After:**
```
--ask-become-pass needs an interactive terminal.
  → why: this process has no TTY on stdin (likely CI or piped invocation).
  → fix: use `--sudo-pass-file <path>` (file must be chmod 0600).
         For one-shot scripts: `MOONCAKE_SUDO_PASS=<...> mooncake apply ...`
         (env-var support: see proposal-06).
```

## Implementation strategy

Catalog every error site in `internal/`:

```bash
$ grep -rn 'fmt.Errorf\|errors.New\|return.*errors.*' internal/ | wc -l
~400 sites
```

Not every error needs the recipe template. Apply where the error
is exposed to humans (planner, executor, fleet, doctor, validator,
agentd request handlers).

For each, define:
1. **What** (the existing message)
2. **Why** (the system's diagnosis)
3. **Fix** (concrete next action with link/example when possible)

Implementation pattern:
```go
type Diagnostic struct {
    What string
    Why  string
    Fix  string
}

func (d Diagnostic) Error() string {
    out := d.What
    if d.Why != "" { out += "\n  → why: " + d.Why }
    if d.Fix != "" { out += "\n  → fix: " + d.Fix }
    return out
}
```

Convert errors site-by-site. Don't try to do all 400 at once;
prioritize:
- Validator errors (MT-77 already gets this for unknown-field)
- Top-3 most-hit errors per command (instrument with a counter and
  capture distribution over a release cycle)
- Errors that fire during onboarding (`apply` no-config, `init`
  no-template, `fleet bootstrap` no-key, `peers.toml` parse, etc.)

## API

No new flags. Always-on by default. Two new shapes:

1. **`--no-recipes`** — strip the `→ why` / `→ fix` lines, for
   scripts that grep only the first line. (Mostly cosmetic; the
   "what" line stays anchored.)
2. **`mooncake doctor` style sectioning when multiple errors**:
   ```
   2 errors in /work/cfg.yml:

   Error 1
     unknown field `sha256` at line 5
     → why: file.download uses `checksum:`, not `sha256:`.
     → fix: rename the field.

   Error 2
     unknown field `name` at line 7
     → why: pkg.list takes no filters in v1.
     → fix: drop the field; filter the result via `as:` + a template.
   ```

## Receipts

Specific errors I hit during audit that would benefit:

| Error | Recipe |
|---|---|
| Generic "command failed with exit code 1" (#81) | `→ why: as_user: alice ran via sudo; sudo not on PATH OR alice doesn't exist. → fix: install sudo / run os.user: { name: alice, state: present } first.` |
| #67's nested-try error (already excellent post-fix) | Keep this as the template. `→ fix:` already points at `continue_on_error: true`. |
| `text.replace` no-match (now idempotent post-MT-47) | If still surfaces in `--strict` mode: `→ fix: use `unless: grep -q ...` to gate, or `must_match: false` for opt-in idempotent.` |
| TLS error in vanilla ubuntu (#21) | `→ why: container has no /etc/ssl/certs. → fix: apt-get install ca-certificates (or use mooncake's official alpine image which includes them).` |
| `Step must have exactly one action ()` (#77; partial fix landed) | `→ why: step has 2+ keys mooncake recognized as actions. → fix: split into separate steps OR check for accidentally step-level fields (creates:/unless:/when: belong at step level, not nested under the action).` |

## Why this lives in dx, not core

Every stream's errors flow through here. Core *produces* errors;
DX *renders* them. Treating "error message" as a UI artifact and
not a core concern lets us iterate on copy without touching action
handlers. The Diagnostic type lives in `internal/errors/` and is
consumed by the formatters proposed in proposal-02.

## What this doesn't address

- **i18n** — recipes are English. Same caveat as everywhere.
- **Recipe drift** — when an action's API changes, the recipe for
  its old errors goes stale. Mitigation: link recipes to actions
  (`Recipe.ForActionVersion("file.copy", "1.0.0")`) so version
  bumps prompt review.
- **AI-generated recipes** — could ask an LLM to produce a recipe
  per error site. Out of scope; do the curated pass first.

## Companion: the "single source of truth" thread

Recipes for errors that come from schema mismatches should be
auto-generated:
- "unknown field X" → look up the action's schema → suggest the
  closest valid field (Levenshtein < 3) → emit "did you mean: `Y`?"
- Already partially done in MT-77 ("likely a typo or a renamed
  field"). Extending to fuzzy suggestions is the next step.

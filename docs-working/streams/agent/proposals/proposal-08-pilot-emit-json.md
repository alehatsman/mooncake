# Proposal 08: pilot emits compact JSON instead of YAML

**Status:** Draft proposal
**Effort:** XS (~half day; ~3 small file edits + eval-fixture refresh)
**Value:** High — every pilot turn currently spends ~30% of its output budget
on YAML syntax overhead (indentation whitespace, quote rules) instead of plan
content. Independent of prompt-cache, multi-turn, and provider work.
**Claim slug:** `pilot-emit-json`
**Depends on:** config-json-input (shipped 2026-05-27, commit `93ee3c37`)
**Blocked by:** nothing — landed work is the unblocker

---

## Problem

`internal/pilot/prompt.go` instructs every model to emit a YAML plan:

```
prompt.go:12     - Output ONLY raw YAML (Mooncake RunConfig format)
prompt.go:105    Output ONLY a YAML array of steps (starting with -), no other text.
prompt_styles.go:30   ... Output a YAML plan containing EXACTLY ONE step.
prompt_styles.go:33   ... emit an empty plan (the YAML literal `[]`) ...
```

`internal/pilot/sanitize.go:17,25,27` then strips ```yaml fences and calls
`yaml.Unmarshal` on the body.

Two costs:

1. **Tokens.** Compact JSON for a typical 5-step plan emits ~30% fewer output
   tokens than the equivalent YAML. Pilots are output-bound (input is cached;
   output is not). 30% off every turn compounds across multi-turn loops.

2. **Reliability.** YAML indentation mistakes are the single most common
   model-output failure on small local models (Llama-3.1-8B, Qwen-2.5-7B).
   JSON has no indentation rules — only brace/bracket structure — so the
   class of error disappears.

`config-json-input` (commit `93ee3c37`) made `mooncake apply`,
`mooncake step`, presets, and variables files all accept JSON via auto-detect.
The decoder path is in place; pilot just doesn't use it yet.

## Proposal

Three small changes, one PR.

### 1. Prompts — swap "YAML" → "compact JSON"

`internal/pilot/prompt.go`:
- Line 12: `Output ONLY raw YAML (Mooncake RunConfig format)`
  → `Output ONLY a compact JSON array of steps (Mooncake RunConfig format), no other text`
- Line 105: `Output ONLY a YAML array of steps (starting with -), no other text.`
  → `Output ONLY a compact JSON array of steps, no other text. Example: [{"shell":{"cmd":"echo hi"}}]`

`internal/pilot/prompt_styles.go`:
- Line 30: `Output a YAML plan containing EXACTLY ONE step`
  → `Output a JSON plan containing EXACTLY ONE step (a single-element array)`
- Line 33: `emit an empty plan (the YAML literal \`[]\`)`
  → `emit an empty plan (the JSON literal \`[]\`)`

### 2. Sanitize — accept both, prefer JSON

`internal/pilot/sanitize.go`:
- Rename `extractYAMLFromFences` → `extractFromFences`; recognize both
  ` ```yaml ` and ` ```json ` fences.
- Swap `yaml.Unmarshal` → `config.DecodeAuto`. The auto-detect already handles
  YAML and JSON, so models that ignore the new prompt and still emit YAML
  keep working (graceful transition).

### 3. Eval-harness fixtures

`testing-next/pilot-evals/` has fixtures with expected YAML output. Two
options:
- **(a)** Regenerate fixtures by running each eval against a known-good
  provider once (Claude); commit the new expected JSON.
- **(b)** Loosen the assertion to "parses cleanly into the expected step
  shape regardless of format" — drop string equality on the raw output.

Recommendation: **(b)** is more robust long-term. The eval cares whether
the *plan* is right, not whether it's byte-identical to a frozen string.

## Token math (back-of-envelope)

For a typical 5-step plan with `shell` + `file.write` + `pkg` mixed:

| Form | Approx output tokens |
|---|---|
| YAML (current) | ~600 |
| Compact JSON (proposed) | ~420 |
| **Savings** | **~30%** |

On a 10-turn pilot loop emitting 5 steps each turn: ~1.8K tokens saved per
loop, ~$0.009/loop on Sonnet at current rates. The gain is bigger on local
providers (no caching, raw token cost dominates).

## Risks and non-issues

- **Schema validation** — already accepts JSON inputs since `93ee3c37`.
  No change needed.
- **Existing YAML-emitting models** — sanitize's auto-detect handles them.
  No flag day.
- **Downstream consumers reading pilot's emitted file** — pilot writes via
  the existing transaction-wrap path, which serializes through `yaml.Marshal`.
  That stays unchanged; only the model's *response* format changes. The
  file on disk is still YAML.
- **Tool-use / structured output paths** (proposal-05.2 territory) —
  orthogonal. Tool-use bypasses the prompt-text format entirely.

## Out of scope

- Prompt caching strategy (separate story: `pilot-prompt-cache`)
- Tool-use spike (separate story: `pilot-tool-use-spike`)
- Changing the on-disk run-config format
- Updating CLAUDE.md / docs (the change is invisible to humans editing configs)

## How another agent picks this up

1. Check `~/.mooncake/claims.jsonl` for an active claim on `pilot-emit-json`.
   If none: append a `claimed` line.
2. Create worktree: `git worktree add ../mooncake-pilot-emit-json -b worktree-pilot-emit-json`
3. Make the four prompt edits + sanitize swap.
4. Choose fixture approach (a or b above) and update `testing-next/pilot-evals/`.
5. Run `mooncake task pilot-eval` (or whatever the eval harness target is —
   check `tasks.yml` after the worktree merges in current master).
6. Smoke: `mooncake pilot --provider anthropic --prompt "create /tmp/foo with hi"`
   and confirm the model emits JSON that parses and applies.
7. Commit, push branch:master, file PR.

Total effort: ~half day including eval-fixture work.

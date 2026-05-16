# Bug — `artifact.capture` silently drops `capture_content` / checksum / size fields

**Tracking:** [#27](https://github.com/alehatsman/mooncake/issues/27)
**Surfaced:** 2026-05-15 during tick-6 of the autonomous test loop —
artifact.capture investigation (post-MT-24 fix).

## Repro

```yaml
- name: capture-multi
  artifact.capture:
    name: multi-ops
    output_dir: /tmp/artest/artifacts
    format: json
    capture_content: true        # ← docs: "Include before/after file content"
    # include_checksums defaults to true (per the schema)
    steps:
      - name: create
        file.write: { path: /tmp/artest/will-create, content: "NEW\n" }
      - name: update
        file.write: { path: /tmp/artest/will-update, content: "UPDATED\n" }
```

The generated `changes.json` has per-file entries with only these
keys:

```
chars_added, file_type, is_test_file, language,
lines_added, lines_modified, lines_removed, lines_total,
operation, path
```

Missing — despite `capture_content: true` and
`include_checksums` defaulting true:

- `content_before`, `content_after` (or any byte-content field)
- `size_before`, `size_after`, `size_bytes`
- `checksum_before`, `checksum_after`, `sha256`

The summary block's `total_size_delta: 0` is also wrong — the test
created 4 bytes and updated 8 bytes (counting `\n`), so the delta
should be non-zero. `total_chars_added: 12` is correct, so the
line/char tracking pipeline works; only the byte-size /
checksum / content pipelines are dark.

## Root cause — the handler comment admits it

`internal/actions/artifact_capture/handler.go`:

```go
if capture.CaptureContent {
    // Read content if requested
    // Note: before content not available in current implementation
    ...
}
```

And the FileChange struct is wired to set `SizeBytes`,
`ChecksumBefore`, `ChecksumAfter`:

```go
// in handler.go
return FileChange{
    ...
    SizeBytes:      data.SizeBytes,
    ChecksumBefore: data.ChecksumBefore,
    ChecksumAfter:  data.ChecksumAfter,
}
```

But the upstream `FileOperationData` event published by the
file actions doesn't carry the values, so the read-back ends up
empty. Combined with `omitempty` on the JSON tags, the fields
vanish from the output without an error.

## Why this matters

The action's struct doc says:

```go
// Designed for LLM agent loops to provide structured output for decision-making.
```

For an agent loop, the empty-but-present-in-spec fields are the
worst kind of API: the agent's prompt sees `capture_content: true`
in the example, plans against the implied schema, then gets back
data without `content`/`checksum`/`size` and has no way to know
whether the field is missing because the file was unchanged or
because the producer dropped it.

Three concrete failure modes for agent consumers:

1. **Diff verification impossible**. Without `content_after` or
   even a checksum, the agent can't verify the file landed with
   the intended bytes — it has to re-read the file separately.
2. **No size budgeting**. spec-30's transactions promise
   diff-size guards; with no per-file size in artifact.capture,
   budget enforcement is loose.
3. **Agent prompt drift**. As prompts mature and start asking
   "did the new file's content match the policy?", the missing
   `content_after` will silently fail every check.

## Fix

Two halves, both shippable independently:

### A. Populate per-file size + checksum

Wire `FileOperationData.SizeBytes` / `.ChecksumBefore` /
`.ChecksumAfter` at the file-action emit site (file.write,
file.template, file.copy, file.download). The event publisher
chain already accepts these fields; they just aren't being set.

This is the cheap win — completes the existing struct/handler
contract, doesn't change the event protocol.

### B. Implement `capture_content`

If the operator explicitly opts in to `capture_content: true`,
the artifact-capture machinery should re-read the file post-step
(or, for events that carry bytes inline, capture from the event).
Pre-change content is harder — for create-only steps there's no
"before"; for update steps the tracker would need to read the
file at the *start* of the artifact.capture step. Either
implement that, or rename the field to
`capture_after_content: true` and document the absence of
before-content explicitly.

The handler's existing comment ("Note: before content not
available in current implementation") is honest about the gap;
the field name + docs hide it from the consumer.

## Test gap

`internal/actions/artifact_capture/mt24_test.go` (the recent
fix's test) asserts the tracker captures events post-flush — but
asserts nothing about the content of `SizeBytes` / `Checksum*` /
content fields. An end-to-end golden-file test of `changes.json`
would catch this.

## Workaround

Operators currently have to re-stat / re-hash files separately
after the run if they need size/checksum/content. That defeats
artifact.capture's purpose as an LLM-agent-consumable summary —
the agent would have to read the artifact, then do its own
filesystem inspection. Better to file's source of truth and just
not use `capture_content` until B lands.

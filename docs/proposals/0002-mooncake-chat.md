Mooncake Chat — Summary

Purpose
mooncake chat is an LLM-driven front-end to Mooncake that generates provisioning configs, validates them with the Mooncake engine itself, repairs errors via a feedback loop, and applies changes only after user approval. Correctness is enforced by the engine, not trust in the LLM.

Core Flow

User describes desired setup (e.g., “set up nginx”).

LLM generates a scaffold:

Mooncake config(s)

Supporting files (vars, templates, README).

Scaffold is written to a temporary workspace.

Mooncake runs dry-run against the temp workspace.

If errors:

Engine diagnostics are fed back to the LLM.

LLM regenerates a full corrected scaffold.

Loop repeats (bounded).

If dry-run succeeds:

Show plan, file list, and diff vs real workspace.

On user approval:

Mooncake applies the scaffold to the real workspace using its own engine.

Why Temp Folder First

Uses the real Mooncake CLI and semantics.

No engine refactor required.

Zero risk to user workspace until approval.

Deterministic validation and reproducible results.

Can later be replaced by an in-memory/VFS approach.

Repair Loop

Triggered by real engine errors (parse, schema, missing files, unsupported actions).

Feedback includes structured diagnostics (file, line, action, message).

LLM must return a full replacement scaffold each iteration.

Iteration cap prevents infinite loops.

Converges on valid Mooncake programs, not “looks correct” output.

Safety Model

Temp-only writes until explicit acceptance.

Action allowlist during scaffold validation (no shell, no sudo).

Path sandbox (no absolute paths, no .., scoped directories).

Dry-run guarantees no side effects.

Apply phase dogfoods Mooncake (directory/file/template actions).

User Experience

Default: no YAML exposure.

Clear preview of what will happen.

Diff shown before apply.

Explicit accept step.

Casual-developer friendly: “set up” language, not infra jargon.

Evolution Path

Now: mooncake chat "set up nginx"
Free-form prompt, validated, safe, explain-first.

Next: Presets
mooncake chat nginx, mooncake chat node.

Later: Command sugar
mooncake setup nginx (internally uses mooncake chat).

Future:

Virtual FS overlay (replace temp folder).

Structured JSON diagnostics.

patch action for safe edits.

Undo/reset primitives.

Positioning

Not a general coding agent.

Not an IDE replacement.

A deterministic, validated LLM front-end to a declarative provisioning engine.

Pitch
Describe the system you want in plain language; Mooncake proves it’s valid before touching your machine.

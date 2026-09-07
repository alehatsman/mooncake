# mooncake

Declarative config-management tool (Go) — safe, idempotent execution runtime.
Path resolution is Node-style: relative paths resolve from the dir of the YAML
file declaring the step.

Reference:

- **Orientation** — `dex query --kind=orient` (live; use it instead of a
  checked-in overview, which is what went stale here).
- **Generated docs** — [dist/docs/](./dist/docs/), indexed by
  [llms.txt](./dist/docs/llms.txt): one page per action, the CLI surface,
  concepts, schema. Regenerated and verified byte-for-byte by `task ci`.
- **Design intent** — [specs/](./specs/).

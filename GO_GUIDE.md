# Mooncake Go Code Guide

> **Audience**: humans and AI agents writing Go for this repo.
> **Status**: living document. Reads top-to-bottom; sections are independent.
> **Companions**: [`CLAUDE.md`](./CLAUDE.md) (rules), [`AGENT.md`](./AGENT.md)
> (shortcuts), [`LLM_GUIDE.md`](./LLM_GUIDE.md) (navigation),
> [`docs-working/vision/kernel.md`](./docs-working/vision/kernel.md)
> (what the project is).

This guide is opinionated. The opinions are not abstract — most of them
come from PRs that went sideways, linter rules we explicitly disabled,
or refactor work that wished the rule had existed already. Where a
section restates something already in `.golangci.yml` or `CLAUDE.md`,
the rationale is here, not duplicated; cross-references are explicit.

It is also pragmatic. The goal is not to win style debates. The goal is
that someone reading code six months later — including a junior dev or
an LLM that has never seen this file before — can pick it up without
ceremony.

---

## Table of contents

1. [The mindset](#1-the-mindset)
2. [How to use this guide](#2-how-to-use-this-guide)
3. [Packages](#3-packages)
4. [Files within a package](#4-files-within-a-package)
5. [Naming](#5-naming)
6. [Functions](#6-functions)
7. [Types and structs](#7-types-and-structs)
8. [Interfaces](#8-interfaces)
9. [Errors](#9-errors)
10. [Context and cancellation](#10-context-and-cancellation)
11. [Concurrency](#11-concurrency)
12. [Logging and observability](#12-logging-and-observability)
13. [State and purity](#13-state-and-purity)
14. [The handler ABI](#14-the-handler-abi)
15. [Idempotency and dry-run](#15-idempotency-and-dry-run)
16. [Templates and variables](#16-templates-and-variables)
17. [Filesystem and subprocess](#17-filesystem-and-subprocess)
18. [Comments and documentation](#18-comments-and-documentation)
19. [Testing](#19-testing)
20. [Performance](#20-performance)
21. [Dependencies](#21-dependencies)
22. [Architecture soft caps](#22-architecture-soft-caps)
23. [Linter discipline](#23-linter-discipline)
24. [Pre-PR checklist](#24-pre-pr-checklist)
25. [Anti-patterns](#25-anti-patterns)
26. [Appendix A: Ousterhout, applied](#appendix-a-ousterhout-applied)
27. [Appendix B: A worked example](#appendix-b-a-worked-example)

---

## 1. The mindset

Three sentences first.

1. **Complexity is the enemy.** Every line of code, every option, every
   interface, every layer of abstraction is paid for forever. The
   cheapest line is the one we did not write.
2. **Three similar lines beat a premature helper.** Wait until you have
   three concrete users before you abstract. Duplication you can see is
   safer than indirection you have to chase.
3. **Code is read more than written.** Optimise for the reader's
   working memory, not the writer's keystrokes. A clear long name beats
   a clever short one.

Everything else in this document is a corollary.

### The two failure modes

Most bad Go code in this repo falls into one of two categories:

- **Tactical clutter**: a function patched eight times by eight people,
  each adding "just one more flag." Eventually nobody understands the
  whole. The fix is usually a small refactor *before* adding the ninth
  thing — never as a parallel cleanup PR.
- **Strategic over-engineering**: someone reads *A Philosophy of
  Software Design*, encounters "deep modules," and immediately
  abstracts a working 80-line function behind three interfaces and a
  factory. The fix is to delete the abstraction. Wait for the second
  caller.

Ousterhout's point is that *good* abstractions are deep. Bad
abstractions — shallow ones, premature ones — are worse than the
duplication they replace.

### What "production-grade Go" means here

Not what FAANG style guides mean. Mooncake is a learning / demo /
pre-release project (see `LLM_GUIDE.md`, "Reshape freely"). There are
no shipped users to protect. So:

- We delete instead of deprecating.
- We rename instead of aliasing.
- We change defaults when defaults are wrong.
- We do not carry "old behavior" forward "in case someone depends on it."

If you find yourself adding a compatibility shim, stop and ask whether
you should be deleting the old thing instead.

### "Junior-dev readable" is a hard constraint

Not a nice-to-have. If a competent Go developer with zero prior
exposure to this codebase cannot read a function and explain it back
within a minute, the function is wrong. The fix is rarely a comment;
it is usually clearer names, fewer branches, or fewer parameters.

Junior-dev-readable code is a forcing function on architecture. You
cannot hide a bad design behind clear local code for very long.

---

## 2. How to use this guide

- **Reading order**: top-to-bottom on first pass. After that, the table
  of contents is the index.
- **Authority order when they conflict**:
  1. `CLAUDE.md` (project hard rules)
  2. `.golangci.yml` (mechanical enforcement)
  3. This document (style, structure, philosophy)
  4. Personal taste
- **Soft caps**: this document does not replace `make budget-status`.
  Run that before any structural change. See §22.
- **When in doubt**, optimise for the next reader. The next reader is
  often you, six months from now, on a Friday.

---

## 3. Packages

### One package per action

Every action handler lives in its own package under `internal/actions/`:

```
internal/actions/git_clone/
    handler.go        # core: Metadata, Validate, Run
    diff.go           # opt-in: actions.Differ
    reverse.go        # opt-in: actions.Reverser
    cost.go           # opt-in: actions.Coster
    credentials.go    # private helpers, only used here
    handler_test.go   # main tests
    diff_test.go      # diff-specific tests
    reverse_test.go   # reverse-specific tests
```

This is not just convention; it is load-bearing:

- Each handler owns its imports. A handler with no network needs
  imports nothing related to HTTP.
- New handlers do not edit a dispatcher. They `Register()` themselves in
  `init()`, and they are dispatched by name.
- A handler's package is the unit of "is this self-contained enough?"
  When a handler grows past ~1,500 LOC, see §22.

### Package naming

- Lowercase, no underscores **except** when they mirror an action name
  with dots: `git_clone` mirrors `git.clone`, `text_patch_json` mirrors
  `text.patch.json`. The linter exclusion at `internal/actions/` in
  `.golangci.yml` is exactly for this.
- One word is best. Two when unavoidable. Three is almost always wrong.
- Don't repeat the package name in exported identifiers:
  `git_clone.GitCloneSnapshot` reads badly outside the package, but
  inside it is unambiguous. Prefer `git_clone.Snapshot` when the type
  is obviously rooted to the package.

### What does NOT deserve its own package

- "Utilities." If a thing is so general it could be a util, it usually
  belongs in the package that uses it. `internal/utils/` exists; it is
  not a model to imitate. New code should not add to it.
- A single struct used only by another package's tests. Put it next
  to the tests, with a `_test.go` suffix so it doesn't ship.
- A "config" package per feature. Configuration lives in
  `internal/config/config.go` because the schema is a unit. Splitting
  it across packages would scatter the closed action set.

### What deserves its own package

- A handler (always).
- A capability that is truly cross-cutting (template rendering,
  expression evaluation, the executor, the planner).
- An external integration with its own state (the LLM client, the
  fleet transport).

If you are about to create a new top-level `internal/` package, run it
past the project owner first. The kernel guard rails (see kernel.md)
make new packages a real architectural decision, not a tidiness move.

### `internal/` is a wall, not a fence

Anything outside `internal/` is API. Anything inside is yours to
reshape. Use this. Move types down into `internal/` as soon as you
realise they don't need to be public. Move them back out only on a real
external need, not "what if someone wants to embed us."

---

## 4. Files within a package

### Splitting by responsibility, not by alphabet

A handler package typically splits like this:

```
handler.go          # required: Metadata, Validate, Run
diff.go             # if implementing Differ
reverse.go          # if implementing Reverser
cost.go             # if implementing Coster
permissions.go      # if Permissions is non-trivial; otherwise it lives in handler.go
<topic>.go          # cohesive private logic (e.g. credentials.go, backoff.go, exec_unix.go)
<topic>_test.go     # tests for the same file
```

Rules:

- **Test file goes next to the file it tests.** `diff.go` →
  `diff_test.go`, not in a separate `tests/` directory.
- **Platform-specific code uses build tags.** `exec_unix.go` and
  `exec_windows.go` in `internal/actions/shell/` are the model. The
  shared API lives in `handler.go`; the per-OS implementation is
  selected at compile time.
- **One file should fit in a reader's head at once.** ~500 LOC is the
  point where you should be looking for a split. ~1,000 is where you
  must.

### File names

- Lowercase, underscores between words: `error_messages.go`, not
  `ErrorMessages.go` or `errormessages.go`.
- Match the dominant exported symbol when there is one: `executor.go`
  contains the `Executor`; `result.go` contains `Result`.
- `doc.go` for the package comment if the comment is long enough to
  deserve its own file (see `internal/actions/doc.go`).

### Build tags

When you need OS-specific code, use file-name suffixes (`_linux.go`,
`_darwin.go`, `_windows.go`, `_unix.go`) — Go picks them up
automatically without `//go:build` lines. Use `//go:build` only when
the suffix isn't expressive enough (e.g. `//go:build linux || darwin`).

---

## 5. Naming

Naming is the most-frequently-litigated category in code review. So
here is the policy.

### Clarity beats brevity

```go
// no
func ProcessPkgInstall(s *config.Step, c actions.Context) error

// yes
func installPackage(ctx actions.Context, step *config.Step) error
```

Hungarian prefixes (`sFoo`, `iCount`), reserved-word avoidance
(`tpe` for type, `pkge` for package), and abbreviations that aren't
universally understood — all banned. `cfg`, `ctx`, `err`, `req`,
`resp`, `i`, `n` are universally understood. `mng`, `prc`, `vld` are
not.

### Receiver names

One letter is fine if it is consistent across the type:

```go
func (h *Handler) Metadata() ActionMetadata
func (h *Handler) Validate(step *config.Step) error
func (h *Handler) Run(ctx Context, step *config.Step) (Result, error)
```

`h` for `Handler`, `e` for an error type, `r` for `Result`, `s` for
`Step`. Pick once per type and never alternate.

### Don't repeat the package in the identifier

```go
// inside package git_clone:
type GitCloneSnapshot struct{ ... }   // bad — reads as git_clone.GitCloneSnapshot
type Snapshot struct{ ... }            // good — reads as git_clone.Snapshot
```

The codebase has historical exceptions (`actions.ActionMetadata`,
`actions.ActionCategory`) — accept those, don't extend them.

### Errors

Error variables start with `Err`, error types end in `Error`:

```go
var ErrNotFound = errors.New("not found")
type ValidationError struct { ... }
```

See §9.

### Booleans

Name booleans for the question they answer:

```go
// no
var disabled = false

// yes
var sudoRequired bool
func (g *GitClone) Update() bool { ... }
```

`hasX`, `isX`, `shouldX`, `canX` — pick the one that reads as a
question.

### Constants and enums

The codebase uses string-typed enums for kernel concepts that need to
appear in JSON (`ResourceKind`, `Operation`, `DiffOp` in
`internal/actions/handler_abi.go`). Follow that pattern when you need
a small closed set with stable wire representations:

```go
type Mode string

const (
    ModeApply Mode = "apply"
    ModePlan  Mode = "plan"
)
```

Don't use `iota` for anything that crosses a process boundary — renames
break wire formats silently.

---

## 6. Functions

### Length: a heuristic, not a rule

- Most functions: < 50 LOC.
- A few functions: 50–150 LOC, where the steps are linear and breaking
  them up would only add ceremony.
- > 150 LOC: needs a defence. Acceptable in CLI dispatchers (`cmd/`),
  big handler `Run` methods that compose smaller helpers, and decision
  trees that don't decompose cleanly. Otherwise: split.

The `.golangci.yml` cap (`cyclop: max-complexity: 35`) is the hard
ceiling, and a handful of functions live just under it on purpose
(`copy.Execute`, `executor.ExecuteStep`). Don't aim for the ceiling.

### Parameter count

Three is comfortable. Four is OK. Five demands a struct. Six is wrong.

```go
// uncomfortable
func runClone(ctx Context, g *GitClone, repo, ref, dest string, env []string) error

// better, when this happens often enough
type cloneParams struct {
    repo, ref, dest string
    env             []string
}
func runClone(ctx Context, g *GitClone, p cloneParams) error
```

Decision rule: parameters that are *always passed together* form a
struct. Parameters that vary independently stay separate.

### Receiver vs free function

Methods are for behaviour tied to a type's state or invariants. A
function that takes a `*Handler` solely to satisfy an interface
shouldn't be a method on a different type.

```go
// no — this is "free function masquerading as method"
func (h *Handler) parseRef(s string) string

// yes
func parseRef(s string) string
```

`parseRef` doesn't touch `h`. It is just a utility. Free function.

### `_` parameters in interfaces

When a method must satisfy an interface but doesn't use a parameter:

```go
func (h *Handler) Cost(_ *config.Step) actions.Cost { ... }
```

That's fine. Don't invent a name for the unused parameter — `_`
communicates "intentionally unused."

### Returning multiple values

Go's multi-return is good. Use it. The conventional shape is
`(value, error)`. Variations:

- `(value, ok bool)` for "lookup that might miss" — same shape as
  `m[key]`.
- `(value, secondaryValue, error)` is fine; three results is the
  ceiling.

If you find yourself returning more than three values, you want a
struct.

### Named returns

Use them sparingly. They are good for:

- Documenting what a result means: `func split(p string) (dir, base string)`.
- Deferred error wrapping (`defer func() { err = wrap(err) }()`).

They are bad for:

- Avoiding `var` declarations. You don't gain much, and "naked returns"
  later in a long function are genuinely confusing.

---

## 7. Types and structs

### Zero values should be useful

A `Result{}` should be a valid empty result, not a panic waiting to
happen. A `Handler{}` should be a usable handler. Avoid hidden
required-init unless you have no choice.

```go
// no
type Cache struct {
    items map[string]Item   // nil; first Set panics
}

// yes
type Cache struct {
    items map[string]Item   // initialized in NewCache; or use sync.Map
}
func NewCache() *Cache {
    return &Cache{items: map[string]Item{}}
}
```

If a type *must* be constructed by a function, name it `NewX` and put
the constructor adjacent to the type.

### Tags are for serialisation, not for decoration

```go
type GitClone struct {
    Repo string `yaml:"repo"`
    URL  string `yaml:"url"`     // alias, see Validate
    Dest string `yaml:"dest"`
}
```

Only tag fields the parser/serialiser will read. `json:"-"` and
`yaml:"-"` are explicit "do not serialise" markers — use them when a
field carries runtime state that shouldn't round-trip.

### Embedding

Embed when a type *is-a* the embedded type. Don't embed for code reuse.

```go
// good — *exec.Cmd's method set is genuinely part of what this is
type sudoCmd struct {
    *exec.Cmd
    askpass string
}

// bad — embedding a logger to "get logging methods"
type Handler struct {
    logger.Logger   // no
}
```

The second case should hold a logger as a named field
(`log logger.Logger`) and forward only what makes sense.

### Pointer vs value receivers

- Pointer receivers when the method mutates, when the type contains a
  mutex, or when the type is large.
- Value receivers when the type is small (≤ 2 words: `time.Time`,
  small structs of primitives) and immutable.
- **Be consistent within a type.** All methods on `*Handler` use
  pointer receivers; do not mix `func (h *Handler)` with `func (h Handler)`.

### `any` vs `interface{}`

`any`. Always. Go 1.21+ — `any` is canonical.

### When to use generics

Sparingly. Generics earn their keep when:

- A container needs to hold values of arbitrary type without type
  assertion at every access site.
- An algorithm is genuinely polymorphic (sort, map, filter), and a
  hand-rolled non-generic version would be repeated.

They do not earn their keep when:

- You have one user. Wait until you have two.
- You are using them to avoid a small `interface{}` cast.
- The constraint takes more lines than the function body.

The codebase has minimal generics on purpose. If you want to introduce
a new generic helper, the bar is high: it must replace at least two
existing non-generic copies, and the resulting code must be more
readable, not less.

---

## 8. Interfaces

This is where most Go style guides get confused. Let's not.

### Define interfaces at the consumer, not the producer

Go's "interfaces are satisfied implicitly" is the whole reason
interfaces work well in Go. Use it. The consumer of a capability
defines the interface; the producer ships a concrete type.

```go
// in internal/executor/ — the consumer
type templateRenderer interface {
    Render(s string, vars map[string]any) (string, error)
}

// in internal/template/ — the producer
type Pongo2Renderer struct { ... }
func (r *Pongo2Renderer) Render(s string, vars map[string]any) (string, error) { ... }
```

The renderer package ships a struct. The executor package declares the
minimal interface it needs. The two packages don't import each other
to talk about interfaces.

**Exceptions** are the kernel ABI (`actions.Handler`, `actions.Runner`,
`actions.Differ` and friends) — those interfaces are defined in the
*producer* package because they are the contract every handler must
implement. That is the rule's exception, not the rule.

### Accept interfaces, return concrete types

```go
// good
func NewExecutor(r templateRenderer, l logger.Logger) *Executor

// bad
func NewExecutor(r templateRenderer, l logger.Logger) Runner
```

Callers want concrete types because concrete types have more methods
than the interface — fields, helpers, the works. Interface return
types lock the caller out of half the surface they paid for.

### The opt-in sub-interface pattern (kernel ABI)

The codebase has a specific advanced pattern in
`internal/actions/handler_abi.go`. Read it before designing a new
extension point:

```go
type Handler interface {       // required
    Metadata() ActionMetadata
    Validate(step *config.Step) error
    Run(ctx Context, step *config.Step) (Result, error)
}

type Differ interface {        // opt-in — only handlers with cheap diffs implement this
    Diff(ctx Context, step *config.Step) (Diff, error)
}

type Reverser interface {      // opt-in — only reversible handlers implement this
    Reverse(result Result, step *config.Step) (*config.Step, error)
}

type Coster interface { ... }       // opt-in
type Permitter interface { ... }    // opt-in
```

Consumers do type assertions to discover the capability:

```go
func ResolveDiffer(h Handler) Differ {
    if d, ok := h.(Differ); ok {
        return d
    }
    return defaultDiffer{}
}
```

Why this works:

1. The required contract stays small. Implementing a new handler
   does not force you to write five methods you don't need.
2. New capabilities are added without breaking existing handlers.
3. Default resolvers in the registry produce safe behaviour for
   non-implementers — `mooncake plan` works even on handlers without
   real `Diff`.

When to copy this pattern: when you have an evolving capability set
on a closed type family. When NOT to copy it: every other situation.
"Opt-in sub-interfaces" is a heavyweight pattern; don't apply it to
solve a small problem.

### Interface size

A good Go interface has one or two methods. Three is OK. Five demands
a defence.

```go
// good
type Differ interface {
    Diff(ctx Context, step *config.Step) (Diff, error)
}

// suspicious — what is the abstraction here?
type Handler interface {
    Init() error
    Configure(map[string]any) error
    Validate() error
    Run() error
    Cleanup() error
    Report() Report
}
```

Big interfaces are almost always *describing one type* rather than
*describing a capability*. Split them.

### Empty interfaces and type switches

`any` is fine at boundaries (YAML, JSON, command results). Don't push
it further into the codebase than necessary. Convert at the boundary,
keep the interior typed.

When you must type-switch on `any`, exhaust it:

```go
switch v := raw.(type) {
case string:
    ...
case int:
    ...
case bool:
    ...
default:
    return fmt.Errorf("unsupported type %T", raw)
}
```

A `default` branch with a clear error is mandatory. Silent fall-through
is a bug magnet.

---

## 9. Errors

Errors carry most of the structural design weight in a Go codebase.
Spend the budget here.

### Always wrap with `%w`

```go
if err := os.Stat(path); err != nil {
    return fmt.Errorf("stat config %s: %w", path, err)
}
```

Never `%v` for an underlying error. `%v` loses the chain;
`errors.Is` / `errors.As` will fail upstream callers that try to
inspect the cause.

### Use typed errors when callers need to inspect

The pattern is in `internal/executor/errors.go`:

```go
type CommandError struct {
    ExitCode int
    Timeout  bool
    Duration string
    Cause    error
}

func (e *CommandError) Error() string { ... }
func (e *CommandError) Unwrap() error { return e.Cause }
```

Callers:

```go
var cmdErr *executor.CommandError
if errors.As(err, &cmdErr) {
    if cmdErr.Timeout { ... }
}
```

### Use sentinel errors when callers only need to compare

```go
var ErrNotFound = errors.New("not found")

if errors.Is(err, ErrNotFound) { ... }
```

Sentinel for `Is`, typed for `As`. The distinction:

- Sentinel — "this exact error happened."
- Typed — "this category of error happened, with this data."

If callers won't compare, use neither — just `fmt.Errorf`.

### Error message style

- Lowercase, no trailing punctuation. `failed to render path` not
  `Failed to render path.`. The linter (`error-strings` revive rule)
  enforces this; the convention is in `revive` defaults.
- Build messages as `<verb> <noun>: <underlying>`. E.g.,
  `stat dest /tmp/x: no such file or directory`.
- Include the operand. `file not found` is worthless; `file not
  found: /etc/mooncake/config.yml` is debuggable.

### Don't double-handle

Either log the error or return it — not both. Returning is almost
always correct; the caller decides what to do.

```go
// no
if err != nil {
    log.Errorf("failed to read: %v", err)
    return err
}

// yes
if err != nil {
    return fmt.Errorf("read config: %w", err)
}
```

### Define errors out of existence

Ousterhout's specific point: many errors are not errors at all. Look
at the call:

```go
// "error"
n, err := strings.Count(s, "x"), nil   // strings.Count cannot fail
```

A function that always returns `nil` for `err` should not return `err`.
A `Delete` that succeeds on "already deleted" doesn't return
`ErrNotFound`; the caller didn't care. The closest mooncake example is
idempotency: a handler that returns `Changed=false` instead of "no-op
error" is doing exactly this.

The reverse anti-pattern: returning `error` from a function that can
never fail "because callers might want to add failures later." Don't.
Add the error when you actually have a failure.

### Panics

Library code never panics. Period.

The narrow exceptions:

- `panic(fmt.Sprintf("BUG: invariant X violated"))` for genuinely
  unreachable code — programmer errors, not runtime conditions.
- `init()` failures that make the package unusable (rare; only when
  there is no other option).

CLI entry points (`cmd/`) can `os.Exit(1)` after printing a friendly
message; they should not panic. Tests panic via `t.Fatalf` — go
through testing.T, never naked `panic` in a test helper.

---

## 10. Context and cancellation

There are **two** "context"s in this codebase. Don't confuse them.

### `context.Context` (stdlib)

Used for cancellation, deadlines, and request-scoped values across
goroutines and process boundaries. Pass it as the **first** parameter:

```go
func doRPC(ctx context.Context, target string) (*Resp, error)
```

Never store it in a struct field except as a transient scope-bound
field that the struct's lifetime matches the context's lifetime.
The standard rule: `ctx` is on the stack.

### `actions.Context` (this repo's handler ABI)

Defined in `internal/actions/interfaces.go`. This is the *handler*
context — it gives access to the template renderer, the logger, the
variables, the event publisher, the dispatch mode. It is not a
substitute for `context.Context`; it carries no deadline.

If a handler needs cancellation, it takes BOTH:

```go
func (h *Handler) doNetworkThing(ctx context.Context, actx actions.Context) error
```

Naming convention: `ctx` for `context.Context`, `actx` for
`actions.Context` when both are in scope. When only one is in scope,
`ctx` (matching the surrounding code style).

### Cancellation is a contract, not decoration

If a function takes `context.Context`, it must actually honour
cancellation:

```go
// no — context is accepted but ignored
func longOp(ctx context.Context) error {
    for {
        doThing()   // never checks ctx
    }
}

// yes
func longOp(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        doThing()
    }
}
```

Accepting `context.Context` and ignoring it is worse than not accepting
it — callers think they have cancellation when they don't.

### When NOT to thread `context.Context`

Single-threaded, in-process, no I/O: don't bother. The closer you are
to leaf logic (string manipulation, validation, struct construction),
the less context belongs. Pushing `ctx` to every function "just in
case" is noise.

---

## 11. Concurrency

Most code in this repo is single-threaded. That is correct.

### Default to sequential

Goroutines are not free. They cost:

- Reasoning overhead (race conditions).
- Stack memory (small, but real).
- Debugging cost (a stack trace for a goroutine started 30 frames ago
  in a different goroutine is not fun).

Add concurrency when you have a measured reason: I/O parallelism (the
fleet substrate), background workers (the daemon), caches with TTL
(metrics). For everything else, write sequential code.

### When you do use goroutines

- **Lifetime**: every goroutine you start has a defined exit. Forever
  loops are forever bugs. Pass `context.Context`, honour it.
- **Synchronisation**: prefer channels for orchestration; prefer mutexes
  for protecting state. Don't mix idioms in one type.
- **`sync.Once`**: the right tool for "cache this expensive
  thing once per process." See `internal/facts/` — facts are computed
  once via `sync.Once`. Do not reinvent this with `if cached == nil`
  + atomic flags; you will get it wrong.

### Races are real

`make test-race` (or `go test -race`) must be clean. Race detector
findings are not flaky; they are real data races. Never disable the
race detector to make a test pass. If a test is flaky under `-race`,
the production code has a race — fix the code.

### Mutexes

Tiny ones, scoped tight:

```go
type cache struct {
    mu    sync.Mutex
    items map[string]Item
}

func (c *cache) get(k string) (Item, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    v, ok := c.items[k]
    return v, ok
}
```

Conventions:

- `mu` for the primary mutex on a type.
- `mu sync.Mutex` (not `*sync.Mutex` — pointer-mutexes are an
  inherited bug from other languages).
- Mutex protects the *immediately-following* fields. Document this if
  the ordering isn't obvious.

### Channels

- Unbuffered for handoff ("one goroutine waits, one sends").
- Small buffered for "fire and forget with backpressure" (event bus
  uses 100; see `LLM_GUIDE.md` "Notes").
- Big buffered: almost always wrong. You are papering over flow
  control.
- The sender closes the channel. The receiver does not close. Closing
  twice panics.

### `errgroup`

For "fan out N parallel operations, wait for all, fail-fast on the
first error," reach for `golang.org/x/sync/errgroup`. Don't reinvent it
with `sync.WaitGroup` + a shared error variable.

---

## 12. Logging and observability

### Use `ctx.GetLogger()` inside handlers

Never `fmt.Println`, never `log.Println`, never `os.Stderr` writes
inside a handler. The logger is configured per run — for the TUI, for
plain text, for JSON output — and direct writes bypass that.

```go
ctx.GetLogger().Debugf("git.clone: %s", strings.Join(args, " "))
```

Levels:

- `Debugf` — diagnostic detail only useful when debugging.
- `Infof` — normal progress that a user wants to see by default.
- `Warnf` — recoverable strangeness ("file already at desired state").
- `Errorf` — failures that affect outcomes.

Default verbosity is `Info`. Make sure each level is justified:
"debug noise" is information you wouldn't want to see *yourself* in
a green run.

### Events for state changes

When a handler makes a state change (created a file, started a
service), emit an event:

```go
ctx.GetEventPublisher().Publish(events.Event{
    Type: events.EventFileCreated,
    Data: events.FileOperationData{Path: path},
})
```

Why: events feed the runlog (audit trail), artifact collector
(rollback), and external observers. A `log.Infof` does not.

Rule of thumb: anything a user might ask "did that actually happen?"
about, emit an event for. Logs are for humans-now; events are for
machines-and-humans-later.

### Don't log secrets

`internal/config/secret_tag.go` defines the secret marking. If a value
flows through code that might log it, check that path. Secrets in logs
land in CI archives and stay there forever.

### Output to stdout

CLI commands (`cmd/`) print to stdout. They do not use the logger for
their primary output — logger output goes to stderr (or TUI). User-
facing output is `fmt.Fprintln(os.Stdout, ...)` or via the rendering
helpers in `internal/cli/` / `internal/render/`.

The `errcheck` exclusions in `.golangci.yml` for `cmd/` and several
`internal/` display helpers reflect this — best-effort writes to
terminal output don't need error handling.

---

## 13. State and purity

### Handlers are stateless

A handler's only state is its method set. Everything that varies per
run lives in `actions.Context` and the `*config.Step` passed to `Run`.

```go
type Handler struct{}   // no fields, ever
```

Why: handlers are constructed once at process start (`init()`),
registered into a shared registry, and called from multiple goroutines.
Mutable fields are races waiting to happen.

If a handler "needs state," the state belongs in the result, the
context, or — at most — process-wide globals owned by a different
package (a cache, a connection pool). Not on the handler.

### Pure functions are gifts

A pure function — same input, same output, no side effects — is the
easiest thing in the world to test. Write helpers as pure functions
whenever you can:

```go
// pure
func shortSHA(sha string) string {
    if len(sha) >= 12 {
        return sha[:12]
    }
    return sha
}

// not pure (touches FS)
func inspectDest(dest string) (destState, error) { ... }
```

Pure helpers can be tested in nanoseconds with no setup. Side-effecting
helpers need a temp dir and cleanup. Push the impurity to the edges of
the package.

### Avoid package-level mutable state

The registry (`internal/actions/registry.go`) is the canonical
exception — it's a process-wide thread-safe map populated at `init()`.
Don't invent more of these.

Package-level mutable state is the enemy of testability. Every test
that touches it needs setup/teardown. Every concurrent test that
touches it can race.

---

## 14. The handler ABI

The contract every handler implements lives in
`internal/actions/handler.go` and `internal/actions/handler_abi.go`.
Read them in that order before writing a new handler.

### Required surface

```go
type Handler interface {
    Metadata() ActionMetadata
    Validate(step *config.Step) error
    Run(ctx Context, step *config.Step) (Result, error)
}
```

That's it. Three methods. Implement them, register the handler in
`init()`, you are done with the minimum:

```go
func init() {
    actions.Register(&Handler{})
}
```

### Optional capabilities

Implement the sub-interface when it is real, not when "it would be
nice":

- **`Differ`** — when you can cheaply tell the user what *would*
  change without doing it. File writes, package installs, git ref
  changes — yes. Shell commands — no (we can't introspect arbitrary
  shell).
- **`Reverser`** — when there's a meaningful undo. File deletes
  (restore from backup) — yes. `shell` — no. `os.service start` —
  ambiguous; the project explicitly refuses for now (see
  `kernel.md`).
- **`Coster`** — when blast radius is non-trivial and a user might
  want to gate on it. Package operations, anything that touches
  /etc, anything that hits the network.
- **`Permitter`** — when permissions are not obvious. Network access,
  sudo, write paths, required binaries on PATH.

The cost of "I implemented this halfheartedly" is real. A `Diff` that
returns wrong information is worse than no `Diff` at all — plan output
lies to the user. If you can't get it right, don't implement the
sub-interface; the registry's default will say "not predictable."

### `Run` is `Plan` and `Apply` in one

Since spec-16:

```go
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
    if ctx.Mode() == actions.ModePlan {
        return h.predict(ctx, step)
    }
    return h.apply(ctx, step)
}
```

`ModePlan`: inspect state, return a result with `WouldChange` set.
`ModeApply`: actually mutate, return a result with `Changed` set.

The two modes share inspection logic. They diverge at the point of
mutation. Side-effecting calls in `ModePlan` are bugs.

### Effects()

Filesystem mutations should go through `ctx.Effects()` — the
`Performer` interface — rather than direct `os.*` calls. The performer
is a no-op in `ModePlan`. This is *the* mechanism that makes
plan/apply parity automatic instead of duplicated.

```go
// good
if err := ctx.Effects().WriteFile(path, content, 0o644); err != nil { ... }

// bad in a handler
if err := os.WriteFile(path, content, 0o644); err != nil { ... }
```

Read-only `os.Stat`, `os.Open` etc. are fine to call directly — they
have no side effect to suppress.

---

## 15. Idempotency and dry-run

Idempotency is the project's central promise. A handler that violates
it is broken.

### The contract

> Run the same step twice. The second run reports `changed=false`.

That is the test. If your handler fails it, the handler is wrong.

### Patterns that work

1. **Inspect, then mutate**: read current state, compare to desired,
   skip if equal.

   ```go
   state, err := inspectDest(dest)
   if state.HeadSHA == desired { return ResultNoChange, nil }
   // mutate
   ```

2. **Mutate idempotently**: prefer underlying operations that are
   themselves idempotent (`os.MkdirAll`, `os.Remove` followed by
   not-exists check, etc.).

3. **Compare checksums** for content-defined idempotency. Don't
   compare timestamps — those drift.

### Patterns that don't work

- "If file doesn't exist, create it" without checking content when it
  does exist. The file might be there but wrong.
- "Run the command and trust the user's shell to be idempotent" —
  shell handlers are not idempotent by default. Use `creates:`,
  `unless:`, `changed_when:`.
- Time-based skipping ("we ran less than an hour ago") — that's
  caching, not idempotency.

### Dry-run is plan-mode

`ctx.Mode() == ModePlan` means "don't actually do anything." Read FS
state, compute the diff, return a result with `WouldChange=true` or
`WouldChange=false`. **No mutating subprocess calls.** No network
writes. Plan is cheap and side-effect-free, by promise.

Cross-reference: kernel.md treats this property — `Diff` per step —
as one of the four typed properties that distinguish Mooncake. If you
break plan-mode purity, you break that claim.

### Testing idempotency

The idempotency tests live in `internal/executor/idempotency_test.go`.
Read them. Mirror that pattern in handler-level tests:

```go
// run once
r1, err := h.Run(ctx, step)
require.NoError(t, err); require.True(t, r1.Changed())

// run again
r2, err := h.Run(ctx, step)
require.NoError(t, err); require.False(t, r2.Changed())
```

If a handler can't pass this, fix the handler — don't paper over it in
the test.

---

## 16. Templates and variables

### Render at the handler boundary

Step fields that may contain templates (`{{ var }}`) are rendered when
the handler reads them, not earlier.

```go
repo, err := ctx.GetTemplate().Render(g.Repo, ctx.GetVariables())
if err != nil { return nil, fmt.Errorf("render repo: %w", err) }
```

Why not earlier? Because the planner does loop expansion before
handler dispatch; rendering twice causes double-expansion bugs. Render
exactly once, at the handler.

### Don't reach into `Variables` directly

```go
// no
home := ctx.GetVariables()["home"].(string)

// yes — render a template that reads home
home, err := ctx.GetTemplate().Render("{{ home }}", ctx.GetVariables())
```

Direct map access bypasses type coercion, fact resolution, and template
filters. It is the source of "works on my machine" handler bugs.
There is one exception: when reading explicitly registered structured
results (`register: prev_step`), which are typed as
`map[string]interface{}` by contract.

### Variable scope

Defined in `internal/executor/scope.go`. Two important rules:

1. **Don't mutate the map returned by `GetVariables()`.** Use
   `MergeUserVars` to write back — it routes to the typed bucket. The
   shared map is process-state.
2. **Loop variables (`item`, `item_index`) are scoped.** They are
   injected by the planner into the per-iteration step. Don't shadow
   them in template helpers.

---

## 17. Filesystem and subprocess

### Paths

- Use `filepath` not `path`. `path` is for URL-like slash-separated
  strings; `filepath` is OS-aware.
- Build paths with `filepath.Join`, not string concatenation. Even on
  Linux. The habit transfers; the bug doesn't.
- Expand `~` and environment variables via `ctx.GetPathUtil()` (or the
  test-friendly path expander), never with ad-hoc `os.Getenv("HOME")`
  prefixing. Path expansion is policy, not a one-liner.

### `os.Stat` and not-exists

```go
info, err := os.Stat(p)
if errors.Is(err, os.ErrNotExist) {
    // expected case; carry on
}
if err != nil {
    return fmt.Errorf("stat %s: %w", p, err)
}
```

Never compare error strings (`if err.Error() == "file does not exist"`).
Use `errors.Is`. Always.

### Subprocess

Construct commands with `exec.Command`, not `exec.CommandContext` —
unless you genuinely want cancellation. Pass arguments as separate
strings:

```go
cmd := exec.Command("git", "clone", "--depth", strconv.Itoa(g.Depth), repo, dest)
```

Never:

```go
cmd := exec.Command("sh", "-c", "git clone " + repo + " " + dest)
```

That second form is a shell injection waiting to happen. The
`gosec` exclusion `G204` (subprocess launched with variable) is
*permitted* in this codebase for specific reviewed call sites
(see `.golangci.yml`); it is not a blanket licence. New code does not
get `G204` exclusions without a written argument.

### Environment

When passing env to a subprocess, build it explicitly:

```go
cmd.Env = append(os.Environ(),
    "GIT_TERMINAL_PROMPT=0",
    "GIT_ASKPASS=" + askpass,
)
```

Don't mutate `os.Environ()` directly via `os.Setenv` for subprocess-
scoped variables — that leaks to other goroutines.

### Permissions

- Use octal literals for permission bits: `0o644`, `0o755`. The `0o`
  prefix is Go 1.13+ and prevents the "did you mean octal" misread.
- The `gosec` exclusions for G301/G302/G306 are because the project's
  config files are 0o644 by design. New file writes still need to
  justify the mode they pick — most often `0o644` for files,
  `0o755` for directories, `0o600` for secrets.

---

## 18. Comments and documentation

Ousterhout's rule: comments describe what the code cannot.

### Don't comment what the code says

```go
// no
// Increment count
count++

// no
// Loop over items
for _, item := range items { ... }
```

The reader can see this. The comment is overhead.

### Do comment why, not what

```go
// good
// gosec exclusion for G204 lives in .golangci.yml; this is a vetted
// call site that builds args from typed config, not from user-supplied
// strings.
cmd := exec.Command(...)
```

```go
// good
// MT-33: accept `url:` as an alias for `repo:`. Resolve before
// validating so the rest of the handler reads g.Repo only.
if g.Repo == "" && g.URL != "" {
    g.Repo = g.URL
}
```

The comment explains the *non-obvious* — a project-specific
convention, a workaround, a constraint that survives a refactor.

### Package comments

Every non-test package has a doc comment. It explains *why the package
exists* in one paragraph.

```go
// Package shell implements the shell action handler.
//
// The shell action executes shell commands with support for:
//   - Cross-platform interpreter dispatch
//   - Sudo/become privilege escalation
//   - Environment variables and working directory
//   - Timeout and retry logic
//   - Stdin, stdout, stderr handling
//
// Platform-specific exec.Cmd construction lives in exec_unix.go and
// exec_windows.go via the Handler.buildCommand method.
package shell
```

If the comment is more than ~10 lines, move it to `doc.go` and keep
the `package shell` line slim.

### Function comments

Exported functions have godoc comments. They follow the convention
"Name verb-phrases the rest":

```go
// Validate checks if the shell configuration is valid.
func (h *Handler) Validate(step *config.Step) error { ... }
```

Unexported functions get a comment only if the function name and
parameters don't convey enough. Don't write `// shortSHA shortens a
SHA.` — the name said that.

### Sticky comments

If a comment encodes a constraint that future refactors must respect
("this must happen before X because Y"), make it loud. Triple-slash
fences, anchor it with an issue number or spec number:

```go
// SPEC-22-INVARIANT: Diff must be side-effect-free. Do not call any
// network or write-flavoured os.* function in this method. Plan mode
// purity depends on it.
```

Future-you will thank you. Future-LLM will also thank you, and listen
to it more reliably than a polite "note that...".

### Don't document the obvious

A comment that says "Get the logger" above `ctx.GetLogger()` is
noise. So is "Loop forever" above `for {}`. Delete those when you
see them.

### Code review pass: read the comments

A specific habit. Before approving a PR, read only the comments. Do
they still match the code? Outdated comments are worse than no
comments — they lie. Remove or rewrite, never "we'll fix it later."

---

## 19. Testing

Tests are not optional. They are the single best way to keep the
codebase reshapable.

### Table-driven, by default

```go
func TestValidate(t *testing.T) {
    cases := []struct {
        name    string
        step    *config.Step
        wantErr bool
    }{
        {"nil", &config.Step{}, true},
        {"missing repo", &config.Step{GitClone: &config.GitClone{Dest: "/tmp/x"}}, true},
        {"ok", &config.Step{GitClone: &config.GitClone{Repo: "https://x", Dest: "/tmp/x"}}, false},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            err := (&Handler{}).Validate(c.step)
            if (err != nil) != c.wantErr {
                t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
            }
        })
    }
}
```

The `t.Run(c.name, ...)` is mandatory — it gives each case its own
sub-test, so `make test-fn FN=TestValidate/missing_repo` works.

### Real packages, real types

Mock at the boundary, not inside the package. The codebase has a
`testutil.MockContext` for handler tests — it implements
`actions.Context` directly. Use it. Don't invent another mock.

For things that hit the filesystem: use `t.TempDir()`. For things that
exec subprocesses: use the real subprocess in CI, or feature-flag
the test with `testing.Short()` to skip in local dev.

```go
func TestRunClone(t *testing.T) {
    if testing.Short() { t.Skip("requires git binary") }
    dir := t.TempDir()
    // ... real git clone into dir ...
}
```

### Test what the public surface promises

A handler's test surface is `Validate`, `Run`, `Diff`, `Reverse`,
`Cost`, `Permissions`. Plus idempotency. Test those. Resist the urge
to test internal helpers directly unless they have a complex enough
contract to deserve their own tests — but then they probably should
be in a separate package.

### Idempotency tests

Every handler with state changes has an idempotency test. The pattern
is in §15. The path of least resistance is to add it; the path of
maximum regret is to skip it.

### Race detector

`make test-race`. Always. CI runs it. Failures are real, not flake;
see §11.

### Coverage

The `ai-development-guide.md` says ">80% coverage" — that is
aspirational and a useful target. It is not a lint gate. What matters:

- Happy paths covered.
- Each error return covered or has a clear "infeasible without
  injecting a fake filesystem" justification.
- Edge cases that survived previous bugs are pinned with regression
  tests, named after the issue (`mt75_test.go`, `mt77_test.go`).

The `mt*_test.go` naming convention in `internal/config/` is the
shape: `<issue-id>_test.go` for regression tests.

### Test output

Test artifacts go to `./testing-output/`, never to `/tmp` (which
collides between concurrent runs) and never to the user's home
directory (which pollutes their machine). The Makefile and CI
arrange cleanup.

### What NOT to test

- Stdlib behaviour. Don't test that `filepath.Join` joins.
- Trivial getters. Don't test `func (r *Result) Changed() bool { return
  r.changed }`.
- Internal type plumbing. If two helpers are private and called only
  by one public function, test the public function; the helpers go
  along for the ride.

### Fuzz tests

When parsing untrusted input (YAML, expressions, templates), a fuzz
test is cheap insurance:

```go
func FuzzParseStep(f *testing.F) {
    f.Add([]byte("shell: echo hi"))
    f.Fuzz(func(t *testing.T, in []byte) {
        _, _ = config.ParseStep(in)   // must not panic
    })
}
```

The bar is "must not panic." Crash-on-bad-input is a denial-of-service
even for a config tool.

---

## 20. Performance

> Premature optimisation is the root of all evil. Late optimisation
> is the root of half the bugs.

### Profile before optimising

`go test -bench`, `pprof`, `runtime/trace`. Real numbers, not vibes.
The codebase has had two profile-driven optimisations stick (facts
caching, metrics TTL caching) and zero "I bet this is slow"
optimisations stick.

### `prealloc` is disabled on purpose

See `.golangci.yml`:

```yaml
- prealloc    # micro-optimisation noise; profile-driven not lint-driven
```

That is the policy. Allocating a slice and appending in a loop is
fine. If profiling shows a hot path, pre-allocate then. Otherwise,
write the obvious code.

### Strings

- `strings.Builder` for "build up a string in a loop." Plain `+=`
  for two or three concatenations.
- `strings.Cut` (Go 1.18+) for "split on first separator" — clearer
  than `strings.Index` + slicing.
- `strconv.Itoa(n)` for int → string, not `fmt.Sprintf("%d", n)` —
  the codebase isn't pedantic about it, but `strconv` is a clearer
  signal that you're not formatting structured output.

### Maps and slices

- A small slice search is faster than a map lookup. Maps win at
  ~hundreds of entries. Don't make a map for a fixed list of three
  things.
- Avoid `append(s, x)` followed by `s = s[:len(s)-1]` patterns — use
  a real stack helper if you need a stack. The append/truncate dance
  fools readers.

### I/O is the bottleneck

If you're optimising and the function is CPU-bound, you might gain
20%. If the function is I/O-bound (disk, network), you might gain 10×
by batching, caching, or async I/O. Look there first.

---

## 21. Dependencies

The constraint is in `LLM_GUIDE.md`:

> No external dependencies beyond Go stdlib (exceptions: yaml
> parsing, expr evaluation).

That is real. The exceptions are explicit. To add a new dependency:

1. Ask: is this in the stdlib? (Usually yes if you look hard.)
2. Ask: can I write 20 lines instead?
3. Ask: would the project owner approve?

The answer to #3 governs. Don't sneak dependencies in via a small PR.

### The dependency math

Every dependency is:

- A supply-chain attack surface.
- A version-bump treadmill.
- A potential CGO requirement.
- A learning cost for every new contributor.
- A reason the binary is 2 MB larger.

A 50-LOC helper costs the project ~1 hour of writer time and ~10
minutes of reader time, ever. A dependency costs all of the above,
forever. The math is rarely close.

### `go.mod` hygiene

- `go mod tidy` after every change that touches imports.
- `go.sum` is committed. Don't `.gitignore` it.
- Don't add `replace` directives to `go.mod` unless the user has
  approved it. Replace is a public statement; it sticks.

### `internal/` is your buffer against deps

When you do take a dep, wrap it in an internal package. Then if you
ever need to swap it, the swap is in one place.

```go
// internal/template/pongo2.go — owns the pongo2 dependency
// internal/executor/ — uses internal/template, not pongo2 directly
```

This is mostly true today. Maintain it.

---

## 22. Architecture soft caps

See `CLAUDE.md` for the canonical statement. Repeated here only
because Go code reviewers should know the numbers.

### The three caps

1. **Handler LOC > 1,500** → split.
2. **`internal/config.Step` universal fields > 40** → flag (today's
   count is 36; check `make budget-status`).
3. **`gocyclo` > 35** in any non-test function → refactor on next
   touch.

These are not CI gates. They are review-time prompts. When a PR
crosses one, the reviewer asks the question; the answer is sometimes
"yes, and here's why" and that's fine. The question is what matters.

### Use `make budget-status` before structural work

```
make budget-status
```

Prints the current state of the three caps. Look at it before
proposing a refactor. Look at it before adding a field to `Step`.
Look at it after merging anything that touches a handler or the
executor.

### Today's known violations (snapshot)

Documented in `CLAUDE.md`, regenerated by `make arch-snapshot`. The
list moves; don't memorise it. Don't make it longer without writing
"why" in the PR description.

---

## 23. Linter discipline

`.golangci.yml` is a style document in disguise. Read it once a
quarter.

### Linters we enabled, and why

- **`gosec`** — catches real security mistakes. We *do* exclude a few
  IDs that produce noise on intentional patterns (G204, G304, G306);
  each exclusion has a rationale in the comment.
- **`errcheck`** — catches unchecked errors. Exclusions for
  `fmt.F*` in `cmd/` and display helpers are because best-effort
  terminal writes don't need error handling. New code does not get
  blanket exclusions.
- **`staticcheck`** — bug-class checks. `QF*`, `ST1005`, `ST1020`,
  `S1008`, `S1017`, `S1034` are off because they are style, not bug,
  preferences.
- **`govet`, `ineffassign`, `unused`** — Go-built-in correctness.
  Never disable. Fix the code.
- **`bodyclose`** — HTTP response bodies must be closed. Always.
- **`revive`** — selected rules only; we run a curated subset
  (see the rules list in `.golangci.yml`).
- **`cyclop`** — complexity ceiling at 35. The same number as the
  `CLAUDE.md` soft cap; the linter is the safety net behind the
  review-time prompt.
- **`misspell`**, **`unconvert`** — cheap to keep clean.

### Linters we disabled, and why

`gocritic`, `unparam`, `dupl`, `prealloc` are explicitly off. The
linter file says why:

> The dropped linters flag legitimate design choices (interface
> contracts that always return nil err, parallel-by-design sibling
> commands, intentional monolithic functions) as problems.

If you find yourself wanting to add one of these back, the
conversation is "what specific bug have we shipped that this would
have caught?" — not "but it's a community standard."

### Lint locally

```
make lint-pkg PKG=internal/actions/git_clone
make lint-new                            # only changed lines
make lint-fix                            # auto-fix what's auto-fixable
```

`make lint-fix` is safe. Run it before sending a PR.

---

## 24. Pre-PR checklist

For every Go PR, before pushing:

- [ ] `make check-pkg PKG=<the package you touched>` (build + test
      with race + lint, sub-second for most edits).
- [ ] `make test-race ./...` if the change crosses packages.
- [ ] `make lint-new` clean.
- [ ] `make budget-status` if you touched a handler, the executor,
      the planner, or `internal/config/config.go`.
- [ ] If you added a new dependency: justification in the PR
      description. (Default answer: don't.)
- [ ] If you added a new `internal/` package: justification in the PR
      description. (Default answer: extend an existing one.)
- [ ] If you bumped a soft-cap violation higher: justification in the
      PR description.
- [ ] If you added `init()` side effects: justification in the PR
      description. (Default answer: don't. Use explicit registration
      called from `cmd/`.)
- [ ] If you used `panic`: justification in the PR description.
      (Default answer: don't.)
- [ ] If you added `// TODO`: tied to an issue number, or removed.
- [ ] You re-read the diff once for *names*. (Cheapest review pass.
      Catches the most regrets.)

This list is short on purpose. The longer it gets, the less anyone
will read it.

---

## 25. Anti-patterns

A non-exhaustive list. Each item has been seen in a real PR.

### "Defensive" code for impossible scenarios

```go
// no
if h == nil {
    return fmt.Errorf("handler is nil")    // h is constructed once at init and never freed
}
```

Trust internal code. Validate at boundaries (user input, external
APIs). Not inside a package talking to itself.

### Logging the same thing the error already says

```go
// no
if err != nil {
    log.Errorf("failed to read file %s: %v", path, err)
    return fmt.Errorf("failed to read file %s: %w", path, err)
}
```

Pick one. Usually returning the error is right.

### Wrapping unrelated changes

```go
type Config struct {
    // ... 30 fields ...
}
```

When fields multiply on a struct, especially `Step`, the answer is
not "one more flag." It is "what is this struct *actually* about?"
See §22.

### Helper functions called once

A helper that has exactly one caller is not a helper. It is an
indirection. Inline it. When it acquires a second caller, extract it
again. (Three-call rule, §1.)

### Interfaces with one implementation

```go
type Reporter interface {
    Report() string
}
type concreteReporter struct { ... }
func (c *concreteReporter) Report() string { ... }
```

If there is one implementation, the interface is decoration. Delete
it. Add it back when the second implementation appears. (This is a
*producer-side* interface; for consumer-side single-impl interfaces,
see §8 — the rule for those is "what data did you actually need?")

### "Just-in-case" generics

```go
func Identity[T any](x T) T { return x }
```

Real example from a different repo. Delete on sight.

### Long return tuples

```go
func parse(s string) (a, b, c, d int, err error)
```

Four ints? Make a struct. The reader has no chance of remembering
which position is which.

### `// removed: oldThing()` and dead commented-out code

The git history is your archive. Delete the line. If it mattered, it
will be in `git log`. If it didn't, you've saved a future reader's
attention.

### Backwards-compat shims

```go
// Deprecated: use NewThing instead
func OldThing() { return NewThing() }
```

For a project with shipped users, sure. For *this* project (see
"Reshape freely" in `LLM_GUIDE.md`), no. Rename the call sites,
delete the shim.

### Re-exporting a type to avoid an import

```go
// in package A:
type Step = config.Step
```

If your package needs to talk about `config.Step`, import `config`.
Type aliases are for migration, not for avoiding the import path.

### Slow-creep panics in production code

```go
panic("not implemented")
```

If it's not implemented, return an error: `errors.New("not yet
implemented")`. Or — better — don't merge the function at all until it
is. Panics in production code crash the daemon. The daemon is a
shared resource.

---

## Appendix A: Ousterhout, applied

*A Philosophy of Software Design* (John Ousterhout, 2018) is the
single book most worth reading for this codebase. Its arguments map
onto Go better than any other source. Below: the principles that
matter here, restated in mooncake-specific terms.

### Complexity is what makes systems hard

Ousterhout: "complexity is anything related to the structure of a
software system that makes it hard to understand and modify."

In this repo: the soft-cap policy (handler LOC, gocyclo, Step
fields) is a direct measure of complexity. It is a number we
deliberately track because we cannot trust ourselves not to add to
it incrementally.

### Modules should be deep

Deep module = simple interface, powerful implementation. Shallow
module = simple interface, simple implementation that adds little
over its caller writing the code inline.

In this repo:

- `actions.Handler` (three methods) over thousands of lines of
  per-action implementation — **deep**.
- A util package that wraps `os.Stat` to return the same `bool, error`
  with a slightly different signature — **shallow**. Delete.

When you propose a new interface, ask: is the implementation behind
this materially more complex than the interface? If no, the interface
is shallow. Inline it.

### Information hiding (and information leakage)

Each module hides design decisions from the rest of the system. When
a design decision is reflected in multiple modules, that's
**information leakage** — change one, you must change them all.

In this repo:

- The action vocabulary is hidden behind the `action:""` struct tags
  on `Step` (see `internal/config/config.go`). The schema, the
  validator's allowed-action list, and `mooncake docs` are generated
  from that single source. That's information hiding.
- When `Step` field naming leaks into preset YAML *and* `cmd/`
  argument parsing *and* the JSON schema *and* tests — that's
  leakage. The fix is a single source of truth; the leak is the
  symptom.

When refactoring, ask: where else does this decision need to be
known? If the answer is "three or more places," you have leakage.

### Define errors out of existence

Ousterhout's least-popular advice. Most error handling exists because
designers didn't push hard enough on whether the error needed to
exist at all.

In this repo:

- `creates:` / `unless:` exist so a handler doesn't have to return
  "already there" as an error.
- Idempotency itself is "define the error of 'state already as
  desired' out of existence by returning `changed=false`."
- `errcheck` exclusions for `fmt.F*` to terminals exist because
  failing-to-print-to-stderr is not an error worth surfacing — the
  user can't see it anyway.

When you write an `error` return, ask: is this a real error, or is
this an awkward result that I'm pretending is an error? If the latter,
fold it into the result type.

### Design it twice

Before committing to an approach, sketch a second one. Compare. Often
the first sketch is good. Often the second is better. Almost always,
the comparison reveals the *first* approach's assumptions.

In this repo: the kernel ABI (`Differ`, `Reverser`, `Coster`,
`Permitter`) was originally one big interface. The second design —
opt-in sub-interfaces with default resolvers — is what shipped. The
first design would have forced every handler to implement five
methods to add one.

For a small change, "design it twice" can be five minutes of
whiteboard. Do it anyway.

### Strategic, not tactical, programming

Tactical programming: "just make it work." Strategic: "what change
to the design would let this — and the next ten things — work?"

This repo accepts both. There are explicit tactical patches (an
`if` to fix a specific manual-test finding) and explicit strategic
investments (the kernel refactor; the soft-cap discipline).
The mistake is doing tactical when strategic was needed, or vice
versa.

Rule of thumb: if the same area of the codebase gets a tactical
patch three times in a quarter, the next patch should be strategic
or the area is wrong.

### Comments are part of the design

Ousterhout argues that *writing* comments is design work — it forces
you to explain what you've built, and the act of explanation often
reveals that you haven't built the right thing. The discipline isn't
"add comments after." It's "write the comment first, see if the code
still wants to be written that way."

In this repo, `docs-working/specs/` is exactly this. Specs are
written before implementation. They are design documents. They get
revised when implementation reveals a flaw in the design.

For per-function comments: write the godoc line. If the line is
trivial, ask whether the function deserves to exist. If the line is
complex, ask whether the function is doing too much.

---

## Appendix B: A worked example

Adding a new handler: `os.hostname` — sets the system hostname,
idempotently, with rollback.

### Step 1: read the existing pattern

```
internal/actions/git_clone/
internal/actions/os_user/
internal/actions/os_systemd/
```

Pick the closest analogue. `os_user` is the closest — it's an OS-level
action with state to inspect, a meaningful "current vs desired"
comparison, sudo requirements, and per-OS implementation.

### Step 2: minimal handler

```go
// internal/actions/os_hostname/handler.go
package os_hostname

import (
    "fmt"
    "os"
    "strings"

    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
)

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:               "os.hostname",
        Description:        "Idempotently set the system hostname",
        Category:           actions.CategorySystem,
        SupportsDryRun:     true,
        SupportedPlatforms: []string{"linux", "darwin"},
        RequiresSudo:       true,
        ImplementsCheck:    true,
    }
}

func (h *Handler) Validate(step *config.Step) error {
    if step.OSHostname == nil {
        return fmt.Errorf("os.hostname requires configuration")
    }
    if strings.TrimSpace(step.OSHostname.Name) == "" {
        return fmt.Errorf("os.hostname: name is required")
    }
    return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
    desired, err := ctx.GetTemplate().Render(step.OSHostname.Name, ctx.GetVariables())
    if err != nil {
        return nil, fmt.Errorf("os.hostname: render name: %w", err)
    }

    current, err := os.Hostname()
    if err != nil {
        return nil, fmt.Errorf("os.hostname: read current: %w", err)
    }

    result := executor.NewResult()
    if current == desired {
        result.Reason = "hostname already " + desired
        return result, nil
    }

    if ctx.Mode() == actions.ModePlan {
        result.WouldChange = true
        result.Reason = fmt.Sprintf("would change hostname %s -> %s", current, desired)
        return result, nil
    }

    if err := setHostname(ctx, desired); err != nil {
        result.SetFailed(true)
        return result, fmt.Errorf("os.hostname: set: %w", err)
    }
    result.SetChanged(true)
    result.Reason = fmt.Sprintf("hostname %s -> %s", current, desired)
    return result, nil
}
```

What's in this minimum:

- Three methods, as the contract requires.
- `init()` registration.
- Template rendering at the boundary, once.
- Inspect → compare → branch on Mode → mutate or predict.
- Wrap errors with `%w` plus operation context.
- Result carries `Reason` for human-readable output.

What's *not* in this minimum:

- No `Diff`, `Reverse`, `Cost`, `Permissions`. Add when you need them.
- No retries, no exotic behaviour. Add when a real use case appears.

### Step 3: the config struct

```go
// internal/config/config.go
type OSHostname struct {
    Name string `yaml:"name"`
}

type Step struct {
    // ...
    OSHostname *OSHostname `yaml:"os.hostname,omitempty" action:"os.hostname"`
}
```

The `action:""` tag is what wires it into the action vocabulary. The
schema is regenerated by `make schema-generate`.

### Step 4: the test

```go
// internal/actions/os_hostname/handler_test.go
package os_hostname

import (
    "testing"

    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
)

func TestRun_ImplementsRunner(t *testing.T) {
    var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
    cases := []struct {
        name    string
        step    *config.Step
        wantErr bool
    }{
        {"nil", &config.Step{}, true},
        {"empty name", &config.Step{OSHostname: &config.OSHostname{Name: " "}}, true},
        {"ok", &config.Step{OSHostname: &config.OSHostname{Name: "host1"}}, false},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            err := (&Handler{}).Validate(c.step)
            if (err != nil) != c.wantErr {
                t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
            }
        })
    }
}
```

What this test buys:

- The interface assertion (`var _ actions.Runner = &Handler{}`) is a
  compile-time check that the contract is satisfied. Cheap; mandatory.
- The validation table is exhaustive for the validation surface.
- It does *not* test `Run` against a real hostname — that's an
  integration test, gated by `testing.Short()` and run only in CI.

### Step 5: add `Diff` when ready

```go
// internal/actions/os_hostname/diff.go
package os_hostname

import (
    "os"

    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
)

type Snapshot struct {
    Name string `json:"name"`
}

func (Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
    desired, _ := ctx.GetTemplate().Render(step.OSHostname.Name, ctx.GetVariables())
    current, _ := os.Hostname()

    resource := actions.ResourceRef{
        Kind:       actions.ResourceOther,
        Identifier: "hostname",
    }
    if current == desired {
        return actions.Diff{
            Resource:  resource,
            Operation: actions.OpNoop,
            Before:    &Snapshot{Name: current},
            After:     &Snapshot{Name: desired},
        }, nil
    }
    return actions.Diff{
        Resource:  resource,
        Operation: actions.OpUpdate,
        Before:    &Snapshot{Name: current},
        After:     &Snapshot{Name: desired},
    }, nil
}

var _ actions.Differ = (*Handler)(nil)
```

The `var _ actions.Differ = (*Handler)(nil)` line is the same trick as
in tests — compile-time assertion that the type satisfies the
interface. Always include it next to opt-in interfaces.

### Step 6: pre-PR

```
make check-pkg PKG=internal/actions/os_hostname
make schema-generate     # if you added a new field to Step
make budget-status       # check Step field count
```

If all green, you're done.

---

## Closing notes

This document will be wrong in places. Let it be wrong, fix it as you
notice. The worst version of this guide is one that became
authoritative through age, not correctness.

When this document and the code disagree about what good code looks
like in this repo:

1. If the code is one example, the document wins.
2. If the code is the consistent practice across many places, the
   document is wrong — update it.
3. If neither is clearly right, talk to the project owner.

Keep it pragmatic. Keep it short where you can. Keep it concrete. The
point is shippable code that the next person can read.

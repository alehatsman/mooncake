# Action System Examples - Side-by-Side Comparison

This directory contains concrete examples showing what it takes to add a "notify" action using three different approaches.

## Quick Comparison

| Metric | Current | Approach 1 (Abstractions) | Approach 2 (Codegen) |
|--------|---------|---------------------------|----------------------|
| **Files you modify** | 7 | 3 | 2 |
| **Lines you write** | ~600 | ~215 | ~160 |
| **Time to implement** | 4-6 hours | 2-3 hours | 1-2 hours |
| **Schema updates** | Manual (150 lines) | Auto-generated | Auto-generated |
| **Dispatcher updates** | Manual | Auto (registry) | Auto (registry) |
| **Validation code** | Manual | Manual | Auto-generated |
| **Error handling** | Manual | Manual | Auto-generated |
| **Documentation** | Manual | Manual | Auto-generated |
| **Dry-run logging** | Manual | Handler method | Auto-generated |

## Files

### [CURRENT_APPROACH.md](./CURRENT_APPROACH.md)
Shows the current state - what developers have to do TODAY to add an action.

**Highlights:**
- 7 files to modify
- 150+ lines of JSON schema boilerplate
- Manual updates to countActions(), DetermineActionType(), Clone()
- Easy to forget steps
- ~600 lines of code

**Pain Points:**
- Must update 14 different oneOf blocks in schema.json
- Schema grows O(N²) with action count
- High chance of copy-paste errors
- No validation that you updated everything

### [APPROACH1_ABSTRACTIONS.md](./APPROACH1_ABSTRACTIONS.md)
Shows the Go abstractions approach using interfaces and registration.

**Highlights:**
- 3 files to modify (handler + config + events)
- ~215 lines of code
- Schema auto-generated from registry
- Standard Go patterns (interfaces, init() registration)
- Type-safe at compile time

**Developer Experience:**
```go
// 1. Implement Handler interface
type Handler struct{}

func (h *Handler) Metadata() ActionMetadata { ... }
func (h *Handler) Validate(step *Step) error { ... }
func (h *Handler) Execute(ctx *Context, step *Step) (*Result, error) { ... }
func (h *Handler) DryRun(ctx *Context, step *Step) error { ... }

// 2. Register in init()
func init() {
    actions.Register(&Handler{})
}

// Done! Everything else is automatic.
```

**Benefits:**
- 60% reduction in boilerplate
- Schema generation eliminates O(N²) growth
- Interface enforcement ensures consistency
- Familiar Go patterns

### [APPROACH2_CODEGEN.md](./APPROACH2_CODEGEN.md)
Shows the code generation approach using specification files.

**Highlights:**
- 2 files to create (spec + custom logic)
- ~160 lines of code
- Everything else auto-generated
- Single source of truth (spec file)

**Developer Experience:**
```yaml
# 1. Write spec file (notify.action.yaml)
action:
  name: notify
  config:
    channel:
      type: string
      required: true
      enum: [slack, email, webhook]
    message:
      type: string
      required: true
```

```go
// 2. Implement ONLY the business logic
func (h *Handler) ExecuteNotification(ctx *Context, cfg *Config) (map[string]interface{}, error) {
    // Your notification logic here
    return map[string]interface{}{
        "status": "sent",
    }, nil
}
```

```bash
# 3. Run generator
make generate-actions
```

**Benefits:**
- 90% reduction in boilerplate
- Config validation auto-generated
- Error types auto-generated
- Documentation auto-generated
- Schema auto-generated
- Dry-run logging auto-generated
- Event emission auto-wired

## Visual Comparison

### Current Approach
```
Developer writes:
├── config.go (25 lines)
├── schema.json (150 lines) ⚠️ O(N²) growth
├── notify_step.go (300 lines)
├── executor.go (5 lines)
├── dryrun.go (8 lines)
├── events.go (15 lines)
└── error_messages.go (5 lines)

Total: ~510 lines across 7 files
Time: 4-6 hours
```

### Approach 1 (Abstractions)
```
Developer writes:
├── notify/handler.go (200 lines)
│   ├── implements Handler interface
│   ├── Metadata()
│   ├── Validate()
│   ├── Execute()
│   └── DryRun()
├── config.go (3 lines - just add field)
└── events.go (8 lines)

Generated automatically:
└── schema.json (from registry)

Total: ~215 lines across 3 files
Time: 2-3 hours
```

### Approach 2 (Codegen)
```
Developer writes:
├── specs/notify.action.yaml (60 lines)
└── notify/custom.go (100 lines - business logic only)

Generated automatically:
├── notify/config.go (validation, defaults)
├── notify/handler.go (all 4 interface methods)
├── notify/errors.go (typed errors)
├── events/notify_events.go (event types)
├── schema_notify.json (merged into schema.json)
└── docs/actions/notify.md (documentation)

Total: ~160 lines across 2 files
Time: 1-2 hours
```

## When to Use Each Approach

### Stick with Current
**When:**
- Not planning to add many more actions (<3 per year)
- Team is very small (1 developer)
- Refactoring isn't a priority

**Tradeoff:** Accept ongoing maintenance burden

### Use Approach 1 (Abstractions)
**When:**
- Want moderate improvement without big investment
- Team is familiar with Go patterns
- Need something working in 2-3 weeks
- Planning to add 5-10 new actions per year

**Best for:** Small to medium teams, faster time to value

### Use Approach 2 (Codegen)
**When:**
- Planning aggressive feature development (10+ actions)
- Hiring multiple developers
- Want maximum productivity gains
- Can invest 4-5 weeks upfront
- Want to minimize onboarding time for new devs

**Best for:** Growing teams, long-term investment

## Hybrid Approach

Start with Approach 1, build generator later:

1. **Week 1-3**: Implement abstractions (interfaces + registry)
2. **Month 1-2**: Use new system, add 5-10 actions
3. **Month 2-3**: Build generator based on proven patterns
4. **Benefit**: Faster start + better end state

## Next Steps

1. Review the [main refactoring proposal](../action-system-refactoring.md)
2. Choose an approach based on your team size and timeline
3. Run a 1-week spike to validate the chosen approach
4. Implement incrementally (start with 2-3 pilot actions)

## Questions?

Consider:
- How many actions do you plan to add in the next year?
- How many developers will be working on mooncake?
- What's your risk tolerance for breaking changes?
- Do you prefer faster delivery or better long-term architecture?

See the [decision matrix](../action-system-refactoring.md#comparison-matrix) in the main proposal.

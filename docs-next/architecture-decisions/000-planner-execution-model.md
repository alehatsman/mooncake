# ADR-003: Planner and Execution Model

**Status**: Accepted
**Date**: 2026-02-05
**Deciders**: Engineering Team
**Technical Story**: Two-phase architecture for deterministic configuration execution

## Context

Early versions of mooncake executed configuration files directly, expanding directives (includes, loops, variables) at runtime as they were encountered. This approach had several problems:

1. **Non-Determinism**: Step order could vary based on runtime conditions
2. **Limited Introspection**: No way to see what would execute before running
3. **Error Discovery**: Syntax errors only discovered when reached
4. **No Dry-Run Support**: Couldn't preview execution without side effects
5. **Circular Dependencies**: Include cycles only detected at runtime
6. **Poor Observability**: No visibility into total steps before execution

Example problematic scenario:
```yaml
- vars:
    items: [a, b, c]

- shell: echo "{{ item }}"
  with_items: "{{ items }}"  # How many steps will this create? Unknown until runtime!

- include: other.yml  # What does this contain? Unknown until now!
  when: "{{ some_condition }}"  # Might not even be evaluated
```

The fundamental issue: **configuration expansion mixed with execution**, making it impossible to answer "What will this do?" before doing it.

## Decision

We adopted a **two-phase architecture** separating configuration expansion (planning) from execution:

**Benefits:**
- **Deterministic**: Same config always produces the same plan
- **Inspectable**: Use `mooncake plan` to see what will execute
- **Traceable**: Every step tracks its origin with include chain
- **Debuggable**: Understand loop expansions and includes before execution

### Phase 1: Planning (Compile-Time)
**Planner** expands configuration into a deterministic execution plan
- Resolves includes recursively
- Expands loops (with_items, with_filetree)
- Processes compile-time variables (vars, include_vars)
- Renders path templates
- Validates configuration structure
- Detects cycles
- Generates step IDs and origin metadata

**Output**: A `Plan` containing a flat list of executable steps

### Phase 2: Execution (Runtime)
**Executor** runs the pre-compiled plan step by step
- Evaluates runtime conditions (when, unless, creates)
- Executes actions through handlers
- Manages variables and results
- Emits events for observability
- Handles errors and failures

**Input**: A `Plan` from phase 1

### Key Architectural Principles

#### 1. Compile-Time vs Runtime Directives

**Compile-Time** (processed by planner):
- `include`: File inclusion
- `with_items`: Loop expansion
- `with_filetree`: Directory tree iteration
- `vars`: Variable setting (when condition evaluable at plan time)
- `include_vars`: Variable file loading (when condition evaluable at plan time)

**Runtime** (processed by executor):
- `when`: Conditional execution
- `unless`: Idempotency check (shell/command only)
- `creates`: Idempotency check (shell/command only)
- `changed_when`: Result override
- `failed_when`: Failure override
- `register`: Result capture

#### 2. Path Resolution Strategy

All relative paths resolved **at plan time** based on the file containing them:

```yaml
# File: /home/user/playbook/main.yml
- include: tasks/setup.yml  # Resolved to /home/user/playbook/tasks/setup.yml

# File: /home/user/playbook/tasks/setup.yml
- template:
    src: templates/config.j2   # Resolved to /home/user/playbook/tasks/templates/config.j2
    dest: /etc/app/config
```

**Rules**:
1. Relative paths joined with `CurrentDir` (directory of containing file)
2. Resolution happens during planning, before execution
3. Absolute paths used as-is
4. Include directives update `CurrentDir` for nested files
5. Loop context (with_filetree) doesn't change `CurrentDir`

#### 3. Variable Handling

Variables split into two categories:

**Plan-Time Variables** (available during expansion):
- Global vars from config (`vars:` at root level)
- CLI-provided vars (`--vars-file`)
- System facts (OS, architecture, etc.)
- Compile-time vars/include_vars (when condition evaluable)

**Runtime-Only Variables**:

- `register` results
- Loop variables (`item`, `index`, `first`, `last`)
- Vars/include_vars with runtime-dependent when conditions

**Why the split?**
- Plan-time: Needed for template expansion during planning
- Runtime-only: Not known until execution, stored in plan for later use

#### 4. Origin Tracking

Every step in the plan tracks its origin:

```go
type Origin struct {
    FilePath     string   // File containing this step
    Line         int      // Line number in file
    Column       int      // Column number in file
    IncludeChain []string // Chain of includes leading here
}
```

**Benefits**:

- Error messages show exact source location
- Debuggability: Can trace step to source
- Observability: Events include origin metadata
- Relative paths resolve correctly

#### 5. Loop Expansion

Loops expanded **during planning** into discrete steps:

**Input** (1 step):
```yaml
- shell: echo "{{ item }}"
  with_items: [a, b, c]
```

**Plan Output** (3 steps):
```yaml
- shell: echo "a"
  loop_context: {type: with_items, item: a, index: 0, first: true, last: false}
- shell: echo "b"
  loop_context: {type: with_items, item: b, index: 1, first: false, last: false}
- shell: echo "c"
  loop_context: {type: with_items, item: c, index: 2, first: false, last: true}
```

**Loop Variables Restored at Runtime**:
Executor uses `loop_context` to restore `item`, `index`, `first`, `last` into execution context before evaluating `when` conditions.

#### 6. Include Expansion

Includes expanded **recursively** during planning:

**Input**:
```yaml
# main.yml
- include: tasks/setup.yml

# tasks/setup.yml
- shell: echo "setup"
- include: common/base.yml

# common/base.yml
- shell: echo "base"
```

**Plan Output** (flat list):
```yaml
- shell: echo "setup"
  origin: {file: tasks/setup.yml, line: 1, chain: [main.yml:1]}
- shell: echo "base"
  origin: {file: common/base.yml, line: 1, chain: [main.yml:1, tasks/setup.yml:2]}
```

**Cycle Detection**:
Planner maintains `seenFiles` map and `includeStack` to detect cycles:

```go
if p.seenFiles[absIncludePath] {
    return fmt.Errorf("include cycle detected: %s\nChain: %s",
        absIncludePath, p.formatIncludeChain())
}
```

## Execution Flow

### Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ User Invokes: mooncake run playbook.yml --vars vars.yml    │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 1: PLANNING (Compile-Time)                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Load Config File                                         │
│     └─> Read playbook.yml                                    │
│     └─> Validate JSON schema                                 │
│                                                              │
│  2. Initialize Variables                                     │
│     └─> Load vars from --vars-file                           │
│     └─> Collect system facts                                 │
│     └─> Merge into variable context                          │
│                                                              │
│  3. Expand Configuration (Recursive)                         │
│     │                                                         │
│     ├─> include: path                                        │
│     │   ├─> Render path template with vars                   │
│     │   ├─> Resolve to absolute path                         │
│     │   ├─> Check for cycles (seenFiles map)                 │
│     │   ├─> Push to includeStack                             │
│     │   ├─> Recursively expand included file                 │
│     │   └─> Pop from includeStack                            │
│     │                                                         │
│     ├─> with_items: expr                                     │
│     │   ├─> Evaluate expression with vars                    │
│     │   ├─> For each item:                                   │
│     │   │   ├─> Create loop context (item, index, first, last)│
│     │   │   ├─> Clone step                                   │
│     │   │   └─> Render templates with loop vars              │
│     │   └─> Append N steps to plan                           │
│     │                                                         │
│     ├─> with_filetree: path                                  │
│     │   ├─> Walk directory tree                              │
│     │   ├─> Sort entries (determinism)                       │
│     │   ├─> For each file/dir:                               │
│     │   │   ├─> Create loop context with file metadata       │
│     │   │   ├─> Clone step                                   │
│     │   │   └─> Render templates with file vars              │
│     │   └─> Append N steps to plan                           │
│     │                                                         │
│     ├─> vars: {...}                                          │
│     │   ├─> Evaluate when condition (if present)             │
│     │   ├─> If false: skip (don't set vars)                  │
│     │   ├─> Render var values with current context           │
│     │   └─> Merge into variable context                      │
│     │                                                         │
│     ├─> include_vars: path                                   │
│     │   ├─> Evaluate when condition (if present)             │
│     │   ├─> If false: skip (don't load vars)                 │
│     │   ├─> Render path with current context                 │
│     │   ├─> Load YAML file                                   │
│     │   └─> Merge into variable context                      │
│     │                                                         │
│     └─> Regular Action (shell, file, etc)                    │
│         ├─> Render step name with vars                       │
│         ├─> Render action fields with vars                   │
│         ├─> Resolve relative paths (src, dest)               │
│         ├─> Generate step ID (step-0001, step-0002, ...)     │
│         ├─> Build origin metadata (file, line, chain)        │
│         ├─> Check tag filtering (skipped flag)               │
│         └─> Append to plan                                   │
│                                                              │
│  4. Plan Complete                                            │
│     └─> Flat list of executable steps with metadata          │
│                                                              │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 2: EXECUTION (Runtime)                                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Initialize Execution Context                             │
│     ├─> Variables: copy from plan.InitialVars                │
│     ├─> CurrentDir: directory of root config file            │
│     ├─> Logger, Template, Evaluator, etc.                    │
│     └─> Event publisher for observability                    │
│                                                              │
│  2. Emit Events                                              │
│     ├─> run.started                                          │
│     └─> plan.loaded                                          │
│                                                              │
│  3. For Each Step in Plan:                                   │
│     │                                                         │
│     ├─> Update Context                                       │
│     │   ├─> CurrentDir = dir of step.Origin.FilePath         │
│     │   └─> Restore loop vars if step.LoopContext present    │
│     │                                                         │
│     ├─> Check Skip Conditions                                │
│     │   ├─> when: evaluate expression with current vars      │
│     │   │   └─> Skip if false                                │
│     │   ├─> tags: check if step tags match filter            │
│     │   │   └─> Skip if no match                             │
│     │   ├─> creates: check if file exists                    │
│     │   │   └─> Skip if exists (idempotency)                 │
│     │   └─> unless: run command silently                     │
│     │       └─> Skip if succeeds (idempotency)               │
│     │                                                         │
│     ├─> If Skipped                                           │
│     │   ├─> Increment skipped counter                        │
│     │   ├─> Emit step.skipped event                          │
│     │   └─> Continue to next step                            │
│     │                                                         │
│     ├─> Generate Step ID                                     │
│     │   └─> Use step.ID from plan (step-0001, etc)           │
│     │                                                         │
│     ├─> Emit step.started Event                              │
│     │   └─> Include step ID, name, action, tags, origin      │
│     │                                                         │
│     ├─> Dispatch to Action Handler                           │
│     │   ├─> Lookup handler in registry by action type        │
│     │   ├─> Validate step configuration                      │
│     │   ├─> Execute or DryRun (based on --dry-run flag)      │
│     │   └─> Return result (changed, stdout, stderr, rc)      │
│     │                                                         │
│     ├─> Handle Result                                        │
│     │   ├─> Check changed_when (override changed flag)       │
│     │   ├─> Check failed_when (override failure)             │
│     │   ├─> Register to variable if step.Register set        │
│     │   └─> Store in context.CurrentResult                   │
│     │                                                         │
│     ├─> If Error                                             │
│     │   ├─> Increment failed counter                         │
│     │   ├─> Emit step.failed event                           │
│     │   └─> Return error (stop execution)                    │
│     │                                                         │
│     └─> If Success                                           │
│         ├─> Increment executed counter                       │
│         ├─> Emit step.completed event                        │
│         └─> Continue to next step                            │
│                                                              │
│  4. Emit run.completed Event                                 │
│     └─> Include stats (executed, skipped, failed, changed)   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Path Expansion Details

#### Include Path Resolution

```go
// 1. Render template
includePath, err := template.Render(*step.Include, ctx.Variables)
// Input:  "{{ env }}/tasks.yml"
// Vars:   {env: "production"}
// Output: "production/tasks.yml"

// 2. Resolve relative to current directory
absPath := filepath.Join(ctx.CurrentDir, includePath)
// CurrentDir: "/home/user/playbook"
// Output:     "/home/user/playbook/production/tasks.yml"

// 3. Make absolute
absPath, err := filepath.Abs(absPath)
// Output: "/home/user/playbook/production/tasks.yml"
```

#### Template Source Resolution

```go
// In planner.renderActionTemplates()
src, err := template.Render(step.Template.Src, ctx.Variables)
// Input:  "templates/{{ app_name }}.j2"
// Vars:   {app_name: "nginx"}
// Output: "templates/nginx.j2"

// Resolve relative to directory containing the step
if !filepath.IsAbs(src) {
    src = filepath.Join(ctx.CurrentDir, src)
}
// CurrentDir: "/home/user/playbook/tasks"
// Output:     "/home/user/playbook/tasks/templates/nginx.j2"
```

#### With FileTree Path Resolution

```go
// 1. Render template
treePath, err := template.Render(*step.WithFileTree, ctx.Variables)
// Input:  "files/{{ env }}"
// Vars:   {env: "prod"}
// Output: "files/prod"

// 2. Walk directory tree
items, err := fileTree.GetFileTree(treePath, ctx.CurrentDir, ctx.Variables)
// CurrentDir: "/home/user/playbook"
// Walks:      "/home/user/playbook/files/prod"
// Returns:    [{src: "/home/user/.../file1", path: "/file1", ...}, ...]

// 3. Sort for determinism
sort.Slice(items, func(i, j int) bool {
    return items[i].Src < items[j].Src
})

// 4. Expand step for each item
for i, item := range items {
    loopCtx := &config.LoopContext{
        Type:  "with_filetree",
        Item:  item,      // Full file metadata
        Index: i,
        Depth: calculateDepth(item.Path),
    }
    // Clone step with loop context...
}
```

## Alternatives Considered

### Alternative 1: Single-Phase Execution

**Approach**: Expand and execute simultaneously (original design)

**Pros**:

- Simpler architecture (no planner)
- Less code

**Cons**:

- Non-deterministic step count
- No dry-run support
- No plan introspection
- Late error discovery
- Poor observability

**Rejected**: Observability and determinism critical for production use

### Alternative 2: Three-Phase (Parse, Plan, Execute)

**Approach**: Add explicit parse phase before planning

**Pros**:

- Cleaner separation
- Earlier syntax error detection

**Cons**:

- More complexity
- Parse + Plan can be combined (current approach)
- No clear benefit

**Rejected**: Two phases sufficient, parsing happens in plan phase

### Alternative 3: Runtime Path Resolution

**Approach**: Resolve relative paths during execution, not planning

**Pros**:

- Paths could use runtime variables
- More flexible

**Cons**:

- Non-deterministic plan (paths change at runtime)
- Harder to cache/reuse plans
- Can't validate paths before execution
- Include resolution requires runtime

**Rejected**: Plan determinism more important than runtime flexibility

### Alternative 4: Lazy Loop Expansion

**Approach**: Don't expand loops during planning, expand at runtime

**Pros**:

- Smaller plans
- Could use runtime variables in with_items

**Cons**:

- Non-deterministic step count
- No way to show "X steps will execute"
- Dry-run can't show individual loop iterations
- Worse observability

**Rejected**: Observability requires knowing exact steps upfront

## Consequences

### Positive

1. **Determinism**
   - Same config = same plan = same execution order
   - Reproducible across runs
   - Testable and debuggable

2. **Observability**
   - Know total steps before execution
   - Show progress: "Step 42/150"
   - Dry-run shows exact steps
   - Events include complete context

3. **Early Error Detection**
   - Syntax errors found during planning
   - Include cycles detected before execution
   - Invalid loops fail fast

4. **Introspection**
   - Inspect plan structure
   - Analyze dependencies
   - Estimate execution time

5. **Optimization Opportunities**
   - Cache plans for reuse
   - Parallelize independent steps (future)
   - Skip unchanged steps (future)

6. **Better Error Messages**
   - Origin tracking shows exact source location
   - Include chain visible in errors
   - Loop context preserved

### Negative

1. **Memory Usage**
   - Full plan stored in memory
   - Large loops create many steps
   - Mitigation: Streaming execution (future)

2. **Two-Phase Complexity**
   - Developers must understand plan vs execute
   - Some logic duplicated (template rendering)
   - Mitigation: Clear documentation

3. **Variable Handling Split**
   - Plan-time vs runtime variables confusing
   - Users might expect runtime vars in templates
   - Mitigation: Clear error messages

4. **Limited Runtime Flexibility**
   - Can't change plan based on execution results
   - Loops must be known at plan time
   - Mitigation: Most use cases don't need this

### Risks

1. **Plan Size Explosion**
   - **Risk**: Very large with_filetree could OOM
   - **Mitigation**: Validate tree size before expansion
   - **Status**: Low risk, not observed in practice

2. **Variable Scope Confusion**
   - **Risk**: Users confused by plan-time vs runtime vars
   - **Mitigation**: Documentation, error messages
   - **Status**: Medium risk, needs good docs

3. **Path Resolution Bugs**
   - **Risk**: Edge cases in relative path handling
   - **Mitigation**: Comprehensive test suite
   - **Status**: Low risk, well tested

## Implementation Details

### Plan Data Structure

```go
type Plan struct {
    Version     string                 // Plan format version
    GeneratedAt time.Time              // When plan was created
    RootFile    string                 // Entry point config file
    Steps       []config.Step          // Fully expanded steps
    InitialVars map[string]interface{} // Variables at plan start
    Tags        []string               // Tag filter
}

type Step struct {
    // Plan metadata
    ID          string         // step-0001, step-0002, ...
    ActionType  string         // shell, file, template, ...
    Origin      *Origin        // Source location
    Skipped     bool           // Filtered by tags at plan time
    LoopContext *LoopContext   // Loop metadata (if from loop)

    // User configuration
    Name        string         // Step name
    When        string         // Runtime condition
    Register    string         // Variable to store result
    Tags        []string       // Step tags

    // Action-specific fields
    Shell       *ShellAction
    File        *FileAction
    Template    *TemplateAction
    // ... etc
}
```

### Planner Interface

```go
type Planner struct {
    template      template.Renderer
    pathUtil      *pathutil.PathExpander
    fileTree      *filetree.Walker
    stepIDCounter int
    includeStack  []IncludeFrame
    seenFiles     map[string]bool
    locationMap   map[int]*IncludeFrame
}

// BuildPlan generates a deterministic execution plan
func (p *Planner) BuildPlan(cfg PlannerConfig) (*Plan, error)

// ExpandStepsWithContext expands steps with given variables (for presets)
func (p *Planner) ExpandStepsWithContext(
    steps []config.Step,
    variables map[string]interface{},
    currentDir string,
) ([]config.Step, error)
```

### Executor Interface

```go
type ExecutionContext struct {
    Variables      map[string]interface{}
    CurrentDir     string
    CurrentFile    string
    CurrentResult  *Result
    CurrentStepID  string
    Level          int
    CurrentIndex   int
    TotalSteps     int

    // Dependencies
    Logger         logger.Logger
    Template       template.Renderer
    Evaluator      expression.Evaluator
    PathUtil       *pathutil.PathExpander
    FileTree       *filetree.Walker
    Redactor       *security.Redactor
    EventPublisher events.Publisher

    // Statistics
    Stats          *ExecutionStats

    // Configuration
    SudoPass       string
    Tags           []string
    DryRun         bool
}

// ExecutePlan executes a pre-compiled plan
func ExecutePlan(
    p *plan.Plan,
    sudoPass string,
    dryRun bool,
    log logger.Logger,
    publisher events.Publisher,
) error

// ExecuteStep executes a single step
func ExecuteStep(step config.Step, ec *ExecutionContext) error
```

## Example: Complete Flow

### Input Configuration

```yaml
# playbook.yml
vars:
  app: myapp
  env: production

- include: tasks/{{ env }}.yml

- shell: echo "Done"
  register: result
```

```yaml
# tasks/production.yml
- vars:
    replicas: 3

- shell: echo "Deploy {{ item }}"
  with_items: [web, api, worker]
```

### Planning Phase

1. **Load playbook.yml**
   - Parse YAML
   - Validate schema

2. **Initialize variables**
   ```go
   variables = {
       app: "myapp",
       env: "production",
       // + system facts
   }
   ```

3. **Expand steps**

   a. Process `vars: {app: myapp, env: production}`
      - Merge into variables
      - No step added to plan

   b. Process `include: tasks/{{ env }}.yml`
      - Render: "tasks/production.yml"
      - Resolve: "/home/user/playbook/tasks/production.yml"
      - Check cycles: not seen
      - Mark seen, push to stack
      - Recursively expand:

        c. Process `vars: {replicas: 3}` in production.yml
           - Merge into variables
           - variables = {app: "myapp", env: "production", replicas: 3}

        d. Process `shell` with `with_items: [web, api, worker]`
           - Evaluate with_items: [web, api, worker]
           - Clone step 3 times:
             - Step 1: `shell: echo "Deploy web"`, loop_context: {item: "web", index: 0, first: true, last: false}
             - Step 2: `shell: echo "Deploy api"`, loop_context: {item: "api", index: 1, first: false, last: false}
             - Step 3: `shell: echo "Deploy worker"`, loop_context: {item: "worker", index: 2, first: false, last: true}
           - Render commands: "Deploy web", "Deploy api", "Deploy worker"
           - Assign IDs: step-0001, step-0002, step-0003
           - Set origins: all from tasks/production.yml with chain [playbook.yml:3]
           - Add to plan

      - Pop from stack

   e. Process `shell: echo "Done"`
      - Render command: "Done"
      - Assign ID: step-0004
      - Set origin: playbook.yml:5
      - Add to plan

4. **Plan complete**
   ```go
   plan = &Plan{
       RootFile: "/home/user/playbook/playbook.yml",
       Steps: [
           {ID: "step-0001", ActionType: "shell", Shell: {Cmd: "echo \"Deploy web\""}, LoopContext: {...}},
           {ID: "step-0002", ActionType: "shell", Shell: {Cmd: "echo \"Deploy api\""}, LoopContext: {...}},
           {ID: "step-0003", ActionType: "shell", Shell: {Cmd: "echo \"Deploy worker\""}, LoopContext: {...}},
           {ID: "step-0004", ActionType: "shell", Shell: {Cmd: "echo \"Done\""}, Register: "result"},
       ],
       InitialVars: {app: "myapp", env: "production", replicas: 3, ...facts},
   }
   ```

### Execution Phase

1. **Initialize context**
   ```go
   ec = &ExecutionContext{
       Variables: plan.InitialVars,
       CurrentDir: "/home/user/playbook",
       TotalSteps: 4,
       // ... other fields
   }
   ```

2. **Emit events**
   - `run.started`: 4 total steps
   - `plan.loaded`: 4 total steps

3. **Execute step-0001**
   - Restore loop vars: item="web", index=0, first=true, last=false
   - Check when: (none)
   - Emit `step.started`: step-0001, "Deploy web"
   - Dispatch to shell handler
   - Execute: `echo "Deploy web"`
   - Result: stdout="Deploy web\n", changed=false, rc=0
   - Emit `step.completed`: step-0001, changed=false

4. **Execute step-0002**
   - Restore loop vars: item="api", index=1, first=false, last=false
   - (similar to step-0001)
   - Execute: `echo "Deploy api"`

5. **Execute step-0003**
   - Restore loop vars: item="worker", index=2, first=false, last=true
   - (similar to step-0001)
   - Execute: `echo "Deploy worker"`

6. **Execute step-0004**
   - Clear loop vars (no loop_context)
   - Check when: (none)
   - Emit `step.started`: step-0004, "Done"
   - Execute: `echo "Done"`
   - Result: stdout="Done\n", changed=false, rc=0
   - Register to variables: result = {stdout: "Done\n", changed: false, ...}
   - Emit `step.completed`: step-0004

7. **Emit run.completed**
   - executed=4, skipped=0, failed=0, changed=0

## Compliance

This ADR complies with:
- Go best practices for package separation
- Event-driven architecture patterns
- Immutable data structures (plan)
- Deterministic execution principles

## References

- [Planner Implementation](../../../internal/plan/planner.go) - Planning logic
- [Executor Implementation](../../../internal/executor/executor.go) - Execution logic
- [Plan Data Structure](../../../internal/plan/plan.go) - Plan format
- [Path Utilities](../../../internal/pathutil/pathutil.go) - Path resolution

## Related Decisions

- [ADR-001: Handler-Based Action Architecture](001-handler-based-action-architecture.md) - How actions are executed
- [ADR-002: Preset Expansion System](002-preset-expansion-system.md) - How presets integrate with planner

## Future Considerations

1. **Plan Caching**: Cache compiled plans for faster repeated execution
2. **Parallel Execution**: Execute independent steps concurrently
3. **Incremental Execution**: Skip unchanged steps based on checksums
4. **Plan Diff**: Show what changed between plan versions
5. **Plan Export**: Export plan as JSON for external tools
6. **Streaming Execution**: Process large plans without loading fully into memory
7. **Plan Optimization**: Reorder steps for efficiency (respecting dependencies)
8. **Conditional Includes**: Support when conditions on include directives

## Appendix: Why Not Ansible's Approach?

Ansible uses a similar two-phase model (parse → execute), but with key differences:

### Ansible's Approach
- **Templates at Runtime**: Ansible renders templates during execution, not planning
- **Dynamic Includes**: `include_tasks` expanded at runtime
- **Late Binding**: Variable resolution happens as late as possible

### Mooncake's Approach
- **Templates at Plan Time**: Most templates rendered during planning
- **Static Includes**: All includes expanded during planning
- **Early Binding**: Variable resolution happens as early as possible

### Why We Differ

1. **Determinism**: We prioritize knowing exact steps upfront
2. **Observability**: We want complete plan before execution
3. **Debugging**: Early errors better than late errors
4. **Simplicity**: Clear separation of concerns

Trade-off: Less runtime flexibility, but better observability and determinism.

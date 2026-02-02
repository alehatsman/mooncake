# Step and Action Architecture Design

Working document to decide on the Step/Action architecture for Approach 1.

## Current State

### Current Step Structure
```go
type Step struct {
    Name string `yaml:"name"`
    When string `yaml:"when"`

    // 14 separate action fields - only ONE can be set
    Template    *Template          `yaml:"template,omitempty"`
    File        *File              `yaml:"file,omitempty"`
    Shell       *ShellAction       `yaml:"shell,omitempty"`
    Command     *CommandAction     `yaml:"command,omitempty"`
    Copy        *Copy              `yaml:"copy,omitempty"`
    Unarchive   *Unarchive         `yaml:"unarchive,omitempty"`
    Download    *Download          `yaml:"download,omitempty"`
    Service     *ServiceAction     `yaml:"service,omitempty"`
    Assert      *Assert            `yaml:"assert,omitempty"`
    Preset      *PresetInvocation  `yaml:"preset,omitempty"`
    Print       *PrintAction       `yaml:"print,omitempty"`
    Include     *string            `yaml:"include,omitempty"`
    IncludeVars *string            `yaml:"include_vars,omitempty"`
    Vars        *map[string]interface{} `yaml:"vars,omitempty"`

    // Universal fields
    Become   bool     `yaml:"become"`
    Tags     []string `yaml:"tags"`
    Register string   `yaml:"register"`
    // ... etc
}
```

### Current YAML
```yaml
- name: Example
  template:
    src: template.j2
    dest: /tmp/output
  register: result
  tags: [deploy]
```

### Problems
1. **14 pointer fields** - wasteful, only 1 is ever non-nil
2. **Manual helper methods** - countActions(), DetermineActionType(), Clone() must check all 14
3. **Scales poorly** - adding action #15 requires updating many places
4. **Type safety** - nothing prevents setting multiple actions (relies on runtime validation)

---

## Design Questions

### Question 1: YAML Format

**Option 1A: Keep Current Format (Backward Compatible)**
```yaml
- name: Example
  template:
    src: template.j2
    dest: /tmp/output
```
- ✅ Zero breaking changes
- ✅ Existing playbooks work
- ❌ Still have 14 fields in Step struct
- ❌ Schema still complex

**Option 1B: Add Unified Format (New, But Support Both)**
```yaml
# New way (optional)
- name: Example
  action:
    type: template
    src: template.j2
    dest: /tmp/output

# Old way still works
- name: Example
  template:
    src: template.j2
    dest: /tmp/output
```
- ✅ No breaking changes (both work)
- ✅ Can migrate gradually
- ❌ Two ways to do same thing
- ❌ More complex during transition

**Option 1C: New Format Only (Breaking Change)**
```yaml
- name: Example
  action:
    type: template
    src: template.j2
    dest: /tmp/output
```
- ✅ Clean architecture
- ✅ Easier schema
- ❌ BREAKS all existing playbooks
- ❌ Requires migration tool

**Which option for YAML format?** _____

---

### Question 2: Step Struct Design

**Option 2A: Keep All Fields + Add Registry Lookup**
```go
type Step struct {
    Name string

    // Keep all 14 existing fields for backward compat
    Template *Template `yaml:"template,omitempty"`
    Shell    *ShellAction `yaml:"shell,omitempty"`
    // ... all 14 fields

    // Universal fields
    Become bool
    Tags   []string
}

// DetermineActionType() returns "template", "shell", etc.
// Dispatcher looks up handler from registry by action type
```
- ✅ Backward compatible
- ✅ Easy migration
- ❌ Still 14 pointer fields
- ❌ countActions/DetermineActionType still needed

**Option 2B: Unified Action Field (Clean Slate)**
```go
type ActionConfig struct {
    Type   string                 `yaml:"type"`   // "template", "shell", etc.
    Config map[string]interface{} `yaml:",inline"` // Actual config
}

type Step struct {
    Name   string
    Action *ActionConfig `yaml:"action"`

    // Universal fields
    Become bool
    Tags   []string
}
```
- ✅ Single field instead of 14
- ✅ DetermineActionType() trivial
- ❌ Loses type safety (map[string]interface{})
- ❌ Breaking change

**Option 2C: Hybrid - Unified + Type-Safe Configs**
```go
type Step struct {
    Name   string
    Action *ActionConfig `yaml:"action,omitempty"` // NEW

    // Keep for backward compat, marked deprecated
    Template *Template     `yaml:"template,omitempty"` // DEPRECATED
    Shell    *ShellAction  `yaml:"shell,omitempty"`    // DEPRECATED
    // ... all existing fields

    // Universal fields
    Become bool
    Tags   []string
}

type ActionConfig struct {
    Type     string      `yaml:"type"`
    Template *Template   `yaml:"template,omitempty"`
    Shell    *ShellAction `yaml:"shell,omitempty"`
    // ... one field per action type, only one set
}
```
- ✅ Backward compatible
- ✅ Type-safe
- ✅ Clear migration path
- ❌ Complex during transition
- ❌ Still many fields

**Option 2D: Use interface{} + Type Assertion**
```go
type Step struct {
    Name       string
    ActionType string      `yaml:"-"` // Computed
    ActionData interface{} `yaml:"-"` // Holds *Template, *Shell, etc.

    // Original fields for unmarshaling
    Template *Template     `yaml:"template,omitempty"`
    Shell    *ShellAction  `yaml:"shell,omitempty"`
    // ... all fields

    // Universal fields
    Become bool
    Tags   []string
}

// After unmarshaling, migrate to ActionData
func (s *Step) AfterUnmarshal() {
    if s.Template != nil {
        s.ActionType = "template"
        s.ActionData = s.Template
    } else if s.Shell != nil {
        s.ActionType = "shell"
        s.ActionData = s.Shell
    }
    // etc.
}
```
- ✅ Backward compatible YAML
- ✅ Clean internal representation
- ✅ Easy to work with in handlers
- ❌ Requires post-processing
- ❌ Need type assertions

**Which option for Step struct?** _____

---

### Question 3: Registry Integration

**Option 3A: Action Type String Lookup**
```go
// Step knows its action type
actionType := step.DetermineActionType() // "template"

// Get handler from registry
handler := actions.Get(actionType)

// Handler extracts its config from Step
handler.Execute(ctx, &step)
```
- ✅ Simple
- ✅ Handler knows which field to read
- ❌ Handler tightly coupled to Step struct

**Option 3B: Pass Config Directly**
```go
// Step extracts action config
actionType, config := step.GetAction() // "template", *Template

// Get handler from registry
handler := actions.Get(actionType)

// Handler receives just its config
handler.Execute(ctx, config)
```
- ✅ Handlers decoupled from Step
- ✅ Cleaner handler interface
- ❌ Need GetAction() method
- ❌ Handlers can't access universal Step fields

**Option 3C: Wrapper Object**
```go
type ActionContext struct {
    Type   string
    Config interface{}
    Step   *Step  // For universal fields
}

// Step creates context
actionCtx := step.ToActionContext()

// Handler receives context
handler.Execute(ctx, actionCtx)
```
- ✅ Best of both worlds
- ✅ Handler gets config + universal fields
- ❌ More objects to manage

**Which option for registry integration?** _____

---

## My Recommendations

Based on the analysis, I recommend:

### Phase 1: Minimal Change (Fastest)
- **YAML**: Option 1A (keep current format)
- **Step**: Option 2A (keep all fields)
- **Registry**: Option 3A (string lookup)

**Rationale**:
- Zero breaking changes
- Fastest implementation (2-3 weeks)
- Can refine later
- Still achieves main goals (registry pattern, consistent handlers)

### Phase 2: Refinement (Later)
- Add Option 1B (new YAML format as alternative)
- Migrate to Option 2D (clean internal representation)
- Update to Option 3C (wrapper object)

**Rationale**:
- Proven the approach works
- Can make breaking changes more confidently
- Users have time to prepare

---

## Decision Template

Please fill in your choices:

1. **YAML Format**: Option ___ because _______________
2. **Step Struct**: Option ___ because _______________
3. **Registry Integration**: Option ___ because _______________
4. **Migration Strategy**:
   - [ ] All at once (big bang)
   - [ ] Phased (start simple, refine later)
   - [ ] Hybrid (some changes now, some later)

5. **Top Priority**:
   - [ ] Backward compatibility
   - [ ] Clean architecture
   - [ ] Fast implementation
   - [ ] Easy to extend

---

## Next Steps

Once we decide:
1. I'll update the implementation plan
2. Create the chosen design
3. Migrate pilot action (print)
4. Validate approach
5. Continue with remaining actions

# Proposal: Dictionary Iteration (with_dict)

- **Author:** Example (@example-user)
- **Status:** Draft
- **Created:** 2026-02-02
- **Updated:** 2026-02-02

## Summary

Add `with_dict` to iterate over dictionaries (maps), providing access to both keys and values in each iteration.

## Motivation

Currently, Mooncake supports iterating over lists with `with_items` and files with `with_filetree`, but there's no way to iterate over dictionaries/maps. This is a common need when working with configurations that use key-value pairs.

### Use Cases

1. **Port Configuration:** Configure multiple services with different ports
```yaml
- vars:
    services:
      web: 80
      api: 8080
      admin: 9000

- name: Configure service port
  shell: echo "{{item.key}} runs on port {{item.value}}"
  with_dict: "{{services}}"
```

2. **Environment Variables:** Set multiple environment variables
```yaml
- vars:
    env_vars:
      NODE_ENV: production
      API_KEY: secret-key
      DB_HOST: localhost

- name: Export environment variable
  shell: export {{item.key}}={{item.value}}
  with_dict: "{{env_vars}}"
```

3. **User Permissions:** Configure permissions for multiple users
```yaml
- vars:
    permissions:
      alice: admin
      bob: editor
      charlie: viewer

- name: Set user permission
  shell: setperm {{item.key}} {{item.value}}
  with_dict: "{{permissions}}"
```

## Proposed Solution

### Configuration Syntax

```yaml
# Basic usage
- vars:
    ports:
      web: 80
      api: 8080

- name: Configure port
  shell: echo "{{item.key}}: {{item.value}}"
  with_dict: "{{ports}}"

# With file operations
- vars:
    configs:
      app: /etc/app/config
      nginx: /etc/nginx/nginx.conf
      db: /etc/postgresql/config

- name: Create config file
  file:
    path: "{{item.value}}"
    state: file
    content: "Configuration for {{item.key}}"
  with_dict: "{{configs}}"

# With conditionals
- vars:
    services:
      web: 80
      api: 8080
      admin: 9000

- name: Configure public service
  shell: configure-service {{item.key}} {{item.value}}
  with_dict: "{{services}}"
  when: item.key != "admin"
```

### Implementation Overview

1. **Config Package:**
   - Add `WithDict *string` field to `Step` struct
   - Update validation to check only one loop type is used

2. **Executor Package:**
   - Add dict iteration handler similar to `HandleWithItems`
   - Create dict item structure with `key` and `value` fields
   - Convert map to slice of key-value pairs for iteration

3. **Template Package:**
   - No changes needed - `item.key` and `item.value` work with existing templating

### Detailed Design

#### Data Structures

```go
// In config/config.go
type Step struct {
    // ... existing fields ...
    WithDict     *string                 `yaml:"with_dict" json:"with_dict,omitempty"`
    // ... rest of fields ...
}

// In executor package
type DictItem struct {
    Key   string
    Value interface{}
}
```

#### Execution Flow

1. Parse `with_dict` expression and resolve variable
2. Validate that value is a map
3. Convert map to sorted slice of DictItem (sorted by key for consistency)
4. For each DictItem:
   - Set `item.key` and `item.value` in execution context
   - Execute the step
5. Clean up and return

#### Error Handling

```yaml
# Error: with_dict value is not a map
- name: Invalid dict
  shell: echo {{item.key}}
  with_dict: "{{not_a_dict}}"
# Error: "with_dict variable 'not_a_dict' is not a map, got: []string"

# Error: with_dict variable not found
- name: Missing dict
  shell: echo {{item.key}}
  with_dict: "{{undefined_dict}}"
# Error: "with_dict variable not found: undefined_dict"

# Error: multiple loop types
- name: Multiple loops
  shell: echo {{item}}
  with_items: "{{list}}"
  with_dict: "{{dict}}"
# Error: "step cannot have multiple loop types (with_items, with_dict)"
```

### Examples

#### Example 1: Service Port Configuration

```yaml
- vars:
    services:
      nginx: 80
      api: 8080
      metrics: 9090

- name: Configure service port
  file:
    path: "/etc/services/{{item.key}}.conf"
    state: file
    content: |
      [Service]
      Name={{item.key}}
      Port={{item.value}}
    mode: "0644"
  with_dict: "{{services}}"
```

#### Example 2: Template Rendering for Multiple Apps

```yaml
- vars:
    apps:
      frontend:
        port: 3000
        env: production
      backend:
        port: 8000
        env: production

- name: Render app config
  template:
    src: ./app-config.j2
    dest: "/etc/apps/{{item.key}}/config.yml"
    mode: "0644"
    vars:
      app_name: "{{item.key}}"
      config: "{{item.value}}"
  with_dict: "{{apps}}"
```

## Alternatives Considered

### Alternative 1: Extend with_items to Handle Dicts

**Approach:** Make `with_items` automatically detect dict type and provide key/value.

**Pros:**
- No new syntax needed
- Single iteration construct

**Cons:**
- Implicit behavior (hard to understand)
- Breaking change for existing configs using dicts
- `item` would have different structure depending on input type
- Harder to document and explain

**Why rejected:** Too implicit, confusing behavior.

### Alternative 2: Use Template Filters

**Approach:** Add template filters to convert dicts to lists.

```yaml
- name: Process dict
  shell: echo {{item}}
  with_items: "{{ my_dict | dict_to_list }}"
```

**Pros:**
- No new step-level feature
- Flexible

**Cons:**
- Verbose and awkward
- Requires understanding of filters
- Less discoverable
- Still need to parse key/value from items

**Why rejected:** Not ergonomic, harder to use.

### Alternative 3: dict2items Filter (Ansible-style)

**Approach:** Provide a filter that transforms dicts for with_items.

```yaml
- name: Process dict
  shell: echo {{item.key}}: {{item.value}}
  with_items: "{{ my_dict | dict2items }}"
```

**Pros:**
- Similar to Ansible
- Reuses existing with_items

**Cons:**
- Less obvious than dedicated with_dict
- Requires filter knowledge
- Extra step for common operation

**Why rejected:** `with_dict` is more intuitive.

## Compatibility

### Backward Compatibility

- [x] No breaking changes
- [ ] Breaking changes (explain below)

This is a pure addition - no existing configurations are affected.

### Migration Path

N/A - no migration needed.

## Implementation Plan

### Phase 1: Core Implementation (4-6 hours)

- [ ] Add `WithDict` field to `Step` struct
- [ ] Update `Step.Validate()` to check for multiple loop types
- [ ] Implement `HandleWithDict` in executor
- [ ] Add dict iteration logic
- [ ] Handle error cases (not a map, missing variable)

### Phase 2: Testing (3-4 hours)

- [ ] Unit tests for HandleWithDict
- [ ] Test with various dict types (string values, int values, nested)
- [ ] Test error cases
- [ ] Test with conditionals
- [ ] Test with file operations
- [ ] Integration tests

### Phase 3: Documentation (2-3 hours)

- [ ] Update README with with_dict section
- [ ] Create example in examples/06-loops/
- [ ] Update 06-loops/README.md
- [ ] Add to Configuration Reference
- [ ] Update ROADMAP.md

### Estimated Effort

- Implementation: 4-6 hours
- Testing: 3-4 hours
- Documentation: 2-3 hours
- **Total:** 9-13 hours

## Open Questions

1. **Iteration order:** Should we guarantee key order (sorted) or allow random?
   - **Proposal:** Sort keys alphabetically for consistency and predictability

2. **Nested dicts:** Should `item.value` support nested access like `item.value.nested`?
   - **Proposal:** Yes, this should work naturally with template engine

3. **Empty dicts:** How to handle empty dicts?
   - **Proposal:** Skip step entirely (no iterations), log debug message

4. **Non-string keys:** Should we support int/float keys?
   - **Proposal:** Convert all keys to strings for consistency

## References

- Similar feature in Ansible: [loop with dict2items](https://docs.ansible.com/ansible/latest/user_guide/playbooks_loops.html#iterating-over-a-dictionary)
- Related issue: #XXX (create when ready)

## Decision

**Date:** TBD
**Decision:** Pending community feedback
**Reason:** Awaiting review and discussion

# AI Specification

**For AI Agents, LLMs, and Autonomous Systems**

This page provides specifications for AI agents to generate, validate, and execute Mooncake configurations safely.

---

## Quick Reference for AI Agents

Mooncake is a **safe, validated execution layer** for system configuration. When generating Mooncake configurations:

 **DO:**
- Always use dry-run mode first (`--dry-run`)
- Validate configurations before execution
- Use idempotent actions (file, template, service)
- Leverage system facts for cross-platform configs
- Include descriptive `name` fields for observability
- Use `when` conditions for platform-specific logic
- Register results for conditional workflows

 **DON'T:**
- Execute arbitrary shell commands without validation
- Assume file paths exist without checking
- Ignore error handling (use `failed_when`, `changed_when`)
- Skip dry-run validation
- Use hard-coded system paths (use facts: `{{home}}`, `{{user}}`)

---

## Safety Model

### Execution Guarantees

1. **Dry-run Validation** - Preview all changes before applying
2. **Idempotency** - Safe to run multiple times
3. **Schema Validation** - Type checking before execution
4. **Audit Trail** - All actions logged with structured events
5. **Rollback Support** - File backups and state tracking

### Risk Levels

| Action Type | Risk | Validation Required |
|-------------|------|---------------------|
| `file`, `template`, `copy` | Low | Schema only |
| `service`, `download` | Medium | Dry-run + review |
| `shell`, `command` | High | Explicit user approval |

---

## Configuration Schema

### Basic Structure

```yaml
- name: "Descriptive step name"
  <action>:
    <properties>
  when: "conditional_expression"  # Optional
  register: result_variable       # Optional
  tags: ["tag1", "tag2"]         # Optional
```

### Available Actions

| Action | Purpose | Safety | Idempotent |
|--------|---------|--------|------------|
| **shell** | Execute shell commands |  High risk |  Manual |
| **command** | Execute binary directly |  High risk |  Manual |
| **file** | Manage files/directories |  Safe |  Yes |
| **template** | Render Jinja2 templates |  Safe |  Yes |
| **copy** | Copy with checksums |  Safe |  Yes |
| **download** | Fetch from URLs |  Medium |  Yes |
| **unarchive** | Extract archives |  Medium |  Yes |
| **service** | Manage systemd/launchd |  Medium |  Yes |
| **assert** | Verify system state |  Safe |  Yes |
| **preset** | Reusable workflows | Varies | Varies |
| **vars** | Define variables |  Safe |  Yes |
| **include_vars** | Load from YAML |  Safe |  Yes |

---

## System Facts Reference

Auto-detected variables available in templates:

### Platform Facts
```yaml
{{os}}              # "linux", "darwin", "windows"
{{arch}}            # "amd64", "arm64", "386"
{{distribution}}    # "ubuntu", "debian", "fedora", "macos", etc.
{{os_version}}      # "22.04", "13.0", etc.
{{kernel}}          # "Linux", "Darwin"
{{package_manager}} # "apt", "dnf", "brew", "port"
```

### Hardware Facts
```yaml
{{cpu_cores}}       # Number of CPU cores
{{cpu_model}}       # CPU model name
{{memory_total_mb}} # Total RAM in MB
{{memory_free_mb}}  # Free RAM in MB
```

### Environment Facts
```yaml
{{home}}            # User home directory
{{user}}            # Current username
{{hostname}}        # System hostname
{{shell}}           # User's default shell
```

### Network Facts
```yaml
{{ip_addresses}}    # List of IP addresses
{{default_gateway}} # Default gateway IP
{{dns_servers}}     # List of DNS servers
```

**Get all facts:** `mooncake facts --format json`

---

## AI-Friendly Patterns

### Pattern 1: Safe Shell Execution

```yaml
#  Unsafe
- name: Install package
  shell: sudo apt-get install -y nginx

#  Safe
- name: Check if nginx installed
  shell: which nginx
  register: nginx_check
  failed_when: false

- name: Install nginx
  shell: apt-get install -y nginx
  become: true
  when: nginx_check.rc != 0 and package_manager == "apt"
```

### Pattern 2: Cross-Platform Configuration

```yaml
# Use facts for platform-specific logic
- name: Install package
  shell: "{{package_manager}} install -y neovim"
  become: true
  when: package_manager in ["apt", "dnf", "yum"]

- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"
```

### Pattern 3: Idempotent File Management

```yaml
#  Idempotent
- name: Create config directory
  file:
    path: "{{home}}/.config/myapp"
    state: directory
    mode: "0755"

- name: Deploy config
  template:
    src: config.j2
    dest: "{{home}}/.config/myapp/config.yml"
    mode: "0644"
```

### Pattern 4: Conditional Workflows

```yaml
- name: Check service status
  shell: systemctl is-active nginx
  register: nginx_status
  failed_when: false

- name: Start nginx if not running
  service:
    name: nginx
    state: started
    enabled: true
  become: true
  when: nginx_status.rc != 0
```

### Pattern 5: Error Handling

```yaml
- name: Download with retry
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
    checksum: sha256:abc123...
  retries: 3
  retry_delay: 5

- name: Verify download
  assert:
    file:
      path: /tmp/file.tar.gz
      exists: true
```

---

## Validation Workflow

### Step 1: Schema Validation

Mooncake validates:
- YAML syntax
- Action types
- Required properties
- Type constraints
- Enum values

**Automatic** - happens before execution.

### Step 2: Dry-run Mode

```bash
mooncake run --config config.yml --dry-run
```

Shows:
- What would change
- Files that would be created/modified
- Commands that would run
- Service state changes

**Required for AI agents** - always validate before execution.

### Step 3: Execution

```bash
mooncake run --config config.yml
```

Provides:
- Real-time progress
- Structured event logs
- Change tracking
- Error reporting

---

## Event Stream

Mooncake emits structured events for observability:

```json
{
  "type": "step_started",
  "step_name": "Install nginx",
  "action": "shell",
  "timestamp": "2026-02-05T21:00:00Z"
}

{
  "type": "step_completed",
  "step_name": "Install nginx",
  "changed": true,
  "failed": false,
  "duration_ms": 1234
}
```

**Available via:** `--events-json <file>` flag

---

## Integration Patterns

### Claude Code Integration

```python
# Generate configuration
config = generate_mooncake_config(user_intent)

# Validate with dry-run
result = subprocess.run(
    ["mooncake", "run", "--config", config, "--dry-run"],
    capture_output=True
)

# Show user what will change
print(result.stdout)

# Get user approval
if approve(result.stdout):
    # Execute
    subprocess.run(["mooncake", "run", "--config", config])
```

### Autonomous Agent Pattern

```python
# 1. Detect system state
facts = get_system_facts()

# 2. Generate configuration
config = generate_config_from_intent(
    user_goal=goal,
    system_facts=facts,
    constraints=safety_rules
)

# 3. Validate
validation = validate_config(config)
if not validation.safe:
    return validation.errors

# 4. Dry-run
preview = dry_run(config)

# 5. Log to audit trail
log_planned_changes(preview)

# 6. Execute if approved
if auto_approve or human_approved:
    execute(config)
    log_execution_results()
```

### Multi-Step Workflow

```yaml
# Step 1: Gather facts
- name: Detect package manager
  shell: |
    if command -v apt &> /dev/null; then
      echo "apt"
    elif command -v dnf &> /dev/null; then
      echo "dnf"
    fi
  register: detected_pm

# Step 2: Use facts
- name: Install packages
  shell: "{{detected_pm.stdout}} install -y git curl"
  become: true
  when: detected_pm.stdout != ""
```

---

## Error Handling Guide

### Common Errors

**1. Schema Validation Failed**
```
Error: Step has multiple actions. Only ONE action allowed.
```
→ Each step must have exactly one action

**2. Template Rendering Failed**
```
Error: Variable 'package_manager' not found
```
→ Check system facts or define custom variables

**3. Command Execution Failed**
```
Error: Command exited with code 1
```
→ Use `failed_when` to customize failure detection

### Debugging

```bash
# Verbose output
mooncake run --config config.yml --log-level debug

# Dry-run with verbose
mooncake run --config config.yml --dry-run --log-level debug

# Output facts for inspection
mooncake facts --format json > facts.json
```

---

## Compliance & Security

### Audit Requirements

For compliance-sensitive environments:

```bash
# Log all events
mooncake run --config config.yml --events-json audit.jsonl

# Facts snapshot
mooncake facts --format json > system-state.json

# Generate report
mooncake run --config config.yml | tee execution.log
```

### Security Best Practices

1. **Validate user input** - Never pass unsanitized input to shell
2. **Use facts** - Avoid hard-coded system paths
3. **Dry-run always** - Preview before executing
4. **Least privilege** - Use `become` only when necessary
5. **Audit trail** - Log all actions with `--events-json`
6. **Checksum validation** - Use checksums for downloads
7. **State assertions** - Verify system state with `assert`

---

## Model-Specific Guidance

### For Claude (Anthropic)

Claude excels at:
- Generating safe, idempotent configurations
- Cross-platform logic with system facts
- Error handling and retry strategies
- Template-driven configs

**Example prompt:**
> Generate a Mooncake configuration to install Docker on Ubuntu/Debian systems. Include error checking, idempotency, and use system facts.

### For GPT Models (OpenAI)

GPT models should:
- Focus on schema compliance
- Use dry-run validation extensively
- Leverage the complete reference docs
- Test cross-platform scenarios

### For Open Models (Llama, Mistral, etc.)

Recommendations:
- Use simpler action patterns first
- Validate each step with `assert`
- Build incrementally
- Reference examples extensively

---

## Complete Examples

### Example 1: Full Stack Setup

```yaml
- name: Detect system
  shell: uname -s
  register: system

- name: Install Docker (Ubuntu)
  shell: |
    apt-get update
    apt-get install -y docker.io
  become: true
  when: distribution in ["ubuntu", "debian"]

- name: Start Docker service
  service:
    name: docker
    state: started
    enabled: true
  become: true

- name: Verify Docker
  assert:
    command:
      cmd: docker --version
```

### Example 2: Dotfiles Deployment

```yaml
- name: Create config directories
  file:
    path: "{{home}}/.config/{{item}}"
    state: directory
    mode: "0755"
  with_items:
    - nvim
    - tmux
    - zsh

- name: Deploy configs
  template:
    src: "templates/{{item}}.j2"
    dest: "{{home}}/.config/{{item}}/config"
    mode: "0644"
  with_items:
    - nvim
    - tmux

- name: Verify deployment
  assert:
    file:
      path: "{{home}}/.config/nvim/config"
      exists: true
```

---

## API Reference

### CLI Commands for AI Integration

```bash
# Run configuration
mooncake run --config <file>

# Dry-run mode
mooncake run --config <file> --dry-run

# With events output
mooncake run --config <file> --events-json <output>

# Get system facts
mooncake facts [--format json|text]

# Validate syntax only
mooncake validate --config <file>

# Show version
mooncake --version
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Execution failed |
| 2 | Validation failed |
| 3 | Configuration error |

---

## Support & Resources

**Documentation:** [mooncake.alehatsman.com](https://mooncake.alehatsman.com)

**Key Pages:**
- [Actions Guide](guide/config/actions.md) - All available actions with examples
- [Complete Reference](guide/config/reference.md) - All properties and types
- [Examples](examples/) - Real-world configurations

**GitHub:** [github.com/alehatsman/mooncake](https://github.com/alehatsman/mooncake)

---

## Changelog

**2026-02-05** - Initial AI Specification
- Added safety model and risk levels
- Documented system facts reference
- Provided AI-friendly patterns
- Integration examples for Claude and autonomous agents

# Using Presets

Presets are reusable, parameterized collections of steps that can be invoked as a single action. They provide a way to encapsulate complex workflows into simple, declarative configurations.

## What is a Preset?

A preset is essentially a YAML file that defines:
- **Parameters**: Configurable inputs with types, defaults, and validation
- **Steps**: A sequence of mooncake steps to execute
- **Metadata**: Name, description, and version information

Think of presets as functions or modules - they take parameters and execute a predefined sequence of operations.

## Why Use Presets?

**Benefits:**
- **Reusability**: Write once, use everywhere
- **Maintainability**: Update logic in one place
- **Discoverability**: Share presets as files, no code changes needed
- **Simplicity**: Complex workflows become single-line declarations
- **Type Safety**: Parameter validation catches errors early

**Example**: Instead of writing 20+ steps to install Ollama, configure the service, and pull models, you can write:

```yaml
- preset: ollama
  with:
    state: present
    service: true
    pull: [llama3.1:8b]
```

## Basic Usage

### Simple Invocation

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
```

### With Parameters

```yaml
- name: Install Ollama with full configuration
  preset: ollama
  with:
    state: present
    service: true
    method: auto
    host: "0.0.0.0:11434"
    models_dir: "/data/ollama"
    pull:
      - "llama3.1:8b"
      - "mistral:latest"
    force: false
  become: true
  register: ollama_result
```

### String Shorthand

For presets without parameters:

```yaml
- name: Quick preset invocation
  preset: my-preset
```

Is equivalent to:

```yaml
- name: Quick preset invocation
  preset:
    name: my-preset
```

## Parameters

### Accessing Parameters in Presets

When a preset is executed, its parameters are available in the `parameters` namespace:

```yaml
# In preset definition
- name: Show parameter value
  shell: echo "State is {{ parameters.state }}"
```

This namespacing prevents collisions with variables and facts.

### Parameter Types

Presets support four parameter types:

| Type | Description | Example |
|------|-------------|---------|
| `string` | Text value | `"present"`, `"localhost:11434"` |
| `bool` | Boolean | `true`, `false` |
| `array` | List of values | `["item1", "item2"]` |
| `object` | Key-value map | `{key: "value"}` |

### Default Values

Parameters can have defaults:

```yaml
# Preset definition
parameters:
  service:
    type: bool
    default: true
    description: Enable service
```

```yaml
# User playbook (uses default service: true)
- preset: ollama
  with:
    state: present
```

### Required Parameters

Mark critical parameters as required:

```yaml
# Preset definition
parameters:
  state:
    type: string
    required: true
    enum: [present, absent]
```

```yaml
# User playbook - fails without 'state'
- preset: ollama  # ERROR: required parameter 'state' not provided
```

### Enum Constraints

Restrict parameters to specific values:

```yaml
# Preset definition
parameters:
  method:
    type: string
    enum: [auto, script, package]
```

```yaml
# User playbook - fails with invalid value
- preset: ollama
  with:
    method: invalid  # ERROR: invalid value, allowed: [auto, script, package]
```

## Preset Discovery

Mooncake searches for presets in this order (highest priority first):

1. `./presets/` - Playbook directory
2. `~/.mooncake/presets/` - User presets
3. `/usr/local/share/mooncake/presets/` - Local installation
4. `/usr/share/mooncake/presets/` - System installation

### Preset File Formats

Presets can use two formats:

**Flat format** (simple presets):
```
presets/
└── mypreset.yml
```

**Directory format** (complex presets with includes):
```
presets/
└── mypreset/
    ├── preset.yml       # Main definition
    ├── tasks/           # Modular task files
    │   ├── install.yml
    │   └── configure.yml
    └── templates/       # Configuration templates
        └── config.j2
```

When both exist, the directory format takes precedence:
- `presets/ollama/preset.yml` is loaded before `presets/ollama.yml`

### Example Directory Structure

```
my-project/
├── playbook.yml
└── presets/
    ├── ollama/          # Directory-based preset
    │   ├── preset.yml
    │   └── tasks/
    │       └── install.yml
    └── myapp.yml        # Flat preset

~/.mooncake/
└── presets/
    └── common.yml       # User-wide preset

/usr/share/mooncake/presets/
└── ollama/              # Built-in directory preset
    ├── preset.yml
    ├── tasks/
    ├── templates/
    └── README.md
```

## Result Registration

Presets support result registration at the preset level:

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
  register: ollama_result

- name: Check if changed
  shell: echo "Changed = {{ ollama_result.changed }}"
```

**Preset results contain:**
- `changed`: `true` if any step changed
- `stdout`: Summary message
- `rc`: Always 0 (success) or error
- `failed`: `false` on success

## Conditionals and Loops

Presets work with all standard step features:

### When Conditions

```yaml
- name: Install Ollama on Linux only
  preset: ollama
  with:
    state: present
  when: os == "linux"
```

### Tags

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
  tags: [setup, llm]
```

### Loops

```yaml
- name: Setup multiple LLM backends
  preset: ollama
  with:
    state: present
    pull: ["{{ item }}"]
  with_items: "{{ llm_models }}"
```

## Error Handling

### Preset Errors

Presets can fail at two levels:

1. **Parameter validation**: Before execution
   ```
   Error: preset 'ollama' parameter validation failed:
   required parameter 'state' not provided
   ```

2. **Step execution**: During execution
   ```
   Error: preset 'ollama' step 3 failed:
   installation via package manager failed
   ```

### Failed When

```yaml
- name: Try installing Ollama
  preset: ollama
  with:
    state: present
  register: ollama_result
  failed_when: false

- name: Handle failure
  shell: echo "Installation failed"
  when: ollama_result.failed
```

## Dry Run Mode

Presets fully support dry-run mode:

```bash
mooncake run -c playbook.yml --dry-run
```

Output:
```
  [DRY-RUN] Would expand preset 'ollama' with 3 parameters
▶ Install Ollama
✓ Install Ollama
```

In dry-run mode, presets:
- Show parameter count
- Don't execute steps (but may expand them for display)
- Return `changed: true` (pessimistic assumption)

## Best Practices

### 1. Use Presets for Complex Workflows

**Good** (preset hides complexity):
```yaml
- preset: ollama
  with:
    state: present
    service: true
```

**Avoid** (simple operations don't need presets):
```yaml
- preset: echo-hello  # Just use: shell: echo "hello"
```

### 2. Provide Sensible Defaults

```yaml
# Good: Service enabled by default (most common use case)
parameters:
  service:
    type: bool
    default: true
```

### 3. Use Descriptive Names

```yaml
# Good
- preset: ollama

# Bad
- preset: install-llm  # Too generic
- preset: ollama-installer-and-service-configurator  # Too verbose
```

### 4. Document Parameters

```yaml
parameters:
  host:
    type: string
    description: Ollama server bind address (e.g., 'localhost:11434', '0.0.0.0:11434')
```

### 5. Handle Platform Differences

Use `when` conditions in preset steps:

```yaml
# In preset definition
- name: Install via apt (Linux)
  shell: apt-get install -y ollama
  when: apt_available and os == "linux"

- name: Install via brew (macOS)
  shell: brew install ollama
  when: brew_available and os == "darwin"
```

## Common Patterns

### Configuration Template

```yaml
- name: Deploy app with generated config
  preset: myapp
  with:
    version: "1.2.3"
    config:
      database_url: "{{ db_url }}"
      cache_enabled: true
```

### Multi-Stage Deployment

```yaml
- name: Stage 1 - Dependencies
  preset: install-deps

- name: Stage 2 - Application
  preset: deploy-app
  with:
    environment: production

- name: Stage 3 - Healthcheck
  preset: verify-deployment
```

### Conditional Installation

```yaml
- name: Check if already installed
  shell: which ollama
  register: check
  failed_when: false

- name: Install if not present
  preset: ollama
  with:
    state: present
  when: check.rc != 0
```

## Limitations

Current preset system limitations:

1. **No nesting**: Presets cannot call other presets
2. **Flat parameters**: No nested parameter validation
3. **No outputs**: Presets don't define explicit output schemas
4. **No dependencies**: Can't declare preset-level dependencies

These are architectural decisions to keep presets simple and predictable.

## Troubleshooting

### Preset Not Found

```
Error: preset 'mypreset' not found in search paths:
[./presets, ~/.mooncake/presets, /usr/share/mooncake/presets]
```

**Solution**: Check preset filename matches (`mypreset.yml`) and is in a search path.

### Parameter Type Mismatch

```
Error: parameter 'service' must be a boolean, got string
```

**Solution**: Check parameter types in your invocation:
```yaml
with:
  service: true  # Not "true"
```

### Unknown Parameter

```
Error: unknown parameter 'services' (preset 'ollama' does not define this parameter)
```

**Solution**: Check parameter spelling in preset definition.

## Next Steps

- [Create your own presets](preset-authoring.md)
- [View available presets](#) <!-- TODO: Add preset catalog -->
- [Examples directory](../../examples/)

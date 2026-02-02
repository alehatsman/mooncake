# 10 - Multi-File Configurations

Learn how to organize large configurations into multiple files.

## What You'll Learn

- Splitting configuration into multiple files
- Using `include` to load other configs
- Using `include_vars` to load variables
- Organizing by environment (dev/prod)
- Organizing by platform (Linux/macOS)
- Relative path resolution

## Quick Start

```bash
# Run with development environment (default)
mooncake run --config main.yml

# Run with specific tags
mooncake run --config main.yml --tags install
mooncake run --config main.yml --tags dev
```

## Directory Structure

```
10-multi-file-configs/
├── main.yml              # Entry point
├── tasks/                # Modular task files
│   ├── common.yml        # Common setup
│   ├── linux.yml         # Linux-specific
│   ├── macos.yml         # macOS-specific
│   └── dev-tools.yml     # Development tools
└── vars/                 # Environment variables
    ├── development.yml   # Dev settings
    └── production.yml    # Prod settings
```

## What It Does

1. Sets project variables
2. Loads environment-specific variables
3. Runs common setup tasks
4. Runs OS-specific tasks (Linux or macOS)
5. Conditionally runs dev tools setup

## Key Concepts

### Entry Point (main.yml)

The main file orchestrates everything:
```yaml
- vars:
    project_name: MyProject
    env: development

- name: Load environment variables
  include_vars: ./vars/{{env}}.yml

- name: Setup common configuration
  include: ./tasks/common.yml

- name: Setup OS-specific configuration
  include: ./tasks/macos.yml
  when: os == "darwin"
```

### Including Variable Files

Load variables from external YAML:
```yaml
- name: Load development vars
  include_vars: ./vars/development.yml
```

**vars/development.yml:**
```yaml
debug: true
port: 8080
database_host: localhost
```

### Including Task Files

Load and execute tasks from other files:
```yaml
- name: Run common setup
  include: ./tasks/common.yml
```

**tasks/common.yml:**
```yaml
- name: Create project directory
  file:
    path: /tmp/{{project_name}}
    state: directory
```

### Relative Path Resolution

Paths are relative to the **current file**, not the working directory:

```
main.yml:
  include: ./tasks/common.yml  # Relative to main.yml

tasks/common.yml:
  template:
    src: ./templates/config.j2  # Relative to common.yml, not main.yml
```

### Organization Strategies

**By Environment:**
```
vars/
  development.yml
  staging.yml
  production.yml
```

**By Platform:**
```
tasks/
  linux.yml
  macos.yml
  windows.yml
```

**By Component:**
```
tasks/
  database.yml
  webserver.yml
  cache.yml
```

**By Phase:**
```
tasks/
  00-prepare.yml
  01-install.yml
  02-configure.yml
  03-deploy.yml
```

## Real-World Example

### Project Structure
```
my-project/
├── setup.yml              # Main entry
├── environments/
│   ├── dev.yml
│   ├── staging.yml
│   └── prod.yml
├── platforms/
│   ├── linux.yml
│   └── macos.yml
├── components/
│   ├── postgres.yml
│   ├── nginx.yml
│   └── app.yml
└── templates/
    ├── nginx.conf.j2
    └── app-config.yml.j2
```

### Main File
```yaml
# setup.yml
- vars:
    environment: "{{ lookup('env', 'ENVIRONMENT') or 'dev' }}"

- include_vars: ./environments/{{ environment }}.yml

- include: ./platforms/{{ os }}.yml

- include: ./components/postgres.yml
- include: ./components/nginx.yml
- include: ./components/app.yml
```

## Switching Environments

**Method 1: Modify main.yml**
```yaml
- vars:
    env: production  # Change this
```

**Method 2: Use environment variable**
```bash
ENVIRONMENT=production mooncake run --config main.yml
```

**Method 3: Different main files**
```bash
mooncake run --config prod-setup.yml
```

## Benefits of Multi-File Organization

1. **Maintainability** - Easier to find and update specific parts
2. **Reusability** - Share tasks across projects
3. **Collaboration** - Team members can work on different files
4. **Testing** - Test components independently
5. **Clarity** - Clear separation of concerns

## Testing

```bash
# Run full configuration
mooncake run --config main.yml

# Preview what will run
mooncake run --config main.yml --dry-run

# Run with debug logging to see includes
mooncake run --config main.yml --log-level debug

# Run specific tagged sections
mooncake run --config main.yml --tags install
```

## Best Practices

1. **Clear naming** - Use descriptive file names
2. **Logical grouping** - Group related tasks together
3. **Document includes** - Comment what each include does
4. **Avoid deep nesting** - Keep include hierarchy shallow (2-3 levels max)
5. **Use variables** - Make includes reusable with variables

## Next Steps

→ Explore [real-world](../real-world/) examples to see complete practical applications!

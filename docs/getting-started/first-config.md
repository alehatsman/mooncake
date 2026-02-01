# Your First Configuration

Let's create a complete configuration that demonstrates key Mooncake features.

## Create Project

```bash
mkdir my-first-mooncake
cd my-first-mooncake
```

## Write Configuration

Create `config.yml`:

```yaml
# Define variables
- vars:
    project_name: MyApp
    version: "1.0.0"

# Create directory structure
- name: Create project directory
  file:
    path: "/tmp/{{project_name}}"
    state: directory
    mode: "0755"

- name: Create config file
  file:
    path: "/tmp/{{project_name}}/config.txt"
    state: file
    content: |
      Project: {{project_name}}
      Version: {{version}}
      OS: {{os}}
      Architecture: {{arch}}
    mode: "0644"

# OS-specific steps
- name: Install on macOS
  shell: echo "Would install with brew"
  when: os == "darwin"

- name: Install on Linux
  shell: echo "Would install with apt/yum"
  when: os == "linux"
```

## Preview with Dry-Run

```bash
mooncake run --config config.yml --dry-run
```

## Run It

```bash
mooncake run --config config.yml
```

## Check Results

```bash
cat /tmp/MyApp/config.txt
```

## What You Learned

- Defining variables
- Creating files and directories
- Using system facts (os, arch)
- Conditional execution

## Next Steps

Explore the [Examples](../examples/index.md) for more advanced features.

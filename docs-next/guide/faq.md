# Frequently Asked Questions

Common questions about Mooncake and their answers.

---

## General Questions

### What is Mooncake?

Mooncake is a configuration management tool designed specifically for AI agents and modern development workflows. It provides a safe, validated execution environment for system configuration with idempotency guarantees, dry-run validation, and full observability.

Think of it as "the standard runtime for AI system configuration" - similar to how Docker provides a standard runtime for containers.

### Why "Mooncake"?

The name comes from the show "Final Space" where mooncakes are a beloved treat. Also, configuration management should be as delightful as eating mooncakes! **Chookity!**

### Is Mooncake production-ready?

Yes! Mooncake is actively used for:

- Personal dotfiles management
- Development environment setup
- System provisioning
- AI agent configuration tasks

It has comprehensive test coverage, runs on multiple platforms, and follows semantic versioning.

---

## Comparison Questions

### How is Mooncake different from Ansible?

| Feature | Mooncake | Ansible |
|---------|----------|---------|
| **Target Audience** | AI agents, developers, dotfiles | Enterprise infrastructure |
| **Installation** | Single binary, zero dependencies | Python + pip packages + galaxy collections |
| **Setup** | None required | Inventory files, host management |
| **Complexity** | Simple YAML | Complex playbooks, roles, collections |
| **AI-Friendly** | Designed for AI generation | Complex for AI to generate correctly |
| **Dry-run** | Built-in, always available | Check mode (limited) |
| **Learning Curve** | Minutes | Hours to days |

**When to use Mooncake**:

- AI agent configuration tasks
- Personal dotfiles and dev environments
- Simple automation scripts
- Cross-platform configurations
- When you want zero dependencies

**When to use Ansible**:

- Large-scale enterprise infrastructure
- Complex role-based architectures
- Existing Ansible investments
- Multi-host orchestration

### How is Mooncake different from shell scripts?

| Feature | Mooncake | Shell Scripts |
|---------|----------|---------------|
| **Idempotency** | Guaranteed | Manual |
| **Dry-run** | Native | Manual implementation |
| **Error Handling** | Built-in | Manual |
| **Cross-platform** | Unified syntax | OS-specific scripts |
| **Validation** | Schema validation | None |
| **Variables** | Built-in with facts | Manual |

**Mooncake provides**:

- Automatic idempotency
- Built-in dry-run mode
- Schema validation
- Cross-platform abstractions
- Structured error handling
- System fact detection

### Can I migrate from Ansible to Mooncake?

Yes! Mooncake uses similar YAML syntax. See the [Migration Guide](guide/migration.md) for details.

Common migrations:

**Ansible playbook**:
```yaml
- hosts: localhost
  tasks:
    - name: Install package
      apt:
        name: neovim
        state: present
      become: yes
```

**Mooncake equivalent**:
```yaml
- name: Install package
  shell: apt install -y neovim
  become: true
  when: package_manager == "apt"
```

---

## AI & LLM Questions

### Can AI agents use Mooncake safely?

Yes! Mooncake was designed specifically for AI agents:

1. **Safe by Default**: Dry-run mode lets AI preview changes before applying
2. **Validated Operations**: Schema validation prevents malformed configurations
3. **Idempotency**: Same config can be run multiple times safely
4. **Full Observability**: Structured events enable AI to understand execution
5. **Simple Format**: YAML is easy for AI models to generate and parse

### How do AI models generate Mooncake configs?

1. **Use the AI Specification**: See [AI Specification](ai-specification.md) for a complete guide LLMs can follow

2. **Provide system context**:
```bash
# Give AI the system facts
mooncake facts --format json > facts.json
```

3. **Let AI generate config**:
```yaml
# AI generates based on request and facts
- name: Install development tools
  shell: {{package_manager}} install {{item}}
  with_items: [git, vim, curl]
  become: true
  when: os == "linux"
```

4. **Validate before executing**:
```bash
# AI can validate without risk
mooncake run --config config.yml --dry-run
```

### What's the AI agent workflow?

```
1. User Request → AI Agent
2. AI generates Mooncake config
3. AI runs dry-run to validate
4. AI shows preview to user
5. User approves
6. AI executes configuration
7. AI observes results via events
```

---

## Security Questions

### Is it safe to give AI agents sudo access?

Mooncake provides several safety layers:

1. **Dry-run First**: Always preview with `--dry-run`
2. **Explicit sudo**: Only steps with `become: true` get sudo
3. **Password Control**: You control sudo password access
4. **Validation**: Schema validation prevents malformed commands
5. **Audit Trail**: Full logging of all operations

**Best practices**:

- Always review dry-run output before executing
- Use tags to limit execution scope
- Run sensitive operations manually
- Monitor execution logs

### How do I handle secrets?

**Option 1: Environment Variables**
```yaml
- name: Use secret
  shell: echo "API_KEY=$API_KEY"
  environment:
    API_KEY: "{{ lookup('env', 'API_KEY') }}"
```

**Option 2: Password Files**
```yaml
# Load from secure file
- include_vars:
    file: ~/.mooncake/secrets.yml

- name: Use secret
  shell: echo "{{api_key}}"
```

**Option 3: External Secret Management**
```yaml
# Fetch from vault/keychain
- name: Get secret
  shell: security find-generic-password -s myapp -w
  register: secret
  no_log: true

- name: Use secret
  shell: curl -H "Authorization: {{secret.stdout}}"
```

**Never**:

- Commit secrets to version control
- Print secrets in logs (`no_log: true`)
- Use plain text passwords in configs

### Can I restrict what AI agents can do?

Yes, several ways:

1. **Tags**: Limit execution to specific operations
```yaml
# AI can only run setup tasks
- name: Setup step
  shell: setup.sh
  tags: [setup]
```
```bash
mooncake run --config config.yml --tags setup
```

2. **Conditional Execution**: Restrict by facts
```yaml
# Only allow dev operations
- name: Dev task
  shell: install-dev-tools.sh
  when: environment == "dev"
```

3. **File Permissions**: Control config access with file permissions

4. **Sudo Control**: Control sudo password access

---

## Technical Questions

### What languages/tools does Mooncake support?

**Built-in language version managers**:

- Python (pyenv)
- Node.js (nvm)
- Ruby (rbenv)
- Go (direct install)
- Rust (rustup)
- Java (OpenJDK)

**Package managers detected automatically**:

- apt, dnf, yum, zypper, pacman, apk (Linux)
- brew, port (macOS)
- choco, scoop (Windows)

**See all presets**:
```bash
mooncake presets list
```

### Does Mooncake work on Windows?

Yes! Mooncake supports Windows with some limitations:

**Fully supported**:

- Shell commands (PowerShell/cmd)
- File operations
- Variable expansion
- Templates
- Downloads

**Limited support**:

- Service management (basic Windows services)
- Package management (via choco)

**Use conditionals for cross-platform configs**:
```yaml
- name: Unix command
  shell: ls -la
  when: os != "windows"

- name: Windows command
  shell: dir
  when: os == "windows"
```

### Can I use Mooncake in CI/CD?

Yes! Mooncake works great in CI/CD:

```bash
# Disable interactive TUI
mooncake run --config config.yml --raw

# JSON output for parsing
mooncake run --config config.yml --raw --output-format json

# Exit codes
# 0 = success
# 1+ = failure
```

**Example GitHub Actions**:
```yaml
- name: Install Mooncake
  run: go install github.com/alehatsman/mooncake@latest

- name: Run configuration
  run: mooncake run --config config.yml --raw

- name: Verify
  run: mooncake facts
```

### Does Mooncake support remote hosts?

Not yet. Mooncake currently executes on localhost only. Remote execution is planned for a future release.

For now, you can:

1. Copy config to remote host and run locally
2. Use SSH wrapper scripts
3. Wait for remote execution support (coming soon!)

### Can I create my own presets?

Yes! Presets are just YAML files:

**Create preset structure**:
```bash
mkdir -p ~/.mooncake/presets/mypreset
cat > ~/.mooncake/presets/mypreset/preset.yml <<EOF
name: mypreset
version: "1.0.0"
description: My custom preset

parameters:
  - name: state
    type: string
    default: present
    enum: [present, absent]

steps:
  - name: Install
    shell: echo "Installing with state={{parameters.state}}"
    when: parameters.state == "present"
EOF
```

**Use it**:
```yaml
- preset: mypreset
  with:
    state: present
```

See [Preset Authoring Guide](guide/preset-authoring.md) for details.

---

## Usage Questions

### Can I use Mooncake for dotfiles?

Yes! Mooncake is excellent for dotfiles:

```yaml
- name: Create config directories
  file:
    path: "{{item}}"
    state: directory
  with_items:
    - ~/.config/nvim
    - ~/.config/tmux
    - ~/.config/zsh

- name: Deploy dotfiles
  copy:
    src: "{{item.src}}"
    dest: "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**Benefits**:

- Idempotent (run multiple times safely)
- Cross-platform (same config for macOS/Linux)
- Dry-run before applying
- Version control friendly

### Can I use loops?

Yes! Two types:

**List loops**:
```yaml
- name: Install packages
  shell: brew install {{item}}
  with_items:
    - neovim
    - ripgrep
    - fzf
```

**File tree loops**:
```yaml
- name: Deploy configs
  copy:
    src: "{{item.src}}"
    dest: "~/{{item.name}}"
  with_filetree: ./configs
  when: item.is_dir == false
```

### Can I split configs across multiple files?

Yes! Use `include_vars` and organize by topic:

**Main config**:
```yaml
- include_vars:
    file: ./vars/dev-tools.yml

- include_vars:
    file: ./vars/languages.yml

- name: Install dev tools
  shell: brew install {{item}}
  with_items: "{{dev_tools}}"
```

**vars/dev-tools.yml**:
```yaml
dev_tools:
  - neovim
  - ripgrep
  - fzf
```

### How do I handle different environments (dev/staging/prod)?

**Option 1: Environment-specific configs**
```bash
mooncake run --config config-dev.yml
mooncake run --config config-prod.yml
```

**Option 2: Variables**
```yaml
- vars:
    environment: dev  # Change per environment

- name: Dev-only task
  shell: install-dev-tools.sh
  when: environment == "dev"
```

**Option 3: Tags**
```yaml
- name: Dev setup
  shell: setup-dev.sh
  tags: [dev]

- name: Prod deployment
  shell: deploy-prod.sh
  tags: [prod]
```
```bash
mooncake run --config config.yml --tags dev
```

---

## Troubleshooting Questions

### Why is my variable undefined?

Common causes:

1. **Variable not defined**:
```yaml
# Wrong
- shell: echo "{{my_var}}"

# Right
- vars:
    my_var: value
- shell: echo "{{my_var}}"
```

2. **Using system fact incorrectly**:
```yaml
# Wrong - no such fact
- shell: echo "{{operating_system}}"

# Right - use 'os'
- shell: echo "{{os}}"
```

3. **Variable scope issue** - variables are scoped to the config file

Check available facts:
```bash
mooncake facts
```

### Why does my step keep running (not idempotent)?

Some operations aren't idempotent by default:

**Problem**:
```yaml
# Runs every time
- shell: echo "test" >> /tmp/file
```

**Solutions**:

1. Use idempotent actions:
```yaml
- file:
    path: /tmp/file
    state: file
    content: "test"  # Idempotent
```

2. Use `creates` condition:
```yaml
- shell: echo "test" > /tmp/file
  args:
    creates: /tmp/file  # Only if doesn't exist
```

3. Use `changed_when`:
```yaml
- shell: echo "test" >> /tmp/file
  register: result
  changed_when: false  # Never report as changed
```

### How do I debug template errors?

1. **Check variable values**:
```yaml
- name: Debug variable
  shell: echo "Value is {{my_var}}"
```

2. **Test template separately**:
```bash
# Create test template
echo "{{ my_var }}" > test.j2

# Test with mooncake
mooncake run --config test-template.yml
```

3. **Use simpler templates first**:
```yaml
# Start simple
- template:
    dest: /tmp/test
    content: "{{ simple_var }}"

# Then add complexity
- template:
    src: complex.j2
    dest: /tmp/test
```

---

## Performance Questions

### Is Mooncake fast?

Yes! Mooncake is written in Go and has minimal overhead:

- Binary size: ~20MB
- Startup time: <100ms
- Memory usage: <50MB typically
- No interpreter overhead (unlike Python-based tools)

### Can I run steps in parallel?

Not yet. Steps currently run sequentially for safety and predictability. Parallel execution is planned for a future release.

### How do I make my configs faster?

1. **Remove unnecessary operations**:
```yaml
# Slow - updates package cache every time
- shell: apt update && apt install {{item}}
  with_items: [vim, git, curl]

# Fast - update once
- shell: apt update
- shell: apt install -y {{item}}
  with_items: [vim, git, curl]
```

2. **Use tags to run only what's needed**:
```bash
mooncake run --config config.yml --tags quick
```

3. **Use `creates`/`removes` for idempotency**:
```yaml
- shell: tar xzf large-file.tar.gz
  args:
    creates: /opt/app/bin/app  # Skip if already extracted
```

---

## Contributing Questions

### How can I contribute?

Contributions are welcome!

1. **Report bugs**: [GitHub Issues](https://github.com/alehatsman/mooncake/issues)
2. **Request features**: [GitHub Issues](https://github.com/alehatsman/mooncake/issues)
3. **Submit PRs**: See [Contributing Guide](development/contributing.md)
4. **Share presets**: Submit to the presets repository
5. **Improve docs**: Documentation PRs always welcome!

### What's the roadmap?

See the [Roadmap](development/roadmap.md) for planned features:

- Remote host execution
- Parallel step execution
- Enhanced service management
- More built-in actions
- Plugin system

---

## See Also

- [Quick Reference](quick-reference.md) - One-page cheat sheet
- [Troubleshooting](guide/troubleshooting.md) - Common issues and solutions
- [Full Documentation](https://mooncake.alehatsman.com) - Complete guide
- [GitHub Issues](https://github.com/alehatsman/mooncake/issues) - Ask questions

# Examples

Learn Mooncake through practical, runnable examples. Follow the learning path below or jump to topics that interest you.

!!! tip "Running Examples"
    All examples are runnable YAML configurations. Try them with:
    ```bash
    mooncake run examples/hello-world/simple.yml
    mooncake run --dry-run examples/conditionals/basic.yml  # Safe preview
    ```

---

## 🎓 Learning Path

Follow these examples in order to build your Mooncake skills from beginner to advanced.

### Level 1: Getting Started (5 minutes)

#### [01 - Hello World](01-hello-world.md) ⭐
**Your first Mooncake configuration**

Learn the basics: running shell commands and printing output.

```yaml
- name: Hello from Mooncake
  shell: echo "Running on {{os}}/{{arch}}"

- name: Show system info
  print: "CPU cores: {{cpu_cores}}, Memory: {{memory_total_mb}}MB"
```

**You'll learn:**

- Basic step syntax (name + action)
- Using system facts (os, arch, cpu_cores)
- Shell and print actions

**Time:** 2 minutes

---

### Level 2: Variables & Facts (10 minutes)

#### [02 - Variables and Facts](02-variables-and-facts.md) ⭐
**Define custom variables and use auto-detected system information**

```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"

- name: Install {{app_name}}
  shell: echo "Installing {{app_name}} v{{version}} on {{os}}"

- name: Show detected facts
  print: |
    OS: {{os}}
    Distribution: {{distribution}}
    Package Manager: {{package_manager}}
    Python: {{python_version}}
```

**You'll learn:**

- Defining variables with `vars`
- Templating with `{{ variable }}`
- Auto-detected facts (os, distribution, package_manager)
- Multi-line output with `print`

**Time:** 5 minutes

---

### Level 3: File Operations (15 minutes)

#### [03 - Files and Directories](03-files-and-directories.md) ⭐⭐
**Create, manage, and modify files and directories**

```yaml
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory
    mode: "0755"

- name: Create config file
  file:
    path: ~/.config/myapp/settings.yml
    state: file
    content: |
      app_name: myapp
      version: 1.0
    mode: "0644"

- name: Create symlink
  file:
    path: ~/bin/myapp
    state: link
    target: /usr/local/bin/myapp
```

**You'll learn:**

- Creating directories with permissions
- Writing file content inline
- Creating symlinks
- File states: file, directory, link, absent

**Time:** 8 minutes

---

### Level 4: Control Flow (20 minutes)

#### [04 - Conditionals](04-conditionals.md) ⭐⭐
**Execute steps based on conditions**

```yaml
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Ubuntu
  shell: apt install -y neovim
  become: true
  when: os == "linux" && package_manager == "apt"

- name: Check if file exists
  shell: test -f ~/.config/app.conf
  register: config_exists
  ignore_errors: true

- name: Create config if missing
  file:
    path: ~/.config/app.conf
    state: file
  when: config_exists.rc != 0
```

**You'll learn:**

- `when` conditionals (==, !=, &&, ||)
- Platform-specific execution
- `register` to capture results
- `ignore_errors` for optional checks

**Time:** 10 minutes

#### [06 - Loops](06-loops.md) ⭐⭐
**Iterate over lists to avoid repetition**

```yaml
- vars:
    packages:
      - neovim
      - ripgrep
      - fzf
      - tmux

- name: Install package
  shell: brew install {{item}}
  with_items: "{{packages}}"

- name: Deploy dotfile
  copy:
    src: "{{item.src}}"
    dest: "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**You'll learn:**

- `with_items` loops over lists
- `with_filetree` loops over directory contents
- Accessing loop item properties

**Time:** 10 minutes

---

### Level 5: Advanced Techniques (30 minutes)

#### [05 - Templates](05-templates.md) ⭐⭐
**Render configuration files from templates**

```yaml
- name: Render nginx config
  template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 8080
      workers: 4
      ssl_enabled: true
```

**Template (nginx.conf.j2):**
```jinja
worker_processes {{ workers }};

server {
    listen {{ port }};

    {% if ssl_enabled %}
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    {% endif %}
}
```

**You'll learn:**

- Jinja2 template syntax
- Variables in templates `{{ var }}`
- Conditionals in templates `{% if %}`
- Loops in templates `{% for %}`

**Time:** 12 minutes

#### [07 - Register](07-register.md) ⭐⭐
**Capture and use step results**

```yaml
- name: Check service status
  shell: systemctl is-active nginx
  register: nginx_status
  ignore_errors: true

- name: Start if not running
  service:
    name: nginx
    state: started
  when: nginx_status.rc != 0
  become: true

- name: Show result
  print: "Service was {{nginx_status.stdout}}"
```

**You'll learn:**

- Capturing command output with `register`
- Accessing result properties (rc, stdout, stderr)
- Conditional execution based on results
- Building dependent workflows

**Time:** 8 minutes

#### [08 - Tags](08-tags.md) ⭐⭐
**Selective execution with tags**

```yaml
- name: Install dependencies
  shell: npm install
  tags: [dev, setup]

- name: Build production
  shell: npm run build
  tags: [prod, build]

- name: Run tests
  shell: npm test
  tags: [test, ci]
```

**Run specific workflows:**
```bash
mooncake run config.yml --tags dev     # Only dev-tagged steps
mooncake run config.yml --tags prod,ci # prod OR ci steps
```

**You'll learn:**

- Tagging steps for organization
- Filtering execution by tags
- Building workflow stages (dev, test, prod)

**Time:** 6 minutes

---

### Level 6: Production Patterns (45 minutes)

#### [09 - Sudo](09-sudo.md) ⭐⭐⭐
**Execute privileged operations securely**

```yaml
- name: Install system package
  shell: apt update && apt install -y docker.io
  become: true

- name: Add user to docker group
  shell: usermod -aG docker {{ansible_user}}
  become: true
```

**Run with sudo password:**
```bash
mooncake run config.yml --ask-become-pass
mooncake run config.yml -K  # shorthand
```

**You'll learn:**

- `become: true` for sudo operations
- Password handling (interactive, file, env var)
- Security best practices

**Time:** 10 minutes

#### [10 - Multi-File Configs](10-multi-file-configs.md) ⭐⭐⭐
**Organize large configurations**

```yaml
# main.yml
- include: ./vars/defaults.yml
- include: ./tasks/setup.yml
- include: ./tasks/install.yml
- include: ./tasks/configure.yml

# vars/defaults.yml
- vars:
    app_name: webapp
    version: "2.0.0"

# tasks/setup.yml
- name: Create directories
  file:
    path: "{{item}}"
    state: directory
  with_items:
    - /opt/{{app_name}}
    - /var/log/{{app_name}}
```

**You'll learn:**

- `include` for splitting configs
- Variable sharing across files
- Organizing by function (vars, tasks, handlers)
- Building modular configurations

**Time:** 15 minutes

#### [11 - Execution Control](11-execution-control.md) ⭐⭐⭐
**Advanced error handling and control**

```yaml
- name: Download with retry
  shell: curl -O https://example.com/file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
  register: download

- name: Verify download
  shell: sha256sum file.tar.gz
  register: checksum
  failed_when: "'abc123' not in checksum.stdout"

- name: Conditional changed detection
  shell: make install
  register: result
  changed_when: "'installed' in result.stdout"
```

**You'll learn:**

- `timeout` for long operations
- `retries` and `retry_delay` for resilience
- `failed_when` for custom failure detection
- `changed_when` for accurate change tracking
- Building robust production workflows

**Time:** 20 minutes

#### [12 - Unarchive](12-unarchive.md) ⭐⭐
**Extract and manage archives**

```yaml
- name: Download tarball
  download:
    url: https://example.com/app.tar.gz
    dest: /tmp/app.tar.gz
    checksum: sha256:abc123...

- name: Extract to destination
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/app
    strip_components: 1
    creates: /opt/app/bin/app

- name: Cleanup
  file:
    path: /tmp/app.tar.gz
    state: absent
```

**You'll learn:**

- Downloading with checksum verification
- Extracting tar, tar.gz, zip archives
- `strip_components` for path manipulation
- `creates` for idempotency
- Complete download-extract-install pattern

**Time:** 12 minutes

---

## 🎯 Real-World Projects

### [Dotfiles Management](real-world-dotfiles.md) ⭐⭐⭐
**Complete dotfiles deployment and configuration**

A production-ready example showing:

- Cross-platform dotfiles (macOS/Linux)
- Git repository management
- Symlink creation with backup
- Shell configuration (zsh, bash)
- Tool installation (vim, tmux, git)
- Platform detection and adaptation

**Code preview:**
```yaml
- name: Clone dotfiles repo
  shell: git clone https://github.com/user/dotfiles.git ~/.dotfiles
  creates: ~/.dotfiles

- name: Backup existing dotfiles
  shell: cp "{{item}}" "{{item}}.backup-{{timestamp}}"
  with_filetree: ~/.dotfiles
  when: item.is_file and lookup('file', '~/' + item.name) is defined

- name: Create symlinks
  file:
    src: ~/.dotfiles/{{item.name}}
    dest: ~/{{item.name}}
    state: link
  with_filetree: ~/.dotfiles
  when: item.is_file
```

**What you'll build:**

- Automated dotfiles deployment
- Backup strategy before changes
- Conditional installation based on OS
- Tool-specific configurations

**Time:** 30 minutes

### [Idempotency Demonstration](idempotency.md) ⭐⭐
**Understanding Mooncake's idempotency guarantees**

Learn how Mooncake ensures operations are safe to run multiple times:

```yaml
# Run this multiple times - only changes on first run
- name: Create directory
  file:
    path: /opt/myapp
    state: directory
  # ✅ First run: changed=true (created)
  # ✅ Second run: changed=false (already exists)

- name: Download file
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
    checksum: sha256:abc123...
  # ✅ First run: changed=true (downloaded)
  # ✅ Second run: changed=false (checksum matches)

- name: Install package
  shell: apt install -y neovim
  creates: /usr/bin/nvim
  # ✅ First run: changed=true (installed)
  # ✅ Second run: skipped (binary exists)
```

**You'll learn:**

- How Mooncake detects changes
- Using `creates` for idempotency
- Checksum-based file operations
- State-based file management
- Why idempotency matters

**Time:** 15 minutes

---

## 📂 Browse By Action Type

<div class="grid cards" markdown>

-   **:material-console:{ .lg } Shell Commands**

    Examples using `shell` and `command` actions

    [01 - Hello World](01-hello-world.md) •
    [04 - Conditionals](04-conditionals.md) •
    [09 - Sudo](09-sudo.md)

-   **:material-file:{ .lg } File Operations**

    Examples using `file`, `copy`, `template` actions

    [03 - Files & Dirs](03-files-and-directories.md) •
    [05 - Templates](05-templates.md) •
    [Dotfiles](real-world-dotfiles.md)

-   **:material-cog:{ .lg } Variables & Logic**

    Examples using variables, conditionals, loops

    [02 - Variables](02-variables-and-facts.md) •
    [04 - Conditionals](04-conditionals.md) •
    [06 - Loops](06-loops.md)

-   **:material-package:{ .lg } Advanced Patterns**

    Error handling, multi-file, execution control

    [10 - Multi-File](10-multi-file-configs.md) •
    [11 - Execution Control](11-execution-control.md) •
    [12 - Unarchive](12-unarchive.md)

</div>

---

## 💡 Quick Tips

!!! success "Start with Hello World"
    New to Mooncake? Begin with [01 - Hello World](01-hello-world.md) and follow the learning path in order.

!!! info "Dry-Run Everything"
    Always test with `--dry-run` first to see what will happen:
    ```bash
    mooncake run --dry-run examples/sudo/install-docker.yml
    ```

!!! tip "Mix and Match"
    All techniques can be combined. For example:
    ```yaml
    - name: Install on Linux
      shell: apt install -y {{item}}
      become: true
      with_items: "{{packages}}"
      when: os == "linux" && package_manager == "apt"
      tags: [setup, packages]
    ```
    This combines: loops, conditionals, sudo, tags, and variables!

!!! warning "Test in Safe Environment"
    Some examples modify system state. Use a VM or container for testing, especially examples 09-12.

---

## 🔗 See Also

- **[Actions Reference](../guide/config/actions.md)** - Complete action documentation
- **[Variables Guide](../guide/config/variables.md)** - Working with variables and facts
- **[Control Flow](../guide/config/control-flow.md)** - Conditionals, loops, and tags
- **[Quick Reference](../guide/quick-reference.md)** - One-page cheat sheet
- **[Best Practices](../guide/best-practices.md)** - Production patterns and tips

**Ready to start?** → [Begin with Hello World](01-hello-world.md) ⭐

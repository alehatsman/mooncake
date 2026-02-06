# Variables

Variables make configurations reusable and dynamic. Use system facts and custom variables throughout your configuration.

## Defining Variables

### Inline Variables

```yaml
- vars:
    app_name: myapp
    version: "1.0.0"
    port: 8080
```

### External Variable Files

```yaml
- name: Load variables
  include_vars: ./vars/common.yml
```

**vars/common.yml:**
```yaml
app_name: myapp
environment: development
debug: true
```

### Dynamic Variable Loading

```yaml
- vars:
    env: production

- include_vars: ./vars/{{env}}.yml
```

## Using Variables

### In Shell Commands

```yaml
- vars:
    package: neovim

- shell: brew install {{package}}
```

### In File Paths

```yaml
- vars:
    app_dir: /opt/myapp

- file:
    path: "{{app_dir}}/config"
    state: directory
```

### In File Content

```yaml
- vars:
    api_key: secret123

- file:
    path: /tmp/config.txt
    state: file
    content: |
      api_key: {{api_key}}
      environment: production
```

### In Templates

**config.yml:**
```yaml
- template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
```

**nginx.conf.j2:**
```nginx
server {
    listen {{port}};
    server_name {{hostname}};
}
```

## System Facts

Mooncake automatically provides system information as variables.

### Basic Facts

```yaml
# Operating system
os: "linux"                    # linux, darwin, windows
arch: "amd64"                  # amd64, arm64, 386, etc.
hostname: "myserver"
username: "admin"
user_home: "/home/admin"
kernel_version: "6.5.0-14"     # Kernel/Darwin version
```

### Distribution Info

```yaml
distribution: "ubuntu"           # ubuntu, debian, centos, macos, etc.
distribution_version: "22.04"    # Full version
distribution_major: "22"         # Major version only
```

### CPU Facts

```yaml
cpu_cores: 8                                        # Number of CPU cores
cpu_model: "Intel(R) Core(TM) i7-10700K"           # CPU model name
cpu_flags: ["avx", "avx2", "sse4_2", "fma", "aes"] # CPU feature flags
cpu_flags_string: "avx avx2 sse4_2 fma aes"        # Flags as string
```

### Memory Facts

```yaml
memory_total_mb: 16384      # Total RAM in megabytes
memory_free_mb: 8192        # Available RAM in megabytes
swap_total_mb: 4096         # Total swap space
swap_free_mb: 2048          # Available swap space
```

### Network Facts

```yaml
# IP addresses
ip_addresses: ["192.168.1.100", "10.0.0.5"]
ip_addresses_string: "192.168.1.100, 10.0.0.5"

# Network configuration
default_gateway: "192.168.1.1"
dns_servers: ["8.8.8.8", "1.1.1.1"]
dns_servers_string: "8.8.8.8, 1.1.1.1"

# Network interfaces (array - can iterate)
network_interfaces:
  - name: "eth0"
    mac_address: "00:11:22:33:44:55"
    mtu: 1500
    addresses: ["192.168.1.100/24"]
    up: true
```

### GPU Facts

```yaml
# GPUs array - can iterate with {% for gpu in gpus %}
gpus:
  - vendor: "nvidia"
    model: "GeForce RTX 4090"
    memory: "24GB"
    driver: "535.54.03"
    cuda_version: "12.3"     # NVIDIA only
```

### Storage Facts

```yaml
# Disks array - can iterate with {% for disk in disks %}
disks:
  - device: "/dev/sda1"
    mount_point: "/"
    filesystem: "ext4"
    size_gb: 500
    used_gb: 250
    avail_gb: 250
    used_pct: 50
```

### Software Detection

```yaml
# Package managers and languages
package_manager: "apt"      # apt, yum, brew, pacman, etc.
python_version: "3.11.5"    # Installed Python version

# Development tools
docker_version: "24.0.7"    # Docker version (if installed)
git_version: "2.43.0"       # Git version (if installed)
go_version: "1.21.5"        # Go version (if installed)
```

## Viewing System Facts

Run `mooncake facts` to see all available facts:

```bash
mooncake facts
```

Output shows:

- Operating system details (OS, distribution, kernel version)
- CPU (cores, model, flags)
- Memory (total, free, swap)
- GPUs (vendor, model, memory, driver, CUDA version)
- Storage devices (disks with mount points and sizes)
- Network (interfaces, gateway, DNS)
- Software (package manager, Python, Docker, Git, Go)

## Using System Facts

### OS Detection

```yaml
- shell: apt update
  when: os == "linux"

- shell: brew update
  when: os == "darwin"
```

### Distribution-Specific Commands

```yaml
- shell: apt install package
  when: distribution == "ubuntu" || distribution == "debian"

- shell: yum install package
  when: distribution == "centos" || distribution == "fedora"
```

### Architecture Detection

```yaml
- shell: install-amd64-binary
  when: arch == "amd64"

- shell: install-arm64-binary
  when: arch == "arm64"
```

### Memory-Based Decisions

```yaml
- name: Configure for high-memory system
  shell: set-large-buffers
  when: memory_total_mb >= 32000

- name: Check available memory
  shell: echo "Free memory: {{memory_free_mb}}MB"
```

### Package Manager Detection

```yaml
- shell: "{{package_manager}} install neovim"
  when: os == "linux"
```

### Iterating Over Arrays

```yaml
# Iterate over disks
- name: Show disk info
  shell: |
    {% for disk in disks %}
    echo "Disk: {{ disk.Device }} mounted at {{ disk.MountPoint }} ({{ disk.SizeGB }}GB)"
    {% endfor %}

# Iterate over GPUs
- name: Setup GPU
  shell: nvidia-smi -i {{loop.index0}}
  with_items: "{{gpus}}"
  when: gpus|length > 0

# Iterate over network interfaces
- name: Configure interface
  shell: |
    {% for iface in network_interfaces %}
    {% if iface.Up %}
    echo "Active: {{ iface.Name }} ({{ iface.MACAddress }})"
    {% endif %}
    {% endfor %}
```

### Toolchain Detection

```yaml
# Check if Docker is installed
- name: Run Docker container
  shell: docker run hello-world
  when: docker_version != ""

# Use Git if available
- name: Clone repository
  shell: git clone https://github.com/user/repo.git
  when: git_version != ""

# Show installed versions
- shell: |
    echo "Docker: {{docker_version}}"
    echo "Git: {{git_version}}"
    echo "Go: {{go_version}}"
```

## Variable Precedence

When the same variable is defined in multiple places:

1. **Template vars** (highest priority)
   ```yaml
   - template:
       vars:
         port: 9000
   ```

2. **Step-level vars**
   ```yaml
   - vars:
       port: 8080
   ```

3. **Included vars**
   ```yaml
   - include_vars: ./vars.yml
   ```

4. **System facts** (lowest priority)
   ```yaml
   # Automatically available
   os: "linux"
   ```

## Variable Scoping

Variables are available to all subsequent steps:

```yaml
# Step 1: Define
- vars:
    app_name: myapp

# Step 2: Use in same file
- shell: echo "{{app_name}}"

# Step 3: Use in included files
- include: ./tasks/setup.yml  # Can use app_name
```

## Register Variables

Capture command output as variables:

```yaml
- shell: whoami
  register: current_user

- shell: echo "User is {{current_user.stdout}}"

- file:
    path: "/home/{{current_user.stdout}}/config"
    state: file
```

## Loop Variables

Special `item` variable in loops:

```yaml
- vars:
    users: [alice, bob]

- name: Create directory for {{item}}
  file:
    path: "/home/{{item}}"
    state: directory
  with_items: "{{users}}"
```

## Best Practices

1. **Use descriptive names**
   ```yaml
   # Good
   database_host: "localhost"

   # Bad
   h: "localhost"
   ```

2. **Quote version strings**
   ```yaml
   # Good
   version: "1.0.0"

   # Bad (may be parsed as number)
   version: 1.0.0
   ```

3. **Group related variables**
   ```yaml
   - vars:
       # Database config
       db_host: localhost
       db_port: 5432
       db_name: myapp

       # App config
       app_port: 8080
       app_debug: false
   ```

4. **Use external files for environments**
   ```
   vars/
     development.yml
     staging.yml
     production.yml
   ```

5. **Use system facts when possible**
   ```yaml
   # Good - adapts to system
   - shell: "{{package_manager}} install curl"

   # Bad - hardcoded for one OS
   - shell: apt install curl
   ```

## See Also

- [Actions](actions.md) - Using variables in actions
- [Control Flow](control-flow.md) - Using variables in conditions
- [Examples](../../examples/index.md#02-variables-and-system-facts) - Variable examples
- [Commands](../commands.md#mooncake-facts) - View system facts

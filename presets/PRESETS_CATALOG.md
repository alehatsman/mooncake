# Mooncake Presets Catalog

Complete list of available presets with configuration templates.

## Web Servers

### nginx
Modern, high-performance web server and reverse proxy.
- ✅ Configuration templates: `nginx.conf`, `site.conf`, `reverse-proxy.conf`
- 📦 Supports: Linux, macOS
- 🔧 Features: Static files, reverse proxy, load balancing

### caddy
Modern web server with automatic HTTPS.
- ✅ Configuration templates: `Caddyfile`, `reverse-proxy`
- 📦 Supports: Linux, macOS
- 🔧 Features: Auto HTTPS, HTTP/3, simple config

## Databases

### mysql
Popular open-source relational database.
- ✅ Configuration templates: `my.cnf`
- 📦 Supports: Linux, macOS
- 🔧 Features: ACID compliance, replication, InnoDB

### postgres
Advanced open-source relational database.
- 📦 Supports: Linux, macOS
- 🔧 Features: ACID, JSONB, full-text search, extensions

### mongodb
NoSQL document database.
- 📦 Supports: Linux, macOS
- 🔧 Features: JSON documents, sharding, replication

### redis
In-memory data structure store.
- ✅ Configuration templates: `redis.conf`
- 📦 Supports: Linux, macOS
- 🔧 Features: Caching, pub/sub, persistence, clustering

## Programming Languages

### nodejs
JavaScript runtime via nvm (Node Version Manager).
- 📦 Supports: Linux, macOS
- 🔧 Features: Multiple versions, global packages

### python
Python via pyenv (Python version manager).
- 📦 Supports: Linux, macOS
- 🔧 Features: Multiple versions, virtual environments

### go
Go programming language.
- 📦 Supports: Linux, macOS
- 🔧 Features: GOPATH setup, module support

### rust
Systems programming language.
- 📦 Supports: Linux, macOS
- 🔧 Features: Cargo, rustup toolchain

## Cloud & DevOps

### docker
Container runtime platform.
- ✅ Configuration templates: `daemon.json`
- 📦 Supports: Linux, macOS
- 🔧 Features: Compose plugin, Buildx, user group setup

### k8s-tools
Kubernetes CLI tools bundle.
- 📦 Supports: Linux, macOS
- 🔧 Includes: kubectl, helm, k9s

### terraform
Infrastructure as Code tool.
- ✅ Configuration templates: `main.tf`, `variables.tf`, `outputs.tf`, `terraform.tfvars`, `.gitignore`
- 📦 Supports: Linux, macOS
- 🔧 Features: Multi-cloud, state management, modules

### awscli
AWS Command Line Interface.
- 📦 Supports: Linux, macOS
- 🔧 Features: v1 and v2, multi-profile, SSO

## Monitoring & Observability

### prometheus
Monitoring and alerting system.
- ✅ Configuration templates: `prometheus.yml`
- 📦 Supports: Linux, macOS
- 🔧 Features: Time-series DB, PromQL, exporters

### grafana
Visualization and analytics platform.
- ✅ Configuration templates: `datasource.yml`
- 📦 Supports: Linux, macOS
- 🔧 Features: Dashboards, alerting, multiple datasources

## Security & Infrastructure

### vault
HashiCorp secrets management.
- 📦 Supports: Linux, macOS
- 🔧 Features: Dev mode, server mode, KV store, PKI

### minio
S3-compatible object storage.
- 📦 Supports: Linux, macOS
- 🔧 Features: S3 API, console UI, mc client

## ML/AI Tools

### miniconda
Lightweight conda package manager.
- 📦 Supports: Linux, macOS
- 🔧 Features: Environment management, conda-forge

### jupyter
Interactive notebook environment.
- 📦 Supports: Linux, macOS, Windows
- 🔧 Features: JupyterLab, kernels, extensions

### pytorch
Deep learning framework.
- 📦 Supports: Linux, macOS, Windows
- 🔧 Features: CUDA support, TorchVision, dynamic graphs

### tensorflow
Machine learning platform.
- 📦 Supports: Linux, macOS, Windows
- 🔧 Features: Keras API, GPU support, TensorBoard

## Development Tools

### neovim
Modern Vim-based text editor.
- 📦 Supports: Linux, macOS
- 🔧 Features: LSP, Lua config, Tree-sitter, plugins

### tmux
Terminal multiplexer.
- 📦 Supports: Linux, macOS
- 🔧 Features: Sessions, windows, panes, plugins

### modern-unix
Collection of modern Unix tools.
- 📦 Supports: Linux, macOS
- 🔧 Includes: bat, exa, ripgrep, fd, zoxide, etc.

## AI Models

### ollama
Local LLM runtime.
- 📦 Supports: Linux, macOS
- 🔧 Features: Model management, API server, multiple models

## Usage

```yaml
# Install with default configuration
- preset: nginx
  with:
    state: present

# Install with custom parameters
- preset: prometheus
  with:
    state: present
    port: "9090"
    retention: "30d"

# Uninstall
- preset: nginx
  with:
    state: absent
```

## Configuration Templates

Most presets include configuration templates in `templates/` directory:

```yaml
# Use template to create config file
- name: Deploy nginx config
  template:
    src: presets/nginx/templates/reverse-proxy.conf.j2
    dest: /etc/nginx/sites-available/myapp.conf
  vars:
    server_name: example.com
    backend_url: http://localhost:3000
  become: true
```

## Getting Help

After installation, each preset displays contextual help with:
- Quick start commands
- Configuration file locations
- Common operations
- Usage examples

View preset help anytime:
```bash
cat presets/<preset-name>/README.md
```

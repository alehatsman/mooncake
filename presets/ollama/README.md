# Ollama Preset

Production-ready preset for installing and managing Ollama LLM runtime.

## Structure

```
ollama/
├── preset.yml              # Main preset definition (orchestration)
├── tasks/                  # Task modules
│   ├── install.yml        # Installation logic
│   ├── configure.yml      # Service configuration
│   ├── models.yml         # Model management
│   └── uninstall.yml      # Cleanup tasks
└── templates/              # Configuration templates
    ├── systemd-dropin.conf.j2   # Linux systemd configuration
    └── launchd.plist.j2         # macOS launchd configuration
```

## Features

- **Cross-platform**: Supports Linux (systemd) and macOS (launchd)
- **Flexible installation**: Package manager or official script
- **Service management**: Automatic service configuration and startup
- **Model management**: Pull and manage LLM models with idempotency
- **Configurable**: Custom bind address and models directory
- **Clean uninstall**: Complete removal with optional model cleanup

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` to install, `absent` to uninstall |
| `pull` | array | `[]` | List of models to pull (e.g., `['llama3.1:8b']`) |
| `service` | bool | `true` | Enable and start Ollama service |
| `method` | string | `auto` | Installation method: `auto`, `script`, `package` |
| `host` | string | - | Server bind address (e.g., `0.0.0.0:11434`) |
| `models_dir` | string | - | Custom models directory path |
| `force` | bool | `false` | Force re-pull models, force remove data on uninstall |

## Quick Start

### Basic installation
```yaml
- name: Install Ollama
  preset:
    name: ollama
  become: true
```

### With model
```yaml
- name: Install Ollama with model
  preset:
    name: ollama
    with:
      pull: [tinyllama]
  become: true
```

### Production setup
```yaml
- name: Install Ollama for production
  preset:
    name: ollama
    with:
      service: true
      host: "0.0.0.0:11434"
      models_dir: "/opt/ollama/models"
      pull: ["llama3.1:8b", "mistral:latest"]
  become: true
```

### Uninstall
```yaml
- name: Remove Ollama
  preset:
    name: ollama
    with:
      state: absent
      force: true  # Also remove models
  become: true
```

## How It Works

### Installation Flow (state: present)
1. **Install**: Checks if Ollama exists, installs via package manager or script
2. **Configure**: Sets up systemd/launchd service with environment variables
3. **Models**: Pulls requested models (idempotent)

### Uninstallation Flow (state: absent)
1. Stop and disable service
2. Remove Ollama binary
3. Optionally remove models directory (if `force: true`)

## Customization

### Custom Host Binding
```yaml
with:
  host: "192.168.1.100:11434"
```
Sets `OLLAMA_HOST` environment variable via service configuration.

### Custom Models Directory
```yaml
with:
  models_dir: "/data/ollama"
```
Sets `OLLAMA_MODELS` environment variable via service configuration.

### Installation Methods

- **auto** (default): Tries package manager, falls back to script
- **package**: Uses system package manager only (apt, dnf, yum, brew)
- **script**: Uses official Ollama installation script

## Platform Support

### Linux (systemd)
- Creates drop-in configuration: `/etc/systemd/system/ollama.service.d/10-mooncake.conf`
- Manages service via `systemctl`
- Requires `sudo` for installation

### macOS (launchd)
- Creates launchd plist: `~/Library/LaunchAgents/com.ollama.ollama.plist`
- Manages service via `launchctl`
- Homebrew installation available

## Examples

See `examples/ollama/` for complete usage examples.

## Maintenance

### Adding a new installation method
Edit `tasks/install.yml` and add a new conditional step.

### Modifying service configuration
Edit templates:
- Linux: `templates/systemd-dropin.conf.j2`
- macOS: `templates/launchd.plist.j2`

### Adding model management features
Edit `tasks/models.yml` to add new model operations.

## Dependencies

- **Linux**: systemd, curl (for script installation)
- **macOS**: launchd, Homebrew (for package installation)
- **Both**: Internet connection for model downloads

## Troubleshooting

### Service won't start
Check logs:
- Linux: `journalctl -u ollama -f`
- macOS: `tail -f ~/Library/Logs/ollama.log`

### Models not pulling
Ensure Ollama service is running:
```bash
systemctl status ollama  # Linux
launchctl list | grep ollama  # macOS
```

### Permission errors
Most operations require `become: true` (sudo).

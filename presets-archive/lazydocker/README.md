# lazydocker - Terminal UI for Docker and Docker Compose

Interactive terminal UI for managing Docker containers, images, volumes, and networks with keyboard shortcuts and real-time updates.

## Quick Start
```yaml
- preset: lazydocker
```

## Features
- **Interactive TUI**: Mouse and keyboard navigation
- **Real-time updates**: Live container stats and logs
- **Multi-container logs**: View logs from multiple containers
- **Resource management**: CPU, memory, network stats
- **Quick actions**: Start, stop, restart, remove with hotkeys
- **Docker Compose**: Full support for compose projects

## Basic Usage
```bash
# Launch lazydocker
lazydocker

# Keyboard shortcuts (in UI):
# x - show all commands menu
# h/l - navigate panels left/right
# j/k - navigate items up/down
# [ - previous tab
# ] - next tab
# space - toggle panel
# e - exec into container shell
# s - stop container
# r - restart container
# d - remove container
# l - view logs
# / - search/filter
# q - quit
```

## Advanced Configuration
```yaml
- preset: lazydocker
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove lazydocker |

## Platform Support
- ✅ Linux (binary download)
- ✅ macOS (Homebrew)
- ✅ Windows (Scoop, binary)

## Configuration
- **Config file**: `~/.config/lazydocker/config.yml` (Linux), `~/Library/Application Support/lazydocker/config.yml` (macOS)
- **Docker socket**: Uses `$DOCKER_HOST` or `/var/run/docker.sock`
- **Custom commands**: Define in config file

## Real-World Examples

### Development Workflow
```bash
# Start lazydocker in project directory
cd my-project
lazydocker

# Quick actions:
# 1. View all containers
# 2. Check resource usage
# 3. View logs in real-time
# 4. Restart problematic containers
# 5. Clean up unused images
```

### Docker Compose Management
```bash
# Navigate Docker Compose stacks
lazydocker

# Features:
# - View all services in project
# - Restart entire stack
# - Scale services
# - View aggregated logs
```

### Custom Configuration
```yaml
# ~/.config/lazydocker/config.yml
gui:
  theme:
    activeBorderColor:
      - cyan
      - bold
    inactiveBorderColor:
      - white
  scrollHeight: 2
  language: en

commandTemplates:
  dockerCompose: docker compose
  restartService: '{{ .DockerCompose }} restart {{ .Service.Name }}'

customCommands:
  containers:
    - name: bash
      attach: true
      command: '{{ .DockerCompose }} exec {{ .Container.Name }} /bin/bash'
      serviceNames: []
```

## Agent Use
- Quick Docker environment inspection
- Container debugging and troubleshooting
- Development environment management
- Resource usage monitoring
- Multi-container log analysis

## Troubleshooting

### Cannot connect to Docker daemon
Ensure Docker is running:
```bash
docker ps
```

Check socket permissions:
```bash
sudo usermod -aG docker $USER
# Logout and login again
```

### Permission denied
On Linux, add user to docker group:
```bash
sudo usermod -aG docker $USER
newgrp docker
```

### Config file not loaded
Check config location:
```bash
# Linux
ls -la ~/.config/lazydocker/config.yml

# macOS
ls -la ~/Library/Application\ Support/lazydocker/config.yml
```

## Uninstall
```yaml
- preset: lazydocker
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/jesseduffield/lazydocker
- Documentation: https://github.com/jesseduffield/lazydocker/blob/master/docs/Config.md
- Search: "lazydocker tutorial", "docker terminal ui"

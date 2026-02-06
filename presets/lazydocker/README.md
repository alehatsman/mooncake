# lazydocker Preset

Install lazydocker - a simple terminal UI for Docker and Docker Compose that makes container management intuitive and efficient.

## Quick Start

```yaml
# Basic installation
- preset: lazydocker

# Requires Docker to be installed
- preset: docker
- preset: lazydocker
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | Install (`present`) or uninstall (`absent`) |

## Usage

### Launch lazydocker

```bash
# In any directory
lazydocker

# With docker-compose
cd /path/to/docker-compose-project
lazydocker
```

## Interface Overview

lazydocker provides 5 main panels:

1. **Project** - Docker Compose services overview
2. **Containers** - Running and stopped containers
3. **Images** - Available Docker images
4. **Volumes** - Docker volumes
5. **Networks** - Docker networks

## Key Features

### Container Management
- View real-time logs
- View container stats (CPU, Memory, Network, Block I/O)
- Start/stop/restart containers
- Remove containers
- Execute commands in containers
- View container details
- Inspect container configuration

### Image Management
- Pull new images
- View image layers and history
- Remove unused images
- Prune dangling images
- View image tags
- Create containers from images

### Volume Management
- View volume details
- Remove volumes
- Prune unused volumes
- View containers using volume

### Network Management
- View network configuration
- Remove networks
- View containers on network

### Docker Compose Integration
- View all services in compose project
- Start/stop services
- View service logs
- Rebuild services
- Scale services

## Navigation

### Panel Navigation
| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Next/previous panel |
| `[` / `]` | Previous/next tab within panel |
| `1-5` | Jump to specific panel |
| `H` / `L` | Scroll left/right in main panel |
| `h` / `l` | Previous/next panel |

### Within Panels
| Key | Action |
|-----|--------|
| `j` / `k` or `↓` / `↑` | Move down/up |
| `g` / `G` | Jump to top/bottom |
| `PgUp` / `PgDn` | Scroll page up/down |
| `Enter` | View details |
| `x` | Open context menu |

### Universal
| Key | Action |
|-----|--------|
| `?` | Show help |
| `q` | Quit |
| `Esc` | Go back |
| `Ctrl+c` | Force quit |
| `/` | Search/filter |
| `r` | Refresh |

## Common Operations

### View Container Logs

```bash
# 1. Navigate to Containers panel (press 1)
# 2. Select container with j/k
# 3. Press 'e' to view logs
# 4. Logs update in real-time
# 5. Press 'Esc' to go back
```

### Restart Container

```bash
# 1. Select container
# 2. Press 'r' to restart
# Or:
# 1. Press 'x' for menu
# 2. Select 'restart'
```

### Execute Command in Container

```bash
# 1. Select container
# 2. Press 'c' to open command prompt
# 3. Type command (e.g., '/bin/bash')
# 4. Press Enter
```

### Remove Unused Images

```bash
# 1. Go to Images panel (press 3)
# 2. Press 'p' to prune dangling images
# Or select specific image and press 'd'
```

### View Container Stats

```bash
# 1. Select container
# 2. Press 's' to view stats
# Shows: CPU %, Memory usage, Network I/O, Block I/O
```

## Docker Compose Workflows

### View Service Logs

```bash
# 1. Go to Project panel (if in compose directory)
# 2. Select service
# 3. Press 'e' to view logs
# 4. All service containers' logs are shown
```

### Restart Service

```bash
# 1. Select service in Project panel
# 2. Press 'r' to restart all containers of the service
```

### Rebuild Service

```bash
# 1. Select service
# 2. Press 'x' for menu
# 3. Select 'rebuild'
# Equivalent to: docker-compose up --build <service>
```

### Scale Service

```bash
# 1. Select service
# 2. Press 'x' for menu
# 3. Select 'scale'
# 4. Enter number of instances
```

## Configuration

### Location

- **Linux**: `~/.config/lazydocker/config.yml`
- **macOS**: `~/Library/Application Support/lazydocker/config.yml`
- **Windows**: `%APPDATA%\lazydocker\config.yml`

### Example Configuration

```yaml
gui:
  scrollHeight: 2
  language: 'auto'
  theme:
    activeBorderColor:
      - green
      - bold
    inactiveBorderColor:
      - white
    optionsTextColor:
      - blue

commandTemplates:
  # Custom docker commands
  restartContainer: 'docker restart {{ .Container.ID }}'
  removeContainer: 'docker rm --force {{ .Container.ID }}'
  stopContainer: 'docker stop {{ .Container.ID }}'

customCommands:
  containers:
    - name: bash
      attach: true
      command: 'docker exec -it {{ .Container.ID }} /bin/bash'
    - name: sh
      attach: true
      command: 'docker exec -it {{ .Container.ID }} /bin/sh'
    - name: logs-last-100
      command: 'docker logs --tail 100 {{ .Container.ID }}'

  images:
    - name: scan
      command: 'docker scan {{ .Image.ID }}'
    - name: dive
      command: 'dive {{ .Image.ID }}'

logs:
  timestamps: false
  since: '60m'
  tail: '200'

stats:
  graphs:
    - caption: CPU (%)
      statPath: DerivedStats.CPUPercentage
      color: blue
    - caption: Memory (%)
      statPath: DerivedStats.MemoryPercentage
      color: green
```

## Custom Commands

### Add Common Shell Access

```yaml
customCommands:
  containers:
    - name: 'bash'
      attach: true
      command: 'docker exec -it {{ .Container.ID }} /bin/bash'
      serviceNames: []
    - name: 'sh'
      attach: true
      command: 'docker exec -it {{ .Container.ID }} /bin/sh'
    - name: 'root bash'
      attach: true
      command: 'docker exec -u root -it {{ .Container.ID }} /bin/bash'
```

### Image Scanning

```yaml
customCommands:
  images:
    - name: 'trivy scan'
      command: 'trivy image {{ .Image.Name }}'
    - name: 'dive'
      command: 'dive {{ .Image.Name }}'
```

### Service Operations

```yaml
customCommands:
  services:
    - name: 'logs follow'
      command: 'docker-compose logs -f {{ .Service.Name }}'
    - name: 'rebuild'
      command: 'docker-compose up -d --build {{ .Service.Name }}'
```

## Keybinding Reference

### Containers
| Key | Action |
|-----|--------|
| `e` | View logs |
| `s` | View stats |
| `c` | Execute command |
| `r` | Restart |
| `t` | Stop |
| `d` | Remove |
| `p` | Pause/Unpause |
| `Enter` | View details |
| `x` | Context menu |
| `b` | Bulk operations |

### Images
| Key | Action |
|-----|--------|
| `d` | Remove image |
| `p` | Prune dangling |
| `Enter` | View details |
| `x` | Context menu |

### Volumes
| Key | Action |
|-----|--------|
| `d` | Remove volume |
| `p` | Prune unused |
| `Enter` | View details |

### Services (Compose)
| Key | Action |
|-----|--------|
| `r` | Restart service |
| `s` | Stop service |
| `u` | Start service |
| `d` | Remove service |
| `e` | View logs |
| `Enter` | View details |

## Integration with Tools

### With dive (image explorer)

```bash
# Install dive
brew install dive  # macOS
# or from https://github.com/wagoodman/dive

# Add to lazydocker config
customCommands:
  images:
    - name: 'explore layers'
      command: 'dive {{ .Image.Name }}'
```

### With trivy (security scanner)

```bash
# Install trivy
brew install trivy  # macOS

# Add to lazydocker
customCommands:
  images:
    - name: 'security scan'
      command: 'trivy image {{ .Image.Name }}'
```

### With docker-slim

```bash
# Optimize images
customCommands:
  images:
    - name: 'slim'
      command: 'docker-slim build {{ .Image.Name }}'
```

## Tips and Tricks

1. **Quick log access**: Press `1` (containers), then `e` on any container
2. **Search containers**: Press `/` and type container name
3. **Bulk delete**: Select multiple containers with `Space`, then `d`
4. **Follow logs**: Logs auto-update in real-time
5. **Container shell**: Press `c`, then type `/bin/bash` or `/bin/sh`
6. **View compose project**: Navigate to directory with docker-compose.yml

## Performance Tips

1. **Limit log tail**: Set `logs.tail: 200` in config to reduce memory
2. **Disable auto-refresh**: Set longer refresh interval
3. **Filter containers**: Use `/` to filter and reduce displayed items

## Common Workflows

### Development Workflow

```bash
# 1. Start lazydocker in project directory
lazydocker

# 2. View all services (press 1)
# 3. Check logs of specific service (press e)
# 4. Restart service if needed (press r)
# 5. Execute commands to debug (press c)
```

### Cleanup Workflow

```bash
# 1. Go to Images panel (press 3)
# 2. Prune dangling images (press p)
# 3. Go to Volumes panel (press 4)
# 4. Prune unused volumes (press p)
# 5. Go to Containers panel (press 1)
# 6. Remove stopped containers (select and press d)
```

### Monitoring Workflow

```bash
# 1. Go to Containers panel
# 2. Press 's' on container
# 3. Watch real-time CPU, Memory, Network stats
# 4. Switch between containers with j/k
```

## Troubleshooting

### lazydocker won't start

```bash
# Check Docker is running
docker ps

# Check Docker socket permissions
ls -la /var/run/docker.sock

# Run with sudo if needed (not recommended)
sudo lazydocker
```

### Can't execute commands in container

```bash
# Check container is running
docker ps

# Verify shell exists in container
docker exec CONTAINER_ID ls /bin/bash
docker exec CONTAINER_ID ls /bin/sh
```

### Logs not showing

```bash
# Check log settings in config
logs:
  timestamps: false
  since: '60m'
  tail: '1000'
```

## Uninstall

```yaml
- preset: lazydocker
  with:
    state: absent
```

Configuration will be preserved at `~/.config/lazydocker/`

## Resources

- **GitHub**: https://github.com/jesseduffield/lazydocker
- **Docs**: https://github.com/jesseduffield/lazydocker/blob/master/docs/Config.md
- **Video Tutorial**: https://www.youtube.com/watch?v=NICqQPxwJWw

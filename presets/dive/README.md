# dive - Docker Image Explorer

Explore Docker image layers, find ways to shrink image size, and discover file changes between layers.

## Quick Start
```yaml
- preset: dive
```

## Features
- **Layer-by-Layer Analysis**: Inspect each Docker image layer individually
- **File Change Tracking**: See exactly what files were added, modified, or removed
- **Efficiency Score**: Calculate wasted space and image bloat
- **Interactive UI**: Navigate layers and files with keyboard shortcuts
- **CI/CD Integration**: Automated efficiency threshold checks
- **Size Optimization**: Identify opportunities to reduce image size
- **TUI Interface**: Terminal-based user interface with color coding

## Basic Usage
```bash
# Analyze image
dive <image>
dive nginx:latest
dive myapp:v1.2.3

# Analyze by image ID
dive sha256:abcd1234...

# Analyze build output
docker build -t myapp . && dive myapp

# CI mode (exit with error if efficiency threshold not met)
CI=true dive myapp:latest
```

## Interface Navigation

### Keyboard Shortcuts
| Key | Action |
|-----|--------|
| **Tab** | Switch between layers and file tree |
| **↑/↓** | Navigate list |
| **PageUp/PageDown** | Fast scroll |
| **Ctrl+U/Ctrl+D** | Half-page scroll |
| **Space** | Collapse/expand directory |
| **Ctrl+A** | Show all file changes (added/modified/removed) |
| **Ctrl+R** | Show only removed files |
| **Ctrl+M** | Show only modified files |
| **Ctrl+A** | Show only added files |
| **Ctrl+/** | Filter files |
| **Ctrl+C** | Exit |

### View Modes
- **Layers Panel**: Shows image layers from top (newest) to bottom (oldest)
- **Current Layer Contents**: Files in selected layer
- **Layer Changes**: What this layer modified (added/removed/modified files)
- **Image Details**: Size efficiency, wasted space percentage

## Understanding the UI

### Layer View
```
Layers:
├── 72MB  FROM ubuntu:20.04
├── 1.2KB RUN apt-get update
├── 45MB  RUN apt-get install -y python3
├── 234KB COPY . /app
└── 12KB  CMD ["python3", "app.py"]
```

### File Changes
- **Added** (green): New files in this layer
- **Modified** (yellow): Files changed from previous layers
- **Removed** (red): Files deleted in this layer
- **Unchanged** (white): Files from previous layers

### Efficiency Score
```
Image efficiency: 87%
Wasted space: 234 MB
```

## CI/CD Integration
```bash
# Set efficiency threshold (default 90%)
CI=true dive --highestUserWastedPercent=20 myapp:latest

# Exit codes:
# 0 - Image meets efficiency threshold
# 1 - Image fails efficiency threshold

# CI pipeline example
if CI=true dive --highestUserWastedPercent=15 myapp:latest; then
  echo "Image efficiency acceptable"
  docker push myapp:latest
else
  echo "Image too bloated, optimize before pushing"
  exit 1
fi
```

## Finding Image Bloat

### Common Sources of Waste
1. **Package manager caches**
```dockerfile
# Bad
RUN apt-get update && apt-get install -y python3

# Good
RUN apt-get update && apt-get install -y python3 \
    && rm -rf /var/lib/apt/lists/*
```

2. **Build artifacts**
```dockerfile
# Bad
COPY . /app
RUN npm install
RUN npm run build

# Good
COPY package*.json /app/
RUN npm ci --only=production
COPY src /app/src
RUN npm run build && rm -rf node_modules src
```

3. **Temporary files**
```dockerfile
# Bad
RUN wget https://example.com/file.tar.gz \
    && tar xzf file.tar.gz \
    && mv file /usr/local/bin/

# Good
RUN wget https://example.com/file.tar.gz \
    && tar xzf file.tar.gz \
    && mv file /usr/local/bin/ \
    && rm file.tar.gz
```

4. **Log files and caches**
```dockerfile
# Clean up in same layer
RUN make install \
    && rm -rf /tmp/* \
    && rm -rf /var/log/* \
    && rm -rf ~/.cache
```

## Optimization Workflow
```bash
# 1. Build initial image
docker build -t myapp:v1 .

# 2. Analyze with dive
dive myapp:v1

# 3. Identify wasted space
# - Look for large removed files
# - Check for package manager caches
# - Find duplicate files across layers

# 4. Optimize Dockerfile
# - Combine RUN commands
# - Clean up in same layer
# - Use multi-stage builds

# 5. Rebuild and compare
docker build -t myapp:v2 .
dive myapp:v2

# 6. Compare images
docker images myapp
```

## Real-World Examples

### Python Application
```dockerfile
# Before (450 MB)
FROM python:3.9
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . /app
WORKDIR /app

# After (180 MB)
FROM python:3.9-slim
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt \
    && rm requirements.txt
COPY app.py /app/
WORKDIR /app
```

### Node.js Application
```dockerfile
# Before (950 MB)
FROM node:16
WORKDIR /app
COPY . .
RUN npm install
RUN npm run build

# After (120 MB)
FROM node:16-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:16-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY package*.json ./
RUN npm ci --only=production
CMD ["node", "dist/main.js"]
```

## CI Configuration

### GitHub Actions
```yaml
- name: Build and analyze image
  run: |
    docker build -t myapp:${{ github.sha }} .
    CI=true dive myapp:${{ github.sha }} --ci-config .dive-ci.yaml
```

### GitLab CI
```yaml
docker-analyze:
  image: wagoodman/dive:latest
  script:
    - dive ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA} --ci
  only:
    - merge_requests
```

### dive-ci.yaml
```yaml
rules:
  # Fail if wasted space > 100MB
  - name: highestWastedBytes
    value: 100000000

  # Fail if efficiency < 90%
  - name: lowestEfficiency
    value: 0.90
```

## Tips for Layer Analysis
- **Newest layer on top**: Dive shows most recent changes first
- **Layer command**: Each layer shows the Dockerfile instruction
- **Permission changes count**: Chmod creates file modifications
- **File size in layer**: Not the total size, just what changed
- **Removed files still take space**: Only removed in view, not from image

## Multi-Stage Build Analysis
```bash
# Dive can't analyze intermediate stages directly
# But you can tag them:
docker build --target builder -t myapp:builder .
dive myapp:builder

docker build -t myapp:final .
dive myapp:final
```

## Common Patterns to Spot

### Package Manager Waste
```bash
# In dive, look for:
/var/cache/apt/archives/     # Debian/Ubuntu
/var/cache/yum/              # CentOS/RHEL
~/.npm/                      # npm cache
~/.cache/pip/                # pip cache
```

### Build Artifacts
```bash
# Common waste:
*.pyc files
node_modules/ (in final image)
.git/ directory
test files in production
source code after compilation
```

### Multi-Layer Inefficiency
```bash
# Pattern to find:
Layer 1: ADD large-file.tar.gz
Layer 2: RUN process large-file
Layer 3: RUN rm large-file.tar.gz  # ← File still in layer 1!

# Solution: Do it in one layer
RUN wget large-file.tar.gz && process && rm large-file.tar.gz
```

## Advanced Usage
```bash
# Analyze without pulling (if image exists locally)
dive --source docker myapp:latest

# Analyze podman image
dive --source podman myapp:latest

# Analyze docker-archive file
dive --source docker-archive image.tar

# Custom CI rules
dive --ci-config custom-rules.yaml myapp:latest

# JSON output for automation
dive --json myapp:latest > analysis.json
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated image optimization validation
- CI/CD quality gates
- Security scan complement (find unexpected files)
- Build optimization guidance
- Container image auditing


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install dive
  preset: dive

- name: Use dive in automation
  shell: |
    # Custom configuration here
    echo "dive configured"
```
## Uninstall
```yaml
- preset: dive
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/wagoodman/dive
- Search: "dive docker optimization", "reduce docker image size"

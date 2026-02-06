# docker - Container Runtime Platform

Platform for building, running, and distributing containerized applications.

## Quick Start
```yaml
- preset: docker
```

## Features
- **Container runtime**: Run isolated applications in containers
- **Image management**: Build, pull, push container images
- **Docker Compose**: Multi-container orchestration
- **Buildx**: Multi-platform image building
- **Networking**: Container networking and port mapping
- **Cross-platform**: Linux, macOS support

## Basic Usage
```bash
# Check version
docker --version

# Run hello-world container
docker run hello-world

# Pull an image
docker pull nginx:latest

# Run container with port mapping
docker run -d -p 8080:80 --name webserver nginx

# List running containers
docker ps

# List all containers (including stopped)
docker ps -a

# Stop container
docker stop webserver

# Remove container
docker rm webserver

# List images
docker images

# Remove image
docker rmi nginx:latest

# View container logs
docker logs webserver

# Execute command in running container
docker exec -it webserver bash

# Build image from Dockerfile
docker build -t myapp:1.0 .

# Tag and push image
docker tag myapp:1.0 registry.example.com/myapp:1.0
docker push registry.example.com/myapp:1.0
```

## Advanced Configuration
```yaml
# Full installation with plugins
- preset: docker
  with:
    install_compose: true
    install_buildx: true
    start_service: true
    add_user_to_group: true
  become: true

# Minimal installation
- preset: docker
  with:
    install_compose: false
    install_buildx: false
  become: true
```

## Docker Compose

```bash
# Check Compose version
docker compose version

# Start services
docker compose up -d

# Stop services
docker compose down

# View logs
docker compose logs -f
```

## Add User to Docker Group (Linux)

```bash
sudo usermod -aG docker $USER
# Log out and back in for changes to take effect
```

## Troubleshooting

```bash
# Restart Docker daemon
sudo systemctl restart docker  # Linux
brew services restart docker  # macOS

# Clean up unused resources
docker system prune

# Clean up everything (including volumes)
docker system prune -a --volumes
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |
| start_service | bool | true | Start Docker service after installation |
| add_user_to_group | bool | true | Add current user to docker group (Linux) |
| install_compose | bool | true | Install Docker Compose plugin |
| install_buildx | bool | true | Install Docker Buildx plugin |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew, Docker Desktop)
- ❌ Windows

## Configuration
- **Config file**: `/etc/docker/daemon.json` (Linux), `~/.docker/daemon.json` (macOS)
- **Socket**: `/var/run/docker.sock`
- **Data directory**: `/var/lib/docker/` (Linux)
- **Default registry**: Docker Hub (hub.docker.com)

## Real-World Examples

### Web Application Stack
```bash
# Create docker-compose.yml
cat > docker-compose.yml <<EOF
version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./html:/usr/share/nginx/html

  api:
    build: ./api
    ports:
      - "8000:8000"
    environment:
      DATABASE_URL: postgres://db:5432/myapp

  db:
    image: postgres:15-alpine
    environment:
      POSTGRES_PASSWORD: secret
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
EOF

# Start stack
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f api
```

### Multi-Stage Build
```dockerfile
# Dockerfile for optimized Python app
FROM python:3.11-slim AS builder
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

FROM python:3.11-slim
WORKDIR /app
COPY --from=builder /root/.local /root/.local
COPY app.py .
ENV PATH=/root/.local/bin:$PATH
CMD ["python", "app.py"]
```

### CI/CD Pipeline
```bash
# Build with cache
docker build --cache-from myapp:latest -t myapp:$CI_COMMIT_SHA .

# Run tests in container
docker run --rm myapp:$CI_COMMIT_SHA pytest

# Push if tests pass
docker tag myapp:$CI_COMMIT_SHA myapp:latest
docker push myapp:latest
```

## Agent Use
- Build and test applications in containers
- Deploy microservices and distributed systems
- Create reproducible development environments
- Package applications for distribution
- Run CI/CD pipelines
- Isolate application dependencies

## Troubleshooting

### Permission denied
```bash
# Add user to docker group (Linux)
sudo usermod -aG docker $USER
# Log out and back in

# Check docker group membership
groups $USER
```

### Service not running
```bash
# Start Docker daemon
sudo systemctl start docker        # Linux (systemd)
sudo service docker start          # Linux (SysV)
open -a Docker                     # macOS (Docker Desktop)

# Enable on boot
sudo systemctl enable docker       # Linux
```

### Disk space issues
```bash
# Remove unused containers, images, networks
docker system prune

# Remove everything including volumes
docker system prune -a --volumes

# Check disk usage
docker system df
```

### Cannot connect to daemon
```bash
# Check if Docker is running
docker info

# Check socket permissions
ls -l /var/run/docker.sock

# Restart Docker
sudo systemctl restart docker      # Linux
brew services restart docker       # macOS (if using Homebrew)
```

### Network conflicts
```bash
# List Docker networks
docker network ls

# Remove unused networks
docker network prune

# Create custom network
docker network create --driver bridge mynetwork
```

## Uninstall
```yaml
- preset: docker
  with:
    state: absent
  become: true
```

**Note**: Images, containers, and volumes are preserved after uninstall unless manually removed.

## Resources
- Official docs: https://docs.docker.com/
- Docker Hub: https://hub.docker.com/
- Search: "docker tutorial", "dockerfile best practices", "docker compose"

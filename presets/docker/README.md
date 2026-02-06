# Docker Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Check version
docker --version

# Run hello-world
docker run hello-world

# Check if daemon is running
docker ps

# View Docker info
docker info
```

## Configuration

- **Config file:** `/etc/docker/daemon.json` (Linux), `~/.docker/daemon.json` (macOS)
- **Socket:** `/var/run/docker.sock`
- **Images:** `/var/lib/docker/` (Linux), `~/Library/Containers/com.docker.docker/` (macOS)

## Common Operations

```bash
# Pull image
docker pull nginx

# Run container
docker run -d -p 80:80 nginx

# List running containers
docker ps

# List all containers
docker ps -a

# Stop container
docker stop <container_id>

# Remove container
docker rm <container_id>

# List images
docker images

# Remove image
docker rmi <image_id>

# View logs
docker logs <container_id>

# Execute command in container
docker exec -it <container_id> bash

# Build image from Dockerfile
docker build -t myapp:latest .
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

## Uninstall

```yaml
- preset: docker
  with:
    state: absent
```

**Note:** Images and containers are preserved after uninstall.

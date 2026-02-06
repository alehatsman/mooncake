# Docker Stack Setup

Install Docker and Docker Compose on Ubuntu, then deploy a simple multi-container stack.

## What This Does

This scenario demonstrates:
- Installing Docker Engine from official Docker repository
- Installing Docker Compose plugin
- Building a custom Flask application image
- Orchestrating multiple containers with docker-compose
- Setting up nginx as a reverse proxy for the Flask app
- Container networking and health checks
- Managing user permissions for Docker

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed
- Internet connection

## Files

- `setup.yml` - Main deployment playbook
- `files/app.py` - Flask web application
- `files/Dockerfile` - Container image definition
- `templates/docker-compose.yml.j2` - Multi-container orchestration config
- `templates/nginx.conf.j2` - Nginx reverse proxy configuration

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom settings
mooncake run setup.yml --var project_name=mystack --var nginx_port=9090
```

## Variables

You can customize these variables:

- `project_name` (default: `mooncake-stack`) - Project name
- `project_dir` (default: `/opt/{{ project_name }}`) - Project directory
- `app_port` (default: `5000`) - Flask application port
- `nginx_port` (default: `8080`) - Nginx public port
- `docker_user` (default: current user) - User to add to docker group

## What Gets Deployed

### Docker Components
- Docker Engine (latest stable)
- Docker Compose plugin
- containerd runtime
- Docker Buildx plugin

### Container Stack
- **Flask App Container** - Python web application
- **Nginx Container** - Reverse proxy and load balancer

### Network
- Custom bridge network for container communication
- Port mappings for external access

## Stack Architecture

```
Internet
    |
    | :8080
    v
[Nginx Container]
    |
    | internal network
    |
    v
[Flask App Container] :5000
```

## Using Your Stack

### Access the Application

```bash
# Through nginx (production-like)
curl http://localhost:8080

# Direct app access
curl http://localhost:5000

# API endpoints
curl http://localhost:8080/api/info
curl http://localhost:8080/api/health
curl http://localhost:8080/api/env
```

### Docker Compose Commands

```bash
cd /opt/mooncake-stack

# View running containers
sudo docker compose ps

# View logs
sudo docker compose logs
sudo docker compose logs -f        # Follow logs
sudo docker compose logs app       # Specific service

# Restart services
sudo docker compose restart
sudo docker compose restart app    # Specific service

# Stop the stack
sudo docker compose down

# Stop and remove volumes
sudo docker compose down -v

# Rebuild and restart
sudo docker compose up -d --build

# Scale services (if stateless)
sudo docker compose up -d --scale app=3
```

### Docker Commands

```bash
# List all containers
sudo docker ps -a

# View container stats
sudo docker stats

# Execute command in container
sudo docker exec -it mooncake-stack-app bash

# View container logs
sudo docker logs mooncake-stack-app

# Inspect container
sudo docker inspect mooncake-stack-app

# View images
sudo docker images

# Remove unused resources
sudo docker system prune
```

### Using Docker Without Sudo

After setup, you'll need to re-login or run:

```bash
newgrp docker
```

Then you can use docker without sudo:

```bash
docker ps
docker compose ps
```

## Project Structure

```
/opt/mooncake-stack/
├── docker-compose.yml    # Orchestration config
├── nginx.conf            # Nginx configuration
└── app/
    ├── Dockerfile        # Image definition
    └── app.py           # Flask application
```

## Customizing the Application

### Modify Flask App

```bash
sudo nano /opt/mooncake-stack/app/app.py
```

### Rebuild and Deploy

```bash
cd /opt/mooncake-stack
sudo docker compose up -d --build
```

### Add Environment Variables

Edit `docker-compose.yml`:

```yaml
services:
  app:
    environment:
      - MY_VAR=value
      - DATABASE_URL=postgresql://...
```

## Monitoring and Debugging

### Check Container Health

```bash
sudo docker compose ps
sudo docker inspect mooncake-stack-app | grep -A 10 Health
```

### View Resource Usage

```bash
sudo docker stats
```

### Troubleshooting

```bash
# View detailed logs
sudo docker compose logs --tail=100

# Check if containers are running
sudo docker compose ps

# Restart problematic service
sudo docker compose restart app

# Rebuild from scratch
sudo docker compose down
sudo docker compose up -d --build

# Check Docker daemon status
sudo systemctl status docker

# View Docker daemon logs
sudo journalctl -u docker -f
```

## Cleanup

To remove the stack:

```bash
# Stop and remove containers
cd /opt/mooncake-stack
sudo docker compose down

# Remove project directory
sudo rm -rf /opt/mooncake-stack

# Remove Docker images
sudo docker rmi mooncake-stack-app nginx:alpine python:3.11-slim

# Optionally remove Docker completely
sudo systemctl stop docker
sudo apt-get remove --purge docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo rm -rf /var/lib/docker
```

## Learning Points

This example teaches:
- Installing Docker from official repositories
- Building custom Docker images with Dockerfile
- Multi-container orchestration with Docker Compose
- Container networking and service discovery
- Reverse proxy configuration with Nginx
- Container health checks
- Volume management
- Docker security basics (user groups)
- Container logs and monitoring

## Production Considerations

For production deployments, also consider:

- **Security:**
  - Use specific image tags, not `latest`
  - Scan images for vulnerabilities
  - Run containers as non-root users
  - Use secrets management

- **Reliability:**
  - Implement proper health checks
  - Configure restart policies
  - Set resource limits (CPU, memory)
  - Use volume backups

- **Monitoring:**
  - Centralized logging (ELK, Grafana Loki)
  - Metrics collection (Prometheus)
  - Alerting (Alertmanager)

- **Scaling:**
  - Load balancing across multiple instances
  - Container orchestration (Kubernetes)
  - Database connection pooling
  - Caching layers (Redis)

## Next Steps

After deployment, try:
- Adding a PostgreSQL database service
- Implementing Redis for caching
- Adding more API endpoints
- Setting up SSL with Let's Encrypt
- Deploying your own application
- Exploring Docker Swarm or Kubernetes
- Adding monitoring with Prometheus and Grafana

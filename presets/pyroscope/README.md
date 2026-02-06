# Pyroscope - Continuous Profiling Platform

Open-source continuous profiling platform for analyzing application performance and identifying bottlenecks in production.

## Quick Start

```yaml
- preset: pyroscope
```

## Features

- **Continuous profiling**: Always-on profiling with minimal overhead (<2% CPU)
- **Multiple languages**: Go, Python, Java, Ruby, .NET, Rust, Node.js, PHP
- **Distributed tracing**: Integrates with Jaeger, Tempo, Datadog
- **Flame graphs**: Interactive visualization of performance hotspots
- **Time-based analysis**: Compare profiles across different time periods
- **Storage efficient**: Compressed profile storage for long-term retention
- **Cross-platform**: Linux, macOS support

## Basic Usage

```bash
# Check version
pyroscope --version

# Start server (default port 4040)
pyroscope server

# View web UI
open http://localhost:4040

# Agent mode (profile application)
pyroscope agent --application-name=myapp --server-address=http://localhost:4040 -- python app.py
```

## Advanced Configuration

```yaml
# Install with custom configuration
- preset: pyroscope
  register: pyroscope_result

# Verify installation
- name: Check Pyroscope version
  shell: pyroscope version
  register: version_check

# Server mode (for collecting profiles)
- name: Start Pyroscope server
  shell: |
    pyroscope server \
      --storage-path=/var/lib/pyroscope \
      --log-level=info \
      --port=4040
  become: true

# Agent mode (profile application)
- name: Profile Python application
  shell: |
    pyroscope exec \
      --application-name=myapp \
      --server-address=http://localhost:4040 \
      --spy-name=pyspy \
      python app.py
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (`present` or `absent`) |

## Platform Support

- ✅ Linux (apt, dnf, yum via binary install)
- ✅ macOS (Homebrew, binary install)
- ❌ Windows (use Docker or WSL)

## Configuration

- **Config file**: `/etc/pyroscope/server.yml` (server), `~/.config/pyroscope/agent.yml` (agent)
- **Data directory**: `/var/lib/pyroscope/` (server mode)
- **Default port**: 4040 (HTTP API and web UI)
- **Binary location**: `/usr/local/bin/pyroscope`

## Real-World Examples

### Profiling Production Python Service

```yaml
# Install Pyroscope server
- preset: pyroscope
  become: true

# Configure as systemd service
- name: Create Pyroscope server service
  service:
    name: pyroscope
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=Pyroscope Server
        After=network.target

        [Service]
        Type=simple
        User=pyroscope
        ExecStart=/usr/local/bin/pyroscope server --storage-path=/var/lib/pyroscope
        Restart=always
        RestartSec=10

        [Install]
        WantedBy=multi-user.target
  when: os == "linux"
  become: true

# Profile application with agent
- name: Start application with profiling
  shell: |
    pyroscope exec \
      --application-name=api-service \
      --server-address=http://localhost:4040 \
      --tag env=production \
      --tag version=v1.2.3 \
      gunicorn app:app --workers 4
  become: true
```

### Go Application Profiling

```yaml
# Install Pyroscope
- preset: pyroscope

# Profile Go application (push mode)
- name: Run Go app with profiling
  shell: |
    # Go app with built-in Pyroscope client
    go run main.go
  environment:
    PYROSCOPE_SERVER_ADDRESS: http://localhost:4040
    PYROSCOPE_APPLICATION_NAME: go-api
```

Go code example:

```go
package main

import (
    "github.com/pyroscope-io/client/pyroscope"
)

func main() {
    pyroscope.Start(pyroscope.Config{
        ApplicationName: "go-api",
        ServerAddress:   "http://localhost:4040",
        Tags:            map[string]string{"env": "production"},
    })
    // Your application code
}
```

### CI/CD Performance Testing

```bash
# Profile benchmark tests
pyroscope exec \
  --application-name=benchmark \
  --server-address=http://pyroscope.example.com \
  --tag branch=$CI_BRANCH \
  --tag commit=$CI_COMMIT \
  pytest tests/benchmarks/

# Compare profiles between commits
curl "http://localhost:4040/api/compare?query=benchmark&from=$OLD_COMMIT&to=$NEW_COMMIT"
```

### Distributed System Profiling

```yaml
# Central Pyroscope server
- preset: pyroscope
  hosts: monitoring-server
  become: true

# Profile multiple services
- name: Profile microservices
  shell: |
    pyroscope exec \
      --application-name={{ service_name }} \
      --server-address=http://pyroscope.example.com:4040 \
      --tag service={{ service_name }} \
      --tag datacenter={{ datacenter }} \
      docker run {{ docker_image }}
  loop:
    - auth-service
    - api-gateway
    - worker-service
```

## Language Integration

### Python

```bash
# Using pip package
pip install pyroscope-io

# Profile with decorator
pyroscope exec --application-name=myapp python app.py

# Or in code
from pyroscope import Profiler
Profiler.start(application_name="myapp", server_address="http://localhost:4040")
```

### Java

```bash
# Using agent JAR
java -javaagent:pyroscope.jar=server=http://localhost:4040,applicationName=myapp -jar app.jar
```

### Ruby

```bash
# Using gem
gem install pyroscope

# Profile Rails app
pyroscope exec --application-name=rails-app -- rails server
```

## Agent Use

- Profile production applications with minimal overhead
- Detect performance regressions in CI/CD pipelines
- Compare performance between releases/branches
- Monitor resource usage patterns over time
- Identify memory leaks and CPU bottlenecks
- Generate performance reports for capacity planning
- Debug slow endpoints in microservices
- Optimize database queries and external API calls

## Troubleshooting

### Server won't start

Check logs and port availability:

```bash
# Check if port 4040 is in use
lsof -i :4040

# Run with debug logging
pyroscope server --log-level=debug

# Check storage directory permissions
ls -la /var/lib/pyroscope
```

### Agent can't connect to server

Verify connectivity:

```bash
# Test connection
curl http://localhost:4040/api/apps

# Check firewall rules
sudo iptables -L | grep 4040

# Verify server is running
ps aux | grep pyroscope
```

### High memory usage

Adjust retention and sampling:

```bash
# Limit retention period
pyroscope server --retention=7d

# Configure storage limits
pyroscope server --max-storage-size=10GB
```

### Missing profiles

Check spy compatibility:

```bash
# List available spies
pyroscope spy list

# Use specific spy
pyroscope exec --spy-name=ebpfspy -- python app.py
```

## Uninstall

```yaml
- preset: pyroscope
  with:
    state: absent
```

**Note**: This removes the Pyroscope binary but preserves profile data in `/var/lib/pyroscope/`.

## Resources

- Official docs: https://pyroscope.io/docs/
- GitHub: https://github.com/grafana/pyroscope
- Examples: https://github.com/grafana/pyroscope/tree/main/examples
- Search: "pyroscope profiling tutorial", "pyroscope flame graphs", "continuous profiling best practices"

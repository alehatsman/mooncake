# Datadog Agent - Infrastructure Monitoring

Official monitoring agent for Datadog. Collects metrics, traces, and logs from hosts and sends them to Datadog for visualization and alerting.

## Quick Start
```yaml
- preset: datadog-agent
```

## Features
- **Full-stack monitoring**: Infrastructure, application, and custom metrics
- **Distributed tracing**: APM for application performance monitoring
- **Log management**: Collect, parse, and analyze logs
- **Real-time alerting**: Monitor health and performance with alerts
- **Integrations**: 700+ integrations for services and platforms
- **Low overhead**: Minimal CPU and memory footprint

## Basic Usage
```bash
# Check agent status
sudo datadog-agent status

# Start agent
sudo systemctl start datadog-agent  # Linux
sudo launchctl start com.datadoghq.agent  # macOS

# Stop agent
sudo systemctl stop datadog-agent

# Restart agent
sudo systemctl restart datadog-agent

# View agent logs
sudo tail -f /var/log/datadog/agent.log

# Run checks
sudo datadog-agent check <check_name>
```

## Advanced Configuration
```yaml
- name: Install Datadog agent
  preset: datadog-agent
  become: true

- name: Configure agent
  template:
    dest: /etc/datadog-agent/datadog.yaml
    content: |
      api_key: {{ dd_api_key }}
      site: datadoghq.com

      logs_enabled: true
      apm_config:
        enabled: true
      process_config:
        enabled: true
  become: true

- name: Restart agent
  service:
    name: datadog-agent
    state: restarted
  become: true
```

## Configuration

### Main Config File
**Linux**: `/etc/datadog-agent/datadog.yaml`
**macOS**: `/opt/datadog-agent/etc/datadog.yaml`

```yaml
# /etc/datadog-agent/datadog.yaml
api_key: your_api_key_here
site: datadoghq.com  # or datadoghq.eu for EU

# Hostname
hostname: my-server-01

# Tags
tags:
  - env:production
  - service:api
  - team:backend

# Logs
logs_enabled: true

# APM
apm_config:
  enabled: true
  env: production

# Process monitoring
process_config:
  enabled: "true"
  scrub_args: true

# Network monitoring
network_config:
  enabled: true
```

### Integration Configs
Directory: `/etc/datadog-agent/conf.d/`

```yaml
# nginx.d/conf.yaml
init_config:

instances:
  - nginx_status_url: http://localhost/nginx_status/
    tags:
      - instance:nginx-primary
```

## Real-World Examples

### Production Server Setup
```yaml
- name: Install Datadog agent
  preset: datadog-agent
  become: true

- name: Configure with API key
  template:
    dest: /etc/datadog-agent/datadog.yaml
    content: |
      api_key: {{ lookup('env', 'DD_API_KEY') }}
      site: datadoghq.com
      hostname: {{ inventory_hostname }}
      tags:
        - env:{{ environment }}
        - role:{{ server_role }}
      logs_enabled: true
      apm_config:
        enabled: true
  become: true

- name: Enable and start agent
  service:
    name: datadog-agent
    state: started
    enabled: true
  become: true

- name: Verify agent is running
  assert:
    command:
      cmd: systemctl is-active datadog-agent
      exit_code: 0
```

### Docker Container Monitoring
```yaml
- name: Configure Docker monitoring
  template:
    dest: /etc/datadog-agent/conf.d/docker.d/conf.yaml
    content: |
      init_config:

      instances:
        - url: "unix://var/run/docker.sock"
          collect_images_stats: true
          collect_container_size: true
          collect_events: true
  become: true

- name: Restart agent
  service:
    name: datadog-agent
    state: restarted
  become: true
```

### Custom Metrics
```yaml
- name: Send custom metric
  shell: |
    echo "custom.metric:42|g|#env:prod" | nc -u -w1 localhost 8125
```

### Log Collection
```yaml
- name: Configure log collection
  template:
    dest: /etc/datadog-agent/conf.d/custom.d/conf.yaml
    content: |
      logs:
        - type: file
          path: /var/log/myapp/*.log
          service: myapp
          source: custom
          tags:
            - env:production
  become: true
```

## Integrations

### NGINX
```yaml
# /etc/datadog-agent/conf.d/nginx.d/conf.yaml
init_config:

instances:
  - nginx_status_url: http://localhost:81/nginx_status
    tags:
      - instance:primary
```

### PostgreSQL
```yaml
# /etc/datadog-agent/conf.d/postgres.d/conf.yaml
init_config:

instances:
  - host: localhost
    port: 5432
    username: datadog
    password: {{ postgres_password }}
    tags:
      - db:production
```

### Redis
```yaml
# /etc/datadog-agent/conf.d/redisdb.d/conf.yaml
init_config:

instances:
  - host: localhost
    port: 6379
    password: {{ redis_password }}
```

### Custom Check
```python
# /etc/datadog-agent/checks.d/my_check.py
from datadog_checks.base import AgentCheck

class MyCheck(AgentCheck):
    def check(self, instance):
        self.gauge('my.custom.metric', 42, tags=['env:prod'])
```

## Commands

```bash
# Status and info
sudo datadog-agent status               # Full status
sudo datadog-agent version              # Agent version
sudo datadog-agent hostname             # Get hostname
sudo datadog-agent config               # Show config

# Checks
sudo datadog-agent check <check_name>   # Run check
sudo datadog-agent check <check_name> --check-rate  # With rate metrics
sudo datadog-agent configcheck          # Validate configs

# Diagnostics
sudo datadog-agent diagnose             # Run diagnostics
sudo datadog-agent health               # Health check
sudo datadog-agent flare                # Create support bundle

# Service
sudo systemctl start datadog-agent
sudo systemctl stop datadog-agent
sudo systemctl restart datadog-agent
sudo systemctl status datadog-agent

# Logs
sudo tail -f /var/log/datadog/agent.log
sudo tail -f /var/log/datadog/trace-agent.log
sudo tail -f /var/log/datadog/process-agent.log
```

## Agent Components

### Core Agent
- Collects metrics from integrations
- Sends metrics to Datadog
- Runs service checks

### Trace Agent (APM)
- Collects application traces
- Processes distributed tracing data
- Port: 8126

### Process Agent
- Monitors running processes
- Collects process metrics
- Live process view in Datadog

### Log Agent
- Collects and forwards logs
- Parses and structures log data
- Tail files and follow containers

## Metrics Collection

### System Metrics
Automatically collected:
- CPU usage
- Memory usage
- Disk I/O
- Network traffic
- System load

### Custom Metrics
```bash
# DogStatsD (UDP 8125)
echo "my.metric:42|g" | nc -u -w1 localhost 8125

# With tags
echo "my.metric:42|g|#env:prod,service:api" | nc -u -w1 localhost 8125

# Counter
echo "page.views:1|c" | nc -u -w1 localhost 8125

# Histogram
echo "response.time:250|h" | nc -u -w1 localhost 8125
```

### Application Code
```python
# Python
from datadog import statsd

statsd.increment('page.views')
statsd.gauge('users.active', 42)
statsd.histogram('request.duration', 250)
statsd.timing('query.time', 150)
```

## Troubleshooting

### Agent Not Sending Data
```bash
# Check status
sudo datadog-agent status

# Verify API key
grep api_key /etc/datadog-agent/datadog.yaml

# Check connectivity
sudo datadog-agent diagnose

# View logs
sudo tail -f /var/log/datadog/agent.log
```

### High CPU/Memory Usage
```bash
# Check which integrations are enabled
sudo datadog-agent status | grep -A 5 "Collector"

# Disable expensive checks
# Edit /etc/datadog-agent/conf.d/<check>.d/conf.yaml
# Set min_collection_interval to higher value

# Restart agent
sudo systemctl restart datadog-agent
```

### Integration Not Working
```bash
# Test integration
sudo datadog-agent check <integration_name>

# Validate config
sudo datadog-agent configcheck

# Check permissions
ls -la /etc/datadog-agent/conf.d/<integration>.d/
```

### Logs Not Appearing
```bash
# Verify logs_enabled
grep logs_enabled /etc/datadog-agent/datadog.yaml

# Check log config
cat /etc/datadog-agent/conf.d/<service>.d/conf.yaml

# Tail agent log
sudo tail -f /var/log/datadog/agent.log | grep -i log

# Test log collection
sudo datadog-agent check logs
```

## Security

### API Key Protection
```yaml
# Use environment variables
api_key: ${DD_API_KEY}

# Or use secrets management
api_key: {{ vault_dd_api_key }}

# Restrict file permissions
sudo chmod 640 /etc/datadog-agent/datadog.yaml
sudo chown dd-agent:dd-agent /etc/datadog-agent/datadog.yaml
```

### Scrubbing Sensitive Data
```yaml
# Scrub process arguments
process_config:
  scrub_args: true
  custom_sensitive_words:
    - password
    - token
    - api_key

# Log scrubbing
logs_config:
  force_use_http: true
  processing_rules:
    - type: exclude_at_match
      name: exclude_secrets
      pattern: (password|token|key)=\S+
```

## Platform Support
- ✅ Linux (apt, dnf, yum, zypper)
- ✅ macOS (Homebrew, installer)
- ✅ Windows (installer)
- ✅ Docker (official image)
- ✅ Kubernetes (Helm chart, DaemonSet)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated infrastructure monitoring setup
- Configuration management in CI/CD
- Multi-cloud monitoring deployment
- Container and Kubernetes monitoring
- Custom metrics collection from applications
- Log aggregation and analysis
- APM and distributed tracing
- Real-time alerting and incident response

## Uninstall
```yaml
- preset: datadog-agent
  with:
    state: absent
```

**Note**: Removes agent but preserves configuration files.

## Resources
- Official docs: https://docs.datadoghq.com/
- Agent docs: https://docs.datadoghq.com/agent/
- Integrations: https://docs.datadoghq.com/integrations/
- APM: https://docs.datadoghq.com/tracing/
- GitHub: https://github.com/DataDog/datadog-agent
- Search: "datadog agent setup", "datadog monitoring", "datadog integrations"

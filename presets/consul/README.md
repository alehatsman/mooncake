# Consul Preset

Install HashiCorp Consul - a service mesh solution providing service discovery, configuration, and segmentation functionality across distributed infrastructure.

## Quick Start

```yaml
# Development mode (single-node, in-memory)
- preset: consul
  with:
    mode: dev

# Production server
- preset: consul
  with:
    mode: server
    bootstrap_expect: "3"
    client_addr: "0.0.0.0"
    start_service: true
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | Install (`present`) or uninstall (`absent`) |
| `version` | string | `latest` | Consul version (e.g., `1.17.0`) |
| `mode` | string | `server` | Mode: `server`, `client`, or `dev` |
| `bootstrap_expect` | string | `1` | Number of servers before bootstrapping cluster |
| `data_dir` | string | `/opt/consul/data` | Data storage directory |
| `bind_addr` | string | `0.0.0.0` | Cluster communication bind address |
| `client_addr` | string | `127.0.0.1` | Client API bind address |
| `ui` | bool | `true` | Enable web UI |
| `datacenter` | string | `dc1` | Datacenter name |
| `start_service` | bool | `false` | Start service after installation |

## Usage Examples

### Development Mode

Perfect for local development and testing:

```yaml
- name: Run Consul in dev mode
  preset: consul
  with:
    mode: dev

- name: Start dev server
  shell: consul agent -dev > /tmp/consul-dev.log 2>&1 &

- name: Wait and verify
  shell: sleep 3 && consul members
```

Dev mode characteristics:
- Single-node server
- In-memory storage (no persistence)
- Automatically bootstrapped
- UI enabled at http://127.0.0.1:8500/ui

### Production Cluster (3-Server HA)

```yaml
# Server 1
- name: Install Consul server 1
  preset: consul
  with:
    mode: server
    bootstrap_expect: "3"
    bind_addr: "{{ ansible_default_ipv4.address }}"
    client_addr: "0.0.0.0"
    datacenter: prod
    start_service: true
  become: true

# Server 2 & 3: Same configuration

# Join cluster from server 2 & 3
- name: Join cluster
  shell: consul join <server1-ip>
  become: true
```

### Client Nodes

```yaml
- name: Install Consul client
  preset: consul
  with:
    mode: client
    bind_addr: "{{ ansible_default_ipv4.address }}"
    datacenter: prod
    start_service: true
  become: true

- name: Join cluster
  shell: consul join <server-ip>
```

## Service Discovery

### Register Service

#### Via API

```bash
# Register web service
curl -X PUT http://localhost:8500/v1/agent/service/register -d '{
  "ID": "web-1",
  "Name": "web",
  "Tags": ["v1", "production"],
  "Address": "192.168.1.10",
  "Port": 8080,
  "Check": {
    "HTTP": "http://192.168.1.10:8080/health",
    "Interval": "10s",
    "Timeout": "1s"
  }
}'
```

#### Via Configuration File

Create `/etc/consul.d/web.json`:

```json
{
  "service": {
    "name": "web",
    "tags": ["production"],
    "port": 8080,
    "check": {
      "http": "http://localhost:8080/health",
      "interval": "10s"
    }
  }
}
```

```bash
# Reload Consul
consul reload
```

#### Via CLI

```bash
# Register service
consul services register -name=web -port=8080 -address=192.168.1.10

# With health check
consul services register \
  -name=web \
  -port=8080 \
  -check-http=http://localhost:8080/health \
  -check-interval=10s
```

### Discover Services

#### DNS Interface

```bash
# Query service
dig @127.0.0.1 -p 8600 web.service.consul

# Get all instances
dig @127.0.0.1 -p 8600 web.service.consul ANY

# Get specific tag
dig @127.0.0.1 -p 8600 production.web.service.consul
```

#### HTTP API

```bash
# List all services
curl http://localhost:8500/v1/catalog/services

# Get service instances
curl http://localhost:8500/v1/catalog/service/web

# Health check
curl http://localhost:8500/v1/health/service/web?passing
```

#### CLI

```bash
# List services
consul catalog services

# Get service nodes
consul catalog nodes -service=web

# Watch service
consul watch -type=service -service=web
```

## Key-Value Store

### Basic Operations

```bash
# Put key-value
consul kv put myapp/config/db_host localhost
consul kv put myapp/config/db_port 5432
consul kv put myapp/feature/newui true

# Get value
consul kv get myapp/config/db_host

# Get with metadata
consul kv get -detailed myapp/config/db_host

# List keys
consul kv get -recurse myapp/
consul kv get -keys myapp/config/

# Delete
consul kv delete myapp/config/db_host

# Delete recursively
consul kv delete -recurse myapp/
```

### Watch for Changes

```bash
# Watch single key
consul watch -type=key -key=myapp/config/db_host cat

# Watch prefix
consul watch -type=keyprefix -prefix=myapp/config/ \
  /usr/local/bin/reload-config.sh

# Watch in background
consul watch -type=key -key=feature/flag \
  'echo "Feature flag changed" | mail -s Alert admin@example.com' &
```

### Transactions

```bash
# Atomic operations
consul kv put myapp/counter 0

# Compare-and-set (CAS)
consul kv put -cas -modify-index=10 myapp/counter 1

# Transaction via API
curl -X PUT http://localhost:8500/v1/txn -d '[
  {
    "KV": {
      "Verb": "set",
      "Key": "myapp/config/host",
      "Value": "bG9jYWxob3N0"
    }
  },
  {
    "KV": {
      "Verb": "delete",
      "Key": "myapp/old_config"
    }
  }
]'
```

## Health Checks

### Script Checks

```json
{
  "check": {
    "id": "mem-check",
    "name": "Memory utilization",
    "args": ["/usr/local/bin/check_mem.sh"],
    "interval": "10s",
    "timeout": "1s"
  }
}
```

### HTTP Checks

```json
{
  "check": {
    "id": "web-health",
    "name": "Web API health",
    "http": "http://localhost:8080/health",
    "interval": "10s",
    "timeout": "1s"
  }
}
```

### TCP Checks

```json
{
  "check": {
    "id": "ssh-check",
    "name": "SSH service",
    "tcp": "localhost:22",
    "interval": "10s",
    "timeout": "1s"
  }
}
```

### gRPC Checks

```json
{
  "check": {
    "id": "grpc-check",
    "name": "gRPC service",
    "grpc": "localhost:50051",
    "grpc_use_tls": false,
    "interval": "10s"
  }
}
```

## Client Libraries

### Python (python-consul)

```python
import consul

# Connect
c = consul.Consul(host='localhost', port=8500)

# Service registration
c.agent.service.register(
    name='my-service',
    service_id='my-service-1',
    address='192.168.1.10',
    port=8080,
    tags=['v1', 'production'],
    check=consul.Check.http('http://192.168.1.10:8080/health', interval='10s')
)

# Service discovery
index, services = c.catalog.service('web')
for service in services:
    print(f"{service['ServiceName']} at {service['ServiceAddress']}:{service['ServicePort']}")

# KV operations
c.kv.put('config/db/host', 'localhost')
index, data = c.kv.get('config/db/host')
if data:
    print(data['Value'].decode('utf-8'))

# Watch for changes
index = None
while True:
    index, data = c.kv.get('config/feature_flag', index=index, wait='30s')
    if data:
        print(f"Config changed: {data['Value'].decode('utf-8')}")
```

### Node.js (consul)

```javascript
const Consul = require('consul');

const consul = new Consul({
  host: 'localhost',
  port: 8500
});

// Register service
await consul.agent.service.register({
  name: 'my-service',
  id: 'my-service-1',
  address: '192.168.1.10',
  port: 8080,
  tags: ['v1', 'production'],
  check: {
    http: 'http://192.168.1.10:8080/health',
    interval: '10s'
  }
});

// Discover services
const services = await consul.catalog.service.nodes('web');
services.forEach(service => {
  console.log(`${service.ServiceName} at ${service.ServiceAddress}:${service.ServicePort}`);
});

// KV operations
await consul.kv.set('config/db/host', 'localhost');
const result = await consul.kv.get('config/db/host');
console.log(result.Value);

// Watch
const watch = consul.watch({
  method: consul.kv.get,
  options: { key: 'config/feature_flag' }
});

watch.on('change', (data) => {
  console.log('Config changed:', data.Value);
});

watch.on('error', (err) => {
  console.error('Watch error:', err);
});
```

### Go (consul/api)

```go
package main

import (
    "fmt"
    "github.com/hashicorp/consul/api"
)

func main() {
    config := api.DefaultConfig()
    config.Address = "localhost:8500"
    client, _ := api.NewClient(config)

    // Register service
    registration := &api.AgentServiceRegistration{
        ID:      "my-service-1",
        Name:    "my-service",
        Address: "192.168.1.10",
        Port:    8080,
        Tags:    []string{"v1", "production"},
        Check: &api.AgentServiceCheck{
            HTTP:     "http://192.168.1.10:8080/health",
            Interval: "10s",
            Timeout:  "1s",
        },
    }
    client.Agent().ServiceRegister(registration)

    // Discover services
    services, _, _ := client.Catalog().Service("web", "", nil)
    for _, service := range services {
        fmt.Printf("%s at %s:%d\n",
            service.ServiceName, service.ServiceAddress, service.ServicePort)
    }

    // KV operations
    kv := client.KV()
    p := &api.KVPair{Key: "config/db/host", Value: []byte("localhost")}
    kv.Put(p, nil)

    pair, _, _ := kv.Get("config/db/host", nil)
    fmt.Println(string(pair.Value))
}
```

## Service Mesh (Consul Connect)

### Enable Connect

```bash
# Enable Connect
consul connect enable

# Or in config
{
  "connect": {
    "enabled": true
  }
}
```

### Register Service with Sidecar

```json
{
  "service": {
    "name": "web",
    "port": 8080,
    "connect": {
      "sidecar_service": {
        "port": 20000
      }
    }
  }
}
```

### Start Sidecar Proxy

```bash
# Start proxy
consul connect proxy -sidecar-for web

# Or use Envoy
consul connect envoy -sidecar-for web
```

### Service Intentions (Access Control)

```bash
# Allow web to call database
consul intention create web database

# Deny api to call admin
consul intention create -deny api admin

# List intentions
consul intention list

# Check access
consul intention check web database
```

## Configuration Management

### Consul Template

```bash
# Install consul-template
brew install consul-template

# Template file: nginx.conf.tpl
upstream backend {
{{- range service "web" }}
  server {{ .Address }}:{{ .Port }};
{{- end }}
}

# Run consul-template
consul-template \
  -template "nginx.conf.tpl:nginx.conf:nginx -s reload" \
  -consul-addr localhost:8500
```

### Environment Variables from KV

```bash
#!/bin/bash
# Load config from Consul

DB_HOST=$(consul kv get config/db/host)
DB_PORT=$(consul kv get config/db/port)
API_KEY=$(consul kv get config/api/key)

export DB_HOST DB_PORT API_KEY

# Start application
./myapp
```

## Multi-Datacenter

### WAN Join

```bash
# Join datacenters via WAN gossip
consul join -wan <dc2-server-ip>

# View WAN members
consul members -wan
```

### Cross-DC Queries

```bash
# Query remote datacenter
consul catalog services -datacenter=dc2

# KV access
consul kv put -datacenter=dc2 key value
consul kv get -datacenter=dc2 key

# Service discovery
dig @127.0.0.1 -p 8600 web.service.dc2.consul
```

### Prepared Queries (Failover)

```bash
# Create prepared query with failover
curl -X POST http://localhost:8500/v1/query -d '{
  "Name": "web-failover",
  "Service": {
    "Service": "web",
    "Failover": {
      "Datacenters": ["dc2", "dc3"]
    }
  }
}'
```

## Security (ACL)

### Bootstrap ACL

```bash
# Bootstrap ACL system
consul acl bootstrap

# Output:
# AccessorID: xxx
# SecretID: xxx (use as token)
```

### Create Policy

```hcl
# app-policy.hcl
service "web" {
  policy = "write"
}

key_prefix "config/app/" {
  policy = "read"
}

session_prefix "" {
  policy = "write"
}
```

```bash
# Create policy
consul acl policy create \
  -name app-policy \
  -rules @app-policy.hcl
```

### Create Token

```bash
# Create token with policy
consul acl token create \
  -description "App token" \
  -policy-name app-policy

# Use token
export CONSUL_HTTP_TOKEN=<token>
consul kv get config/app/db_host
```

## Configuration Files

### Server Configuration

`/etc/consul.d/consul.hcl`:

```hcl
datacenter = "dc1"
data_dir = "/opt/consul/data"
log_level = "INFO"

server = true
bootstrap_expect = 3

bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"

ui_config {
  enabled = true
}

# Performance
performance {
  raft_multiplier = 1
}

# Ports
ports {
  http = 8500
  https = 8501
  dns = 8600
  server = 8300
  serf_lan = 8301
  serf_wan = 8302
  grpc = 8502
}
```

### Client Configuration

```hcl
datacenter = "dc1"
data_dir = "/opt/consul/data"

server = false

bind_addr = "{{ GetInterfaceIP \"eth0\" }}"
client_addr = "127.0.0.1"

retry_join = ["10.0.1.10", "10.0.1.11", "10.0.1.12"]
```

## Service Management

```bash
# systemd (Linux)
sudo systemctl start consul
sudo systemctl stop consul
sudo systemctl restart consul
sudo systemctl status consul

# View logs
sudo journalctl -u consul -f

# Reload configuration
consul reload
sudo systemctl reload consul
```

## CLI Commands

```bash
# Cluster
consul members          # List members
consul join <ip>        # Join cluster
consul leave           # Gracefully leave
consul operator raft list-peers  # Raft peers

# Services
consul catalog services  # List services
consul catalog nodes -service=web  # Service nodes
consul services register service.json  # Register
consul services deregister -id=web-1   # Deregister

# KV Store
consul kv put key value  # Write
consul kv get key       # Read
consul kv delete key    # Delete
consul kv export > backup.json  # Backup
consul kv import @backup.json   # Restore

# Health
consul catalog health service-name
consul monitor  # Stream logs
```

## Troubleshooting

### Cluster Not Forming

```bash
# Check members
consul members

# Check logs
sudo journalctl -u consul -n 100

# Verify ports are open
netstat -tuln | grep -E "8300|8301|8500"

# Manual join
consul join <server-ip>
```

### Service Not Discoverable

```bash
# Check registration
consul catalog services
consul catalog service web

# Check health
consul catalog health web

# Re-register service
consul services deregister -id=web-1
consul services register web-service.json
```

### DNS Not Resolving

```bash
# Test DNS
dig @127.0.0.1 -p 8600 consul.service.consul
dig @127.0.0.1 -p 8600 web.service.consul

# Check DNS config in /etc/resolv.conf
# Add: nameserver 127.0.0.1

# Test resolution
nslookup web.service.consul 127.0.0.1
```

## Uninstall

```yaml
- preset: consul
  with:
    state: absent
```

**Note**: Data directory preserved. Remove manually:
```bash
sudo rm -rf /opt/consul/data
```

## Resources

- **Official Site**: https://www.consul.io/
- **Docs**: https://www.consul.io/docs
- **Tutorials**: https://learn.hashicorp.com/consul
- **API Reference**: https://www.consul.io/api-docs
- **GitHub**: https://github.com/hashicorp/consul

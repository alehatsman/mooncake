# Quickwit - Cloud-Native Search Engine

Quickwit is a distributed search engine designed for log management and analytics at scale.

## Quick Start

```yaml
- preset: quickwit
```

## Features

- **Sub-second search**: Fast full-text search on petabyte-scale data
- **Cost-effective**: Built for cloud object storage (S3, GCS)
- **Jaeger integration**: Native support for distributed tracing
- **OpenTelemetry**: First-class OTLP support
- **Schemaless**: Flexible indexing without upfront schema
- **Distributed**: Horizontal scaling for indexing and search

## Basic Usage

```bash
# Check version
quickwit --version

# Start server
quickwit run

# Create an index
quickwit index create --index-config my-index.yaml

# Ingest data
cat data.ndjson | quickwit index ingest --index my-index

# Search
quickwit index search --index my-index --query "error"

# Check cluster status
quickwit cluster status
```

## Advanced Configuration

```yaml
# Simple installation
- preset: quickwit

# Remove installation
- preset: quickwit
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Config file**: `~/.quickwit/` (user), `/etc/quickwit/` (system)
- **Data directory**: Configurable via `data_dir` in config
- **Default port**: 7280 (HTTP REST API)

## Real-World Examples

### Log Search System

```bash
# Start Quickwit server
quickwit run &

# Create logs index
cat > logs-index.yaml << EOF
version: 0.7
index_id: logs
doc_mapping:
  field_mappings:
    - name: timestamp
      type: datetime
      input_formats: [unix_timestamp]
    - name: level
      type: text
    - name: message
      type: text
EOF

quickwit index create --index-config logs-index.yaml

# Ingest logs
tail -f /var/log/app.log | quickwit index ingest --index logs
```

### Jaeger Tracing Backend

```bash
# Start Quickwit with Jaeger support
QUICKWIT_ENABLE_JAEGER_ENDPOINT=true quickwit run

# Configure Jaeger agent to use Quickwit
export JAEGER_AGENT_HOST=localhost
export JAEGER_AGENT_PORT=7281

# Search traces
quickwit index search --index otel-traces --query "service_name:my-app"
```

### OpenTelemetry Integration

```yaml
# OpenTelemetry Collector configuration
exporters:
  otlp/quickwit:
    endpoint: http://localhost:7281
    tls:
      insecure: true

service:
  pipelines:
    traces:
      exporters: [otlp/quickwit]
    logs:
      exporters: [otlp/quickwit]
```

## Agent Use

- Ingest and search application logs at scale
- Store and query distributed traces
- Analyze time-series data from monitoring systems
- Build custom search interfaces for operational data
- Archive and search historical event data

## Troubleshooting

### Port already in use

```bash
# Check what's using port 7280
lsof -i :7280

# Start on different port
quickwit run --rest-listen-port 8080
```

### Index not found

```bash
# List all indexes
quickwit index list

# Verify index exists
quickwit index describe --index my-index
```

## Uninstall

```yaml
- preset: quickwit
  with:
    state: absent
```

## Resources

- Official docs: https://quickwit.io/docs
- GitHub: https://github.com/quickwit-oss/quickwit
- Search: "quickwit tutorial", "quickwit vs elasticsearch", "quickwit log management"

# Uptrace - Distributed Tracing and Metrics

Open-source APM (Application Performance Monitoring) for distributed systems. Visualize traces, analyze metrics, and debug performance issues.

## Quick Start

```yaml
- preset: uptrace
```

## Features

- **Distributed Tracing**: OpenTelemetry-compatible tracing for microservices
- **Metrics Collection**: Prometheus-compatible metrics with custom dashboards
- **Error Tracking**: Exception monitoring and aggregation
- **Performance Profiling**: Identify slow queries and bottlenecks
- **Service Map**: Visualize service dependencies
- **Anomaly Detection**: Automated performance regression detection
- **Multi-Tenant**: Isolated projects for different teams
- **Cross-platform**: Linux and macOS support

## Basic Usage

```bash
# Start Uptrace server
uptrace serve

# Check version
uptrace version

# Run migrations
uptrace migrate

# Create project
uptrace project create myapp

# Generate API key
uptrace token create
```

## Advanced Configuration

### Basic Installation
```yaml
- preset: uptrace
```

### With Custom Configuration
```yaml
- preset: uptrace
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove uptrace |

## Platform Support

- ✅ Linux (binary, Docker)
- ✅ macOS (binary, Docker)
- ❌ Windows (use Docker)

## Configuration

- **Config file**: `uptrace.yml` (server configuration)
- **Data directory**: `/var/lib/uptrace/` (traces and metrics storage)
- **Web UI**: `http://localhost:14318` (default)
- **API endpoint**: `http://localhost:14317` (OpenTelemetry receiver)
- **Database**: PostgreSQL (required) + ClickHouse (recommended)

## Real-World Examples

### Docker Compose Setup
```yaml
# docker-compose.yml
version: '3.8'

services:
  uptrace:
    image: uptrace/uptrace:latest
    restart: always
    ports:
      - "14317:14317"  # OTLP gRPC
      - "14318:14318"  # HTTP UI
    environment:
      - UPTRACE_DSN=postgres://user:pass@postgres:5432/uptrace?sslmode=disable
      - UPTRACE_CH_DSN=clickhouse://clickhouse:9000/uptrace
    volumes:
      - ./uptrace.yml:/etc/uptrace/uptrace.yml
    depends_on:
      - postgres
      - clickhouse

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: uptrace
      POSTGRES_USER: uptrace
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres-data:/var/lib/postgresql/data

  clickhouse:
    image: clickhouse/clickhouse-server:latest
    environment:
      CLICKHOUSE_DB: uptrace
    volumes:
      - clickhouse-data:/var/lib/clickhouse

volumes:
  postgres-data:
  clickhouse-data:
```

### Application Integration (Go)
```go
import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer() (*trace.TracerProvider, error) {
    ctx := context.Background()

    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint("localhost:14317"),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
    )
    otel.SetTracerProvider(tp)

    return tp, nil
}
```

### Application Integration (Node.js)
```javascript
const { NodeTracerProvider } = require('@opentelemetry/sdk-trace-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { BatchSpanProcessor } = require('@opentelemetry/sdk-trace-base');

const provider = new NodeTracerProvider();

const exporter = new OTLPTraceExporter({
  url: 'http://localhost:14317',
});

provider.addSpanProcessor(new BatchSpanProcessor(exporter));
provider.register();
```

### Application Integration (Python)
```python
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

trace.set_tracer_provider(TracerProvider())
tracer = trace.get_tracer(__name__)

otlp_exporter = OTLPSpanExporter(
    endpoint="http://localhost:14317",
    insecure=True,
)

span_processor = BatchSpanProcessor(otlp_exporter)
trace.get_tracer_provider().add_span_processor(span_processor)
```

## Configuration File

### uptrace.yml
```yaml
##
## Uptrace configuration
##

# Project configuration
projects:
  - id: 1
    name: myapp
    token: secret_token_here
    pinned_attrs:
      - service.name
      - host.name
      - deployment.environment

# Server configuration
listen:
  http: :14318
  grpc: :14317

# PostgreSQL (required for metadata)
ch:
  dsn: postgres://uptrace:secret@localhost:5432/uptrace?sslmode=disable

# ClickHouse (recommended for traces/metrics)
ch_cluster:
  dsn: clickhouse://localhost:9000/uptrace

# Metrics
metrics:
  drop_attrs:
    - host.id
    - process.pid

# Tracing
spans:
  max_events: 5

# Logging
logging:
  level: info

# Security
auth:
  users:
    - username: admin
      password: secret
      projects: [1]
```

## Dashboard Examples

### Service Performance
```yaml
# View all services
http://localhost:14318/overview

# Service details
http://localhost:14318/services/api

# Trace timeline
http://localhost:14318/traces

# Error tracking
http://localhost:14318/errors
```

### Custom Dashboards
```yaml
# CPU usage by service
metrics:
  query: |
    avg(system.cpu.utilization) by (service.name)

# Request rate
metrics:
  query: |
    rate(http.server.request.count[5m])

# Error rate
metrics:
  query: |
    rate(http.server.errors[5m])
```

## Monitoring Patterns

### Microservices Tracing
```bash
# Trace distributed request across services
# 1. Frontend receives request
# 2. Calls API gateway
# 3. Gateway calls auth service
# 4. Auth validates token
# 5. Gateway calls backend
# 6. Backend queries database
# 7. Response flows back

# All steps visible in single trace view
```

### Performance Analysis
```bash
# Identify slow endpoints
# Sort by p99 latency
# View span timeline
# Find bottlenecks (DB queries, external APIs)
# Compare before/after optimization
```

## CI/CD Integration

### Performance Regression Detection
```bash
#!/bin/bash
# check-performance.sh

# Get baseline metrics
BASELINE_P95=$(uptrace query --project=1 --metric=latency_p95 --last=7d)

# Current metrics
CURRENT_P95=$(uptrace query --project=1 --metric=latency_p95 --last=1h)

# Compare
if (( $(echo "$CURRENT_P95 > $BASELINE_P95 * 1.2" | bc -l) )); then
  echo "Performance regression detected: ${CURRENT_P95}ms vs ${BASELINE_P95}ms"
  exit 1
fi
```

### Automated Alerting
```yaml
# Alert on high error rate
alerts:
  - name: high_error_rate
    condition: |
      rate(http.server.errors[5m]) > 0.05
    notify:
      - slack
      - email

  - name: slow_response_time
    condition: |
      p99(http.server.duration) > 1000ms
    notify:
      - pagerduty
```

## Troubleshooting

### No traces appearing
```bash
# Check Uptrace is running
curl http://localhost:14318/health

# Verify OTLP endpoint
curl http://localhost:14317

# Check application configuration
# Ensure endpoint URL is correct
# Verify no firewall blocking
```

### Database connection issues
```bash
# Test PostgreSQL connection
psql -h localhost -U uptrace -d uptrace

# Test ClickHouse connection
clickhouse-client --host localhost --database uptrace

# Check credentials in uptrace.yml
```

### High memory usage
```yaml
# Adjust retention in uptrace.yml
spans:
  retention: 7d  # Reduce from default

metrics:
  retention: 30d  # Reduce from default

# Limit trace size
spans:
  max_events: 5
  max_attrs: 50
```

## Best Practices

- **Instrument key paths** (not everything)
- **Add custom attributes** for business context
- **Set sampling rate** to control costs (100% in dev, 10% in prod)
- **Use semantic conventions** (OpenTelemetry standard attributes)
- **Create custom dashboards** for your specific needs
- **Set up alerts** for critical metrics
- **Regular data retention** policies
- **Monitor Uptrace itself** (resource usage)

## Comparison with Alternatives

| Feature | Uptrace | Jaeger | Zipkin | DataDog |
|---------|---------|--------|--------|---------|
| Open Source | Yes | Yes | Yes | No |
| Metrics | Yes | No | No | Yes |
| Logs | No | No | No | Yes |
| Self-hosted | Yes | Yes | Yes | No |
| Cloud option | No | No | No | Yes |
| Storage | PG+CH | Various | Various | Cloud |
| UI | Modern | Basic | Basic | Advanced |
| Cost | Free | Free | Free | Paid |

## Agent Use

- Distributed tracing for microservices
- Performance monitoring and optimization
- Error tracking and debugging
- Service dependency mapping
- Anomaly detection automation
- SLA monitoring and reporting
- Capacity planning analysis
- Production debugging without logs

## Uninstall

```yaml
- preset: uptrace
  with:
    state: absent
```

## Resources

- Official site: https://uptrace.dev/
- GitHub: https://github.com/uptrace/uptrace
- Documentation: https://uptrace.dev/get/
- OpenTelemetry: https://opentelemetry.io/
- Community: https://github.com/uptrace/uptrace/discussions
- Search: "uptrace opentelemetry", "uptrace tutorial", "distributed tracing"

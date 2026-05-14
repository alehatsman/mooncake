# SigNoz - Open-Source Observability Platform

Full-stack observability platform with distributed tracing, metrics, and logs in a single application. Built on OpenTelemetry, provides APM, infrastructure monitoring, and log analytics. Open-source alternative to Datadog, New Relic, and Honeycomb.

## Quick Start
```yaml
- preset: signoz
```

Install SigNoz CLI for deployment management.

**Docker Compose (quickest):**
```bash
git clone https://github.com/SigNoz/signoz.git
cd signoz/deploy
./install.sh
```

Access UI: `http://localhost:3301`

## Features
- **Distributed tracing**: End-to-end request tracing with flame graphs
- **Metrics monitoring**: Infrastructure and application metrics
- **Logs management**: Centralized log aggregation and analysis
- **OpenTelemetry native**: Built on OTel collector and SDKs
- **Service maps**: Visualize dependencies and service topology
- **Alerts**: Configure alerts on metrics, traces, and logs
- **Dashboards**: Custom dashboards with PromQL support
- **Query builder**: No-code query interface for traces and logs
- **Anomaly detection**: Automatic detection of performance issues
- **Single pane of glass**: Unified view of all telemetry data

## Basic Usage
```bash
# Deploy with Docker Compose
cd signoz/deploy
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f

# Stop SigNoz
docker compose down

# Update SigNoz
./upgrade.sh
```

## Architecture

### Components
```
┌──────────────────────────────────────────────────────┐
│                   SigNoz Platform                    │
│                                                       │
│  ┌────────────────────────────────────────────────┐ │
│  │         Application Instrumentation            │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐      │ │
│  │  │ Go SDK   │ │Python SDK│ │ Node.js  │ ...  │ │
│  │  └────┬─────┘ └────┬─────┘ └────┬─────┘      │ │
│  │       │            │            │             │ │
│  │       └────────────┼────────────┘             │ │
│  │                    │                          │ │
│  │       ┌────────────▼──────────────┐           │ │
│  │       │   OTel Collector          │           │ │
│  │       │   (Receive, Process)      │           │ │
│  │       └────────────┬──────────────┘           │ │
│  │                    │                          │ │
│  │       ┌────────────▼──────────────┐           │ │
│  │       │   ClickHouse Database     │           │ │
│  │       │   (Traces, Metrics, Logs) │           │ │
│  │       └────────────┬──────────────┘           │ │
│  │                    │                          │ │
│  │       ┌────────────▼──────────────┐           │ │
│  │       │   Query Service           │           │ │
│  │       │   (API Backend)           │           │ │
│  │       └────────────┬──────────────┘           │ │
│  │                    │                          │ │
│  │       ┌────────────▼──────────────┐           │ │
│  │       │   Frontend (React)        │           │ │
│  │       │   (UI Dashboard)          │           │ │
│  │       └───────────────────────────┘           │ │
│  └────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

### Data Flow
1. **Instrumentation**: Application sends telemetry to OTel Collector
2. **Collection**: OTel Collector receives and processes data
3. **Storage**: Data stored in ClickHouse (columnar database)
4. **Query**: Query service retrieves data via API
5. **Visualization**: Frontend displays traces, metrics, logs

## Advanced Configuration

### Docker Compose deployment
```yaml
- name: Clone SigNoz repository
  shell: |
    git clone https://github.com/SigNoz/signoz.git
    cd signoz/deploy

- name: Configure environment
  template:
    src: signoz-env.j2
    dest: signoz/deploy/.env
  vars:
    clickhouse_password: "{{ signoz_db_password }}"
    admin_password: "{{ signoz_admin_password }}"

- name: Deploy SigNoz
  shell: |
    cd signoz/deploy
    docker compose -f docker-compose.yaml up -d

- name: Wait for SigNoz
  shell: |
    for i in {1..60}; do
      curl -f http://localhost:3301 && break
      sleep 5
    done
```

### Kubernetes deployment (Helm)
```yaml
- name: Add SigNoz Helm repository
  shell: |
    helm repo add signoz https://charts.signoz.io
    helm repo update

- name: Create namespace
  shell: kubectl create namespace signoz

- name: Install SigNoz
  shell: |
    helm install signoz signoz/signoz \
      --namespace signoz \
      --set frontend.service.type=LoadBalancer \
      --set clickhouse.persistence.size=100Gi

- name: Get LoadBalancer IP
  shell: kubectl get svc -n signoz signoz-frontend -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
  register: signoz_ip
```

### Custom OTel Collector configuration
```yaml
- name: Create OTel Collector config
  template:
    src: otel-collector-config.yml.j2
    dest: /etc/signoz/otel-collector-config.yml
  vars:
    clickhouse_endpoint: "tcp://clickhouse:9000"
    enable_debug: false

# otel-collector-config.yml.j2
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 1s
    send_batch_size: 1024

  memory_limiter:
    check_interval: 1s
    limit_mib: 512

exporters:
  clickhousetraces:
    endpoint: {{ clickhouse_endpoint }}
    username: default
    password: {{ clickhouse_password }}

  clickhousemetricswrite:
    endpoint: {{ clickhouse_endpoint }}
    username: default
    password: {{ clickhouse_password }}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [clickhousetraces]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [clickhousemetricswrite]
```

## Application Instrumentation

### Go application
```yaml
- name: Install OpenTelemetry SDK
  shell: |
    go get go.opentelemetry.io/otel
    go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
    go get go.opentelemetry.io/otel/sdk

# main.go
package main

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func initTracer() (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(context.Background(),
        otlptracegrpc.WithEndpoint("localhost:4317"),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("my-service"),
        )),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}

func main() {
    tp, err := initTracer()
    if err != nil {
        panic(err)
    }
    defer tp.Shutdown(context.Background())

    // Application code with tracing
    tracer := otel.Tracer("my-service")
    ctx, span := tracer.Start(context.Background(), "my-operation")
    defer span.End()

    // Do work...
}
```

### Python application
```yaml
- name: Install OpenTelemetry SDK
  shell: |
    pip install opentelemetry-api opentelemetry-sdk
    pip install opentelemetry-exporter-otlp

# app.py
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource

# Configure tracer
resource = Resource(attributes={"service.name": "my-python-service"})
provider = TracerProvider(resource=resource)
processor = BatchSpanProcessor(OTLPSpanExporter(endpoint="http://localhost:4317"))
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)

tracer = trace.get_tracer(__name__)

# Use in code
with tracer.start_as_current_span("my-operation"):
    # Do work...
    pass
```

### Node.js application
```yaml
- name: Install OpenTelemetry SDK
  shell: |
    npm install @opentelemetry/sdk-node
    npm install @opentelemetry/auto-instrumentations-node
    npm install @opentelemetry/exporter-trace-otlp-grpc

# tracing.js
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter({
    url: 'http://localhost:4317',
  }),
  instrumentations: [getNodeAutoInstrumentations()],
  serviceName: 'my-nodejs-service',
});

sdk.start();

// app.js
require('./tracing');
const express = require('express');
const app = express();

app.get('/', (req, res) => {
  res.send('Hello World');
});
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove SigNoz CLI |

## Platform Support
- ✅ Linux (Docker, Kubernetes)
- ✅ macOS (Docker Desktop)
- ✅ Cloud (AWS, GCP, Azure via Kubernetes)
- ⚠️ Windows (Docker Desktop, WSL2 recommended)

## Configuration

### Environment variables
```bash
# ClickHouse configuration
CLICKHOUSE_HOST=clickhouse
CLICKHOUSE_PORT=9000
CLICKHOUSE_USER=default
CLICKHOUSE_PASSWORD=password

# Query service
QUERY_SERVICE_PORT=8080
ALERTMANAGER_API_PREFIX=http://alertmanager:9093

# Frontend
FRONTEND_PORT=3301
```

### Data retention
```yaml
# docker-compose.yaml or Helm values
clickhouse:
  ttl:
    traces: 7d      # Keep traces for 7 days
    metrics: 30d    # Keep metrics for 30 days
    logs: 7d        # Keep logs for 7 days
```

## Dashboards and Queries

### Service metrics
```yaml
- name: Access service overview
  shell: curl http://localhost:3301/api/v1/services

# View in UI:
# - Request rate
# - Error rate
# - P99 latency
# - Apdex score
```

### Custom dashboard
```yaml
# Create dashboard via UI or API
- name: Create custom dashboard
  shell: |
    curl -X POST http://localhost:3301/api/v1/dashboards \
      -H "Content-Type: application/json" \
      -d '{
        "name": "My Dashboard",
        "widgets": [
          {
            "title": "Request Rate",
            "query": "sum(rate(http_requests_total[5m]))"
          }
        ]
      }'
```

## Alerts

### Configure alerts
```yaml
- name: Create alert rule
  shell: |
    curl -X POST http://localhost:3301/api/v1/rules \
      -H "Content-Type: application/json" \
      -d '{
        "name": "High Error Rate",
        "query": "sum(rate(http_requests_total{status=\"500\"}[5m])) > 10",
        "condition": {
          "threshold": 10,
          "compareOp": "gt"
        },
        "labels": {
          "severity": "critical"
        },
        "annotations": {
          "description": "Error rate is above threshold"
        }
      }'
```

### Alert channels
```yaml
# Configure Slack notification
- name: Add Slack channel
  shell: |
    curl -X POST http://localhost:3301/api/v1/channels \
      -H "Content-Type: application/json" \
      -d '{
        "name": "slack-alerts",
        "type": "slack",
        "config": {
          "webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
        }
      }'
```

## Use Cases

### Microservices observability
```yaml
- name: Deploy SigNoz
  preset: signoz

- name: Install SigNoz via Helm
  shell: |
    helm install signoz signoz/signoz -n signoz

- name: Instrument services
  template:
    src: otel-config.j2
    dest: /etc/app/otel-config.yml
  vars:
    otel_endpoint: "http://signoz-otel-collector:4317"
    service_name: "{{ app_name }}"

- name: Deploy instrumented applications
  shell: kubectl apply -f manifests/
```

### Log aggregation
```yaml
- name: Configure log collection
  template:
    src: otel-collector-logs.yml.j2
    dest: /etc/signoz/otel-collector-config.yml

# Send logs to SigNoz
- name: Configure application logging
  template:
    src: logging-config.j2
    dest: /etc/app/logging.yml
  vars:
    log_format: json
    log_output: otlp
    otlp_endpoint: "http://localhost:4318/v1/logs"
```

### APM for web applications
```yaml
- name: Auto-instrument Node.js app
  shell: |
    export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317"
    export OTEL_SERVICE_NAME="web-app"
    node --require @opentelemetry/auto-instrumentations-node/register app.js

- name: View traces in SigNoz
  shell: |
    # Access UI at http://localhost:3301
    # Navigate to Services > web-app > Traces
```

## CLI Commands

### Docker management
```bash
# Start SigNoz
cd signoz/deploy && docker compose up -d

# Stop SigNoz
docker compose down

# Restart services
docker compose restart

# View logs
docker compose logs -f query-service
docker compose logs -f otel-collector

# Update SigNoz
./upgrade.sh
```

### Query API
```bash
# Get services
curl http://localhost:3301/api/v1/services

# Get traces
curl 'http://localhost:3301/api/v1/traces?service=my-service&start=1640000000&end=1640100000'

# Get metrics
curl 'http://localhost:3301/api/v1/query_range?query=http_requests_total&start=1640000000&end=1640100000&step=60'

# Get logs
curl 'http://localhost:3301/api/v1/logs/tail?follow=true&limit=100'
```

## Monitoring

### SigNoz metrics
```bash
# Query service health
curl http://localhost:8080/api/v1/health

# OTel collector metrics
curl http://localhost:8888/metrics

# ClickHouse metrics
curl http://localhost:9363/metrics
```

### Prometheus integration
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'signoz-query-service'
    static_configs:
      - targets: ['localhost:8080']

  - job_name: 'signoz-otel-collector'
    static_configs:
      - targets: ['localhost:8888']
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install SigNoz CLI
  preset: signoz
```

### Docker Compose deployment
```yaml
- name: Install SigNoz
  preset: signoz

- name: Clone SigNoz repository
  shell: |
    git clone https://github.com/SigNoz/signoz.git /opt/signoz

- name: Deploy SigNoz
  shell: |
    cd /opt/signoz/deploy
    docker compose up -d

- name: Wait for SigNoz
  shell: |
    for i in {1..60}; do
      curl -f http://localhost:3301 && break
      sleep 5
    done

- name: Display access information
  shell: |
    echo "SigNoz UI: http://localhost:3301"
    echo "OTel Collector: http://localhost:4317 (gRPC)"
    echo "OTel Collector: http://localhost:4318 (HTTP)"
```

### Kubernetes production deployment
```yaml
- name: Install SigNoz CLI
  preset: signoz

- name: Add Helm repository
  shell: |
    helm repo add signoz https://charts.signoz.io
    helm repo update

- name: Create namespace
  shell: kubectl create namespace signoz

- name: Deploy SigNoz
  shell: |
    helm install signoz signoz/signoz \
      --namespace signoz \
      --set clickhouse.persistence.size=200Gi \
      --set frontend.service.type=LoadBalancer \
      --set alertmanager.enabled=true

- name: Get LoadBalancer URL
  shell: kubectl get svc -n signoz signoz-frontend
  register: signoz_svc
```

## Agent Use
- **Application monitoring**: Distributed tracing for microservices
- **Infrastructure monitoring**: Host and container metrics
- **Log management**: Centralized log aggregation and search
- **Performance debugging**: Identify bottlenecks with flame graphs
- **Service dependency mapping**: Visualize service topology
- **Alerting**: Configure alerts on metrics, traces, and logs
- **Cost optimization**: Self-hosted alternative to commercial APM tools

## Troubleshooting

### SigNoz not accessible
```bash
# Check containers
docker compose ps

# Check logs
docker compose logs query-service
docker compose logs frontend

# Restart services
docker compose restart

# Check ports
netstat -tuln | grep -E "3301|4317|4318"
```

### No traces appearing
```bash
# Check OTel Collector
docker compose logs otel-collector

# Test OTLP endpoint
curl -i http://localhost:4318/v1/traces

# Verify application configuration
echo $OTEL_EXPORTER_OTLP_ENDPOINT
echo $OTEL_SERVICE_NAME

# Check ClickHouse
docker compose exec clickhouse clickhouse-client -q "SELECT count() FROM signoz_traces.distributed_signoz_index_v2"
```

### High memory usage
```bash
# Check ClickHouse memory
docker stats clickhouse

# Reduce ClickHouse memory limit
# docker-compose.yaml
clickhouse:
  deploy:
    resources:
      limits:
        memory: 4G

# Reduce retention
# Update TTL settings in ClickHouse tables
```

### Query performance issues
```bash
# Check ClickHouse query log
docker compose exec clickhouse clickhouse-client -q "SELECT query, query_duration_ms FROM system.query_log ORDER BY query_duration_ms DESC LIMIT 10"

# Optimize tables
docker compose exec clickhouse clickhouse-client -q "OPTIMIZE TABLE signoz_traces.signoz_index_v2 FINAL"

# Add more ClickHouse replicas (Kubernetes)
helm upgrade signoz signoz/signoz --set clickhouse.replicaCount=3
```

## Best Practices

1. **Use automatic instrumentation** when available for faster setup
2. **Set appropriate retention** to balance storage costs and data availability
3. **Configure sampling** for high-traffic applications (1-10% trace sampling)
4. **Use consistent service names** across all telemetry signals
5. **Add custom attributes** for business context (user_id, tenant_id, etc.)
6. **Set up alerts** for critical service metrics (error rate, latency)
7. **Use dashboards** to visualize service dependencies
8. **Monitor SigNoz itself** with Prometheus/Grafana
9. **Regular backups** of ClickHouse data for production
10. **Resource planning**: 4GB RAM + 20GB disk per service minimum

## Backup and Restore

### Backup ClickHouse data
```yaml
- name: Create ClickHouse backup
  shell: |
    docker compose exec clickhouse clickhouse-client -q "BACKUP DATABASE signoz_traces TO Disk('backups', 'backup-$(date +%Y%m%d).zip')"

- name: Copy backup to remote storage
  shell: |
    rsync -av /var/lib/clickhouse/disks/backups/ \
      s3://my-bucket/signoz-backups/
```

### Restore from backup
```yaml
- name: Stop SigNoz
  shell: docker compose down

- name: Restore ClickHouse data
  shell: |
    docker compose exec clickhouse clickhouse-client -q "RESTORE DATABASE signoz_traces FROM Disk('backups', 'backup-20240101.zip')"

- name: Start SigNoz
  shell: docker compose up -d
```

## Uninstall
```yaml
# Docker Compose
- name: Stop and remove SigNoz
  shell: |
    cd signoz/deploy
    docker compose down -v

- name: Remove data volumes
  shell: docker volume prune -f

# Kubernetes
- name: Uninstall SigNoz
  shell: helm uninstall signoz -n signoz

- name: Delete namespace
  shell: kubectl delete namespace signoz

# CLI
- name: Remove SigNoz CLI
  preset: signoz
  with:
    state: absent
```

**Note**: Uninstalling with `-v` flag removes all data. Omit for data preservation.

## Resources
- Official: https://signoz.io/
- Documentation: https://signoz.io/docs/
- GitHub: https://github.com/SigNoz/signoz
- Community: https://signoz.io/slack
- Blog: https://signoz.io/blog/
- Tutorials: https://signoz.io/docs/tutorial/
- OpenTelemetry: https://opentelemetry.io/
- ClickHouse: https://clickhouse.com/docs
- Search: "signoz vs datadog", "signoz opentelemetry", "signoz kubernetes deployment"

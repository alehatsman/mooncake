# vegeta - HTTP Load Testing

HTTP load testing tool with constant request rate. Measure latency, throughput, and success rates under load.

## Features
- **Constant rate**: Precise request rate control
- **Rich metrics**: Latencies, throughput, success rates
- **Multiple targets**: Load test multiple endpoints
- **Flexible output**: Text, JSON, histogram, binary
- **Visualization**: Generate HTML plots
- **Custom headers**: Authentication, custom headers
- **HTTP/2**: Support for HTTP/2 protocol
- **Keep-alive**: Connection reuse control

## Quick Start
```yaml
- preset: vegeta
```

## Basic Usage
```bash
# Simple GET test
echo "GET http://localhost:8080" | vegeta attack -duration=30s -rate=100 | vegeta report

# POST request
echo "POST http://localhost:8080/api
Content-Type: application/json
@data.json" | vegeta attack -duration=10s | vegeta report

# Multiple targets
cat targets.txt | vegeta attack -duration=60s -rate=50 | vegeta report
```

## Target File Format
```txt
# targets.txt
GET http://localhost:8080/

POST http://localhost:8080/api/users
Content-Type: application/json
{"name": "Alice"}

GET http://localhost:8080/health
X-Auth-Token: secret

DELETE http://localhost:8080/users/1
```

## Attack Options
```bash
# Rate: requests per second
vegeta attack -rate=100/s

# Duration
vegeta attack -duration=1m

# Connections: max open connections
vegeta attack -connections=10

# Timeout per request
vegeta attack -timeout=10s

# Workers: concurrent attackers
vegeta attack -workers=10

# HTTP2
vegeta attack -http2

# Keep-alive
vegeta attack -keepalive=false
```

## Output Formats
```bash
# Text report (default)
vegeta attack ... | vegeta report

# JSON report
vegeta attack ... | vegeta report -type=json

# Histogram
vegeta attack ... | vegeta report -type=hist[0,2ms,4ms,6ms]

# JSON output for plotting
vegeta attack ... | vegeta report -type=json > results.json

# Binary results (for later analysis)
vegeta attack ... -output=results.bin
vegeta report < results.bin
```

## Metrics and Reports
```bash
# Text report shows:
# - Requests: total count
# - Duration: test duration
# - Rate: actual req/s achieved
# - Throughput: successful req/s
# - Success: percentage
# - Latencies: min/mean/50th/95th/99th/max

# JSON report includes:
# - Latencies distribution
# - Bytes in/out
# - Status codes
# - Errors

# Example output:
Requests      [total, rate, throughput]  3000, 100.03, 99.95
Duration      [total, attack, wait]      30s, 29.99s, 10ms
Latencies     [min, mean, 50, 95, 99, max]  1ms, 5ms, 4ms, 12ms, 20ms, 50ms
Bytes In      [total, mean]              1500000, 500.00
Bytes Out     [total, mean]              90000, 30.00
Success       [ratio]                    99.90%
Status Codes  [code:count]               200:2997  500:3
Error Set:
500 Internal Server Error
```

## Load Testing Patterns
```bash
# Ramp up test
for rate in 10 50 100 200; do
  echo "Testing at $rate req/s"
  echo "GET http://localhost:8080" | \
    vegeta attack -rate=$rate -duration=30s | \
    vegeta report
done

# Sustained load
echo "GET http://localhost:8080" | \
  vegeta attack -rate=100 -duration=5m | \
  tee results.bin | vegeta report

# Spike test
echo "GET http://localhost:8080" | \
  vegeta attack -rate=0 -max-body=0 | \
  vegeta attack -rate=1000 -duration=10s | \
  vegeta report

# Stress test (find breaking point)
for rate in 100 200 500 1000 2000; do
  success=$(echo "GET http://localhost:8080" | \
    vegeta attack -rate=$rate -duration=30s | \
    vegeta report -type=json | jq -r '.success')
  echo "Rate: $rate, Success: $success"
  if (( $(echo "$success < 0.95" | bc -l) )); then
    echo "Breaking point found at $rate req/s"
    break
  fi
done
```

## Visualization
```bash
# Generate HTML plot
cat results.bin | vegeta plot > plot.html

# Custom plot
cat results.bin | vegeta plot -title="Load Test" > plot.html

# Time series
cat results.bin | vegeta report -type=json | \
  jq -r '.latencies.mean' > latencies.txt
```

## Advanced Scenarios
```bash
# Authentication
echo "GET http://api.example.com/data
Authorization: Bearer $TOKEN" | vegeta attack -duration=1m

# Random data
cat <<EOF | vegeta attack -duration=30s
POST http://localhost:8080/api
Content-Type: application/json
{
  "user": "user-@(seq 1000)",
  "timestamp": "@(date +%s)"
}
EOF

# Rate limiting test
echo "GET http://localhost:8080/api" | \
  vegeta attack -rate=1000 -duration=10s | \
  vegeta report -type=json | \
  jq '.status_codes'

# Connection pooling
echo "GET http://localhost:8080" | \
  vegeta attack -rate=100 -connections=1 -duration=30s | \
  vegeta report
```

## CI/CD Integration
```bash
# Performance regression test
#!/bin/bash
THRESHOLD_P95=100ms

results=$(echo "GET http://staging.example.com" | \
  vegeta attack -rate=50 -duration=1m | \
  vegeta report -type=json)

p95=$(echo $results | jq -r '.latencies."95th"')

if (( $(echo "$p95 > $THRESHOLD_P95" | bc -l) )); then
  echo "FAIL: p95 latency ${p95}ms exceeds ${THRESHOLD_P95}"
  exit 1
fi

echo "PASS: p95 latency ${p95}ms"
```

## Comparison with Other Tools
| Feature | vegeta | wrk | ab | hey |
|---------|--------|-----|----|----|
| Request rate control | Yes | No | No | Yes |
| Real-time output | Yes | No | Yes | No |
| JSON output | Yes | No | No | Yes |
| HTTP/2 | Yes | Yes | No | Yes |
| Scripting | Limited | Lua | No | No |

## Best Practices
- **Start low**: Begin with conservative rates
- **Warm up**: Run short test first to warm caches
- **Monitor**: Watch server metrics during tests
- **Realistic**: Use production-like request patterns
- **Gradual**: Ramp up load slowly
- **Multiple runs**: Average results from several runs
- **Clean state**: Reset between tests

## Common Patterns
```bash
# Health check load test
echo "GET http://localhost:8080/health" | \
  vegeta attack -rate=1000 -duration=1m

# API endpoint mix (realistic load)
cat <<EOF > targets.txt
GET http://localhost:8080/
GET http://localhost:8080/api/users
GET http://localhost:8080/api/products
POST http://localhost:8080/api/orders
Content-Type: application/json
{"item": "widget"}
EOF

cat targets.txt | vegeta attack -rate=100 -duration=5m

# Latency percentiles
cat results.bin | vegeta report | grep -A5 Latencies
```

## Troubleshooting
```bash
# Too many open files
ulimit -n 10000

# Connection refused
# Check server is running and listening

# Rate not achieved
# Increase -workers or reduce -rate

# High latencies
# Check network, server load, database
```

## Advanced Configuration

### Target File Templates
```bash
# targets.txt with variables
GET http://{{HOST}}/users/{{USER_ID}}
Authorization: Bearer {{TOKEN}}

POST http://{{HOST}}/orders
Content-Type: application/json
{"item": "{{ITEM}}"}
```

### Automated Load Testing
```bash
#!/bin/bash
# load-test.sh
RATES=(10 50 100 200)
for rate in "${RATES[@]}"; do
  echo "Testing at $rate req/s..."
  echo "GET http://api.example.com/health" | \
    vegeta attack -rate=$rate -duration=30s | \
    vegeta report -type=json > "results-${rate}.json"
done
```

### CI/CD Integration
```yaml
# .gitlab-ci.yml
load-test:
  script:
    - echo "GET $API_URL" | vegeta attack -rate=100 -duration=1m | tee results.bin
    - vegeta report -type=json < results.bin > report.json
    - jq '.success < 0.99' report.json && exit 1 || exit 0
```

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (binary, Scoop)
- ✅ BSD systems
- ✅ Docker container

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove vegeta |

## Agent Use
- Performance regression testing
- Load testing in CI/CD
- Capacity planning
- API endpoint validation
- SLA verification

## Uninstall
```yaml
- preset: vegeta
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/tsenart/vegeta
- Search: "vegeta load testing examples"

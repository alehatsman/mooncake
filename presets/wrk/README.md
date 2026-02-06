# wrk - HTTP Benchmarking

Multi-threaded HTTP benchmarking tool. Lua scripting for complex scenarios, low overhead, high throughput testing.

## Features
- **Multi-threaded**: Leverage all CPU cores
- **Lua scripting**: Complex request scenarios
- **Low overhead**: Minimal resource usage
- **High throughput**: Millions of requests per second
- **Latency distribution**: Detailed percentile stats
- **Custom headers**: Full HTTP customization
- **HTTP/1.1 and HTTP/2**: Modern protocol support
- **Connection pooling**: Keep-alive optimization

## Quick Start
```yaml
- preset: wrk
```

## Basic Usage
```bash
# Simple benchmark
wrk http://localhost:8080

# 12 threads, 400 connections, 30 seconds
wrk -t12 -c400 -d30s http://localhost:8080

# With custom timeout
wrk -t4 -c100 -d1m --timeout 10s http://localhost:8080

# Keep-alive connections
wrk -t4 -c100 -d30s -H "Connection: keep-alive" http://localhost:8080
```

## Parameters
```bash
# -t threads (typically CPU cores)
wrk -t8 http://localhost:8080

# -c connections (concurrent)
wrk -c200 http://localhost:8080

# -d duration (s, m, h)
wrk -d60s http://localhost:8080
wrk -d5m http://localhost:8080

# --timeout request timeout
wrk --timeout 2s http://localhost:8080

# --latency show latency distribution
wrk --latency http://localhost:8080
```

## Lua Scripting
```bash
# POST request
cat > post.lua <<'EOF'
wrk.method = "POST"
wrk.body   = '{"key":"value"}'
wrk.headers["Content-Type"] = "application/json"
EOF

wrk -t4 -c100 -d30s -s post.lua http://localhost:8080/api

# Dynamic requests
cat > dynamic.lua <<'EOF'
request = function()
  local id = math.random(1, 10000)
  return wrk.format("GET", "/users/" .. id)
end
EOF

wrk -t4 -c100 -d30s -s dynamic.lua http://localhost:8080

# Custom headers
cat > auth.lua <<'EOF'
wrk.headers["Authorization"] = "Bearer token123"
wrk.headers["Accept"] = "application/json"
EOF

wrk -s auth.lua http://localhost:8080
```

## Advanced Scripting
```lua
-- setup.lua: Initialization
function setup(thread)
  thread:set("id", counter())
end

function counter()
  local i = 0
  return function()
    i = i + 1
    return i
  end
end

-- request.lua: Dynamic requests
function request()
  local headers = {}
  headers["X-Request-ID"] = uuid()
  return wrk.format("GET", "/api/data", headers)
end

-- response.lua: Response processing
function response(status, headers, body)
  if status ~= 200 then
    io.write("Error: " .. status .. "\n")
  end
end

-- done.lua: Final stats
function done(summary, latency, requests)
  io.write("Total requests: " .. summary.requests .. "\n")
  io.write("Total errors: " .. summary.errors.connect + summary.errors.read + summary.errors.write + summary.errors.timeout .. "\n")
end
```

## Output Interpretation
```
Running 30s test @ http://localhost:8080
  12 threads and 400 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    25.45ms   15.23ms 250.00ms   85.67%
    Req/Sec     1.32k   120.45     1.80k    72.15%
  475823 requests in 30.10s, 98.45MB read
Requests/sec:  15812.45
Transfer/sec:      3.27MB
```

**Key Metrics**:
- **Latency Avg**: Mean response time
- **Latency Stdev**: Standard deviation (consistency)
- **Latency Max**: Worst case
- **Req/Sec**: Requests per second per thread
- **Requests/sec**: Total throughput
- **Transfer/sec**: Bandwidth used

## CI/CD Integration
```bash
# Performance regression test
BASELINE=10000
CURRENT=$(wrk -t4 -c100 -d10s http://localhost:8080 | grep 'Requests/sec' | awk '{print int($2)}')

if [ $CURRENT -lt $BASELINE ]; then
  echo "Performance regression: $CURRENT < $BASELINE req/sec"
  exit 1
fi

# Load test before deploy
wrk -t8 -c200 -d30s --latency http://staging.example.com > load-test.txt
if grep -q "errors" load-test.txt; then
  echo "Errors detected during load test"
  exit 1
fi
```

## Load Patterns
```bash
# Ramp-up test (manual steps)
wrk -t2 -c50 -d10s http://localhost:8080
wrk -t4 -c100 -d10s http://localhost:8080
wrk -t8 -c200 -d10s http://localhost:8080
wrk -t12 -c400 -d10s http://localhost:8080

# Sustained load
wrk -t8 -c200 -d5m http://localhost:8080

# Burst test (many connections)
wrk -t12 -c1000 -d30s http://localhost:8080

# Endurance test
wrk -t4 -c100 -d1h http://localhost:8080
```

## Common Scenarios
```bash
# JSON API test
cat > api-test.lua <<'EOF'
wrk.method = "POST"
wrk.body = '{"action":"test","data":"sample"}'
wrk.headers["Content-Type"] = "application/json"
wrk.headers["Authorization"] = "Bearer token"
EOF

wrk -t4 -c100 -d30s -s api-test.lua http://localhost:8080/api

# Multiple endpoints
cat > multi.lua <<'EOF'
local paths = {"/", "/about", "/api/users", "/api/posts"}
request = function()
  local path = paths[math.random(#paths)]
  return wrk.format("GET", path)
end
EOF

wrk -t4 -c100 -d30s -s multi.lua http://localhost:8080

# Authentication flow
cat > auth-flow.lua <<'EOF'
local token = "static-token"

setup = function(thread)
  -- In production, fetch token dynamically
end

request = function()
  return wrk.format("GET", "/protected", {["Authorization"] = "Bearer " .. token})
end
EOF

wrk -s auth-flow.lua http://localhost:8080
```

## Comparison with Other Tools
| Feature | wrk | ab | vegeta | hey |
|---------|-----|-----|--------|-----|
| Threads | Multi | Single | Multi | Multi |
| Scripting | Lua | No | Go | No |
| Latency dist | Yes | Basic | Yes | Yes |
| Speed | Fastest | Slow | Fast | Fast |
| Complexity | Medium | Low | Low | Low |

## Best Practices
- **Match threads to CPU cores** (`-t` = number of cores)
- **Start low, scale up** (avoid overloading test machine)
- **Run multiple iterations** for consistency
- **Monitor server metrics** during test (CPU, memory, network)
- **Test different patterns** (steady, burst, ramp)
- **Use Lua for complex scenarios**
- **Compare baseline vs current** for regression detection

## Tips
- wrk is CPU-bound, ensure test machine isn't bottleneck
- Use `--latency` flag to see percentile distribution
- Higher connections (-c) simulate more concurrent users
- Keep duration (-d) at least 30s for stable results
- Use Lua scripts for stateful testing
- Monitor target server during test
- Compare with production traffic patterns

## Advanced Configuration

### Complex Lua Script
```lua
-- advanced.lua
-- Setup
setup = function(thread)
  thread:set("id", counter())
end

function counter()
  local i = 0
  return function()
    i = i + 1
    return i
  end
end

-- Request
request = function()
  local id = thread:get("id")()
  local method = "POST"
  local path = "/api/users"
  local headers = {}
  headers["Content-Type"] = "application/json"
  headers["X-Request-ID"] = tostring(id)
  local body = string.format('{"id":%d,"timestamp":%d}', id, os.time())
  return wrk.format(method, path, headers, body)
end

-- Response validation
response = function(status, headers, body)
  if status ~= 200 then
    wrk:close()
  end
end

-- Summary
done = function(summary, latency, requests)
  io.write(string.format("Success rate: %.2f%%\n",
    100 * (summary.requests - summary.errors.connect - summary.errors.read - summary.errors.write) / summary.requests))
end
```

### Performance Baseline Script
```bash
#!/bin/bash
# establish-baseline.sh
DURATION=60s
THREADS=8
CONNECTIONS=200

wrk -t${THREADS} -c${CONNECTIONS} -d${DURATION} \
  --latency \
  http://api.example.com/health > baseline.txt

# Extract key metrics
THROUGHPUT=$(grep "Requests/sec" baseline.txt | awk '{print $2}')
P99=$(grep "99%" baseline.txt | awk '{print $2}')

echo "Baseline established: ${THROUGHPUT} req/s, p99: ${P99}"
```

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, compile from source)
- ✅ BSD systems
- ❌ Windows (use WSL)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove wrk |

## Agent Use
- Automated performance testing
- CI/CD load gates
- Regression detection
- Capacity planning
- API stress testing
- Baseline establishment

## Uninstall
```yaml
- preset: wrk
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/wg/wrk
- Search: "wrk load testing", "wrk lua examples"

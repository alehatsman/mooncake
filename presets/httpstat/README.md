# httpstat - HTTP Timing Visualization

Visualize curl timing statistics. See DNS, TCP, TLS, server, and transfer timings in beautiful color-coded output.

## Quick Start
```yaml
- preset: httpstat
```

## Basic Usage
```bash
# Simple GET
httpstat https://example.com

# With full response
httpstat -v https://example.com

# Follow redirects
httpstat -L https://bit.ly/shortlink

# Custom headers
httpstat https://api.example.com -H "Authorization: Bearer token"
```

## HTTP Methods
```bash
# GET (default)
httpstat https://api.example.com/users

# POST with data
httpstat -X POST -d '{"name":"john"}' https://api.example.com/users

# POST with JSON
httpstat -X POST \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com"}' \
  https://api.example.com/register

# PUT request
httpstat -X PUT -d '{"status":"active"}' https://api.example.com/users/1

# DELETE request
httpstat -X DELETE https://api.example.com/users/1
```

## Timing Breakdown
```
DNS Lookup   TCP Connection   TLS Handshake   Server Processing   Content Transfer
[   25ms    |     35ms      |     89ms      |      125ms        |      45ms        ]
             |                |               |                   |                  |
    namelookup:25ms           |               |                   |                  |
                        connect:60ms          |                   |                  |
                                    pretransfer:149ms             |                  |
                                                      starttransfer:274ms            |
                                                                                total:319ms
```

**Phases**:
- **DNS Lookup**: Domain name resolution
- **TCP Connection**: TCP 3-way handshake
- **TLS Handshake**: SSL/TLS negotiation (HTTPS only)
- **Server Processing**: Server generates response
- **Content Transfer**: Download response body

## Output Metrics
```bash
httpstat https://api.github.com
```

Shows:
- **Connected to**: IP address and port
- **HTTP version**: HTTP/1.1, HTTP/2, HTTP/3
- **Status code**: 200, 404, 500, etc.
- **Response headers**: All headers
- **Speed metrics**: Download speed, upload speed
- **Timing phases**: Color-coded visual timeline

## Performance Analysis
```bash
# Identify slow phase
httpstat https://slow-api.example.com

# If DNS is slow (yellow bar large):
# - DNS server issues
# - Use CDN or better DNS

# If TCP is slow (green bar large):
# - Network latency
# - Server far away
# - Consider CDN

# If TLS is slow (cyan bar large):
# - Certificate chain issues
# - Cipher negotiation slow
# - Use session resumption

# If Server Processing is slow (magenta bar large):
# - Backend performance issue
# - Database query slow
# - Optimize application code

# If Content Transfer is slow (blue bar large):
# - Large response body
# - Bandwidth limited
# - Enable compression
```

## CI/CD Integration
```bash
# Check API response time
RESPONSE_TIME=$(httpstat https://api.example.com 2>&1 | grep 'total:' | awk '{print $2}' | tr -d 'ms')

if [ $RESPONSE_TIME -gt 1000 ]; then
  echo "API too slow: ${RESPONSE_TIME}ms"
  exit 1
fi

# Verify TLS performance
httpstat https://api.example.com | grep 'TLS Handshake' || echo "TLS issue detected"

# Save timing report
httpstat https://api.example.com > timing-report.txt
```

## Authentication
```bash
# Bearer token
httpstat https://api.example.com/protected \
  -H "Authorization: Bearer token123"

# Basic auth
httpstat -u username:password https://api.example.com/auth

# API key
httpstat https://api.example.com/data \
  -H "X-API-Key: secret123"
```

## Advanced Usage
```bash
# Custom timeout
httpstat --max-time 5 https://api.example.com

# Insecure SSL (skip verification)
httpstat -k https://self-signed.example.com

# Save response body
httpstat https://api.example.com -o response.json

# Show request headers
httpstat -v https://example.com

# Use proxy
httpstat -x http://proxy:8080 https://api.example.com

# IPv4 only
httpstat -4 https://example.com

# IPv6 only
httpstat -6 https://example.com
```

## Debugging Scenarios
```bash
# Slow DNS lookup
httpstat https://example.com
# Fix: Update DNS servers, use /etc/hosts, enable DNS caching

# High TLS handshake time
httpstat https://api.example.com
# Fix: Enable TLS session resumption, use modern ciphers

# Large server processing time
httpstat https://api.example.com/slow-endpoint
# Fix: Optimize backend, add caching, scale horizontally

# Geographic latency
httpstat https://api-us-east.example.com
httpstat https://api-eu-west.example.com
# Compare to choose optimal region

# HTTP/2 vs HTTP/1.1
httpstat https://http2.example.com
httpstat --http1.1 https://http2.example.com
# Compare protocol performance
```

## Monitoring Workflow
```bash
# Baseline timing
httpstat https://api.example.com > baseline.txt

# After optimization
httpstat https://api.example.com > optimized.txt

# Compare
diff baseline.txt optimized.txt

# Continuous monitoring
while true; do
  echo "=== $(date) ===" >> api-timing.log
  httpstat https://api.example.com >> api-timing.log
  sleep 300
done
```

## Real-World Examples
```bash
# GitHub API timing
httpstat https://api.github.com \
  -H "Authorization: Bearer ghp_token"

# Test CDN performance
httpstat https://cdn.example.com/assets/app.js

# Check API regions
for region in us-east us-west eu-west ap-south; do
  echo "Region: $region"
  httpstat https://api-${region}.example.com
done

# POST form data
httpstat -X POST \
  -d "username=alice&password=secret" \
  https://example.com/login

# File upload timing
httpstat -X POST \
  -F "file=@document.pdf" \
  https://api.example.com/upload

# Compare HTTP/2 vs HTTP/1.1
httpstat https://example.com
httpstat --http1.1 https://example.com
```

## Environment Variables
```bash
# Custom colors
export HTTPSTAT_SHOW_BODY=false
export HTTPSTAT_SHOW_IP=true
export HTTPSTAT_SHOW_SPEED=true

# Curl options
export HTTPSTAT_CURL_BIN=/usr/local/bin/curl
```

## Comparison
| Feature | httpstat | curl | xh | vegeta |
|---------|----------|------|-----|--------|
| Timing viz | Beautiful | Numbers | No | Yes |
| Easy to read | Yes | No | Yes | No |
| Load testing | No | No | No | Yes |
| Scripting | Limited | Full | Good | Good |

## Best Practices
- Use for **one-off requests**, not load testing
- Identify performance bottlenecks visually
- Test from different geographic locations
- Compare HTTP/2 vs HTTP/1.1 performance
- Monitor TLS handshake times
- Track DNS resolution issues
- Measure CDN effectiveness

## Tips
- Color-coded output makes issues obvious
- Great for debugging slow APIs
- Visual timeline easier than raw numbers
- Works with all curl options
- Perfect for documentation/reports
- Screenshot output for issue reports
- Use in CI to track performance trends

## Agent Use
- API response time verification
- Performance regression detection
- Endpoint health validation
- Geographic latency measurement
- TLS configuration verification
- CDN effectiveness testing

## Uninstall
```yaml
- preset: httpstat
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/reorx/httpstat
- Search: "httpstat examples", "http timing visualization"

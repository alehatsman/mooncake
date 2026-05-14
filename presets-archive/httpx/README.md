# httpx - Fast HTTP Toolkit

Fast and multi-purpose HTTP toolkit for probing, security testing, and web reconnaissance.

## Quick Start
```yaml
- preset: httpx
```

## Features
- **Blazing fast**: Concurrent HTTP probing with connection pooling
- **Multiple protocols**: HTTP, HTTPS, HTTP/2, HTTP/3 support
- **Smart detection**: Technology detection, title extraction, status codes
- **Flexible output**: JSON, CSV, plain text formats
- **Pipeline friendly**: Accepts input from stdin
- **Security testing**: Header analysis, TLS/SSL inspection

## Basic Usage
```bash
# Probe single URL
httpx -u https://example.com

# Probe multiple URLs from file
cat urls.txt | httpx

# Probe with status code
httpx -u https://example.com -status-code

# Extract titles
httpx -u https://example.com -title

# Technology detection
httpx -u https://example.com -tech-detect

# Full scan
httpx -u https://example.com -status-code -title -tech-detect -server -content-length
```

## Input Methods

### From stdin
```bash
# Pipe from other tools
echo "example.com" | httpx
subfinder -d example.com | httpx
cat domains.txt | httpx
```

### From file
```bash
# File with URLs/domains
httpx -l urls.txt

# One URL per line
cat urls.txt
https://example.com
https://test.com
example.org
```

### Direct URL
```bash
httpx -u https://example.com
```

## Advanced Configuration
```yaml
- preset: httpx
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove httpx |

## Real-World Examples

### Web Reconnaissance
```bash
# Discover live hosts from subdomain enumeration
subfinder -d example.com -silent | httpx -silent -status-code -title

# Check multiple targets
cat targets.txt | httpx -threads 50 -status-code -tech-detect -json -o results.json

# Find specific status codes
cat urls.txt | httpx -mc 200,201,204 -silent
```

### Security Testing
```bash
# Check for common security headers
httpx -u https://example.com -include-response-header \
  -match-header "X-Frame-Options" \
  -match-header "Content-Security-Policy" \
  -match-header "Strict-Transport-Security"

# TLS/SSL version detection
httpx -u https://example.com -tls-grab

# Find endpoints with specific content
httpx -l urls.txt -match-string "admin" -match-regex "api.*key"
```

### Performance Testing
```bash
# Response time measurement
httpx -u https://example.com -response-time

# Content length check
httpx -l urls.txt -content-length -cl-ranges 0-1000,1000-5000

# Follow redirects
httpx -u https://example.com -follow-redirects -max-redirects 3
```

## Output Formats

### JSON Output
```bash
httpx -l urls.txt -json -o results.json

# Example output
{
  "url": "https://example.com",
  "status-code": 200,
  "title": "Example Domain",
  "content-length": 1256,
  "tech": ["Nginx"],
  "server": "nginx/1.18.0"
}
```

### CSV Output
```bash
httpx -l urls.txt -csv -o results.csv
```

### Custom fields
```bash
httpx -u https://example.com \
  -status-code \
  -title \
  -content-length \
  -tech-detect \
  -server \
  -response-time
```

## Filtering and Matching

### Status Code Filters
```bash
# Match specific codes
httpx -l urls.txt -mc 200,201,301,302

# Exclude codes
httpx -l urls.txt -fc 404,403,500

# Match code ranges
httpx -l urls.txt -mc 200-299
```

### Content Matching
```bash
# Match string in response
httpx -l urls.txt -match-string "Welcome"

# Match regex
httpx -l urls.txt -match-regex "api.*key"

# Filter by content length
httpx -l urls.txt -cl-ranges 1000-5000
```

### Header Matching
```bash
# Match specific header
httpx -l urls.txt -match-header "X-Powered-By"

# Exclude header
httpx -l urls.txt -filter-header "Server: cloudflare"
```

## Performance Options
```bash
# Concurrent threads
httpx -l urls.txt -threads 100

# Rate limiting
httpx -l urls.txt -rate-limit 50  # requests per second

# Timeout
httpx -l urls.txt -timeout 10  # seconds

# Retries
httpx -l urls.txt -retries 3
```

## Advanced Features

### Technology Detection
```bash
# Detect web technologies
httpx -u https://example.com -tech-detect

# Technologies detected:
# - Web server (Nginx, Apache)
# - Frameworks (React, Vue, Django)
# - CDN (Cloudflare, Akamai)
# - Analytics (Google Analytics)
```

### Screenshot Capture
```bash
# Capture screenshot (requires chromium)
httpx -u https://example.com -screenshot -srd screenshots/
```

### Pipeline Integration
```bash
# Full reconnaissance pipeline
subfinder -d example.com -silent | \
  httpx -silent -status-code -title | \
  tee discovered.txt | \
  nuclei -t cves/ -silent
```

## Configuration File
```yaml
# ~/.config/httpx/config.yaml
threads: 50
timeout: 10
retries: 2
status-code: true
title: true
tech-detect: true
follow-redirects: true
max-redirects: 3
```

Use with:
```bash
httpx -config ~/.config/httpx/config.yaml -l urls.txt
```

## Platform Support
- ✅ Linux (binary installation)
- ✅ macOS (Homebrew, binary)
- ❌ Windows (not yet supported in preset)

## Agent Use
- Web service discovery and reconnaissance
- HTTP endpoint validation in CI/CD
- Security testing automation
- Service health monitoring
- API endpoint enumeration
- Technology stack identification

## Troubleshooting

### Too many open files
```bash
# Increase file descriptor limit
ulimit -n 10000

# Or reduce concurrency
httpx -l urls.txt -threads 25
```

### Rate limiting issues
```bash
# Add rate limiting
httpx -l urls.txt -rate-limit 10

# Add delay between requests
httpx -l urls.txt -delay 1s
```

### TLS/SSL errors
```bash
# Skip TLS verification (testing only)
httpx -u https://example.com -skip-tls-verification

# Use specific TLS version
httpx -u https://example.com -tls-minimum-version 1.2
```

## Common Workflows

### Subdomain Validation
```bash
subfinder -d example.com | httpx -silent -status-code | tee live-subdomains.txt
```

### Port Scanning Integration
```bash
nmap -p 80,443,8000,8080,8443 example.com -oG - | \
  awk '/open/{print $2":"$5}' | \
  sed 's|/||g' | \
  httpx -silent
```

### Content Discovery
```bash
# Find admin panels
cat urls.txt | httpx -path /admin -mc 200,301,302 -silent

# Find APIs
cat urls.txt | httpx -path /api/v1 -mc 200 -title -silent
```

## Uninstall
```yaml
- preset: httpx
  with:
    state: absent
```

## Resources
- Official docs: https://github.com/projectdiscovery/httpx
- GitHub: https://github.com/projectdiscovery/httpx
- Project Discovery: https://projectdiscovery.io/
- Search: "httpx tutorial", "httpx reconnaissance"

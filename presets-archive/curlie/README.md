# curlie - HTTP Client

curl with httpie syntax. Power of curl with the ease of httpie. Best of both worlds for API testing.

## Quick Start
```yaml
- preset: curlie
```

## Features
- **HTTPie Syntax**: Simple, intuitive command syntax for API calls
- **curl Power**: Full curl features and compatibility
- **Syntax Highlighting**: Colored JSON/XML output for readability
- **Session Support**: Save and reuse headers and cookies
- **Form and File Uploads**: Easy multipart form handling
- **Authentication**: Built-in support for basic, bearer, and custom auth
- **Best of Both**: Combines httpie's UX with curl's performance

## Basic Usage
```bash
# Simple GET
curlie example.com
curlie https://api.github.com/users/octocat

# With headers
curlie example.com User-Agent:custom

# POST JSON
curlie POST example.com/api name=john age:=25

# PUT request
curlie PUT example.com/api/1 status=active
```

## HTTP Methods
```bash
# GET (default)
curlie example.com/api/users

# POST
curlie POST example.com/api/users name=alice

# PUT
curlie PUT example.com/api/users/1 name=bob

# PATCH
curlie PATCH example.com/api/users/1 status=active

# DELETE
curlie DELETE example.com/api/users/1

# HEAD
curlie HEAD example.com
```

## Request Body
```bash
# JSON data (key=value)
curlie POST api.example.com name=john email=john@example.com

# JSON with types
curlie POST api.example.com \
  name=john \
  age:=30 \
  active:=true \
  tags:='["dev","ops"]'

# Raw JSON
echo '{"name":"john"}' | curlie POST api.example.com

# Form data
curlie --form POST example.com name=john file@./upload.txt

# Multipart
curlie --form POST example.com file@./image.png description='My image'
```

## Headers
```bash
# Custom headers
curlie example.com Authorization:'Bearer token123'

# Multiple headers
curlie example.com \
  Authorization:'Bearer token' \
  Content-Type:application/json \
  Accept:application/json

# Remove default headers
curlie example.com User-Agent:
```

## Authentication
```bash
# Basic auth
curlie -u username:password example.com

# Bearer token
curlie example.com Authorization:'Bearer token123'

# API key
curlie example.com X-API-Key:secret123
```

## Response Handling
```bash
# Show headers
curlie -v example.com

# Headers only
curlie -I example.com

# Follow redirects
curlie -L example.com

# Download file
curlie example.com/file.zip > file.zip

# Save response
curlie example.com -o output.json
```

## Query Parameters
```bash
# Query string
curlie example.com/search?q=golang

# httpie style
curlie example.com/search q==golang limit==10

# Multiple params
curlie example.com/api page==1 size==50 sort==name
```

## CI/CD Integration
```bash
# Health check
if curlie --fail https://api.example.com/health; then
  echo "API is healthy"
fi

# With timeout
curlie --max-time 5 example.com/api

# Silent mode
curlie -s example.com/api > response.json

# Exit on error
curlie --fail-with-body example.com/api || exit 1
```

## API Testing
```bash
# Test endpoint
curlie POST api.example.com/users name=test

# Verify response
curlie api.example.com/users/1 | jq '.status'

# Load test prep
for i in {1..100}; do
  curlie POST api.example.com/events type=test id:=$i &
done
wait

# Response time
time curlie example.com/api
```

## Advanced Features
```bash
# Custom method
curlie -X OPTIONS example.com

# Proxy
curlie --proxy http://proxy:8080 example.com

# Insecure SSL
curlie -k https://self-signed.example.com

# Verbose mode
curlie -v example.com

# Include response headers
curlie -i example.com

# Follow redirects (max 10)
curlie -L --max-redirs 10 example.com
```

## File Operations
```bash
# Upload file
curlie --form POST example.com/upload file@document.pdf

# Upload with metadata
curlie --form POST example.com/upload \
  file@image.png \
  title='My Image' \
  tags:='["nature","landscape"]'

# Multiple files
curlie --form POST example.com/upload \
  files@file1.txt \
  files@file2.txt

# Download file
curlie example.com/download/file.zip -o file.zip
```

## Session Management
```bash
# Cookie jar
curlie --cookie-jar cookies.txt example.com/login
curlie --cookie cookies.txt example.com/dashboard

# Send cookies
curlie example.com Cookie:session=abc123
```

## Comparison
| Feature | curlie | curl | httpie |
|---------|--------|------|--------|
| Syntax | httpie | Complex | httpie |
| Speed | Fast | Fast | Slow |
| curl features | Full | Full | Limited |
| Colors | Yes | No | Yes |
| JSON | Easy | Manual | Easy |

## Real-World Examples
```bash
# GitHub API
curlie https://api.github.com/repos/owner/repo \
  Authorization:'Bearer ghp_token'

# Create resource
curlie POST https://api.example.com/posts \
  title='My Post' \
  body='Content here' \
  userId:=1

# Update resource
curlie PUT https://api.example.com/posts/1 \
  title='Updated Title'

# Delete resource
curlie DELETE https://api.example.com/posts/1 \
  Authorization:'Bearer token'

# Pagination
for page in {1..5}; do
  curlie "https://api.example.com/items?page=$page" >> items.json
done

# Error handling
if ! curlie --fail https://api.example.com/endpoint; then
  echo "Request failed"
  exit 1
fi
```

## Debugging
```bash
# Verbose output
curlie -v example.com

# Timing information
curlie -w "@curl-format.txt" example.com

# Show request
curlie --print=HhBb example.com

# Trace requests
curlie --trace-ascii debug.txt example.com
```

## Best Practices
- Use `--fail` in scripts for error handling
- Set timeouts with `--max-time` to prevent hangs
- Use `-s` for silent mode in automation
- Leverage httpie syntax for readability
- Use `--form` for multipart uploads
- Save responses with `-o` for processing

## Tips
- No installation of curl needed (uses system curl)
- Supports all curl flags
- httpie syntax for ease of use
- Colored output for readability
- Fast startup time
- Works offline

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- API endpoint testing
- CI/CD health checks
- Integration testing
- Webhook delivery
- Service verification
- Response validation


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install curlie
  preset: curlie

- name: Use curlie in automation
  shell: |
    # Custom configuration here
    echo "curlie configured"
```
## Uninstall
```yaml
- preset: curlie
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/rs/curlie
- Search: "curlie examples", "curlie vs httpie"

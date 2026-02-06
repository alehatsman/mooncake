# xh - Fast HTTP Client

httpie-compatible HTTP client in Rust. Faster than httpie, drop-in replacement with additional features.

## Quick Start
```yaml
- preset: xh
```

## Basic Usage
```bash
# Simple GET
xh example.com
xh https://api.github.com/users/octocat

# POST JSON
xh POST example.com/api name=john age:=25

# With headers
xh example.com Authorization:'Bearer token'
```

## HTTP Methods
```bash
# GET (default)
xh example.com/api/users

# POST
xh POST example.com/api/users name=alice email=alice@example.com

# PUT
xh PUT example.com/api/users/1 name=bob

# PATCH
xh PATCH example.com/api/users/1 status=active

# DELETE
xh DELETE example.com/api/users/1

# HEAD
xh HEAD example.com
```

## Request Body
```bash
# JSON data
xh POST api.example.com name=john email=john@example.com

# JSON with types
xh POST api.example.com \
  name=john \
  age:=30 \
  active:=true \
  score:=95.5 \
  tags:='["dev","ops"]'

# Raw JSON
echo '{"name":"john"}' | xh POST api.example.com

# From file
xh POST api.example.com < data.json

# Form data
xh --form POST example.com name=john file@upload.txt

# Multipart
xh --multipart POST example.com file@image.png caption='My photo'
```

## Headers
```bash
# Custom headers
xh example.com Authorization:'Bearer token123'

# Multiple headers
xh example.com \
  Authorization:'Bearer token' \
  Accept:application/json \
  User-Agent:MyApp/1.0

# Remove headers
xh example.com User-Agent:
```

## Authentication
```bash
# Basic auth
xh -a username:password example.com

# Bearer token
xh example.com Authorization:'Bearer token123'

# API key header
xh example.com X-API-Key:secret123

# Prompt for password
xh -a username example.com
```

## Query Parameters
```bash
# Query string
xh example.com/search?q=golang&limit=10

# httpie style
xh example.com/search q==golang limit==10

# URL encoded
xh example.com/search q=='hello world' page==1
```

## Response Handling
```bash
# Verbose output (headers + body)
xh -v example.com

# Headers only
xh -h example.com

# Body only (default)
xh example.com

# Download file
xh example.com/file.zip --download
xh example.com/file.zip -d -o myfile.zip

# Print options
xh -p HhBb example.com  # Headers and body
xh -p H example.com     # Response headers only
xh -p B example.com     # Response body only
```

## Sessions
```bash
# Named session
xh --session=mysession example.com/login username=alice password=secret

# Use session (cookies + auth persist)
xh --session=mysession example.com/dashboard

# Session with specific host
xh --session=api example.com/endpoint

# Read-only session
xh --session-read-only=mysession example.com/data
```

## File Operations
```bash
# Upload single file
xh --form POST example.com/upload file@document.pdf

# Upload with metadata
xh --form POST example.com/upload \
  file@image.png \
  title='Vacation Photo' \
  tags:='["summer","2024"]'

# Multiple files
xh --multipart POST example.com/upload \
  photo1@pic1.jpg \
  photo2@pic2.jpg

# Download with progress
xh --download example.com/large-file.zip

# Resume download
xh --download --continue example.com/large-file.zip
```

## CI/CD Integration
```bash
# Health check with exit code
xh --check-status https://api.example.com/health

# Timeout
xh --timeout 5 example.com/api

# Silent output
xh --quiet example.com/api > response.json

# Follow redirects
xh --follow example.com/redirect

# Fail on HTTP errors
if ! xh --check-status api.example.com/endpoint; then
  echo "API returned error status"
  exit 1
fi
```

## API Testing
```bash
# Test POST endpoint
xh POST api.example.com/users \
  name=testuser \
  email=test@example.com

# Verify response
xh api.example.com/users/1 | jq '.id'

# Test with different content types
xh POST api.example.com/data \
  Content-Type:application/xml \
  --raw '<?xml version="1.0"?><data>test</data>'

# Benchmark endpoint
time xh example.com/api

# Multiple requests
for i in {1..50}; do
  xh POST api.example.com/events type=test id:=$i &
done
wait
```

## Advanced Features
```bash
# Custom method
xh OPTIONS example.com

# Proxy
xh --proxy http://proxy:8080 example.com
xh --proxy https://secure-proxy:8443 example.com

# Insecure SSL
xh --verify no https://self-signed.example.com

# Follow redirects with limit
xh --follow --max-redirects 5 example.com

# Pretty print
xh --pretty all example.com       # Colors and formatting
xh --pretty format example.com    # Formatting only
xh --pretty colors example.com    # Colors only
xh --pretty none example.com      # No formatting

# Output format
xh example.com --json              # Force JSON
xh example.com --format-options indent:4
```

## Comparison
| Feature | xh | httpie | curlie | curl |
|---------|-----|--------|--------|------|
| Speed | Fastest | Slow | Fast | Fast |
| Syntax | httpie | httpie | httpie | curl |
| Sessions | Yes | Yes | No | Manual |
| Downloads | Built-in | Limited | No | Yes |
| Rust | Yes | No | Go | C |

## GitHub Integration
```bash
# Get repo info
xh https://api.github.com/repos/owner/repo

# With authentication
xh https://api.github.com/user \
  Authorization:'Bearer ghp_token'

# Create issue
xh POST https://api.github.com/repos/owner/repo/issues \
  Authorization:'Bearer ghp_token' \
  title='Bug report' \
  body='Description here'

# List PRs
xh https://api.github.com/repos/owner/repo/pulls \
  Accept:'application/vnd.github.v3+json'
```

## Real-World Examples
```bash
# REST API CRUD
xh GET api.example.com/posts
xh POST api.example.com/posts title='New Post' body='Content'
xh PUT api.example.com/posts/1 title='Updated'
xh DELETE api.example.com/posts/1

# Pagination
for page in {1..5}; do
  xh "api.example.com/items?page=$page" >> all-items.json
done

# JWT authentication
TOKEN=$(xh POST api.example.com/login username=alice password=secret | jq -r '.token')
xh api.example.com/protected Authorization:"Bearer $TOKEN"

# File upload pipeline
xh --form POST api.example.com/upload \
  file@document.pdf \
  category=reports | \
  jq -r '.file_id' | \
  xargs -I {} xh POST api.example.com/process file_id={}

# Health monitoring
while true; do
  if ! xh --check-status api.example.com/health; then
    echo "Health check failed at $(date)"
  fi
  sleep 60
done
```

## Debugging
```bash
# Verbose output
xh -v example.com

# Print request and response
xh -p HhBb example.com

# Show timings
xh --print=HhBb --verbose example.com

# Trace redirects
xh --follow --verbose example.com
```

## Configuration
```bash
# Config file: ~/.config/xh/config.json
{
  "default_options": [
    "--pretty=all",
    "--print=HhBb"
  ]
}

# Environment variables
export XH_HTTPIE_COMPAT_MODE=true
```

## Best Practices
- Use `--check-status` for CI/CD
- Leverage sessions for authenticated workflows
- Use `--download` for large files
- Set timeouts to prevent hangs
- Use `--quiet` in scripts
- Store sensitive tokens in environment variables
- Enable `--follow` for APIs with redirects

## Tips
- 10x faster than httpie
- Drop-in httpie replacement
- Native binary (no Python runtime)
- Built-in download manager
- Session support out of the box
- JSON syntax highlighting
- Small binary size (~5MB)

## Agent Use
- Automated API testing
- CI/CD endpoint verification
- Integration test workflows
- File upload automation
- Health check monitoring
- Response validation

## Uninstall
```yaml
- preset: xh
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/ducaale/xh
- Search: "xh http client", "xh vs httpie"

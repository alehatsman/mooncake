# HTTPie Preset

Install HTTPie - a modern, user-friendly command-line HTTP client designed for testing APIs.

## Quick Start

```yaml
- preset: httpie
```

## Features

- ✅ Intuitive syntax
- ✅ JSON support
- ✅ Syntax highlighting
- ✅ Wget-like downloads
- ✅ Sessions
- ✅ Forms and file uploads
- ✅ HTTPS, proxies, authentication

## Basic Usage

### GET Requests

```bash
# Simple GET
http https://api.github.com/users/github

# With headers
http GET https://api.example.com \
  Authorization:"Bearer token" \
  Accept:application/json

# Query parameters
http GET https://api.example.com/search \
  q=="httpie" \
  limit==10
```

### POST Requests

```bash
# POST JSON (default)
http POST https://httpbin.org/post \
  name=John \
  age:=30 \
  active:=true

# POST form data
http --form POST https://httpbin.org/post \
  name=John \
  email=john@example.com

# POST with file
http --form POST https://httpbin.org/post \
  file@/path/to/file.txt
```

### PUT/PATCH/DELETE

```bash
# PUT request
http PUT https://api.example.com/users/123 \
  name=Jane \
  email=jane@example.com

# PATCH request
http PATCH https://api.example.com/users/123 \
  email=newemail@example.com

# DELETE request
http DELETE https://api.example.com/users/123
```

### Authentication

```bash
# Basic auth
http -a username:password https://api.example.com

# Bearer token
http https://api.example.com \
  Authorization:"Bearer YOUR_TOKEN"

# API key
http https://api.example.com \
  X-API-Key:YOUR_API_KEY
```

### Download Files

```bash
# Download file
http --download https://example.com/file.zip

# Download to specific location
http --download --output=/tmp/file.zip https://example.com/file.zip

# Continue download
http --download --continue https://example.com/large-file.zip
```

### Sessions

```bash
# Create session
http --session=./session.json https://api.example.com/login \
  username=user \
  password=pass

# Use session
http --session=./session.json https://api.example.com/profile

# Named sessions
http --session=mysession https://api.example.com/login
http --session=mysession https://api.example.com/profile
```

### Output Options

```bash
# Pretty print (default)
http https://api.github.com/users/github

# Only headers
http --headers https://api.github.com/users/github

# Only body
http --body https://api.github.com/users/github

# Raw output
http --print=b https://api.github.com/users/github

# Save to file
http https://api.github.com/users/github > response.json

# Quiet mode
http --quiet https://api.github.com/users/github
```

### Request Types

```bash
# JSON (default)
http POST https://httpbin.org/post name=John

# Form
http --form POST https://httpbin.org/post name=John

# Multipart
http --multipart POST https://httpbin.org/post \
  name=John \
  file@avatar.png

# Raw body
echo '{"name":"John"}' | http POST https://httpbin.org/post
```


## Advanced Configuration
```yaml
- preset: httpie
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove httpie |
## vs curl

HTTPie vs curl examples:

```bash
# HTTPie
http POST https://api.example.com/users name=John age:=30

# curl equivalent
curl -X POST https://api.example.com/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John","age":30}'
```

## Configuration

Config file: `~/.config/httpie/config.json`

```json
{
  "default_options": ["--style=monokai", "--timeout=300"],
  "implicit_content_type": "json"
}
```

## Color Themes

```bash
# List themes
http --print=B --style=monokai https://httpie.io/hello

# Available themes
- monokai
- solarized
- fruity
- native
- vim
```

## Tips

1. **Shortcuts**: `http` is shorthand for `http GET`
2. **localhost**: `:3000` expands to `http://localhost:3000`
3. **JSON types**: Use `:=` for non-string values (`:=30` for numbers, `:=true` for booleans)
4. **Arrays**: `items:='[1,2,3]'` for JSON arrays
5. **Headers**: `Header:Value` syntax

## Common Use Cases

### REST API Testing

```bash
# List resources
http GET https://api.example.com/users

# Get resource
http GET https://api.example.com/users/123

# Create resource
http POST https://api.example.com/users \
  name=John \
  email=john@example.com

# Update resource
http PUT https://api.example.com/users/123 \
  name="John Updated"

# Delete resource
http DELETE https://api.example.com/users/123
```

### GraphQL

```bash
http POST https://api.example.com/graphql \
  query='{ users { id name email } }'
```

### WebSockets (via httpie-websockets plugin)

```bash
pip install httpie-websockets
ws wss://echo.websocket.org
```

## Plugins

Popular HTTPie plugins:

```bash
# Install plugin
pip install httpie-jwt-auth
pip install httpie-aws-auth
pip install httpie-oauth

# Use JWT auth
http --auth-type=jwt --auth="token" https://api.example.com
```

## Agent Use
- Automated environment setup
- CI/CD pipeline integration
- Development environment provisioning
- Infrastructure automation

## Uninstall

```yaml
- preset: httpie
  with:
    state: absent
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Resources
- Search: "httpie documentation", "httpie tutorial"

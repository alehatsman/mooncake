# Download Action

The `download` action downloads files from remote URLs with support for checksums, retries, timeouts, and idempotency.

## Features

-  HTTP/HTTPS downloads
-  Checksum verification (SHA256 or MD5)
-  Idempotent (skips download if checksum matches)
-  Retry logic with configurable attempts
-  Timeout configuration
-  Custom HTTP headers (for authentication)
-  Backup existing files before overwriting
-  File permissions configuration
-  Atomic writes (download to temp, then move)
-  Dry-run support

## Basic Usage

```yaml
steps:
  - name: Download file
    download:
      url: "https://example.com/file.tar.gz"
      dest: "/tmp/file.tar.gz"
      mode: "0644"
```

## Parameters

### Required

- **url** (string): Remote URL to download from
- **dest** (string): Destination file path

### Optional

- **checksum** (string): Expected SHA256 (64 chars) or MD5 (32 chars) checksum
  - Used for verification after download
  - Enables idempotency - skips download if file exists with matching checksum
- **mode** (string): File permissions in octal format (e.g., "0644", "0755")
  - Default: 0644
- **timeout** (string): Maximum download time (e.g., "30s", "5m", "1h")
- **retries** (integer): Number of retry attempts on failure (0-100)
  - Default: 1 (single attempt)
- **force** (boolean): Force re-download even if destination exists
  - Default: false
- **backup** (boolean): Create .bak backup before overwriting
  - Default: false
- **headers** (map): Custom HTTP headers
  - Useful for Authorization, User-Agent, etc.

## Examples

### Basic Download

```yaml
- name: Download Go tarball
  download:
    url: "https://go.dev/dl/go1.21.5.linux-amd64.tar.gz"
    dest: "/tmp/go.tar.gz"
    mode: "0644"
```

### Idempotent Download with Checksum

```yaml
- name: Download Node.js (idempotent)
  download:
    url: "https://nodejs.org/dist/v18.19.0/node-v18.19.0-linux-x64.tar.gz"
    dest: "/tmp/node.tar.gz"
    checksum: "f27e33ebe5a0c2ec8d5d6b5f5c7c2c0c1c3f7b1a2a3d4e5f6g7h8i9j0k1l2m3n"
    mode: "0644"
  register: node_download

- name: Extract only if downloaded
  unarchive:
    src: "/tmp/node.tar.gz"
    dest: "/opt/node"
  when: node_download.changed
```

### Download with Retry and Timeout

```yaml
- name: Download large ISO with retry
  download:
    url: "https://releases.ubuntu.com/22.04/ubuntu-22.04.3-live-server-amd64.iso"
    dest: "/tmp/ubuntu.iso"
    timeout: "10m"
    retries: 3
    mode: "0644"
```

### Authenticated Download

```yaml
- name: Download from private API
  download:
    url: "https://api.example.com/files/document.pdf"
    dest: "/tmp/document.pdf"
    headers:
      Authorization: "Bearer {{ api_token }}"
      User-Agent: "Mooncake/1.0"
    mode: "0644"
  when: api_token is defined
```

### Download with Backup

```yaml
- name: Update config file safely
  download:
    url: "https://example.com/config/app.conf"
    dest: "/etc/myapp/app.conf"
    backup: true
    force: true
    become: true
```

### Conditional Download

```yaml
- name: Download only if missing
  download:
    url: "https://github.com/moby/moby/raw/master/README.md"
    dest: "/tmp/docker-readme.md"
  creates: "/tmp/docker-readme.md"
```

## Idempotency

The download action is idempotent when:

1. **With checksum**: File exists and checksum matches → skip download
2. **Without checksum + without force**: File exists → skip download (not recommended)
3. **With force**: Always re-download

**Best practice**: Always use `checksum` for reliable idempotency.

## Atomic Operations

Downloads are performed atomically:

1. Download to temporary file
2. Set permissions on temp file
3. Verify checksum (if provided)
4. Move temp file to destination (atomic rename)

This ensures partial downloads never corrupt the destination file.

## Error Handling

The action fails if:

- URL is unreachable or returns non-200 status
- Download times out
- Checksum verification fails
- Destination directory doesn't exist
- Insufficient permissions

Use `retries` to automatically retry on transient failures.

## Integration with Other Actions

### Download and Extract

```yaml
- name: Download archive
  download:
    url: "https://example.com/app.tar.gz"
    dest: "/tmp/app.tar.gz"
    checksum: "abc123..."
  register: download

- name: Extract archive
  unarchive:
    src: "/tmp/app.tar.gz"
    dest: "/opt/app"
  when: download.changed
```

### Download and Verify

```yaml
- name: Download binary
  download:
    url: "https://releases.example.com/app/v1.2.3/app"
    dest: "/usr/local/bin/app"
    checksum: "def456..."
    mode: "0755"
  register: app_binary

- name: Verify binary works
  shell: /usr/local/bin/app --version
  when: app_binary.changed
```

## Register Variables

When using `register`, the following variables are available:

- `changed` (boolean): Whether the file was downloaded
- `start_time` (string): Start timestamp
- `end_time` (string): End timestamp
- `duration` (string): Duration of the operation

Example:

```yaml
- name: Download file
  download:
    url: "https://example.com/file.zip"
    dest: "/tmp/file.zip"
  register: result

- name: Show result
  shell: echo "Download changed={{ result.changed }}"
```

## Performance Tips

1. Use `checksum` to enable idempotency and skip unnecessary downloads
2. Set appropriate `timeout` values based on file size
3. Use `retries` for unstable connections
4. Consider downloading to `/tmp` first, then copying to final destination
5. Use `creates` parameter for simple existence checks

## Security Notes

1. Always verify downloads with `checksum` when possible
2. Use HTTPS URLs to prevent man-in-the-middle attacks
3. Store authentication tokens in variables, not hardcoded
4. Be cautious with `force: true` - it always re-downloads
5. Use `backup: true` when overwriting important files

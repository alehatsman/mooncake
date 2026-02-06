# Preset Lifecycle & Registry

This guide covers the preset distribution system, including how to add, manage, and share presets using Mooncake's registry.

## Overview

Mooncake includes a built-in registry system for distributing and managing presets from external sources. The registry provides:

- **Local caching** - Downloaded presets are cached for offline use
- **SHA256 verification** - Integrity checking for all downloaded presets
- **Multiple sources** - Support for URLs, git repositories, and local paths
- **Manifest tracking** - Track installed presets and their sources

## Adding Presets

### From URL

Download a preset from a direct URL:

```bash
mooncake presets add https://example.com/presets/foo.yml
```

The preset is downloaded, verified, and installed to `~/.mooncake/presets/`.

### From Local Path

Install a preset from your local filesystem:

```bash
# Single file
mooncake presets add ./my-presets/custom.yml

# Directory (must contain preset.yml)
mooncake presets add ./my-presets/custom/
```

### From Git Repository (Coming in v2)

Clone a git repository containing presets:

```bash
# Not yet implemented
mooncake presets add https://github.com/user/repo.git
```

## Registry Structure

The registry uses two key directories:

### Cache Directory

Location: `~/.mooncake/cache/presets/`

```
~/.mooncake/cache/presets/
├── manifest.json          # Tracking file
├── abc123.../             # Cached preset (SHA256 hash)
│   └── foo.yml
└── def456.../             # Another preset
    └── bar/
        └── preset.yml
```

The cache stores original preset files organized by their SHA256 hash. This enables:

- **Integrity verification** - Detect tampering or corruption
- **Deduplication** - Multiple sources with same content share one cache entry
- **Offline mode** - Presets available without network access

### User Presets Directory

Location: `~/.mooncake/presets/`

```
~/.mooncake/presets/
├── foo.yml               # Installed preset (flat format)
└── bar/                  # Installed preset (directory format)
    └── preset.yml
```

Presets are copied from cache to the user directory during installation. This is the directory Mooncake searches when loading presets.

## Manifest File

The manifest tracks all presets added through the registry:

**Location**: `~/.mooncake/cache/presets/manifest.json`

```json
{
  "presets": [
    {
      "name": "foo",
      "source": "https://example.com/presets/foo.yml",
      "type": "url",
      "sha256": "abc123...",
      "installed_at": "2026-02-06T12:00:00Z",
      "version": "1.0.0"
    }
  ]
}
```

The manifest enables:

- Tracking preset origin
- Verification of installed presets
- Future update detection (v2)
- Audit trail for compliance

## Managing Presets

### List All Presets

Show all available presets (including registry-installed):

```bash
mooncake presets list
```

Detailed view:

```bash
mooncake presets list --detailed
```

### Show Preset Info

View detailed information about a specific preset:

```bash
mooncake presets info foo
```

### Check Status

Show status of installed presets:

```bash
# All presets
mooncake presets status

# Specific preset
mooncake presets status foo
```

### Uninstall

Remove a preset (runs preset with `state: absent`):

```bash
mooncake presets uninstall foo
```

**Note**: This executes the preset's uninstall logic but does not delete the preset files. To remove preset files, manually delete them from `~/.mooncake/presets/`.

## Preset Search Path Priority

When loading presets, Mooncake searches in this order:

1. `./presets/` - Local project presets (highest priority)
2. `~/.mooncake/presets/` - User presets (registry-installed)
3. `/usr/local/share/mooncake/presets/` - Local installation
4. `/usr/share/mooncake/presets/` - System installation

Local presets always override user and system presets.

## Offline Mode

Once a preset is cached, it's available offline:

1. **First install** - Requires network to download
2. **Subsequent use** - Loaded from `~/.mooncake/presets/`
3. **Cache verification** - SHA256 checked against manifest

The cache ensures your pipelines work even without network access.

## Security Considerations

### SHA256 Verification

All downloaded presets are verified using SHA256 hashes:

- Hash calculated immediately after download
- Stored in manifest for future verification
- Detects tampering or corruption

### Source Validation

When adding presets from URLs:

- **HTTPS recommended** - Use HTTPS URLs for secure transport
- **Verify source** - Only add presets from trusted sources
- **Review content** - Inspect preset files before execution

Example workflow:

```bash
# 1. Download to temporary location
curl -O https://example.com/presets/foo.yml

# 2. Review content
cat foo.yml

# 3. Add from local file
mooncake presets add ./foo.yml

# 4. Clean up
rm foo.yml
```

### Preset Execution

Presets execute with the same permissions as `mooncake run`:

- Steps with `become: true` require sudo password
- File operations respect user permissions
- Network access follows system policies

Always review preset content before installation and execution.

## Examples

### Example 1: Install Public Preset

```bash
# Add preset from GitHub
mooncake presets add https://raw.githubusercontent.com/user/mooncake-presets/main/nginx.yml

# Verify installation
mooncake presets info nginx

# Use in config
cat > mooncake.yml <<EOF
steps:
  - preset:
      name: nginx
      with:
        state: present
EOF

# Execute
mooncake run -c mooncake.yml
```

### Example 2: Share Custom Preset

```bash
# Create custom preset
cat > my-app.yml <<EOF
name: my-app
version: 1.0.0
description: My application deployment

parameters:
  state:
    type: string
    default: present
    enum: [present, absent]

steps:
  - name: Deploy application
    shell: echo "Deploying..."
EOF

# Add to registry
mooncake presets add ./my-app.yml

# Share with team (publish to shared location)
cp ~/.mooncake/presets/my-app.yml /shared/presets/

# Team members add
mooncake presets add /shared/presets/my-app.yml
```

### Example 3: Override Preset Name

```bash
# Add preset with custom name
mooncake presets add --name my-custom-name https://example.com/preset.yml

# Use custom name
mooncake presets info my-custom-name
```

## Preset Formats

The registry supports two preset formats:

### Flat Format

Single YAML file:

```
foo.yml          # Preset definition
```

Use for simple presets without additional files.

### Directory Format

Directory with multiple files:

```
foo/
├── preset.yml   # Preset definition
├── templates/   # Optional templates
│   └── config.j2
└── files/       # Optional static files
    └── script.sh
```

Use for complex presets with templates or supporting files.

## Troubleshooting

### Preset Not Found After Adding

Check search paths:

```bash
# Verify preset exists in user directory
ls ~/.mooncake/presets/

# Check status
mooncake presets status foo
```

### SHA256 Mismatch

If cache verification fails:

```bash
# Remove cached preset
rm -rf ~/.mooncake/cache/presets/<hash>/

# Re-add preset
mooncake presets add <source>
```

### Manifest Corruption

If manifest is corrupted:

```bash
# Backup current manifest
cp ~/.mooncake/cache/presets/manifest.json manifest.json.bak

# Remove manifest (will be recreated empty)
rm ~/.mooncake/cache/presets/manifest.json

# Re-add presets
mooncake presets add <source>
```

## Future Enhancements (v2+)

Planned features for future releases:

### Git Repository Support

```bash
# Clone entire repository of presets
mooncake presets add https://github.com/user/mooncake-presets.git

# Add specific preset from repo
mooncake presets add https://github.com/user/repo.git#presets/foo.yml
```

### Preset Updates

```bash
# Check for updates
mooncake presets update --check

# Update all presets
mooncake presets update

# Update specific preset
mooncake presets update foo
```

### Registry Mirrors

```bash
# Configure registry mirror
mooncake config set registry.mirror https://mirror.example.com/

# Use mirror for all downloads
mooncake presets add foo  # Fetches from mirror
```

### Signed Presets

```bash
# Verify preset signature
mooncake presets add --verify-signature https://example.com/foo.yml

# Trust specific signing key
mooncake presets trust-key <key-id>
```

## Best Practices

1. **Use HTTPS URLs** - Ensure secure transport for downloaded presets
2. **Review content first** - Always inspect presets before installation
3. **Pin versions** - Include version in preset definitions for reproducibility
4. **Document sources** - Track where presets come from in team documentation
5. **Regular audits** - Periodically review installed presets and their sources
6. **Backup manifest** - Include manifest in system backups for disaster recovery

## See Also

- [Preset Authoring Guide](preset-authoring.md) - Creating custom presets
- [Presets Overview](presets.md) - Using presets in configurations
- [Security Best Practices](best-practices.md#security) - General security guidelines

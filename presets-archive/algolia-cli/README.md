# Algolia CLI - Search Platform Interface

Command-line interface for the Algolia search and analytics platform. Manage indices, configure settings, and import data.

## Quick Start
```yaml
- preset: algolia-cli
```

## Features
- **Index management**: Create, update, and delete search indices
- **Data import**: Bulk import JSON data to indices
- **Settings configuration**: Configure ranking, faceting, and filters
- **API key management**: Create and manage API keys
- **Analytics**: Query search analytics and insights
- **Synonyms**: Manage synonym dictionaries
- **Rules**: Configure search rules and merchandising

## Basic Usage
```bash
# List indices
algolia indices list

# Export index
algolia indices export my-index > data.json

# Import data
algolia indices import my-index < data.json

# Search index
algolia indices search my-index --query "search term"

# Configure settings
algolia indices settings my-index --set-settings settings.json
```

## Advanced Configuration
```yaml
- preset: algolia-cli
  with:
    state: present
  become: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated deployment and configuration
- Infrastructure as code workflows
- CI/CD pipeline integration
- Development environment setup
- Production service management

## Uninstall
```yaml
- preset: algolia-cli
  with:
    state: absent
```

## Resources
- Search: "algolia-cli documentation", "algolia-cli tutorial"

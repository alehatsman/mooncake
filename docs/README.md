# Mooncake Documentation

This directory contains the source for the Mooncake documentation website hosted at **https://mooncake.alehatsman.com**

## Quick Start

### Local Development

```bash
# Install dependencies (first time only)
pipenv install

# Start development server
pipenv run mkdocs serve

# Open http://127.0.0.1:8000
```

### Building

```bash
# Build static site
pipenv run mkdocs build

# Output in site/ directory
```

## Structure

```
docs/
├── index.md                    # Homepage
├── getting-started/            # Getting started guides
│   ├── quick-start.md
│   ├── installation.md
│   └── first-config.md
├── guide/                      # User guide
│   ├── core-concepts.md
│   ├── commands.md
│   ├── config/                 # Configuration reference
│   └── best-practices.md
├── examples/                   # Example documentation
├── development/                # Development docs
│   ├── contributing.md
│   ├── roadmap.md
│   ├── releasing.md
│   └── development.md
└── stylesheets/
    └── extra.css              # Custom styles
```

## Adding Content

### Create New Page

1. Create markdown file in appropriate directory
2. Add to `nav` section in `mkdocs.yml`
3. Write content using markdown
4. Preview with `pipenv run mkdocs serve`

### Markdown Features

**Code blocks:**
````markdown
```yaml
- name: Example
  shell: echo "Hello"
```
````

**Admonitions:**
```markdown
!!! note "Optional title"
    Content here

!!! tip
    Helpful tip

!!! warning
    Warning content
```

**Tabs:**
```markdown
=== "Linux"
    Linux instructions

=== "macOS"
    macOS instructions
```

## Deployment

Documentation is automatically deployed to Cloudflare Pages when you push to master.

**Build command:** `pip install pipenv && pipenv install && pipenv run mkdocs build`
**Output directory:** `site`
**URL:** https://mooncake.alehatsman.com

## Contributing

When adding features:
1. Update relevant docs pages
2. Add examples if needed
3. Update navigation in `mkdocs.yml`
4. Test locally with `pipenv run mkdocs serve`
5. Commit and push

## Dependencies

All dependencies are managed via `pipenv`:

```bash
# Update dependencies
pipenv update

# Install new package
pipenv install package-name

# Show dependency tree
pipenv graph
```

## Resources

- [MkDocs Documentation](https://www.mkdocs.org/)
- [Material Theme](https://squidfunk.github.io/mkdocs-material/)
- [pipenv Documentation](https://pipenv.pypa.io/)
- [Contributing Guide](development/contributing.md)

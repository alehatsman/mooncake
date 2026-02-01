# Documentation Directory

This directory contains the source for the Mooncake documentation website.

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
│   └── development.md
└── stylesheets/
    └── extra.css              # Custom styles

```

## Building Locally

### Install MkDocs

```bash
pip install mkdocs-material
```

### Serve Locally

```bash
mkdocs serve
```

Open http://127.0.0.1:8000

### Build Static Site

```bash
mkdocs build
```

Output in `site/` directory.

## Adding Content

### Create New Page

1. Create markdown file in appropriate directory
2. Add to `nav` section in `mkdocs.yml`
3. Write content using markdown

### Add Images

Place in `docs/assets/` and reference:
```markdown
![Alt text](../assets/image.png)
```

### Code Blocks

````markdown
```yaml
- name: Example
  shell: echo "Hello"
```
````

### Admonitions

```markdown
!!! note "Optional title"
    Content here

!!! warning
    Warning content

!!! tip
    Helpful tip
```

### Tabs

```markdown
=== "Linux"
    Linux instructions

=== "macOS"
    macOS instructions
```

## Deployment

Documentation is automatically deployed to Cloudflare Pages when you push to master.

Cloudflare Pages handles:
1. Building the documentation
2. Deploying to the edge network
3. Serving from custom domain

**URL:** https://mooncake.alehatsman.com

## Contributing

When adding features:
1. Update relevant docs pages
2. Add examples if needed
3. Update navigation in `mkdocs.yml`
4. Test locally with `mkdocs serve`

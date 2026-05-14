# pelican - Static Site Generator

Static site generator written in Python that doesn't require a database or server-side logic.

## Quick Start
```yaml
- preset: pelican
```

## Features
- **Simple**: Plain text files (Markdown, reStructuredText) to HTML
- **Flexible**: Jinja2 templates for theming
- **Fast**: Static sites load quickly and scale easily
- **Extensible**: Plugin system for added functionality
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Create new project
pelican-quickstart

# Generate site
pelican content

# Start development server
pelican --listen

# Generate with specific settings
pelican content -s publishconf.py
```

## Advanced Configuration
```yaml
- preset: pelican
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove pelican |

## Platform Support
- ✅ Linux (pip3)
- ✅ macOS (pip3)
- ❌ Windows (not supported)

## Configuration
- **Content directory**: `content/` (Markdown/reStructuredText files)
- **Output directory**: `output/` (Generated HTML)
- **Config file**: `pelicanconf.py` (Site configuration)
- **Publish config**: `publishconf.py` (Production settings)

## Real-World Examples

### Blog Setup
```bash
# Initialize blog
pelican-quickstart

# Create article
cat > content/first-post.md <<EOF
Title: My First Post
Date: 2026-02-06
Category: Blog

This is my first blog post!
EOF

# Generate and serve
pelican content
pelican --listen
```

### Deploy to CI/CD
```yaml
- preset: pelican

- name: Generate static site
  shell: pelican content -s publishconf.py

- name: Deploy to S3
  shell: aws s3 sync output/ s3://my-bucket/ --delete
```

## Agent Use
- Generate documentation sites from Markdown files
- Create blogs and content-heavy sites without databases
- Build static landing pages for rapid deployment
- Convert documentation to HTML for hosting
- Automate content publishing pipelines

## Uninstall
```yaml
- preset: pelican
  with:
    state: absent
```

## Resources
- Official docs: https://docs.getpelican.com/
- GitHub: https://github.com/getpelican/pelican
- Search: "pelican static site", "pelican tutorial"

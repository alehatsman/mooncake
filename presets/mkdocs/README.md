# MkDocs Preset

Install MkDocs - a fast, simple static site generator for building project documentation.

## Quick Start

```yaml
- preset: mkdocs
  with:
    install_material: true
    install_plugins: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `install_material` | bool | `true` | Install Material theme |
| `install_plugins` | bool | `true` | Install popular plugins |
| `create_project` | bool | `false` | Create new project |
| `project_dir` | string | `~/mkdocs-project` | Project directory |
| `site_name` | string | `My Documentation` | Site name |

## Usage

### Basic Installation
```yaml
- preset: mkdocs
```

### With Material Theme
```yaml
- preset: mkdocs
  with:
    install_material: true
```

### Create New Project
```yaml
- preset: mkdocs
  with:
    create_project: true
    project_dir: "~/my-docs"
    site_name: "My Project Documentation"
```

## Common Commands

```bash
# Create new project
mkdocs new my-project
cd my-project

# Start development server
mkdocs serve
# Opens at http://127.0.0.1:8000

# Build static site
mkdocs build
# Output in site/ directory

# Deploy to GitHub Pages
mkdocs gh-deploy
```

## Project Structure

```
my-project/
├── mkdocs.yml      # Configuration file
├── docs/
│   ├── index.md    # Homepage
│   ├── about.md    # About page
│   └── guide.md    # Guide page
└── site/           # Built site (generated)
```

## Configuration (mkdocs.yml)

### Basic Config
```yaml
site_name: My Documentation
site_url: https://example.com
site_author: Your Name

nav:
  - Home: index.md
  - User Guide: guide.md
  - API: api.md
```

### Material Theme
```yaml
theme:
  name: material
  palette:
    scheme: default
    primary: indigo
    accent: indigo
  features:
    - navigation.tabs
    - navigation.sections
    - navigation.expand
    - search.suggest
    - search.highlight
    - content.code.copy
```

### Markdown Extensions
```yaml
markdown_extensions:
  - admonition
  - codehilite
  - pymdownx.highlight
  - pymdownx.superfences
  - pymdownx.tabbed:
      alternate_style: true
  - pymdownx.emoji:
      emoji_index: !!python/name:material.extensions.emoji.twemoji
      emoji_generator: !!python/name:material.extensions.emoji.to_svg
  - toc:
      permalink: true
```

### Plugins
```yaml
plugins:
  - search
  - minify:
      minify_html: true
  - git-revision-date-localized:
      enable_creation_date: true
  - awesome-pages
```

## Markdown Features

### Admonitions
```markdown
!!! note
    This is a note.

!!! warning
    This is a warning.

!!! tip
    This is a tip.
```

### Code Blocks
````markdown
```python
def hello():
    print("Hello, world!")
```
````

### Tabs
```markdown
=== "Python"
    ```python
    print("Hello")
    ```

=== "JavaScript"
    ```javascript
    console.log("Hello");
    ```
```

## Deployment

### GitHub Pages
```bash
# Deploy
mkdocs gh-deploy

# Deploy with clean build
mkdocs gh-deploy --clean
```

### Custom Domain
```yaml
# mkdocs.yml
site_url: https://docs.example.com

# Add docs/CNAME file
echo "docs.example.com" > docs/CNAME
```

### Netlify/Vercel
```yaml
# Build command
mkdocs build

# Publish directory
site
```

## Popular Themes

- **Material** - Modern, feature-rich (recommended)
- **ReadTheDocs** - Classic documentation theme
- **Bootswatch** - Bootstrap-based themes
- **Cinder** - Clean, responsive theme

## Popular Plugins

- **search** - Built-in search
- **minify** - Minify HTML output
- **git-revision-date** - Show last updated dates
- **awesome-pages** - Custom page ordering
- **redirects** - URL redirects
- **macros** - Jinja2 templating
- **pdf-export** - Generate PDF
- **blog** - Blog functionality

## Tips

1. **Live reload**: MkDocs auto-reloads on changes during `mkdocs serve`
2. **Search**: Material theme includes powerful search
3. **Versioning**: Use mike plugin for version management
4. **API docs**: Use mkdocstrings for Python API docs
5. **Internationalization**: Use i18n plugin for translations

## Example Workflow

```bash
# 1. Install MkDocs with Material
pip install mkdocs mkdocs-material

# 2. Create project
mkdocs new my-docs
cd my-docs

# 3. Edit configuration
nano mkdocs.yml

# 4. Write documentation
nano docs/index.md

# 5. Preview locally
mkdocs serve

# 6. Build and deploy
mkdocs build
mkdocs gh-deploy
```

## Uninstall

```yaml
- preset: mkdocs
  with:
    state: absent
```

**Note:** Project files are preserved after uninstall.

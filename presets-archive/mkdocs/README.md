# MkDocs - Fast Static Site Generator for Documentation

Build beautiful project documentation with Markdown, live preview, automatic site generation, and deployment to GitHub Pages.

## Quick Start

```yaml
- preset: mkdocs
```

## Features

- **Markdown-based**: Write documentation in plain Markdown with automatic HTML generation
- **Live preview**: Built-in development server with hot reload during editing
- **Material theme**: Modern, responsive Material Design theme (optional)
- **Search**: Full-text search functionality across entire documentation
- **Plugins**: Extensible plugin system (git-revision-date, minify, PDF export, etc.)
- **GitHub Pages deployment**: One-command deployment to GitHub Pages
- **Responsive design**: Mobile-friendly documentation site
- **SEO friendly**: Proper meta tags, sitemaps, and structured data support

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (`present`) or remove (`absent`) |
| install_material | bool | true | Install Material Design theme |
| install_plugins | bool | true | Install popular plugins (search, minify, git-revision-date) |
| create_project | bool | false | Create new documentation project |
| project_dir | string | ~/mkdocs-project | Directory for new project |
| site_name | string | My Documentation | Site name for new project |

## Basic Usage

```bash
# Create new documentation project
mkdocs new my-project
cd my-project

# Edit configuration and documentation
nano mkdocs.yml
nano docs/index.md

# Start development server with live reload
mkdocs serve
# Opens at http://127.0.0.1:8000

# Build static site (creates site/ directory)
mkdocs build

# Deploy to GitHub Pages
mkdocs gh-deploy

# View help
mkdocs --help
mkdocs serve --help
```

## Advanced Configuration

```yaml
# Full installation with all components
- preset: mkdocs
  with:
    install_material: true     # Modern Material theme
    install_plugins: true       # Plugins: search, minify, git-revision-date
    create_project: true        # Create new project
    project_dir: "~/my-docs"    # Project location
    site_name: "My Docs"        # Site title
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

## Configuration

### mkdocs.yml Settings

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

## Real-World Examples

### Technical Documentation Site

```bash
# Create and setup documentation
mkdocs new api-docs
cd api-docs

# Configure mkdocs.yml with Material theme
cat > mkdocs.yml << 'EOF'
site_name: API Documentation
site_url: https://docs.example.com
theme:
  name: material
nav:
  - Home: index.md
  - Getting Started: guide/setup.md
  - API Reference: api/overview.md
  - Examples: examples/basic.md
EOF

# Create documentation structure
mkdir -p docs/guide docs/api docs/examples
echo "# API Docs" > docs/index.md

# Start local development
mkdocs serve
```

### Team Documentation with GitHub Pages

```yaml
# Deploy team documentation from Mooncake
- preset: mkdocs
  with:
    create_project: true
    project_dir: ~/team-docs
    site_name: "Team Handbook"

- name: Initialize git and deploy
  shell: |
    cd ~/team-docs
    git init
    git add .
    git commit -m "Initial documentation"
    mkdocs gh-deploy
  when: os == "linux"
```

### Documentation with Custom Theme

```yaml
# Install MkDocs and configure with ReadTheDocs theme
- preset: mkdocs
  with:
    install_plugins: true

- name: Configure ReadTheDocs theme
  file:
    path: ~/my-docs/mkdocs.yml
    content: |
      site_name: My Project
      theme: readthedocs
      plugins:
        - search
        - minify
```

## Agent Use

- Automated documentation site generation from project repositories
- Continuous documentation deployment to GitHub Pages in CI/CD pipelines
- Building multi-version documentation with version switching
- Extracting and formatting code documentation (Python, JavaScript, etc.)
- Creating team handbooks and process documentation
- API reference generation from OpenAPI/Swagger specifications

## Platform Support

- ✅ Linux (Python 3.6+, pip3)
- ✅ macOS (Homebrew, Python 3.6+)
- ❌ Windows (not yet supported)

## Troubleshooting

### Module not found errors

Ensure all dependencies are installed:

```bash
# Reinstall MkDocs and plugins
pip3 install mkdocs mkdocs-material mkdocs-minify
```

### Theme not available

```bash
# Check installed themes
pip3 list | grep mkdocs

# Install specific theme
pip3 install mkdocs-material
```

### Port 8000 already in use

```bash
# Use different port
mkdocs serve -a localhost:8001

# Or find and kill process using port
lsof -ti:8000 | xargs kill -9
```

### Build fails with encoding errors

```bash
# Ensure UTF-8 encoding
export LANG=en_US.UTF-8
mkdocs build
```

### Live reload not working

Increase file watcher limit:

```bash
echo "fs.inotify.max_user_watches=524288" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

## Uninstall

```yaml
- preset: mkdocs
  with:
    state: absent
```

This removes MkDocs and dependencies. Project files and documentation are preserved.

## Resources

- Official docs: https://www.mkdocs.org/
- Material theme: https://squidfunk.github.io/mkdocs-material/
- GitHub: https://github.com/mkdocs/mkdocs
- Plugins: https://www.mkdocs.org/user-guide/configuration/#plugins
- Search: "mkdocs getting started", "mkdocs material theme", "mkdocs deployment"

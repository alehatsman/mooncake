# Hugo - The World's Fastest Static Site Generator

Build websites fast with the world's fastest static site generator written in Go.

## Quick Start
```yaml
- preset: hugo
```

## Features
- **Blazing fast**: Builds sites in milliseconds
- **Powerful content management**: Flexible content organization with archetypes
- **Beautiful themes**: Hundreds of open-source themes
- **Multilingual support**: Build sites in multiple languages
- **Shortcodes**: Extend Markdown with custom HTML snippets
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Create new site
hugo new site mysite
cd mysite

# Add theme
git init
git submodule add https://github.com/theNewDynamic/gohugo-theme-ananke themes/ananke
echo "theme = 'ananke'" >> hugo.toml

# Create new content
hugo new posts/my-first-post.md

# Start development server
hugo server -D

# Build static site
hugo

# Build for production
hugo --minify
```

## Project Structure
```
mysite/
├── archetypes/          # Content templates
├── assets/              # Files to be processed (SASS, JS)
├── content/             # All content (Markdown files)
│   ├── posts/
│   ├── about.md
│   └── _index.md
├── data/                # Data files (JSON, YAML, TOML)
├── layouts/             # Custom templates
├── static/              # Static files (images, CSS, JS)
├── themes/              # Installed themes
├── hugo.toml            # Configuration file
└── public/              # Generated site (gitignored)
```

## Creating Content

### New Post
```bash
# Create post
hugo new posts/hello-world.md

# Content file front matter
---
title: "Hello World"
date: 2024-01-15T10:00:00Z
draft: true
tags: ["hugo", "blog"]
categories: ["Tutorial"]
---

Your content here...
```

### Content Organization
```bash
# Blog posts
hugo new posts/my-post.md

# Documentation
hugo new docs/getting-started.md

# Pages
hugo new about.md
```

## Configuration

### hugo.toml (or config.toml)
```toml
baseURL = "https://example.com/"
languageCode = "en-us"
title = "My Hugo Site"
theme = "ananke"

[params]
  description = "My awesome website"
  author = "Your Name"

[menu]
  [[menu.main]]
    name = "Home"
    url = "/"
    weight = 1
  [[menu.main]]
    name = "Posts"
    url = "/posts/"
    weight = 2
  [[menu.main]]
    name = "About"
    url = "/about/"
    weight = 3
```

## Themes

### Install Theme
```bash
# As Git submodule
cd mysite
git submodule add https://github.com/user/theme themes/theme-name

# Update hugo.toml
theme = "theme-name"

# Preview
hugo server
```

### Popular Themes
- **PaperMod**: Fast, clean, minimal blog theme
- **Stack**: Card-style blog theme
- **Ananke**: Default Hugo theme, responsive
- **Docsy**: Technical documentation theme
- **Book**: Documentation theme with sidebar

### Theme Customization
```bash
# Override theme layouts
layouts/
  └── _default/
      └── single.html    # Override single page template

# Override theme assets
assets/
  └── css/
      └── custom.css     # Add custom CSS
```

## Advanced Configuration
```yaml
- preset: hugo
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Hugo |

## Real-World Examples

### Blog with Multiple Authors
```toml
# hugo.toml
[params]
  [params.author]
    name = "John Doe"
    email = "john@example.com"

# content/posts/post1.md
---
title: "My Post"
author: "Jane Smith"
---
```

### Multilingual Site
```toml
# hugo.toml
defaultContentLanguage = "en"

[languages]
  [languages.en]
    languageName = "English"
    weight = 1
  [languages.fr]
    languageName = "Français"
    weight = 2
  [languages.es]
    languageName = "Español"
    weight = 3
```

### Documentation Site
```bash
# Install documentation theme
git submodule add https://github.com/google/docsy themes/docsy

# Create documentation structure
hugo new docs/_index.md
hugo new docs/getting-started/_index.md
hugo new docs/tutorials/_index.md
hugo new docs/api/_index.md
```

## Shortcodes

### Built-in Shortcodes
```markdown
<!-- YouTube video -->
{{< youtube id="dQw4w9WgXcQ" >}}

<!-- Figure with caption -->
{{< figure src="/images/sunset.jpg" title="Beautiful Sunset" >}}

<!-- Highlight code -->
{{< highlight go >}}
package main
func main() {
    println("Hello, Hugo!")
}
{{< /highlight >}}

<!-- Gist embed -->
{{< gist username gist-id >}}
```

### Custom Shortcodes
```html
<!-- layouts/shortcodes/note.html -->
<div class="note">
    {{ .Inner }}
</div>

<!-- Usage in content -->
{{< note >}}
This is an important note!
{{< /note >}}
```

## Deployment

### GitHub Pages
```yaml
# .github/workflows/hugo.yml
name: Deploy Hugo site to Pages

on:
  push:
    branches: ["main"]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          submodules: true
      - name: Setup Hugo
        uses: peaceiris/actions-hugo@v2
        with:
          hugo-version: 'latest'
          extended: true
      - name: Build
        run: hugo --minify
      - name: Deploy
        uses: peaceiris/actions-gh-pages@v3
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: ./public
```

### Netlify
```toml
# netlify.toml
[build]
  publish = "public"
  command = "hugo --gc --minify"

[build.environment]
  HUGO_VERSION = "0.120.0"

[context.production.environment]
  HUGO_ENV = "production"
```

### Vercel
```json
{
  "build": {
    "env": {
      "HUGO_VERSION": "0.120.0"
    }
  },
  "builds": [
    {
      "src": "package.json",
      "use": "@vercel/static-build",
      "config": {
        "distDir": "public"
      }
    }
  ]
}
```

## Asset Processing

### SASS/SCSS
```html
<!-- layouts/partials/head.html -->
{{ $style := resources.Get "sass/main.scss" | resources.ToCSS | resources.Minify }}
<link rel="stylesheet" href="{{ $style.Permalink }}">
```

### JavaScript Bundling
```html
{{ $js := resources.Get "js/main.js" | js.Build | minify }}
<script src="{{ $js.Permalink }}"></script>
```

### Image Processing
```html
{{ $image := resources.Get "images/sunset.jpg" }}
{{ $resized := $image.Resize "800x" }}
<img src="{{ $resized.Permalink }}" alt="Sunset">
```

## SEO Optimization
```toml
# hugo.toml
[params]
  description = "Site description"
  images = ["images/og-image.jpg"]

[sitemap]
  changefreq = "monthly"
  priority = 0.5

[taxonomies]
  tag = "tags"
  category = "categories"
```

## Performance Tips
```bash
# Enable caching
hugo --gc

# Minify output
hugo --minify

# Parallel processing
hugo --parallel

# Fast render during development
hugo server --disableFastRender=false
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported in preset)

## Agent Use
- Generate static documentation sites
- Build marketing websites programmatically
- Create blog platforms
- Deploy knowledge bases
- Generate project documentation

## Troubleshooting

### Build errors
```bash
# Verbose output
hugo --verbose

# Debug mode
hugo --debug

# Check configuration
hugo config
```

### Theme not working
```bash
# Verify theme installed
ls themes/

# Check theme name in config
grep theme hugo.toml

# Update theme
git submodule update --remote themes/theme-name
```

### Slow build times
```bash
# Enable caching
hugo --gc

# Check content size
find content/ -type f | wc -l

# Profile build
hugo --profile
```

## Uninstall
```yaml
- preset: hugo
  with:
    state: absent
```

## Resources
- Official docs: https://gohugo.io/documentation/
- Themes: https://themes.gohugo.io/
- GitHub: https://github.com/gohugoio/hugo
- Forum: https://discourse.gohugo.io/
- Search: "hugo tutorial", "hugo static site"

# Jekyll - Static Site Generator

Transform plain text into static websites and blogs using Markdown, Liquid templates, and YAML front matter.

## Quick Start
```yaml
- preset: jekyll
```

## Features
- **Blog-aware**: Built-in support for permalinks, categories, tags, and drafts
- **Markdown**: Write content in Markdown with custom extensions
- **Liquid templates**: Powerful templating with includes and filters
- **Themes**: Extensive theme ecosystem with easy customization
- **GitHub Pages**: Native hosting integration
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Create new site
jekyll new my-blog
cd my-blog

# Serve locally with live reload
jekyll serve
# Site available at http://localhost:4000

# Build for production
jekyll build
# Output in _site/ directory

# Serve with drafts
jekyll serve --drafts

# Incremental builds (faster)
jekyll serve --incremental
```

## Advanced Configuration
```yaml
- preset: jekyll
  with:
    version: "4.3.0"          # Specific Jekyll version
    install_bundler: true     # Install Bundler for dependency management
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Jekyll |
| version | string | latest | Jekyll version (e.g., "4.3.0", "latest") |
| install_bundler | bool | true | Install Bundler for Gemfile management |

## Site Structure
```
my-blog/
├── _config.yml           # Site configuration
├── _posts/              # Blog posts
│   └── 2024-01-01-title.md
├── _drafts/             # Unpublished posts
├── _layouts/            # Page templates
│   ├── default.html
│   └── post.html
├── _includes/           # Reusable components
│   ├── header.html
│   └── footer.html
├── _data/               # Data files (YAML, JSON, CSV)
│   └── navigation.yml
├── assets/              # CSS, JS, images
│   ├── css/
│   └── images/
└── _site/               # Generated site (git ignore)
```

## Configuration
```yaml
# _config.yml
title: My Blog
description: A blog about technology
url: "https://example.com"
baseurl: ""

# Build settings
markdown: kramdown
theme: minima
plugins:
  - jekyll-feed
  - jekyll-seo-tag
  - jekyll-sitemap

# Collections
collections:
  projects:
    output: true
    permalink: /projects/:name

# Exclude from build
exclude:
  - Gemfile
  - Gemfile.lock
  - node_modules/
  - vendor/
```

## Creating Content

### Blog Post
```markdown
---
layout: post
title: "My First Post"
date: 2024-01-01 10:00:00 -0800
categories: [tutorial, jekyll]
tags: [static-sites, blogging]
---

# Hello World

This is my first Jekyll post with **Markdown** formatting.

{% highlight ruby %}
def hello
  puts "Hello, Jekyll!"
end
{% endhighlight %}
```

### Custom Page
```markdown
---
layout: default
title: About
permalink: /about/
---

# About This Site

Built with Jekyll and hosted on GitHub Pages.
```

### Data-Driven Pages
```yaml
# _data/team.yml
- name: Alice
  role: Developer
  github: alice
- name: Bob
  role: Designer
  github: bob
```

```liquid
{% for member in site.data.team %}
  <h3>{{ member.name }}</h3>
  <p>{{ member.role }}</p>
  <a href="https://github.com/{{ member.github }}">GitHub</a>
{% endfor %}
```

## Real-World Examples

### Documentation Site
```yaml
# _config.yml for docs
collections:
  docs:
    output: true
    permalink: /docs/:path/

defaults:
  - scope:
      path: ""
      type: docs
    values:
      layout: documentation
      sidebar: true
```

### Portfolio Site
```yaml
# _config.yml for portfolio
collections:
  projects:
    output: true
    permalink: /projects/:name/

defaults:
  - scope:
      type: projects
    values:
      layout: project
```

### Multi-Language Site
```yaml
# _config.yml
languages: ["en", "es", "fr"]
default_lang: "en"

plugins:
  - jekyll-multiple-languages
```

## Themes and Customization

### Install Theme
```ruby
# Gemfile
gem "minima", "~> 2.5"
gem "jekyll-theme-cayman"
```

```yaml
# _config.yml
theme: minima
```

### Override Theme Files
```bash
# Copy theme files to customize
bundle info --path minima
cp -r $(bundle info --path minima)/_layouts .
cp -r $(bundle info --path minima)/_includes .
```

## Plugins
```ruby
# Gemfile
group :jekyll_plugins do
  gem "jekyll-feed"           # RSS/Atom feeds
  gem "jekyll-seo-tag"        # SEO meta tags
  gem "jekyll-sitemap"        # XML sitemap
  gem "jekyll-paginate"       # Blog pagination
  gem "jekyll-archives"       # Category/tag archives
  gem "jekyll-redirect-from"  # Page redirects
end
```

## Deployment

### GitHub Pages
```yaml
# _config.yml
url: "https://username.github.io"
baseurl: "/repo-name"
```

### Build and Deploy
```bash
# Build production site
JEKYLL_ENV=production bundle exec jekyll build

# Deploy to any static host
rsync -avz _site/ user@server:/var/www/html/
```

### Netlify
```toml
# netlify.toml
[build]
  command = "bundle exec jekyll build"
  publish = "_site"

[build.environment]
  RUBY_VERSION = "3.1.0"
```

## Performance Optimization
```yaml
# _config.yml
sass:
  style: compressed

# Exclude development files
exclude:
  - README.md
  - Gemfile
  - Gemfile.lock
  - node_modules/
  - vendor/

# Enable incremental builds
incremental: true

# Limit posts in development
limit_posts: 10
```

## Agent Use
- Generate documentation sites from markdown files
- Build and deploy static marketing sites
- Create blog platforms with automated content publishing
- Generate project portfolio sites
- Build multi-language documentation portals
- Automated site regeneration on content changes

## Troubleshooting

### Dependency issues
```bash
bundle update
bundle install
```

### Port already in use
```bash
jekyll serve --port 4001
```

### Clean build
```bash
jekyll clean
jekyll build
```

### Debug mode
```bash
jekyll serve --verbose --trace
```

## Uninstall
```yaml
- preset: jekyll
  with:
    state: absent
```

## Resources
- Official docs: https://jekyllrb.com/docs/
- Themes: https://jekyllrb.com/docs/themes/
- Plugins: https://jekyllrb.com/docs/plugins/
- GitHub: https://github.com/jekyll/jekyll
- Search: "jekyll tutorial", "jekyll themes", "jekyll github pages"

## Platform Support
- ✅ Linux (Ruby via package manager or rbenv)
- ✅ macOS (Ruby via Homebrew or rbenv)
- ❌ Windows (not yet supported)

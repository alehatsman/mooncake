# Hexo - Fast and Simple Blog Framework

A fast, simple, and powerful Node.js-based blog framework for creating static websites.

## Quick Start
```yaml
- preset: hexo
```

## Features
- **Blazing fast**: Generates hundreds of pages in seconds
- **Markdown support**: Write posts in Markdown with front-matter
- **One-command deployment**: Deploy to GitHub Pages, Netlify, Vercel
- **Plugin ecosystem**: Extend functionality with hundreds of plugins
- **Theme system**: Beautiful responsive themes
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Create new blog
hexo init myblog
cd myblog
npm install

# Create new post
hexo new "My First Post"

# Generate static files
hexo generate

# Start local server
hexo server

# Deploy
hexo deploy
```

## Project Structure
```
myblog/
├── _config.yml          # Site configuration
├── package.json         # Dependencies
├── scaffolds/           # Post templates
├── source/              # Source files
│   ├── _posts/         # Blog posts
│   └── about/          # Pages
├── themes/              # Themes
└── public/              # Generated files (gitignored)
```

## Creating Content

### New Post
```bash
# Create post
hexo new post "My Post Title"

# Create draft
hexo new draft "Work in Progress"

# Publish draft
hexo publish draft "Work in Progress"

# Create page
hexo new page about
```

### Post Front Matter
```markdown
---
title: Hello World
date: 2024-01-15 10:00:00
tags: [hexo, blog]
categories: [Tutorial]
---

Post content here...
```

## Configuration

### _config.yml Essentials
```yaml
# Site
title: My Blog
subtitle: 'Thoughts and musings'
description: 'Personal blog about tech'
author: Your Name
language: en
timezone: 'America/New_York'

# URL
url: https://yourblog.com
root: /

# Writing
new_post_name: :title.md
default_layout: post
auto_spacing: true

# Deployment
deploy:
  type: git
  repo: https://github.com/username/username.github.io.git
  branch: main
```

## Themes

### Install Theme
```bash
# Clone theme
cd themes/
git clone https://github.com/theme/repo theme-name

# Configure in _config.yml
theme: theme-name

# Generate and preview
hexo clean
hexo generate
hexo server
```

### Popular Themes
- **NexT**: Feature-rich, elegant theme
- **Landscape**: Default minimal theme
- **Cactus**: Clean, responsive design
- **Fluid**: Material Design inspired
- **Inside**: Card-based layout

## Advanced Configuration
```yaml
- preset: hexo
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Hexo CLI |

## Real-World Examples

### Blog with Custom Domain
```yaml
# Create and deploy blog
- name: Initialize Hexo blog
  shell: |
    hexo init blog
    cd blog
    npm install

- name: Configure site
  template:
    src: _config.yml.j2
    dest: blog/_config.yml

- name: Add CNAME file
  copy:
    content: "blog.example.com"
    dest: blog/source/CNAME

- name: Generate and deploy
  shell: |
    cd blog
    hexo generate
    hexo deploy
```

### Multi-language Blog
```yaml
# _config.yml
language:
  - en
  - zh-CN
  - ja

i18n:
  type: [page, post]
  generator: [index, archive, category, tag]
```

### SEO Optimization
```bash
# Install SEO plugin
npm install hexo-generator-sitemap --save
npm install hexo-generator-feed --save

# Add to _config.yml
sitemap:
  path: sitemap.xml

feed:
  type: atom
  path: atom.xml
  limit: 20
```

## Deployment

### GitHub Pages
```yaml
# _config.yml
deploy:
  type: git
  repo: https://github.com/username/username.github.io.git
  branch: main
```

```bash
# Install deployer
npm install hexo-deployer-git --save

# Deploy
hexo clean && hexo deploy
```

### Netlify
```yaml
# netlify.toml
[build]
  command = "hexo generate"
  publish = "public"

[build.environment]
  NODE_VERSION = "18"
```

### Vercel
```json
{
  "buildCommand": "hexo generate",
  "outputDirectory": "public"
}
```

## Plugins

### Essential Plugins
```bash
# Search
npm install hexo-generator-search --save

# Sitemap
npm install hexo-generator-sitemap --save

# RSS Feed
npm install hexo-generator-feed --save

# Image optimization
npm install hexo-imagemin --save

# Table of contents
npm install hexo-toc --save
```

### Configure Plugins
```yaml
# _config.yml
search:
  path: search.xml
  field: post

toc:
  maxdepth: 3
```

## Writing Workflow
```bash
# Create draft
hexo new draft "New Ideas"

# Preview drafts locally
hexo server --draft

# Publish when ready
hexo publish draft "New Ideas"

# Generate and deploy
hexo clean
hexo generate
hexo deploy
```

## Platform Support
- ✅ Linux (npm)
- ✅ macOS (npm, Homebrew)
- ❌ Windows (not yet supported in preset)

## Agent Use
- Generate static blog sites programmatically
- Automate content publishing workflows
- Deploy documentation sites
- Create marketing landing pages
- Build knowledge bases

## Troubleshooting

### Port already in use
```bash
# Use different port
hexo server -p 5000

# Kill process on default port
lsof -ti:4000 | xargs kill
```

### Theme not applied
```bash
# Clean and regenerate
hexo clean
rm -rf public/
hexo generate

# Verify theme name in _config.yml
grep theme _config.yml
```

### Deployment fails
```bash
# Verify git deployer installed
npm list hexo-deployer-git

# Check repo URL
git remote -v

# Manual deploy
cd public/
git init
git add -A
git commit -m "Deploy"
git push -f <repo> main
```

## Uninstall
```yaml
- preset: hexo
  with:
    state: absent
```

## Resources
- Official docs: https://hexo.io/docs/
- Themes: https://hexo.io/themes/
- Plugins: https://hexo.io/plugins/
- GitHub: https://github.com/hexojs/hexo
- Search: "hexo tutorial", "hexo blog deployment"

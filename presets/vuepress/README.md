# VuePress - Vue-Powered Static Site Generator

Minimalistic static site generator with Vue-powered theming, optimized for technical documentation.

## Quick Start
```yaml
- preset: vuepress
```

## Features
- **Vue-Powered**: Write content in Markdown with Vue components
- **Fast**: Pre-rendering for instant page loads, SPA navigation
- **SEO Friendly**: Server-side rendered with meta tags support
- **Plugin System**: Extensible with rich plugin ecosystem
- **Theme Customization**: Built-in default theme or custom themes
- **Internationalization**: Multi-language support out of the box

## Basic Usage
```bash
# Create new VuePress site
mkdir my-docs && cd my-docs
npm init
npm install -D vuepress@next

# Add scripts to package.json
# "docs:dev": "vuepress dev docs"
# "docs:build": "vuepress build docs"

# Create docs directory and first page
mkdir docs
echo '# Hello VuePress' > docs/README.md

# Start development server
npm run docs:dev

# Build for production
npm run docs:build
```

## Advanced Configuration
```yaml
- preset: vuepress
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove VuePress |

## Platform Support
- ✅ Linux (npm/yarn/pnpm)
- ✅ macOS (npm/yarn/pnpm)
- ✅ Windows (npm/yarn/pnpm)

## Configuration
- **Config file**: `docs/.vuepress/config.js` or `config.ts`
- **Default dev port**: 8080
- **Build output**: `docs/.vuepress/dist/`
- **Node.js requirement**: 14+ or 16+

## Project Structure

```
my-docs/
├── docs/
│   ├── .vuepress/
│   │   ├── config.js       # Site config
│   │   ├── public/         # Static assets
│   │   ├── styles/         # Custom styles
│   │   └── components/     # Custom Vue components
│   ├── README.md           # Home page
│   ├── guide/
│   │   ├── README.md       # /guide/
│   │   └── getting-started.md  # /guide/getting-started.html
│   └── api/
│       └── README.md       # /api/
└── package.json
```

## Configuration File

Basic `.vuepress/config.js`:
```javascript
module.exports = {
  title: 'My Documentation',
  description: 'Awesome documentation site',

  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/' },
      { text: 'API', link: '/api/' }
    ],

    sidebar: {
      '/guide/': [
        {
          title: 'Guide',
          collapsable: false,
          children: [
            '',
            'getting-started',
            'configuration'
          ]
        }
      ]
    },

    repo: 'username/repo',
    editLinks: true,
    editLinkText: 'Edit this page on GitHub'
  },

  plugins: [
    '@vuepress/plugin-back-to-top',
    '@vuepress/plugin-medium-zoom'
  ]
}
```

## Real-World Examples

### Documentation Site
```bash
# Install VuePress
npm install -D vuepress@next

# Create documentation structure
mkdir -p docs/.vuepress
cat > docs/.vuepress/config.js <<'EOF'
module.exports = {
  title: 'Project Documentation',
  description: 'Complete project documentation',

  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'API', link: '/api/' }
    ],
    sidebar: 'auto'
  }
}
EOF

# Write content
echo '# Home' > docs/README.md
mkdir docs/guide
echo '# Getting Started' > docs/guide/README.md

# Develop
npm run docs:dev
```

### CI/CD Deployment
```yaml
- name: Install VuePress
  preset: vuepress

- name: Install dependencies
  shell: npm ci
  cwd: /app

- name: Build documentation
  shell: npm run docs:build
  cwd: /app

- name: Deploy to web server
  copy:
    src: /app/docs/.vuepress/dist/
    dest: /var/www/html/docs/
  become: true
```

### GitHub Pages Deployment
```bash
# Create deploy script
cat > deploy.sh <<'EOF'
#!/bin/bash
set -e

npm run docs:build

cd docs/.vuepress/dist
git init
git add -A
git commit -m 'Deploy documentation'
git push -f git@github.com:username/repo.git master:gh-pages

cd -
EOF

chmod +x deploy.sh
./deploy.sh
```

### Custom Theme
```javascript
// .vuepress/config.js
module.exports = {
  theme: '@vuepress/theme-default',

  themeConfig: {
    logo: '/logo.png',

    // Custom navbar
    nav: [
      { text: 'Home', link: '/' },
      {
        text: 'Learn',
        items: [
          { text: 'Guide', link: '/guide/' },
          { text: 'Tutorial', link: '/tutorial/' }
        ]
      }
    ],

    // Sidebar groups
    sidebar: {
      '/guide/': [
        {
          title: 'Getting Started',
          children: ['', 'installation', 'quick-start']
        },
        {
          title: 'Advanced',
          children: ['configuration', 'theming', 'plugins']
        }
      ]
    },

    // Search
    search: true,
    searchMaxSuggestions: 10,

    // Last updated
    lastUpdated: 'Last Updated',

    // Repo links
    repo: 'username/repo',
    docsDir: 'docs',
    editLinks: true
  }
}
```

## Markdown Extensions

VuePress extends Markdown with:

```markdown
# YAML Front Matter
---
title: Page Title
lang: en-US
---

# Table of Contents
[[toc]]

# Custom Containers
::: tip
This is a tip
:::

::: warning
This is a warning
:::

::: danger
This is dangerous
:::

# Code Blocks with Highlighting
```js{2,4-6}
export default {
  data () {  // highlighted
    return {
      msg: 'Hello'  // highlighted
    }  // highlighted
  }
}
```

# Line Numbers
```js:line-numbers
console.log('Line 1')
console.log('Line 2')
```

# Import Code Snippets
<<< @/filepath/file.js

# Vue Components in Markdown
<Badge text="beta" type="warning"/>
<CustomComponent :prop="value"/>
```

## Agent Use
- Automated documentation generation and deployment
- API reference documentation generation
- Product documentation management
- Technical blog platform
- Knowledge base creation
- Multi-language documentation sites

## Troubleshooting

### Port already in use
```bash
# Use different port
vuepress dev docs --port 8081

# Or in config.js
module.exports = {
  port: 8081
}
```

### Build errors
```bash
# Clear cache
rm -rf docs/.vuepress/.cache docs/.vuepress/.temp

# Rebuild
npm run docs:build
```

### Slow development server
```bash
# Disable some plugins during development
# Or use vite mode (VuePress 2+)
npm install -D @vuepress/cli@next
vuepress dev docs
```

## Uninstall
```yaml
- preset: vuepress
  with:
    state: absent
```

## Resources
- Official docs: https://vuepress.vuejs.org/
- GitHub: https://github.com/vuejs/vuepress
- Search: "vuepress documentation", "vuepress tutorial"

# docusaurus - Documentation Site Generator

Modern static site generator built with React, optimized for documentation websites.

## Quick Start
```yaml
- preset: docusaurus
```

## Features
- **React-powered**: Build interactive documentation sites
- **MDX support**: Use JSX components in Markdown
- **Versioning**: Maintain multiple documentation versions
- **i18n**: Built-in internationalization support
- **Search**: Integrated Algolia DocSearch
- **Fast**: Optimized for performance with code splitting

## Basic Usage
```bash
# Create new site
npx create-docusaurus@latest my-website classic

# Start development server
cd my-website
npm start

# Build for production
npm run build

# Serve production build
npm run serve

# Deploy to GitHub Pages
USE_SSH=true npm run deploy
```

## Project Structure
```
my-website/
├── blog/                   # Blog posts (optional)
├── docs/                   # Documentation Markdown files
├── src/
│   ├── components/        # React components
│   ├── css/              # Custom CSS
│   └── pages/            # Custom pages
├── static/               # Static assets
├── docusaurus.config.js  # Site configuration
├── sidebars.js           # Sidebar navigation
└── package.json
```

## Advanced Configuration
```yaml
# Install docusaurus globally
- preset: docusaurus

# Uninstall
- preset: docusaurus
  with:
    state: absent
```

## Configuration File
```javascript
// docusaurus.config.js
module.exports = {
  title: 'My Site',
  tagline: 'Documentation made easy',
  url: 'https://example.com',
  baseUrl: '/',
  organizationName: 'myorg',
  projectName: 'my-site',

  themeConfig: {
    navbar: {
      title: 'My Site',
      logo: {
        alt: 'Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'doc',
          docId: 'intro',
          position: 'left',
          label: 'Docs',
        },
        {to: '/blog', label: 'Blog', position: 'left'},
        {
          href: 'https://github.com/myorg/my-site',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [{label: 'Tutorial', to: '/docs/intro'}],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} My Project.`,
    },
  },

  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
        },
        blog: {
          showReadingTime: true,
        },
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      },
    ],
  ],
};
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (npm/npx)
- ✅ macOS (npm/npx)
- ✅ Windows (npm/npx)

## Real-World Examples

### API Documentation Site
```markdown
---
id: intro
title: Getting Started
slug: /
---

# Welcome to Our API

Get started with our REST API in minutes.

## Quick Start

```bash
curl -X POST https://api.example.com/v1/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"name": "John Doe"}'
```

## Authentication

All API requests require an API key...
```

### Versioned Documentation
```bash
# Create version 1.0
npm run docusaurus docs:version 1.0

# Structure:
# docs/          (latest, in development)
# versioned_docs/
#   version-1.0/
# versions.json
```

### Multi-Language Support
```javascript
// docusaurus.config.js
module.exports = {
  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'fr', 'es'],
  },
};
```

```bash
# Generate translations
npm run write-translations -- --locale fr

# Build with locale
npm run build -- --locale fr
```

### Custom React Components
```jsx
// src/components/Button.js
import React from 'react';

export default function Button({children, href}) {
  return (
    <a href={href} className="button button--primary">
      {children}
    </a>
  );
}
```

```mdx
<!-- docs/example.md -->
import Button from '@site/src/components/Button';

# Example Page

<Button href="/docs/next-page">Get Started</Button>
```

## Deployment Options

### GitHub Pages
```bash
# package.json
{
  "scripts": {
    "deploy": "docusaurus deploy"
  }
}

# Deploy
USE_SSH=true npm run deploy
```

### Netlify
```toml
# netlify.toml
[build]
  command = "npm run build"
  publish = "build"

[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200
```

### Vercel
```json
{
  "buildCommand": "npm run build",
  "outputDirectory": "build",
  "framework": "docusaurus"
}
```

## Algolia DocSearch
```javascript
// docusaurus.config.js
module.exports = {
  themeConfig: {
    algolia: {
      apiKey: 'YOUR_API_KEY',
      indexName: 'YOUR_INDEX_NAME',
      appId: 'YOUR_APP_ID',
    },
  },
};
```

## Plugins

### Install Plugin
```bash
npm install --save @docusaurus/plugin-content-pages
```

### Configure Plugin
```javascript
// docusaurus.config.js
module.exports = {
  plugins: [
    [
      '@docusaurus/plugin-content-docs',
      {
        id: 'community',
        path: 'community',
        routeBasePath: 'community',
        sidebarPath: require.resolve('./sidebarsCommunity.js'),
      },
    ],
  ],
};
```

## Themes

### Install Theme
```bash
npm install --save @docusaurus/theme-live-codeblock
```

### Configure Theme
```javascript
module.exports = {
  themes: ['@docusaurus/theme-live-codeblock'],
  themeConfig: {
    liveCodeBlock: {
      playgroundPosition: 'bottom',
    },
  },
};
```

## Agent Use
- Generate documentation sites for APIs and SDKs
- Create developer portals
- Build knowledge bases
- Publish technical documentation
- Maintain versioned documentation
- Deploy multilingual documentation sites

## Troubleshooting

### Build errors
```bash
# Clear cache
npm run clear

# Rebuild
npm run build
```

### Port in use
```bash
# Use different port
npm start -- --port 3001
```

### Broken links
```bash
# Check for broken links
npm run build -- --no-minify
```

## Uninstall
```yaml
- preset: docusaurus
  with:
    state: absent
```

## Resources
- Official docs: https://docusaurus.io/docs
- GitHub: https://github.com/facebook/docusaurus
- Search: "docusaurus tutorial", "docusaurus examples"

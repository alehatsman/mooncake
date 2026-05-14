# eleventy - Static Site Generator

Simpler static site generator with powerful template options.

## Quick Start
```yaml
- preset: eleventy
```

## Features
- **Flexible templating**: Markdown, Liquid, Nunjucks, Handlebars, and more
- **Zero config**: Works out of the box
- **Data cascade**: Powerful data management
- **Hot reload**: Instant browser refresh
- **Collections**: Group and filter content
- **Plugins**: Extensible architecture

## Basic Usage
```bash
# Create project
mkdir my-site && cd my-site
npm init -y
npm install @11ty/eleventy --save-dev

# Create content
echo '# Hello World' > index.md

# Start dev server
npx @11ty/eleventy --serve

# Build for production
npx @11ty/eleventy
```

## Project Structure
```
my-site/
├── _includes/           # Layouts and partials
│   └── base.njk
├── _data/              # Global data files
│   └── site.json
├── posts/              # Blog posts
│   ├── post-1.md
│   └── post-2.md
├── index.md            # Home page
├── .eleventy.js        # Configuration
└── _site/              # Output directory
```

## Platform Support
- ✅ Linux (npm)
- ✅ macOS (npm)
- ✅ Windows (npm)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Configuration
```javascript
// .eleventy.js
module.exports = function(eleventyConfig) {
  // Copy static assets
  eleventyConfig.addPassthroughCopy("css");
  eleventyConfig.addPassthroughCopy("images");

  // Add collection
  eleventyConfig.addCollection("posts", function(collectionApi) {
    return collectionApi.getFilteredByGlob("posts/*.md");
  });

  // Custom filter
  eleventyConfig.addFilter("dateFormat", function(date) {
    return date.toLocaleDateString();
  });

  return {
    dir: {
      input: "src",
      output: "_site"
    }
  };
};
```

## Real-World Examples

### Blog Layout
```liquid
---
layout: base.njk
---

<article>
  <h1>{{ title }}</h1>
  <time>{{ date | dateFormat }}</time>
  {{ content }}
</article>
```

### Collections
```markdown
---
tags: post
title: My First Post
date: 2024-01-01
---

This is my blog post content.
```

## Agent Use
- Generate documentation sites
- Build blogs and portfolios
- Create landing pages
- Generate static marketing sites
- Maintain JAMstack applications


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install eleventy
  preset: eleventy

- name: Use eleventy in automation
  shell: |
    # Custom configuration here
    echo "eleventy configured"
```
## Uninstall
```yaml
- preset: eleventy
  with:
    state: absent
```

## Resources
- Official docs: https://www.11ty.dev/docs/
- GitHub: https://github.com/11ty/eleventy
- Search: "eleventy tutorial", "11ty examples"

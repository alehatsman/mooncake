# Astro - Web Framework for Content-Driven Sites

Modern static site generator with zero JavaScript by default. Ship 40% faster websites with component islands from any framework.

## Quick Start
```yaml
- preset: astro
```

## Features
- **Zero JavaScript by default**: Ships HTML and CSS only, hydrating components on demand
- **Island architecture**: Mix React, Vue, Svelte, Solid components in one project
- **Content collections**: Type-safe Markdown/MDX with frontmatter validation
- **Server-first**: SSG, SSR, and hybrid rendering in one framework
- **Fast builds**: Optimized for large content sites (blogs, docs, e-commerce)
- **Cross-platform**: Node.js-based, runs everywhere

## Basic Usage
```bash
# Create new project
npm create astro@latest

# Start dev server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Check project health
npx astro check
```

## Project Structure
```bash
# Typical Astro project
my-astro-site/
├── src/
│   ├── pages/           # Routes (index.astro, about.astro)
│   ├── layouts/         # Reusable layouts
│   ├── components/      # UI components
│   └── content/         # Content collections (blog posts, docs)
├── public/              # Static assets
├── astro.config.mjs     # Configuration
└── package.json
```

## Creating Pages
```astro
---
// src/pages/index.astro
const title = "Welcome";
---
<html>
  <head>
    <title>{title}</title>
  </head>
  <body>
    <h1>{title}</h1>
    <p>Zero JavaScript by default!</p>
  </body>
</html>
```

## Components with Islands
```astro
---
// src/pages/interactive.astro
import Counter from '../components/Counter.jsx';
import Chart from '../components/Chart.vue';
---
<html>
  <body>
    <!-- Static content (no JS) -->
    <h1>My Dashboard</h1>

    <!-- Interactive islands (hydrated on demand) -->
    <Counter client:load />
    <Chart client:visible />
  </body>
</html>
```

## Content Collections
```typescript
// src/content/config.ts
import { defineCollection, z } from 'astro:content';

const blog = defineCollection({
  type: 'content',
  schema: z.object({
    title: z.string(),
    date: z.date(),
    author: z.string(),
    tags: z.array(z.string()).optional(),
  }),
});

export const collections = { blog };
```

```astro
---
// src/pages/blog/[...slug].astro
import { getCollection } from 'astro:content';

export async function getStaticPaths() {
  const posts = await getCollection('blog');
  return posts.map(post => ({
    params: { slug: post.slug },
    props: { post },
  }));
}

const { post } = Astro.props;
const { Content } = await post.render();
---
<article>
  <h1>{post.data.title}</h1>
  <Content />
</article>
```

## Hydration Strategies
```astro
<!-- Load immediately -->
<Component client:load />

<!-- Load when visible (lazy) -->
<Component client:visible />

<!-- Load when idle -->
<Component client:idle />

<!-- Load based on media query -->
<Component client:media="(max-width: 768px)" />

<!-- Never hydrate (SSR only) -->
<Component client:only="react" />
```

## Markdown/MDX
```astro
---
// src/pages/blog/post.mdx
import { Image } from 'astro:assets';
import myImage from './image.png';

export const title = "My Post";
export const date = "2026-01-01";
---

# {title}

Regular Markdown content...

<Image src={myImage} alt="Example" />

<CustomComponent prop="value" />
```

## Configuration
```javascript
// astro.config.mjs
import { defineConfig } from 'astro/config';
import react from '@astrojs/react';
import tailwind from '@astrojs/tailwind';

export default defineConfig({
  site: 'https://example.com',
  integrations: [react(), tailwind()],
  output: 'static', // or 'server', 'hybrid'
});
```

## Advanced Configuration
```yaml
# Deploy Astro site
- preset: astro
  with:
    state: present

- name: Create Astro project
  shell: npm create astro@latest my-site -- --template blog --yes
  args:
    creates: my-site

- name: Install dependencies
  shell: npm install
  args:
    chdir: my-site

- name: Build site
  shell: npm run build
  args:
    chdir: my-site
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (requires Node.js)
- ✅ macOS (requires Node.js)
- ✅ Windows (requires Node.js)

## Configuration
- **Config file**: `astro.config.mjs`
- **Content**: `src/content/` (Markdown, MDX)
- **Pages**: `src/pages/` (routes)
- **Components**: `src/components/`
- **Static assets**: `public/`
- **Build output**: `dist/`
- **Dev server**: http://localhost:4321

## Real-World Examples

### Blog Setup
```bash
# Create blog from template
npm create astro@latest my-blog -- --template blog

# Start dev server
cd my-blog && npm run dev

# Add new post
cat > src/content/blog/new-post.md <<EOF
---
title: "My New Post"
date: 2026-02-06
---
Post content here...
EOF
```

### Multi-Framework Dashboard
```astro
---
// Mix frameworks in one page
import ReactChart from '../components/Chart.jsx';
import VueTable from '../components/Table.vue';
import SvelteForm from '../components/Form.svelte';
---
<html>
  <body>
    <ReactChart client:load data={chartData} />
    <VueTable client:visible items={items} />
    <SvelteForm client:idle onSubmit={handler} />
  </body>
</html>
```

### CI/CD Deployment
```bash
# GitHub Actions example
npm ci
npm run astro check  # Type checking
npm run build
# Deploy dist/ to hosting
```

## Agent Use
- Generate documentation sites from content
- Build marketing sites with minimal JavaScript
- Create blogs with type-safe content management
- Rapid prototyping with multiple UI frameworks
- Static site generation for performance-critical sites
- Content-driven applications (e-commerce catalogs, portfolios)

## Troubleshooting

### Build errors
```bash
# Check for type errors
npx astro check

# Clear cache
rm -rf node_modules/.astro
npm run build
```

### Components not hydrating
Ensure client directive is set:
```astro
<!-- ❌ Wrong - won't hydrate -->
<MyComponent />

<!-- ✅ Correct -->
<MyComponent client:load />
```

### Content collection errors
Validate frontmatter schema:
```bash
npx astro check
```

## Uninstall
```yaml
- preset: astro
  with:
    state: absent
```

## Resources
- Official site: https://astro.build/
- Documentation: https://docs.astro.build/
- GitHub: https://github.com/withastro/astro
- Templates: https://astro.build/themes/
- Search: "astro framework tutorial", "astro content collections"

Sources:
- [Astro Official Site](https://astro.build/)
- [What Is Astro?](https://kinsta.com/blog/astro-js/)
- [Top Static Site Generators for 2025](https://cloudcannon.com/blog/the-top-five-static-site-generators-for-2025-and-when-to-use-them/)

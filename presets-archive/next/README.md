# Next.js - React Framework for Production

React framework with server-side rendering, static generation, API routes, and automatic code splitting.

## Quick Start

```yaml
- preset: next
```

## Features

- **Hybrid rendering**: Server-side rendering (SSR) and static site generation (SSG)
- **API routes**: Build backend API endpoints alongside frontend code
- **File-based routing**: Pages automatically mapped from files
- **Automatic code splitting**: Load only what's needed per page
- **Image optimization**: Built-in next/image component
- **TypeScript support**: First-class TypeScript integration
- **Fast refresh**: Instant feedback on code changes

## Basic Usage

```bash
# Create new Next.js app
npx create-next-app@latest my-app
cd my-app

# Start development server
npm run dev
# App runs at http://localhost:3000

# Build for production
npm run build

# Start production server
npm start

# Export static site
npm run build && npm run export
```

## Advanced Configuration

```yaml
# Install Next.js CLI globally
- preset: next

# Create new project with Mooncake
- name: Create Next.js application
  shell: npx create-next-app@latest myapp --typescript --tailwind --app
  creates: myapp/package.json

- name: Install dependencies
  shell: npm install
  cwd: myapp

- name: Start development server
  shell: npm run dev
  cwd: myapp
  async: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Next.js CLI tools |

## Platform Support

- ✅ Linux (via npm/npx)
- ✅ macOS (via npm/npx)
- ✅ Windows (via npm/npx)

**Note**: Requires Node.js 18.17 or later.

## Configuration

- **Config file**: `next.config.js` or `next.config.mjs`
- **Environment variables**: `.env.local`, `.env.production`
- **TypeScript config**: `tsconfig.json` (auto-generated)
- **Output directory**: `.next/` (build artifacts)

## Real-World Examples

### Static Blog with Markdown
```javascript
// pages/blog/[slug].js
import fs from 'fs'
import matter from 'gray-matter'
import { marked } from 'marked'

export async function getStaticPaths() {
  const files = fs.readdirSync('posts')
  const paths = files.map(filename => ({
    params: { slug: filename.replace('.md', '') }
  }))
  return { paths, fallback: false }
}

export async function getStaticProps({ params }) {
  const fileContent = fs.readFileSync(`posts/${params.slug}.md`, 'utf-8')
  const { data, content } = matter(fileContent)
  return {
    props: {
      frontmatter: data,
      content: marked(content)
    }
  }
}

export default function BlogPost({ frontmatter, content }) {
  return (
    <article>
      <h1>{frontmatter.title}</h1>
      <div dangerouslySetInnerHTML={{ __html: content }} />
    </article>
  )
}
```

### API Route with Database
```javascript
// pages/api/users/[id].js
import { query } from '@/lib/db'

export default async function handler(req, res) {
  const { id } = req.query

  if (req.method === 'GET') {
    const user = await query('SELECT * FROM users WHERE id = ?', [id])
    res.status(200).json(user)
  } else if (req.method === 'PUT') {
    const { name, email } = req.body
    await query('UPDATE users SET name = ?, email = ? WHERE id = ?', [name, email, id])
    res.status(200).json({ success: true })
  }
}
```

### Server-Side Rendering with Authentication
```javascript
// pages/dashboard.js
import { getSession } from 'next-auth/react'

export async function getServerSideProps(context) {
  const session = await getSession(context)

  if (!session) {
    return {
      redirect: {
        destination: '/login',
        permanent: false
      }
    }
  }

  const userData = await fetch(`${process.env.API_URL}/user/${session.user.id}`)
  const data = await userData.json()

  return {
    props: { user: data }
  }
}

export default function Dashboard({ user }) {
  return <div>Welcome, {user.name}</div>
}
```

### Deployment Configuration
```javascript
// next.config.js
module.exports = {
  reactStrictMode: true,
  images: {
    domains: ['cdn.example.com'],
  },
  env: {
    API_URL: process.env.API_URL,
  },
  async redirects() {
    return [
      {
        source: '/old-blog/:slug',
        destination: '/blog/:slug',
        permanent: true,
      },
    ]
  },
}
```

### CI/CD Deployment
```yaml
# Build and deploy Next.js app
- name: Install Node.js
  preset: nodejs
  with:
    version: "20"

- name: Install Next.js CLI
  preset: next

- name: Install dependencies
  shell: npm ci
  cwd: /app

- name: Run tests
  shell: npm test
  cwd: /app

- name: Build application
  shell: npm run build
  cwd: /app
  environment:
    NEXT_PUBLIC_API_URL: "{{ api_url }}"
    DATABASE_URL: "{{ database_url }}"

- name: Start production server
  shell: npm start
  cwd: /app
  async: true
```

## Project Structure

```
my-app/
├── pages/              # File-based routes
│   ├── index.js        # Homepage (/)
│   ├── about.js        # About page (/about)
│   ├── api/            # API routes
│   │   └── hello.js    # API endpoint (/api/hello)
│   └── blog/
│       └── [slug].js   # Dynamic route (/blog/*)
├── public/             # Static files
├── styles/             # CSS/SCSS files
├── components/         # React components
├── lib/                # Utility functions
├── next.config.js      # Next.js configuration
└── package.json
```

## Rendering Strategies

### Static Generation (SSG)
```javascript
export async function getStaticProps() {
  const data = await fetchData()
  return { props: { data }, revalidate: 60 }  // Revalidate every 60s
}
```

### Server-Side Rendering (SSR)
```javascript
export async function getServerSideProps(context) {
  const data = await fetchData(context.params)
  return { props: { data } }
}
```

### Client-Side Rendering (CSR)
```javascript
import { useEffect, useState } from 'react'

export default function Page() {
  const [data, setData] = useState(null)
  useEffect(() => {
    fetch('/api/data').then(r => r.json()).then(setData)
  }, [])
  return <div>{data ? data.title : 'Loading...'}</div>
}
```

## Agent Use

- Scaffold new Next.js applications with consistent structure
- Deploy full-stack applications with SSR and API routes
- Build static marketing sites with dynamic content
- Create e-commerce platforms with hybrid rendering
- Automate builds and deployments in CI/CD pipelines
- Generate documentation sites with incremental static regeneration

## Troubleshooting

### Build errors
```bash
# Clear Next.js cache
rm -rf .next

# Clean install dependencies
rm -rf node_modules package-lock.json
npm install

# Check for outdated packages
npm outdated
```

### Port already in use
```bash
# Use different port
npm run dev -- -p 3001

# Or set in package.json
"dev": "next dev -p 3001"
```

## Uninstall

```yaml
- preset: next
  with:
    state: absent
```

## Resources

- Official docs: https://nextjs.org/docs
- Learn Next.js: https://nextjs.org/learn
- Examples: https://github.com/vercel/next.js/tree/canary/examples
- GitHub: https://github.com/vercel/next.js
- Search: "nextjs tutorial", "nextjs ssr example", "nextjs api routes"

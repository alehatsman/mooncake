# Gatsby - React-Based Static Site Generator

Build blazing-fast websites and apps with React. GraphQL-powered static site generator with modern web tooling built-in.

## Quick Start
```yaml
- preset: gatsby
```

## Features
- **React-based**: Build with React components
- **GraphQL data layer**: Query data from any source
- **Performance**: Automatic code splitting, image optimization, prefetching
- **Plugin ecosystem**: 2,500+ plugins for CMS, analytics, SEO
- **Progressive enhancement**: Works without JavaScript, enhanced with it
- **Hot reload**: See changes instantly during development

## Basic Usage
```bash
# Create new Gatsby site
npx gatsby new my-site

# Start development server
cd my-site
gatsby develop

# Build for production
gatsby build

# Serve production build
gatsby serve

# Clean cache and public directory
gatsby clean
```

## Advanced Configuration
```yaml
- preset: gatsby
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Gatsby CLI |

## Platform Support
- ✅ Linux (npm, yarn)
- ✅ macOS (npm, yarn, Homebrew)
- ✅ Windows (npm, yarn, Chocolatey)

## Configuration
- **Config file**: `gatsby-config.js`
- **Node modules**: `gatsby-node.js` (build-time)
- **Browser APIs**: `gatsby-browser.js` (runtime)
- **SSR APIs**: `gatsby-ssr.js` (server-side rendering)
- **Dev server**: `http://localhost:8000`
- **GraphQL**: `http://localhost:8000/___graphql`

## Real-World Examples

### Blog with Markdown
```javascript
// gatsby-config.js
module.exports = {
  siteMetadata: {
    title: 'My Blog',
    description: 'A blog built with Gatsby',
  },
  plugins: [
    'gatsby-plugin-react-helmet',
    {
      resolve: 'gatsby-source-filesystem',
      options: {
        name: 'posts',
        path: `${__dirname}/content/posts`,
      },
    },
    'gatsby-transformer-remark',
    'gatsby-plugin-image',
    'gatsby-plugin-sharp',
  ],
}
```

```javascript
// src/pages/blog/{MarkdownRemark.frontmatter__slug}.js
import React from 'react'
import { graphql } from 'gatsby'

export default function BlogPost({ data }) {
  const { markdownRemark } = data
  const { frontmatter, html } = markdownRemark

  return (
    <article>
      <h1>{frontmatter.title}</h1>
      <time>{frontmatter.date}</time>
      <div dangerouslySetInnerHTML={{ __html: html }} />
    </article>
  )
}

export const query = graphql`
  query($id: String!) {
    markdownRemark(id: { eq: $id }) {
      html
      frontmatter {
        title
        date(formatString: "MMMM DD, YYYY")
      }
    }
  }
`
```

### Headless CMS Integration
```javascript
// gatsby-config.js - Contentful example
module.exports = {
  plugins: [
    {
      resolve: 'gatsby-source-contentful',
      options: {
        spaceId: process.env.CONTENTFUL_SPACE_ID,
        accessToken: process.env.CONTENTFUL_ACCESS_TOKEN,
      },
    },
  ],
}
```

```javascript
// Query Contentful data
export const query = graphql`
  query {
    allContentfulBlogPost(sort: { fields: publishDate, order: DESC }) {
      nodes {
        title
        slug
        publishDate(formatString: "MMMM Do, YYYY")
        excerpt
        heroImage {
          gatsbyImageData(width: 800)
        }
      }
    }
  }
`
```

### Image Optimization
```javascript
import React from 'react'
import { StaticImage, GatsbyImage, getImage } from 'gatsby-plugin-image'

// Static image (compile-time)
function Hero() {
  return (
    <StaticImage
      src="../images/hero.jpg"
      alt="Hero image"
      placeholder="blurred"
      width={1200}
    />
  )
}

// Dynamic image (runtime)
function BlogPost({ data }) {
  const image = getImage(data.post.heroImage)
  return <GatsbyImage image={image} alt={data.post.title} />
}
```

### SEO Component
```javascript
import React from 'react'
import { Helmet } from 'react-helmet'
import { useStaticQuery, graphql } from 'gatsby'

function SEO({ title, description, image, article }) {
  const { site } = useStaticQuery(graphql`
    query {
      site {
        siteMetadata {
          title
          description
          author
          siteUrl
        }
      }
    }
  `)

  const metaDescription = description || site.siteMetadata.description
  const defaultTitle = site.siteMetadata.title

  return (
    <Helmet
      htmlAttributes={{ lang: 'en' }}
      title={title}
      titleTemplate={`%s | ${defaultTitle}`}
      meta={[
        { name: 'description', content: metaDescription },
        { property: 'og:title', content: title },
        { property: 'og:description', content: metaDescription },
        { property: 'og:type', content: article ? 'article' : 'website' },
      ]}
    />
  )
}
```

## Agent Use
- Build documentation sites from Markdown files
- Create marketing websites with CMS integration
- Generate static sites from APIs or databases
- Build blogs with automatic optimization
- Create portfolio sites with image galleries
- Deploy JAMstack applications to CDNs

## Troubleshooting

### Build failures
```bash
# Clear cache and rebuild
gatsby clean
gatsby build

# Check Node.js version (requires v18+)
node --version

# Install dependencies
npm install

# View detailed build logs
gatsby build --verbose
```

### GraphQL errors
```bash
# Explore GraphQL schema
open http://localhost:8000/___graphql

# Check query syntax
gatsby develop

# Verify data source plugins are configured
# Check gatsby-config.js
```

### Out of memory
```bash
# Increase Node.js memory limit
NODE_OPTIONS="--max-old-space-size=4096" gatsby build

# Or in package.json
"scripts": {
  "build": "NODE_OPTIONS='--max-old-space-size=4096' gatsby build"
}
```

### Plugin conflicts
```bash
# Check plugin versions
npm list gatsby-plugin-*

# Update plugins
npm update

# Clear cache
gatsby clean
rm -rf node_modules package-lock.json
npm install
```

## Uninstall
```yaml
- preset: gatsby
  with:
    state: absent
```

## Resources
- Official docs: https://www.gatsbyjs.com/docs/
- Tutorial: https://www.gatsbyjs.com/docs/tutorial/
- Plugins: https://www.gatsbyjs.com/plugins/
- Starters: https://www.gatsbyjs.com/starters/
- Search: "gatsby tutorial", "gatsby blog", "gatsby cms"

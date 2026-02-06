# Vite - Next Generation Frontend Build Tool

Fast and modern build tool for frontend development with instant HMR and optimized builds.

## Quick Start
```yaml
- preset: vite
```

## Features
- **Lightning Fast**: Native ES modules with instant hot module replacement (HMR)
- **Framework Agnostic**: Supports React, Vue, Svelte, Preact, Lit, and vanilla JS
- **Optimized Builds**: Rollup-based production builds with automatic code splitting
- **Rich Plugin Ecosystem**: Extensive plugin support for enhanced functionality
- **TypeScript Support**: First-class TypeScript support out of the box
- **CSS Preprocessing**: Built-in support for PostCSS, Sass, Less, Stylus

## Basic Usage
```bash
# Create new project
npm create vite@latest my-app
cd my-app
npm install

# Development server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Using Vite CLI directly
vite                    # Start dev server
vite build              # Build for production
vite preview            # Preview production build
vite optimize           # Pre-bundle dependencies
```

## Advanced Configuration
```yaml
- preset: vite
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Vite |

## Platform Support
- ✅ Linux (npm/yarn/pnpm)
- ✅ macOS (npm/yarn/pnpm)
- ✅ Windows (npm/yarn/pnpm)

## Configuration
- **Config file**: `vite.config.js` or `vite.config.ts`
- **Default dev port**: 5173
- **Build output**: `dist/`
- **Node.js requirement**: 14.18+ or 16+

## Project Setup

### React Project
```bash
npm create vite@latest my-react-app -- --template react
cd my-react-app
npm install
npm run dev
```

### Vue Project
```bash
npm create vite@latest my-vue-app -- --template vue
cd my-vue-app
npm install
npm run dev
```

### TypeScript Projects
```bash
npm create vite@latest my-app -- --template react-ts    # React + TS
npm create vite@latest my-app -- --template vue-ts      # Vue + TS
```

## Configuration File

Basic `vite.config.js`:
```javascript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    open: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, '')
      }
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom']
        }
      }
    }
  }
})
```

## Real-World Examples

### Development Workflow
```bash
# Install Vite in existing project
npm install -D vite

# Add scripts to package.json
# "dev": "vite"
# "build": "vite build"
# "preview": "vite preview"

# Start development
npm run dev
```

### Production Build
```bash
# Build with environment variables
VITE_API_URL=https://api.example.com npm run build

# Build with type checking
tsc && vite build

# Build with custom config
vite build --config vite.prod.config.js
```

### CI/CD Integration
```yaml
- name: Install Vite
  preset: vite

- name: Install dependencies
  shell: npm ci

- name: Build application
  shell: npm run build
  env:
    VITE_API_URL: "{{ api_url }}"
    NODE_ENV: production

- name: Run tests
  shell: npm run test

- name: Deploy build artifacts
  copy:
    src: dist/
    dest: /var/www/html/
  become: true
```

### Library Mode
```javascript
// vite.config.js for library
import { defineConfig } from 'vite'

export default defineConfig({
  build: {
    lib: {
      entry: 'src/index.js',
      name: 'MyLib',
      fileName: (format) => `my-lib.${format}.js`
    },
    rollupOptions: {
      external: ['react', 'react-dom'],
      output: {
        globals: {
          react: 'React',
          'react-dom': 'ReactDOM'
        }
      }
    }
  }
})
```

## Agent Use
- Modern web application build and development
- Frontend CI/CD pipeline integration
- Static site generation and deployment
- Component library development
- Rapid prototyping and scaffolding
- Development server for testing and previewing

## Troubleshooting

### Port already in use
```bash
# Use different port
vite --port 3000

# Or in vite.config.js
server: { port: 3000 }
```

### Slow dependency pre-bundling
```bash
# Clear cache and rebuild
rm -rf node_modules/.vite
vite optimize --force
```

### Build errors
```bash
# Check Node.js version
node --version  # Should be 14.18+ or 16+

# Clear cache
rm -rf node_modules dist
npm install
npm run build
```

## Uninstall
```yaml
- preset: vite
  with:
    state: absent
```

## Resources
- Official docs: https://vitejs.dev/
- GitHub: https://github.com/vitejs/vite
- Search: "vite getting started", "vite configuration guide"

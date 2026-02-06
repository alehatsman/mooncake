# Bun - All-in-One JavaScript Runtime

Blazing fast JavaScript runtime, bundler, test runner, and package manager designed as a drop-in replacement for Node.js.

## Quick Start
```yaml
- preset: bun
```

## Features
- **Blazing fast**: Written in Zig, starts 4x faster than Node.js
- **All-in-one**: Runtime, bundler, transpiler, package manager in one tool
- **Drop-in replacement**: Compatible with Node.js and npm packages
- **Built-in tooling**: Test runner, bundler, transpiler included
- **TypeScript support**: Native TypeScript and JSX execution
- **Performance**: 3x faster npm installs, optimized for speed

## Basic Usage
```bash
# Check version
bun --version

# Run a JavaScript file
bun run index.js

# Run a TypeScript file (no compilation needed)
bun run index.ts

# Start a script from package.json
bun run dev

# Execute code directly
bun -e "console.log('Hello from Bun')"

# REPL
bun repl
```

## Package Management
```bash
# Install dependencies (reads package.json)
bun install

# Install a package
bun add react
bun add -d typescript      # Dev dependency
bun add -g pm2             # Global install

# Remove a package
bun remove react

# Update packages
bun update

# Run scripts from package.json
bun run build
bun run test
bun run start
```

## Creating Projects
```bash
# Create a new project
bun init

# Create a Next.js app
bun create next-app my-app

# Create a React app
bun create react my-app

# Create from any template
bun create github-user/repo-name
```

## Bundler
```bash
# Bundle an application
bun build ./index.tsx --outdir ./build

# Bundle for production (minified)
bun build ./index.tsx --outdir ./build --minify

# Create executable
bun build ./cli.ts --compile --outfile mycli

# Watch mode
bun build ./index.tsx --outdir ./build --watch
```

## Test Runner
```bash
# Run all tests
bun test

# Run specific test file
bun test ./tests/auth.test.ts

# Watch mode
bun test --watch

# Coverage
bun test --coverage
```

Example test file:
```typescript
// math.test.ts
import { expect, test, describe } from "bun:test";

describe("Math", () => {
  test("addition", () => {
    expect(2 + 2).toBe(4);
  });

  test("multiplication", () => {
    expect(3 * 4).toBe(12);
  });
});
```

## HTTP Server
```typescript
// server.ts
Bun.serve({
  port: 3000,
  fetch(req) {
    return new Response("Hello from Bun!");
  },
});

console.log("Server running at http://localhost:3000");
```

```bash
# Run the server
bun run server.ts
```

## Environment Variables
```bash
# Bun automatically loads .env files
# .env
DATABASE_URL=postgresql://localhost/mydb
API_KEY=secret123

# Access in code
console.log(process.env.DATABASE_URL);
```

## Real-World Examples

### Fast API Development
```typescript
// api.ts
Bun.serve({
  port: 3000,
  async fetch(req) {
    const url = new URL(req.url);

    if (url.pathname === "/api/users") {
      const users = await db.query("SELECT * FROM users");
      return Response.json(users);
    }

    if (url.pathname === "/api/health") {
      return Response.json({ status: "ok" });
    }

    return new Response("Not Found", { status: 404 });
  },
});
```

### Build Pipeline in CI/CD
```yaml
- name: Install Bun
  preset: bun

- name: Install dependencies
  shell: bun install --frozen-lockfile
  cwd: /app

- name: Run tests
  shell: bun test

- name: Build application
  shell: bun build ./src/index.tsx --outdir ./dist --minify

- name: Check bundle size
  shell: ls -lh dist/
```

### Monorepo Workspace
```json
// package.json
{
  "name": "my-monorepo",
  "workspaces": ["packages/*"],
  "scripts": {
    "build": "bun run --filter '*' build",
    "test": "bun run --filter '*' test",
    "dev": "bun run --filter '*' dev"
  }
}
```

```bash
# Install all workspace dependencies
bun install

# Run command in all packages
bun run build
```

## Advanced Configuration

### bunfig.toml
```toml
# Project configuration file
[install]
# Configure package installation
optional = true
dev = true
peer = true
production = false

[install.cache]
# Cache directory
dir = "~/.bun/install/cache"

# Disable cache
disable = false

[test]
# Test configuration
preload = ["./setup.ts"]
coverage = true
```

### Custom Scripts
```yaml
# Deploy with Bun
- name: Install Bun
  preset: bun

- name: Checkout code
  shell: git clone https://github.com/user/repo.git /app

- name: Install dependencies
  shell: bun install
  cwd: /app

- name: Build optimized bundle
  shell: |
    bun build ./src/index.ts \
      --outdir ./dist \
      --minify \
      --sourcemap \
      --target browser
  cwd: /app

- name: Create standalone executable
  shell: bun build ./src/cli.ts --compile --outfile /usr/local/bin/mycli
  become: true
```

## Performance Benchmarks

### Install Speed
```bash
# Compare install times
time npm install     # ~30s
time yarn install    # ~25s
time pnpm install    # ~15s
time bun install     # ~10s (3x faster than npm)
```

### Startup Time
```bash
# Node.js
time node -e "console.log('hello')"  # ~50ms

# Bun
time bun -e "console.log('hello')"   # ~10ms (5x faster)
```

## Bun vs Node.js

| Feature | Bun | Node.js |
|---------|-----|---------|
| Startup time | ~10ms | ~50ms |
| Package install | Fast (native) | Slower |
| TypeScript | Native | Requires setup |
| Bundler | Built-in | webpack/esbuild |
| Test runner | Built-in | jest/vitest |
| API compatibility | Web standards | Node.js APIs |

## Migration from Node.js

### Package.json
```json
{
  "scripts": {
    "start": "bun run src/index.ts",
    "dev": "bun --watch src/index.ts",
    "test": "bun test",
    "build": "bun build src/index.ts --outdir dist"
  }
}
```

### Common Patterns
```typescript
// File I/O (Bun-optimized)
const file = Bun.file("./data.json");
const json = await file.json();

// Fetch API (built-in)
const response = await fetch("https://api.example.com/data");
const data = await response.json();

// Environment variables (auto-loaded)
const apiKey = process.env.API_KEY;
```

## Troubleshooting

### Command not found
```bash
# Check installation
which bun

# Verify PATH
echo $PATH

# Reinstall
curl -fsSL https://bun.sh/install | bash
source ~/.bashrc  # or ~/.zshrc
```

### Package installation fails
```bash
# Clear cache
rm -rf ~/.bun/install/cache

# Try install again
bun install

# Use npm fallback for problematic packages
npm install problematic-package
```

### TypeScript errors
```bash
# Install TypeScript definitions
bun add -d @types/node @types/react

# Generate types
bun run tsc --noEmit
```

### Native modules
Some native Node.js modules may not work with Bun. Options:
- Use Bun-compatible alternatives
- Use Node.js for that specific package
- Check Bun compatibility: https://bun.sh/docs/runtime/nodejs-apis

## Platform Support
- ✅ Linux (x64, ARM64)
- ✅ macOS (Apple Silicon, Intel)
- ✅ Windows (WSL2)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Fast dependency installation in CI/CD pipelines
- Quick prototyping and script execution
- High-performance API servers
- Bundle and optimize frontend applications
- Run TypeScript without compilation step
- Replace Node.js in performance-critical workflows

## Uninstall
```yaml
- preset: bun
  with:
    state: absent
```

## Resources
- Official site: https://bun.sh
- Documentation: https://bun.sh/docs
- GitHub: https://github.com/oven-sh/bun
- Discord: https://bun.sh/discord
- Search: "bun javascript runtime", "bun vs node", "bun tutorial"

# tsx - TypeScript Execute (Enhanced)

Fast TypeScript execution engine built on esbuild. Drop-in replacement for ts-node with instant startup.

## Quick Start
```yaml
- preset: tsx
```

## Features
- **Instant Startup**: 10x faster than ts-node using esbuild
- **Watch Mode**: Auto-reload on file changes
- **ESM Support**: Native ES modules and CommonJS
- **No Config**: Works without tsconfig.json
- **Source Maps**: Full debugging support
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Execute TypeScript file
tsx script.ts

# With arguments
tsx script.ts arg1 arg2

# Watch mode
tsx watch server.ts

# Evaluate expression
tsx --eval "console.log('Hello')"

# Check TypeScript types
tsx --tsconfig tsconfig.json script.ts
```

## Watch Mode
```bash
# Auto-reload on changes
tsx watch app.ts

# Watch with arguments
tsx watch server.ts --port 3000

# Ignore patterns
tsx watch --ignore dist/** --ignore '*.test.ts' app.ts

# Clear console on reload
tsx watch --clear-screen=false app.ts
```

## Real-World Examples

### Express Development Server
```typescript
// server.ts
import express from 'express';

const app = express();
const port = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.json({ message: 'Hello from tsx!' });
});

app.listen(port, () => {
  console.log(`Server running on port ${port}`);
});
```

Run with hot reload:
```bash
tsx watch server.ts
```

### CLI Tool
```typescript
#!/usr/bin/env tsx
// bin/cli.ts
import { Command } from 'commander';

const program = new Command();

program
  .name('my-cli')
  .version('1.0.0')
  .argument('<name>', 'Name to greet')
  .option('-l, --loud', 'Uppercase the greeting')
  .action((name, options) => {
    let greeting = `Hello, ${name}!`;
    if (options.loud) {
      greeting = greeting.toUpperCase();
    }
    console.log(greeting);
  });

program.parse();
```

### Build Script
```typescript
// scripts/build.ts
import { build } from 'esbuild';

async function buildProject() {
  await build({
    entryPoints: ['src/index.ts'],
    bundle: true,
    outdir: 'dist',
    platform: 'node',
  });
  console.log('Build complete!');
}

buildProject().catch(console.error);
```

## Advanced Configuration
```yaml
- preset: tsx
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tsx |

## Platform Support
- ✅ Linux (npm global install)
- ✅ macOS (npm global install)
- ✅ Windows (npm global install)

## Performance Comparison
```bash
# tsx (instant startup)
time tsx script.ts  # ~0.05s

# ts-node (slower)
time ts-node script.ts  # ~0.5s

# 10x faster startup time
```

## Agent Use
- Rapid development with instant TypeScript execution
- Build scripts that run without compilation
- Database migrations and seeders
- CLI tools with fast startup times
- Development servers with hot reload
- CI/CD automation scripts

## Uninstall
```yaml
- preset: tsx
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/esbuild-kit/tsx
- npm: https://www.npmjs.com/package/tsx
- Search: "tsx typescript", "tsx vs ts-node", "tsx watch mode"

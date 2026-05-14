# Deno - Modern JavaScript/TypeScript Runtime

Secure runtime for JavaScript and TypeScript with built-in tooling and modern features.

## Quick Start
```yaml
- preset: deno
```

## Features
- **Secure by default**: No file, network, or environment access without permission
- **TypeScript first**: Native TypeScript support, no config needed
- **Modern APIs**: Web-standard APIs (fetch, WebSocket, etc.)
- **Single executable**: No separate package manager needed
- **Built-in tooling**: Formatter, linter, test runner, bundler
- **ES modules**: Native ESM support, no CommonJS

## Basic Usage
```bash
# Run a script
deno run script.ts

# Run with permissions
deno run --allow-net --allow-read server.ts

# REPL
deno

# Run remote script
deno run https://deno.land/std/examples/welcome.ts

# Check types
deno check script.ts

# Format code
deno fmt

# Lint code
deno lint

# Run tests
deno test

# Bundle for distribution
deno bundle app.ts app.bundle.js
```

## Advanced Configuration
```yaml
# Basic installation
- preset: deno

# Uninstall
- preset: deno
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows (use official installer)

## Permissions System
```bash
# Network access
deno run --allow-net server.ts

# File system read
deno run --allow-read script.ts

# File system write
deno run --allow-write script.ts

# Environment variables
deno run --allow-env script.ts

# Run subprocess
deno run --allow-run script.ts

# All permissions
deno run -A script.ts

# Specific hosts/paths
deno run --allow-net=api.example.com script.ts
deno run --allow-read=/tmp script.ts
```

## Configuration
- **Config file**: `deno.json` or `deno.jsonc` (optional)
- **Import maps**: Define module aliases
- **Tasks**: Define reusable commands
- **Lint/format**: Configure linter and formatter

### deno.json Example
```json
{
  "tasks": {
    "dev": "deno run --watch --allow-net --allow-read server.ts",
    "test": "deno test --allow-read",
    "fmt": "deno fmt",
    "lint": "deno lint"
  },
  "imports": {
    "@/": "./src/",
    "std/": "https://deno.land/std@0.208.0/"
  },
  "lint": {
    "rules": {
      "tags": ["recommended"]
    }
  },
  "fmt": {
    "useTabs": false,
    "lineWidth": 100,
    "indentWidth": 2
  },
  "compilerOptions": {
    "allowJs": true,
    "lib": ["deno.window"],
    "strict": true
  }
}
```

## Real-World Examples

### Simple HTTP Server
```typescript
// server.ts
import { serve } from "https://deno.land/std@0.208.0/http/server.ts";

serve((req: Request) => {
  return new Response("Hello, World!");
}, { port: 8000 });

console.log("Server running on http://localhost:8000");
```

```bash
deno run --allow-net server.ts
```

### File Operations
```typescript
// file-ops.ts
// Read file
const text = await Deno.readTextFile("./file.txt");
console.log(text);

// Write file
await Deno.writeTextFile("./output.txt", "Hello from Deno");

// Directory listing
for await (const entry of Deno.readDir(".")) {
  console.log(entry.name);
}
```

```bash
deno run --allow-read --allow-write file-ops.ts
```

### API Client
```typescript
// api-client.ts
interface User {
  id: number;
  name: string;
  email: string;
}

const response = await fetch("https://api.example.com/users/1");
const user: User = await response.json();
console.log(`User: ${user.name} (${user.email})`);
```

```bash
deno run --allow-net api-client.ts
```

### Testing
```typescript
// math.ts
export function add(a: number, b: number): number {
  return a + b;
}

// math.test.ts
import { assertEquals } from "https://deno.land/std@0.208.0/assert/mod.ts";
import { add } from "./math.ts";

Deno.test("add function", () => {
  assertEquals(add(2, 3), 5);
  assertEquals(add(-1, 1), 0);
});
```

```bash
deno test
```

### Web Worker
```typescript
// worker.ts
self.onmessage = (e) => {
  const result = e.data * 2;
  self.postMessage(result);
};

// main.ts
const worker = new Worker(new URL("./worker.ts", import.meta.url).href, {
  type: "module",
});

worker.onmessage = (e) => {
  console.log("Result:", e.data);
  worker.terminate();
};

worker.postMessage(42);
```

```bash
deno run --allow-read main.ts
```

## Built-in Tools

### Formatter
```bash
# Format all files
deno fmt

# Check formatting
deno fmt --check

# Specific files
deno fmt src/
```

### Linter
```bash
# Lint all files
deno lint

# Specific files
deno lint src/

# List available rules
deno lint --rules
```

### Test Runner
```bash
# Run all tests
deno test

# Watch mode
deno test --watch

# Filter tests
deno test --filter "user"

# Coverage
deno test --coverage=./cov
deno coverage ./cov
```

### Bundler
```bash
# Bundle for browser
deno bundle app.ts app.bundle.js

# Include import map
deno bundle --import-map=import_map.json app.ts bundle.js
```

### Documentation
```bash
# Generate docs
deno doc mod.ts

# Generate JSON
deno doc --json mod.ts > docs.json
```

### REPL
```bash
# Start REPL
deno

# With TypeScript types
deno repl --eval-file=types.ts
```

## Package Management

### Dependencies
```typescript
// deps.ts - centralize dependencies
export { serve } from "https://deno.land/std@0.208.0/http/server.ts";
export { parse } from "https://deno.land/std@0.208.0/flags/mod.ts";
export { DB } from "https://deno.land/x/sqlite@v3.8/mod.ts";

// Use in other files
import { serve, parse, DB } from "./deps.ts";
```

### Lock File
```bash
# Generate lock file
deno cache --lock=deno.lock --lock-write deps.ts

# Verify dependencies
deno cache --lock=deno.lock --reload deps.ts
```

## CI/CD Integration

### GitHub Actions
```yaml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: denoland/setup-deno@v1
        with:
          deno-version: v1.x
      - run: deno lint
      - run: deno fmt --check
      - run: deno test --allow-all
```

### Docker
```dockerfile
FROM denoland/deno:1.39.0

WORKDIR /app
COPY . .

# Cache dependencies
RUN deno cache server.ts

USER deno
EXPOSE 8000

CMD ["run", "--allow-net", "--allow-read", "server.ts"]
```

## Migration from Node.js

### npm Packages
```typescript
// Use npm packages via CDN
import express from "npm:express@4.18.2";
import chalk from "npm:chalk@5";

console.log(chalk.blue("Hello from Deno!"));
```

### Node.js Compatibility
```typescript
// Use Node.js built-ins
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";

// Most Node.js APIs work in Deno
```

### package.json Scripts
```json
{
  "tasks": {
    "start": "deno run --allow-net server.ts",
    "dev": "deno run --watch --allow-net server.ts",
    "test": "deno test --allow-all"
  }
}
```

## Agent Use
- Build serverless functions and edge computing applications
- Create secure CLI tools with strict permissions
- Develop TypeScript applications without build setup
- Run untrusted code safely with permission system
- Build microservices with minimal dependencies
- Create deployment scripts with modern JavaScript
- Test API endpoints and data pipelines
- Generate static sites and documentation

## Troubleshooting

### Import errors
```bash
# Clear cache
deno cache --reload script.ts

# Check remote imports
deno info script.ts
```

### Permission denied
```bash
# Check required permissions
deno run --allow-net --allow-read script.ts

# Or grant all permissions
deno run -A script.ts
```

### TypeScript errors
```bash
# Check types explicitly
deno check script.ts

# Skip type checking (faster)
deno run --no-check script.ts
```

## Comparison with Node.js
| Feature | Deno | Node.js |
|---------|------|---------|
| Security | Secure by default | All access granted |
| TypeScript | Native support | Requires setup |
| Package manager | Built-in | npm/yarn/pnpm |
| Modules | ES modules only | CommonJS + ESM |
| Standard library | Included | Minimal |
| Tooling | Built-in | Third-party |

## Uninstall
```yaml
- preset: deno
  with:
    state: absent
```

## Resources
- Official site: https://deno.land/
- Documentation: https://deno.land/manual
- Standard library: https://deno.land/std
- Third-party modules: https://deno.land/x
- GitHub: https://github.com/denoland/deno
- Search: "deno tutorial", "deno examples", "deno vs node"

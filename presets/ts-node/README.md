# ts-node - TypeScript Execution Engine

Execute TypeScript files directly without pre-compilation.

## Quick Start
```yaml
- preset: ts-node
```

## Features
- **Direct Execution**: Run TypeScript files without compilation step
- **REPL Support**: Interactive TypeScript shell
- **Module Support**: CommonJS and ESM module resolution
- **Source Maps**: Full debugging support with source maps
- **Type Checking**: Optional type checking during execution
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Execute TypeScript file
ts-node script.ts

# With arguments
ts-node script.ts arg1 arg2

# Interactive REPL
ts-node

# Skip type checking (faster)
ts-node --transpileOnly script.ts

# ESM mode
ts-node --esm script.ts

# Specific tsconfig
ts-node --project tsconfig.build.json script.ts
```

## REPL Usage
```bash
# Start REPL
ts-node

# In REPL
> const greeting: string = 'Hello';
> console.log(greeting);
Hello
> .type greeting
string
> .exit
```

## Execution Options
```bash
# Fast mode (skip type checking)
ts-node --transpileOnly app.ts

# With environment
NODE_ENV=production ts-node server.ts

# Custom TypeScript config
ts-node --project ./config/tsconfig.json app.ts

# Compiler options inline
ts-node -O '{"module":"commonjs"}' app.ts

# Ignore diagnostics
ts-node --ignore-diagnostics 2304,2307 app.ts
```

## Module Resolution
```bash
# CommonJS (default)
ts-node script.ts

# ESM mode
ts-node --esm script.mts

# With experimental features
node --loader ts-node/esm script.ts
```

## Integration Examples

### npm scripts
```json
{
  "scripts": {
    "start": "ts-node src/index.ts",
    "dev": "ts-node --transpileOnly src/server.ts",
    "debug": "node --inspect -r ts-node/register src/index.ts"
  }
}
```

### Express server
```typescript
// server.ts
import express from 'express';

const app = express();
const port = 3000;

app.get('/', (req, res) => {
  res.send('Hello from TypeScript!');
});

app.listen(port, () => {
  console.log(`Server running on port ${port}`);
});
```

Run with:
```bash
ts-node server.ts
```

### Task runner
```typescript
// tasks.ts
async function deploy(env: string): Promise<void> {
  console.log(`Deploying to ${env}...`);
  // Deployment logic
}

const env = process.argv[2] || 'development';
deploy(env);
```

Run with:
```bash
ts-node tasks.ts production
```

## Configuration

### tsconfig.json
```json
{
  "ts-node": {
    "transpileOnly": true,
    "files": true,
    "compilerOptions": {
      "module": "commonjs"
    }
  },
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "esModuleInterop": true
  }
}
```

### Environment Variables
```bash
# Skip type checking
TS_NODE_TRANSPILE_ONLY=true ts-node script.ts

# Project path
TS_NODE_PROJECT=./tsconfig.build.json ts-node script.ts

# Compiler options
TS_NODE_COMPILER_OPTIONS='{"module":"commonjs"}' ts-node script.ts
```

## Real-World Examples

### Database Migration
```typescript
// migrations/001_create_users.ts
import { Pool } from 'pg';

async function migrate() {
  const pool = new Pool();
  await pool.query(`
    CREATE TABLE users (
      id SERIAL PRIMARY KEY,
      email VARCHAR(255) UNIQUE NOT NULL,
      created_at TIMESTAMP DEFAULT NOW()
    )
  `);
  await pool.end();
  console.log('Migration complete');
}

migrate();
```

Run with:
```bash
ts-node migrations/001_create_users.ts
```

### CLI Tool
```typescript
#!/usr/bin/env ts-node
// bin/tool.ts
import { program } from 'commander';

program
  .version('1.0.0')
  .option('-d, --debug', 'Debug mode')
  .action((options) => {
    console.log('Debug:', options.debug);
  });

program.parse();
```

Make executable and run:
```bash
chmod +x bin/tool.ts
./bin/tool.ts --debug
```

### API Testing
```typescript
// test-api.ts
import axios from 'axios';

async function testAPI() {
  const response = await axios.get('https://api.example.com/status');
  console.log('Status:', response.status);
  console.log('Data:', response.data);
}

testAPI().catch(console.error);
```

## Performance Tips
- Use `--transpileOnly` for faster execution
- Use `--files` to respect tsconfig files array
- Cache with `TS_NODE_SKIP_IGNORE=true`
- Consider tsx for even faster execution

## Debugging
```bash
# With Chrome DevTools
node --inspect -r ts-node/register script.ts

# With breakpoint
node --inspect-brk -r ts-node/register script.ts

# VS Code launch.json
{
  "type": "node",
  "request": "launch",
  "name": "Debug TypeScript",
  "runtimeArgs": ["-r", "ts-node/register"],
  "args": ["${workspaceFolder}/src/index.ts"]
}
```

## Advanced Configuration
```yaml
- preset: ts-node
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove ts-node |

## Platform Support
- ✅ Linux (npm global install)
- ✅ macOS (npm global install)
- ✅ Windows (npm global install)

## Troubleshooting

### Cannot find module errors
```bash
# Install type definitions
npm install --save-dev @types/node

# Or use skipLibCheck
ts-node -O '{"skipLibCheck":true}' script.ts
```

### ESM module errors
```bash
# Use ESM loader
node --loader ts-node/esm script.ts

# Or use .mts extension
ts-node script.mts
```

## Agent Use
- Execute TypeScript scripts in CI/CD pipelines without build step
- Run database migrations written in TypeScript
- Create CLI tools with TypeScript for better type safety
- Rapid prototyping with TypeScript features
- Run tests written in TypeScript directly
- Development and debugging workflows

## Uninstall
```yaml
- preset: ts-node
  with:
    state: absent
```

## Resources
- Official docs: https://typestrong.org/ts-node/
- GitHub: https://github.com/TypeStrong/ts-node
- Search: "ts-node examples", "ts-node esm", "ts-node configuration"

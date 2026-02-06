# Ignite CLI - React Native Boilerplate & CLI

Battle-tested React Native boilerplate with opinionated architecture, generators, and best practices built-in.

## Quick Start
```yaml
- preset: ignite
```

## Features
- **Project scaffolding**: Create production-ready React Native apps instantly
- **Boilerplate templates**: Pre-configured with navigation, state management, and testing
- **Code generators**: Generate screens, components, and models from CLI
- **Best practices**: TypeScript, ESLint, Prettier, and testing configured
- **Plugin system**: Extend functionality with community plugins
- **Cross-platform**: iOS and Android support

## Basic Usage
```bash
# Create new React Native app
ignite new MyApp
cd MyApp

# Generate screen
ignite generate screen Login

# Generate component
ignite generate component Button

# Generate model
ignite generate model User

# Run app
npm start
npm run ios
npm run android

# Run tests
npm test
```

## Configuration
- **Config**: `ignite/ignite.json` (project config)
- **Templates**: `ignite/templates/` (custom generators)
- **Plugins**: Managed via `ignite/ignite.json`

## Real-World Examples

### Create App with TypeScript
```bash
# Initialize new project with Expo
ignite new PizzaApp --expo

# Or without Expo (bare React Native)
ignite new PizzaApp --no-expo
```

### Generate Complete Feature
```bash
# Create new screen with navigation
ignite generate screen RestaurantList

# Generate component for screen
ignite generate component RestaurantCard

# Create model for data
ignite generate model Restaurant
```

### Project Structure
```
my-app/
├── app/
│   ├── components/     # Reusable components
│   ├── models/         # MobX-State-Tree models
│   ├── navigators/     # React Navigation setup
│   ├── screens/        # App screens
│   ├── services/       # API services
│   ├── theme/          # Colors, spacing, typography
│   └── utils/          # Helper functions
├── test/               # Jest tests
└── ignite/             # Ignite config and templates
```

## CI/CD Integration

### Build and Test in GitHub Actions
```yaml
- name: Install dependencies
  shell: npm install

- name: Run linter
  shell: npm run lint

- name: Run tests
  shell: npm test

- name: Build iOS
  shell: npm run build:ios
  when: platform == ios

- name: Build Android
  shell: npm run build:android
  when: platform == android
```

## Agent Use
- Scaffold mobile applications with best practices
- Generate boilerplate code for screens and components
- Automate React Native project setup in CI/CD
- Standardize mobile app architecture across teams
- Bootstrap MVPs and prototypes quickly
- Create consistent project structure for mobile teams

## Advanced Configuration
```yaml
- preset: ignite
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Ignite CLI |

## Troubleshooting

### Installation Issues
```bash
# Clear npm cache
npm cache clean --force

# Reinstall globally
npm uninstall -g ignite-cli
npm install -g ignite-cli

# Check version
ignite --version
```

### Generator Problems
```bash
# List available generators
ignite generate --list

# Use verbose mode
ignite generate screen MyScreen --verbose
```

## Platform Support
- ✅ Linux (npm)
- ✅ macOS (npm, Homebrew)
- ✅ Windows (npm)

## Uninstall
```yaml
- preset: ignite
  with:
    state: absent
```

## Resources
- Official docs: https://ignitecookbook.com/
- GitHub: https://github.com/infinitered/ignite
- Infinite Red: https://infinite.red/
- Search: "ignite cli tutorial", "react native boilerplate", "ignite generators"

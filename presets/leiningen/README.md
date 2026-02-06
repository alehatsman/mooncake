# leiningen - Clojure Build Tool

Build automation and dependency management tool for Clojure projects, analogous to Maven for Java.

## Quick Start
```yaml
- preset: leiningen
```

## Features
- **Project scaffolding**: Create new projects from templates
- **Dependency management**: Maven and Clojars repository integration
- **REPL**: Interactive development with nREPL
- **Task automation**: Custom tasks and plugins
- **Uberjar creation**: Standalone executable JAR files
- **Testing**: Built-in test runner

## Basic Usage
```bash
# Create new project
lein new app my-project
lein new lib my-library

# Run REPL
lein repl

# Run application
lein run

# Run tests
lein test

# Build uberjar
lein uberjar

# Install to local Maven repository
lein install

# Deploy to repository
lein deploy clojars

# Check for outdated dependencies
lein ancient

# Show dependency tree
lein deps :tree
```

## Advanced Configuration
```yaml
- preset: leiningen
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove leiningen |

## Platform Support
- ✅ Linux (script install)
- ✅ macOS (Homebrew)
- ✅ Windows (manual install)

## Configuration
- **Project file**: `project.clj` in project root
- **Profiles**: `~/.lein/profiles.clj` for global config
- **Plugins**: Defined in project.clj or profiles.clj

## Real-World Examples

### Project Setup
```clojure
; project.clj
(defproject myapp "0.1.0-SNAPSHOT"
  :description "My Clojure application"
  :url "https://example.com/myapp"
  :dependencies [[org.clojure/clojure "1.11.1"]
                 [ring/ring-core "1.9.6"]
                 [compojure "1.7.0"]]
  :main ^:skip-aot myapp.core
  :target-path "target/%s"
  :profiles {:uberjar {:aot :all}
             :dev {:dependencies [[ring/ring-mock "0.4.0"]]}})
```

### CI/CD Build Pipeline
```yaml
- name: Install Leiningen
  preset: leiningen

- name: Run tests
  shell: lein test
  cwd: /path/to/project

- name: Build uberjar
  shell: lein uberjar
  register: build

- name: Deploy artifact
  shell: lein deploy clojars
  when: build.rc == 0
```

### Development Profiles
```clojure
; ~/.lein/profiles.clj
{:user {:plugins [[cider/cider-nrepl "0.28.5"]
                  [lein-ancient "0.7.0"]
                  [lein-kibit "0.1.8"]]
        :dependencies [[slamhound "1.5.5"]
                       [criterium "0.4.6"]]}}
```

### Custom Task
```clojure
; project.clj
:aliases {"build-and-test" ["do" "clean," "test," "uberjar"]}
```

Run with:
```bash
lein build-and-test
```

## Agent Use
- Automated Clojure project builds
- Dependency management in CI/CD
- REPL-driven development workflows
- Artifact deployment automation
- Project scaffolding and initialization

## Troubleshooting

### Dependencies not downloading
Clear local repository cache:
```bash
rm -rf ~/.m2/repository
lein deps
```

### OutOfMemoryError
Increase heap size:
```bash
export LEIN_JVM_OPTS="-Xmx2g"
lein run
```

### Plugin not found
Update dependencies:
```bash
lein deps
```

Check plugin coordinates in project.clj.

## Uninstall
```yaml
- preset: leiningen
  with:
    state: absent
```

## Resources
- Official site: https://leiningen.org/
- Tutorial: https://github.com/technomancy/leiningen/blob/stable/doc/TUTORIAL.md
- Sample project.clj: https://github.com/technomancy/leiningen/blob/stable/sample.project.clj
- Search: "leiningen clojure tutorial", "lein project setup"

# maven - Java Build Tool and Dependency Manager

Apache Maven is a build automation and dependency management tool primarily for Java projects, following the convention-over-configuration paradigm.

## Quick Start
```yaml
- preset: maven
```

## Features
- **Dependency management**: Automatic download and version control
- **Build lifecycle**: Standardized build phases (compile, test, package, deploy)
- **Convention over configuration**: Minimal setup required
- **Plugin ecosystem**: 1000+ plugins for various tasks
- **Multi-module support**: Build multiple related projects
- **Repository integration**: Maven Central and custom repositories

## Basic Usage
```bash
# Create new project
mvn archetype:generate \
  -DgroupId=com.example \
  -DartifactId=myapp \
  -DarchetypeArtifactId=maven-archetype-quickstart

# Build project
mvn clean install

# Run tests
mvn test

# Package application
mvn package

# Skip tests
mvn install -DskipTests

# Clean build artifacts
mvn clean

# Show dependency tree
mvn dependency:tree

# Update dependencies
mvn versions:display-dependency-updates
```

## Advanced Configuration
```yaml
- preset: maven
  with:
    version: "3.9.6"            # Specific Maven version
    state: present              # Install or remove (present/absent)
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether Maven should be installed (present) or removed (absent) |
| version | string | latest | Specific version to install (e.g., "3.9.6") |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, binary)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (binary, Chocolatey)

## Configuration

**Settings file**: `~/.m2/settings.xml`

Example `settings.xml`:
```xml
<settings>
  <mirrors>
    <mirror>
      <id>central-mirror</id>
      <url>https://repo1.maven.org/maven2</url>
      <mirrorOf>central</mirrorOf>
    </mirror>
  </mirrors>

  <profiles>
    <profile>
      <id>dev</id>
      <properties>
        <env>development</env>
      </properties>
    </profile>
  </profiles>
</settings>
```

**POM file**: `pom.xml` (Project Object Model)
```xml
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>myapp</artifactId>
  <version>1.0.0</version>

  <dependencies>
    <dependency>
      <groupId>junit</groupId>
      <artifactId>junit</artifactId>
      <version>4.13.2</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>
```

## Real-World Examples

### CI/CD Pipeline
```bash
# Build and test
mvn clean verify

# Deploy to repository
mvn clean deploy -DskipTests

# Build Docker image
mvn clean package
docker build -t myapp:latest .
```

### Multi-Module Project
```bash
# Build all modules
mvn clean install

# Build specific module
mvn clean install -pl module-name

# Build module and dependencies
mvn clean install -pl module-name -am
```

### Release Management
```bash
# Prepare release
mvn release:prepare

# Perform release
mvn release:perform

# Create snapshot
mvn deploy -DaltDeploymentRepository=snapshots::default::https://repo.example.com/snapshots
```

### Dependency Management
```bash
# Check for dependency updates
mvn versions:display-dependency-updates

# Update all dependencies
mvn versions:use-latest-releases

# Analyze dependencies
mvn dependency:analyze
```

## Agent Use
- Automate Java application builds in CI/CD pipelines
- Manage dependencies and versions programmatically
- Generate project templates and scaffolding
- Run automated tests and quality checks
- Deploy artifacts to repositories

## Troubleshooting

### Dependency resolution failures
Clear local repository cache:
```bash
rm -rf ~/.m2/repository
mvn clean install
```

### Out of memory
Increase heap size:
```bash
export MAVEN_OPTS="-Xmx2048m -XX:MaxPermSize=512m"
mvn clean install
```

### Plugin execution errors
Update plugin versions in `pom.xml` or use `-U` flag:
```bash
mvn clean install -U
```

## Common Commands

```bash
# Lifecycle phases
mvn validate        # Validate project
mvn compile         # Compile source
mvn test           # Run tests
mvn package        # Create JAR/WAR
mvn verify         # Run integration tests
mvn install        # Install to local repo
mvn deploy         # Deploy to remote repo

# Plugin goals
mvn help:describe   # Describe plugin
mvn dependency:tree # Show dependencies
mvn clean          # Remove target directory
```

## Uninstall
```yaml
- preset: maven
  with:
    state: absent
```

## Resources
- Official docs: https://maven.apache.org/guides/
- Central repository: https://search.maven.org/
- Search: "maven tutorial", "maven pom.xml examples", "maven lifecycle"

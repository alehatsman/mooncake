# Gradle - Build Automation Tool

Flexible build automation tool for JVM languages. Modern alternative to Maven with Groovy/Kotlin DSL and powerful dependency management.

## Quick Start
```yaml
- preset: gradle
```

## Features
- **Multi-language**: Java, Kotlin, Groovy, Scala support
- **Incremental builds**: Only rebuild what changed
- **Flexible DSL**: Groovy or Kotlin build scripts
- **Dependency management**: Maven Central, custom repositories
- **Plugin ecosystem**: Thousands of community plugins
- **Build cache**: Local and remote caching for faster builds

## Basic Usage
```bash
# Create new project
gradle init

# Build project
gradle build

# Run tests
gradle test

# Clean build artifacts
gradle clean

# Run application
gradle run

# List tasks
gradle tasks

# Build without tests
gradle build -x test

# Continuous build (watch mode)
gradle build --continuous
```

## Advanced Configuration
```yaml
- preset: gradle
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Gradle |

## Platform Support
- ✅ Linux (SDKMAN, package managers, binary download)
- ✅ macOS (Homebrew, SDKMAN, binary download)
- ✅ Windows (Scoop, Chocolatey, binary download)

## Configuration
- **Build file**: `build.gradle` (Groovy) or `build.gradle.kts` (Kotlin)
- **Settings**: `settings.gradle` or `settings.gradle.kts`
- **Properties**: `gradle.properties`
- **Wrapper**: `gradlew` (recommended for projects)
- **Cache**: `~/.gradle/caches/`

## Real-World Examples

### Java Spring Boot Application
```groovy
// build.gradle
plugins {
    id 'java'
    id 'org.springframework.boot' version '3.2.0'
    id 'io.spring.dependency-management' version '1.1.4'
}

group = 'com.example'
version = '1.0.0'
sourceCompatibility = '17'

repositories {
    mavenCentral()
}

dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web'
    implementation 'org.springframework.boot:spring-boot-starter-data-jpa'
    runtimeOnly 'com.h2database:h2'
    testImplementation 'org.springframework.boot:spring-boot-starter-test'
}

tasks.named('test') {
    useJUnitPlatform()
}
```

### Multi-Module Project
```groovy
// settings.gradle
rootProject.name = 'my-app'
include 'api', 'core', 'web'

// build.gradle (root)
subprojects {
    apply plugin: 'java'

    repositories {
        mavenCentral()
    }

    dependencies {
        testImplementation 'junit:junit:4.13.2'
    }
}

// api/build.gradle
dependencies {
    implementation project(':core')
    implementation 'com.google.guava:guava:32.1.3-jre'
}
```

### Kotlin DSL
```kotlin
// build.gradle.kts
plugins {
    kotlin("jvm") version "1.9.21"
    application
}

group = "com.example"
version = "1.0.0"

repositories {
    mavenCentral()
}

dependencies {
    implementation(kotlin("stdlib"))
    implementation("com.google.code.gson:gson:2.10.1")
    testImplementation(kotlin("test"))
}

tasks.test {
    useJUnitPlatform()
}

application {
    mainClass.set("com.example.MainKt")
}
```

### Docker Build Integration
```groovy
// build.gradle
plugins {
    id 'java'
    id 'com.bmuschko.docker-spring-boot-application' version '9.4.0'
}

docker {
    springBootApplication {
        baseImage = 'eclipse-temurin:17-jre'
        ports = [8080]
        images = ["myapp:${project.version}", "myapp:latest"]
    }
}

// Build Docker image
// gradle dockerBuildImage
```

### Custom Tasks
```groovy
// build.gradle
task generateDocs(type: Javadoc) {
    source = sourceSets.main.allJava
    classpath = configurations.compileClasspath
}

task copyResources(type: Copy) {
    from 'src/main/resources'
    into "$buildDir/output"
}

task bundle(type: Zip) {
    from 'build/libs'
    archiveFileName = "app-${version}.zip"
}

build.finalizedBy bundle
```

## Agent Use
- Build and test Java/Kotlin projects in CI/CD
- Manage multi-module application dependencies
- Generate distribution packages and artifacts
- Run automated tests and code quality checks
- Build Docker images from Java applications
- Publish libraries to Maven repositories

## Troubleshooting

### Build fails
```bash
# Clean and rebuild
gradle clean build

# Build with stacktrace
gradle build --stacktrace

# Debug build
gradle build --debug

# Refresh dependencies
gradle build --refresh-dependencies
```

### Dependency resolution errors
```bash
# List dependencies
gradle dependencies

# Check for conflicts
gradle dependencyInsight --dependency <name>

# Force dependency version
dependencies {
    implementation('com.example:lib:1.0') {
        force = true
    }
}

# Exclude transitive dependency
implementation('com.example:lib:1.0') {
    exclude group: 'org.unwanted', module: 'dependency'
}
```

### Out of memory
```bash
# Increase heap in gradle.properties
org.gradle.jvmargs=-Xmx4g -XX:MaxMetaspaceSize=512m

# Or via environment
export GRADLE_OPTS="-Xmx4g"
gradle build
```

### Slow builds
```bash
# Enable build cache
gradle build --build-cache

# Or in gradle.properties
org.gradle.caching=true

# Enable parallel execution
org.gradle.parallel=true
org.gradle.workers.max=4

# Use daemon
org.gradle.daemon=true

# Profile build
gradle build --profile
# See report in build/reports/profile/
```

### Gradle wrapper issues
```bash
# Generate wrapper
gradle wrapper --gradle-version 8.5

# Use wrapper
./gradlew build  # Linux/macOS
gradlew.bat build  # Windows

# Update wrapper
./gradlew wrapper --gradle-version 8.5
```

## Uninstall
```yaml
- preset: gradle
  with:
    state: absent
```

## Resources
- Official docs: https://docs.gradle.org/
- User manual: https://docs.gradle.org/current/userguide/userguide.html
- Plugin portal: https://plugins.gradle.org/
- Build scans: https://scans.gradle.com/
- Search: "gradle tutorial", "gradle vs maven", "gradle kotlin dsl"

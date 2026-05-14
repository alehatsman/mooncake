# SBT - Scala Build Tool

Interactive build tool for Scala and Java projects. Manages dependencies, compiles code, runs tests, packages applications.

## Quick Start
```yaml
- preset: sbt
```

## Features
- **Incremental compilation**: Recompiles only changed files
- **Interactive shell**: Run commands without restart
- **Parallel execution**: Tasks run concurrently
- **Dependency management**: Ivy and Maven repository support
- **Multi-project builds**: Monorepo support
- **Plugin ecosystem**: 1000+ community plugins
- **Cross-building**: Target multiple Scala versions

## Basic Usage
```bash
# Compile project
sbt compile

# Run application
sbt run

# Run tests
sbt test

# Package JAR
sbt package

# Interactive mode
sbt
> compile
> test
> run
```

## Advanced Configuration
```yaml
# Install SBT (default)
- preset: sbt

# Uninstall SBT
- preset: sbt
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ✅ Windows (Chocolatey, Scoop)
- ✅ Universal (manual install)

## Configuration
- **Global config**: `~/.sbt/1.0/`
- **Project config**: `build.sbt`, `project/build.properties`
- **Plugins**: `project/plugins.sbt`
- **Repository cache**: `~/.ivy2/cache/` (Ivy), `~/.m2/repository/` (Maven)
- **JVM options**: `.sbtopts`, `.jvmopts`

## Project Structure
```bash
myproject/
├── build.sbt                 # Build definition
├── project/
│   ├── build.properties     # SBT version
│   └── plugins.sbt          # SBT plugins
├── src/
│   ├── main/
│   │   ├── scala/           # Scala sources
│   │   ├── java/            # Java sources
│   │   └── resources/       # Resources
│   └── test/
│       ├── scala/           # Test sources
│       └── resources/       # Test resources
└── target/                   # Build output
```

## Common Tasks
```bash
# Compilation
sbt compile              # Compile main sources
sbt test:compile         # Compile test sources
sbt clean               # Remove generated files
sbt clean compile       # Clean build

# Running
sbt run                 # Run main class
sbt "run arg1 arg2"     # Run with arguments
sbt test:run            # Run test main

# Testing
sbt test                # Run all tests
sbt testOnly com.example.MyTest  # Run specific test
sbt testQuick           # Run failed tests
sbt "testOnly *MyTest"  # Pattern matching

# Packaging
sbt package             # Create JAR
sbt packageBin          # Binary JAR
sbt packageSrc          # Source JAR
sbt packageDoc          # Scaladoc JAR

# Publishing
sbt publish             # Publish to repo
sbt publishLocal        # Publish to local Ivy
sbt publishM2           # Publish to local Maven
```

## Build Definition
```scala
// build.sbt
name := "myproject"
version := "1.0.0"
scalaVersion := "3.3.1"

// Dependencies
libraryDependencies ++= Seq(
  "org.typelevel" %% "cats-core" % "2.10.0",
  "org.scalatest" %% "scalatest" % "3.2.17" % Test
)

// Compiler options
scalacOptions ++= Seq(
  "-deprecation",
  "-feature",
  "-unchecked"
)

// Resolvers
resolvers += "Artima Maven Repository" at "https://repo.artima.com/releases"

// Custom task
lazy val hello = taskKey[Unit]("Prints Hello")
hello := println("Hello!")
```

## Multi-Project Builds
```scala
// build.sbt
lazy val root = (project in file("."))
  .aggregate(core, api, web)
  .settings(
    name := "myapp"
  )

lazy val core = (project in file("modules/core"))
  .settings(
    name := "myapp-core",
    libraryDependencies ++= Seq(
      "org.typelevel" %% "cats-core" % "2.10.0"
    )
  )

lazy val api = (project in file("modules/api"))
  .dependsOn(core)
  .settings(
    name := "myapp-api",
    libraryDependencies ++= Seq(
      "com.typesafe.akka" %% "akka-http" % "10.5.3"
    )
  )

lazy val web = (project in file("modules/web"))
  .dependsOn(api)
  .settings(
    name := "myapp-web"
  )
```

## Plugins
```scala
// project/plugins.sbt
// Assembly plugin
addSbtPlugin("com.eed3si9n" % "sbt-assembly" % "2.1.5")

// Scalafmt
addSbtPlugin("org.scalameta" % "sbt-scalafmt" % "2.5.2")

// Native packager
addSbtPlugin("com.github.sbt" % "sbt-native-packager" % "1.9.16")

// Dependency updates
addSbtPlugin("com.timushev.sbt" % "sbt-updates" % "0.6.4")

// Coverage
addSbtPlugin("org.scoverage" % "sbt-scoverage" % "2.0.11")
```

## Interactive Commands
```bash
# Start interactive mode
sbt

# Inside SBT shell
> compile              # Compile
> ~compile             # Continuous compile on file change
> test                 # Run tests
> ~test                # Continuous testing
> projects             # List projects
> project api          # Switch project
> reload               # Reload build definition
> help                 # Show help
> exit                 # Exit SBT
```

## Dependency Management
```scala
// Single dependency
libraryDependencies += "org.scala-lang" % "scala-library" % "2.13.12"

// Scala version dependent (%%  adds _2.13)
libraryDependencies += "org.typelevel" %% "cats-core" % "2.10.0"

// Multiple dependencies
libraryDependencies ++= Seq(
  "com.typesafe.akka" %% "akka-actor" % "2.8.5",
  "com.typesafe.akka" %% "akka-stream" % "2.8.5",
  "com.typesafe.akka" %% "akka-http" % "10.5.3"
)

// Test dependencies
libraryDependencies ++= Seq(
  "org.scalatest" %% "scalatest" % "3.2.17" % Test,
  "org.scalatestplus" %% "mockito-4-11" % "3.2.17.0" % Test
)

// Exclude transitive dependencies
libraryDependencies += "org.apache.spark" %% "spark-core" % "3.5.0" exclude("org.slf4j", "slf4j-log4j12")
```

## Cross-Building
```scala
// Cross-build for multiple Scala versions
crossScalaVersions := Seq("2.13.12", "3.3.1")

// Version-specific dependencies
libraryDependencies ++= {
  CrossVersion.partialVersion(scalaVersion.value) match {
    case Some((2, 13)) =>
      Seq("org.scala-lang.modules" %% "scala-parallel-collections" % "1.0.4")
    case _ =>
      Seq.empty
  }
}
```

## Assembly Plugin
```scala
// project/plugins.sbt
addSbtPlugin("com.eed3si9n" % "sbt-assembly" % "2.1.5")

// build.sbt
assembly / assemblyMergeStrategy := {
  case PathList("META-INF", xs @ _*) => MergeStrategy.discard
  case x => MergeStrategy.first
}

assembly / mainClass := Some("com.example.Main")
```

```bash
# Create fat JAR
sbt assembly

# Run fat JAR
java -jar target/scala-3.3.1/myproject-assembly-1.0.0.jar
```

## Real-World Examples

### Web Application
```scala
// build.sbt
name := "webapp"
version := "1.0.0"
scalaVersion := "3.3.1"

libraryDependencies ++= Seq(
  "com.typesafe.akka" %% "akka-http" % "10.5.3",
  "com.typesafe.akka" %% "akka-stream" % "2.8.5",
  "com.typesafe.akka" %% "akka-http-spray-json" % "10.5.3",
  "ch.qos.logback" % "logback-classic" % "1.4.14",
  "org.scalatest" %% "scalatest" % "3.2.17" % Test
)
```

```bash
# Development
sbt ~compile            # Auto-compile on changes
sbt ~test              # Auto-test on changes
sbt run                # Start server

# Production
sbt assembly           # Build fat JAR
java -jar target/scala-3.3.1/webapp-assembly-1.0.0.jar
```

### Library Project
```scala
// build.sbt
organization := "com.example"
name := "mylib"
version := "0.1.0"
scalaVersion := "3.3.1"

libraryDependencies ++= Seq(
  "org.typelevel" %% "cats-core" % "2.10.0",
  "org.scalatest" %% "scalatest" % "3.2.17" % Test
)

// Publishing
publishTo := Some("Sonatype Snapshots" at "https://oss.sonatype.org/content/repositories/snapshots")
```

### Microservices Monorepo
```scala
// build.sbt
lazy val commonSettings = Seq(
  scalaVersion := "3.3.1",
  libraryDependencies ++= Seq(
    "com.typesafe.akka" %% "akka-actor" % "2.8.5",
    "org.scalatest" %% "scalatest" % "3.2.17" % Test
  )
)

lazy val root = (project in file("."))
  .aggregate(auth, users, orders)

lazy val auth = (project in file("services/auth"))
  .settings(commonSettings)

lazy val users = (project in file("services/users"))
  .dependsOn(auth)
  .settings(commonSettings)

lazy val orders = (project in file("services/orders"))
  .dependsOn(auth)
  .settings(commonSettings)
```

## CI/CD Integration
```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-java@v4
        with:
          java-version: '17'
          distribution: 'temurin'

      - name: Install SBT
        preset: sbt

      - name: Compile
        shell: sbt compile

      - name: Run tests
        shell: sbt test

      - name: Build assembly
        shell: sbt assembly
```

## Performance Tuning
```bash
# .sbtopts
-J-Xmx2G
-J-Xss4M
-J-XX:MaxMetaspaceSize=512M
-J-XX:ReservedCodeCacheSize=256M
-J-XX:+UseG1GC

# Parallel execution
-Dsbt.task.timings=true
-Dsbt.task.timings.on.shutdown=true
```

## Troubleshooting

### Out of memory errors
Increase JVM heap:
```bash
# .sbtopts
-J-Xmx4G
-J-Xss8M
```

### Slow compilation
Enable parallel compilation:
```scala
// build.sbt
Global / concurrentRestrictions := Seq(
  Tags.limitAll(4)  // 4 parallel tasks
)
```

### Dependency conflicts
Show dependency tree:
```bash
sbt dependencyTree
sbt evicted  # Show evicted dependencies
```

### Clear cache
```bash
# Clear Ivy cache
rm -rf ~/.ivy2/cache

# Clear SBT cache
rm -rf ~/.sbt/1.0/

# Clear project target
sbt clean
```

## Best Practices
- Use `build.sbt` for simple builds, `project/` for complex setups
- Enable continuous compilation: `sbt ~compile`
- Commit `project/build.properties` to lock SBT version
- Use `.sbtopts` for JVM options
- Keep dependencies up to date: `sbt dependencyUpdates`
- Run `sbt clean` when switching branches
- Use `reload` in SBT shell after config changes

## Agent Use
- Automate Scala project compilation and testing
- Manage multi-project builds in CI/CD
- Generate deployment artifacts (JARs, Docker images)
- Run code quality checks (Scalafmt, Scalafix)
- Publish libraries to artifact repositories
- Cross-compile for multiple Scala versions
- Execute database migrations as SBT tasks

## Uninstall
```yaml
- preset: sbt
  with:
    state: absent
```

## Resources
- Official site: https://www.scala-sbt.org/
- Documentation: https://www.scala-sbt.org/1.x/docs/
- GitHub: https://github.com/sbt/sbt
- Community plugins: https://www.scala-sbt.org/release/docs/Community-Plugins.html
- Search: "sbt tutorial", "sbt best practices", "sbt multi-project"

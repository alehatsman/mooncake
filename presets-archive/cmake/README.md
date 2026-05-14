# cmake - Cross-Platform Build System

CMake is an extensible, open-source build system that manages the build process in an operating system and compiler-independent manner.

## Quick Start
```yaml
- preset: cmake
```

## Features
- **Cross-platform**: Linux, macOS, and Windows support
- **Generator-agnostic**: Makefiles, Ninja, Visual Studio, Xcode
- **Language support**: C, C++, Fortran, CUDA, Objective-C
- **Modern CMake**: Target-based builds with properties
- **Package management**: Built-in find_package() system
- **Testing**: Integrated CTest framework

## Basic Usage
```bash
# Check version
cmake --version

# Generate build files (Unix Makefiles)
cmake -S . -B build

# Generate with specific generator
cmake -S . -B build -G Ninja
cmake -S . -B build -G Xcode

# Configure with options
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release

# Build project
cmake --build build

# Install project
cmake --install build --prefix /usr/local

# Run tests
cd build && ctest

# Clean build
cmake --build build --target clean
```

## Advanced Configuration
```yaml
- preset: cmake
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove cmake |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration
- **CMake cache**: `build/CMakeCache.txt` (generated per-project)
- **CMake files**: `build/CMakeFiles/` (generated per-project)
- **Config files**: `~/.cmake/` (user-specific settings)
- **Package registry**: `~/.cmake/packages/` (found packages)

## Real-World Examples

### C++ Project Build
```bash
# Configure with debug symbols
cmake -S . -B build -DCMAKE_BUILD_TYPE=Debug

# Build with multiple cores
cmake --build build -j$(nproc)

# Install to custom prefix
cmake --install build --prefix ~/.local
```

### Cross-Compilation
```bash
# Configure for ARM
cmake -S . -B build-arm \
  -DCMAKE_TOOLCHAIN_FILE=toolchain-arm.cmake \
  -DCMAKE_BUILD_TYPE=Release

# Build cross-compiled binaries
cmake --build build-arm
```

### CI/CD Pipeline
```bash
# Configure, build, test in CI
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build --config Release
cd build && ctest --output-on-failure
```

## CMakeLists.txt Example
```cmake
cmake_minimum_required(VERSION 3.15)
project(MyApp VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_executable(myapp src/main.cpp src/app.cpp)
target_include_directories(myapp PRIVATE include)

# Link libraries
find_package(Threads REQUIRED)
target_link_libraries(myapp PRIVATE Threads::Threads)

# Install rules
install(TARGETS myapp DESTINATION bin)

# Testing
enable_testing()
add_test(NAME mytest COMMAND myapp --test)
```

## Common Build Types
- **Debug**: `-DCMAKE_BUILD_TYPE=Debug` (symbols, no optimization)
- **Release**: `-DCMAKE_BUILD_TYPE=Release` (optimized, no symbols)
- **RelWithDebInfo**: `-DCMAKE_BUILD_TYPE=RelWithDebInfo` (optimized + symbols)
- **MinSizeRel**: `-DCMAKE_BUILD_TYPE=MinSizeRel` (size-optimized)

## Agent Use
- Automated C/C++ project builds
- Cross-platform compilation workflows
- CI/CD build automation
- Multi-configuration testing (Debug, Release)
- Package generation and installation
- Build dependency management

## Troubleshooting

### Cache issues
Clear the cache if configuration fails:
```bash
rm -rf build/CMakeCache.txt build/CMakeFiles
cmake -S . -B build
```

### Generator not found
Install the build tool:
```bash
# For Ninja
sudo apt-get install ninja-build  # Linux
brew install ninja                 # macOS

# For Make (usually pre-installed)
sudo apt-get install build-essential  # Linux
```

### Package not found
Specify package hints:
```bash
cmake -S . -B build -DCMAKE_PREFIX_PATH=/path/to/packages
```

## Uninstall
```yaml
- preset: cmake
  with:
    state: absent
```

## Resources
- Official docs: https://cmake.org/documentation/
- CMake tutorial: https://cmake.org/cmake/help/latest/guide/tutorial/index.html
- Modern CMake book: https://cliutils.gitlab.io/modern-cmake/
- Search: "cmake tutorial", "modern cmake best practices"

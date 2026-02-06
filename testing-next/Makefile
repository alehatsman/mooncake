# Mooncake Multi-Architecture Testing Framework
# Supports linux/amd64 and linux/arm64

.PHONY: help setup-buildx binary-amd64 binary-arm64 binaries
.PHONY: build-ubuntu-amd64 build-ubuntu-arm64 build-ubuntu build-all
.PHONY: test-ubuntu-amd64 test-ubuntu-arm64 test-ubuntu test-all
.PHONY: clean clean-binaries clean-docker clean-all

# Configuration
PROJECT_ROOT := $(CURDIR)/..
ARTIFACTS_DIR := $(PROJECT_ROOT)/artifacts
IMAGE_PREFIX := mooncake-test
BINARY_AMD64 := $(PROJECT_ROOT)/mooncake-linux-amd64
BINARY_ARM64 := $(PROJECT_ROOT)/mooncake-linux-arm64

help: ## Show this help
	@echo "Mooncake Multi-Architecture Testing Framework"
	@echo ""
	@echo "Setup:"
	@echo "  setup-buildx    - Setup Docker buildx for multi-platform builds"
	@echo ""
	@echo "Build Binaries:"
	@echo "  binary-amd64    - Build mooncake for linux/amd64"
	@echo "  binary-arm64    - Build mooncake for linux/arm64"
	@echo "  binaries        - Build both architectures"
	@echo ""
	@echo "Build Images:"
	@echo "  build-ubuntu-amd64  - Build Ubuntu amd64 test image"
	@echo "  build-ubuntu-arm64  - Build Ubuntu arm64 test image"
	@echo "  build-ubuntu        - Build Ubuntu for both architectures"
	@echo "  build-all           - Build all images (all distros, all arches)"
	@echo ""
	@echo "Run Tests:"
	@echo "  test-ubuntu-amd64   - Test on Ubuntu amd64"
	@echo "  test-ubuntu-arm64   - Test on Ubuntu arm64"
	@echo "  test-ubuntu         - Test on Ubuntu (native arch)"
	@echo "  test-all            - Test all configurations"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean-binaries  - Remove compiled binaries"
	@echo "  clean-docker    - Remove Docker images"
	@echo "  clean-all       - Remove everything (artifacts, binaries, images)"
	@echo ""
	@echo "Examples:"
	@echo "  make setup-buildx"
	@echo "  make test-ubuntu-amd64"
	@echo "  make test-all"

# Setup Docker buildx for multi-platform builds
setup-buildx: ## Setup Docker buildx
	@echo "Setting up Docker buildx..."
	@docker buildx create --name mooncake-builder --use --bootstrap 2>/dev/null || \
		docker buildx use mooncake-builder 2>/dev/null || \
		echo "Buildx already configured"
	@docker buildx inspect --bootstrap

# Build binaries for different architectures
binary-amd64: ## Build linux/amd64 binary
	@echo "Building mooncake for linux/amd64..."
	@cd $(PROJECT_ROOT) && GOOS=linux GOARCH=amd64 go build -o mooncake-linux-amd64 ./cmd
	@echo "Binary: $(BINARY_AMD64)"
	@ls -lh $(BINARY_AMD64)

binary-arm64: ## Build linux/arm64 binary
	@echo "Building mooncake for linux/arm64..."
	@cd $(PROJECT_ROOT) && GOOS=linux GOARCH=arm64 go build -o mooncake-linux-arm64 ./cmd
	@echo "Binary: $(BINARY_ARM64)"
	@ls -lh $(BINARY_ARM64)

binaries: binary-amd64 binary-arm64 ## Build both architectures

# Build Docker images (amd64)
build-ubuntu-amd64: binary-amd64 ## Build Ubuntu amd64 test image
	@echo "Building Ubuntu amd64 test image..."
	@docker buildx build \
		--platform linux/amd64 \
		--load \
		--build-arg ARCH=amd64 \
		-f images/Dockerfile.ubuntu \
		-t $(IMAGE_PREFIX):ubuntu-amd64 \
		$(PROJECT_ROOT)

# Build Docker images (arm64)
build-ubuntu-arm64: binary-arm64 ## Build Ubuntu arm64 test image
	@echo "Building Ubuntu arm64 test image..."
	@docker buildx build \
		--platform linux/arm64 \
		--load \
		--build-arg ARCH=arm64 \
		-f images/Dockerfile.ubuntu \
		-t $(IMAGE_PREFIX):ubuntu-arm64 \
		$(PROJECT_ROOT)

# Build both architectures for Ubuntu
build-ubuntu: build-ubuntu-amd64 build-ubuntu-arm64 ## Build Ubuntu for both arches

# Detect native architecture
NATIVE_ARCH := $(shell uname -m)
ifeq ($(NATIVE_ARCH),x86_64)
    NATIVE_TAG := amd64
else ifeq ($(NATIVE_ARCH),arm64)
    NATIVE_TAG := arm64
else ifeq ($(NATIVE_ARCH),aarch64)
    NATIVE_TAG := arm64
else
    NATIVE_TAG := amd64
endif

# Test targets
test-ubuntu-amd64: build-ubuntu-amd64 ## Run tests on Ubuntu amd64
	@echo "Running preset tests on Ubuntu amd64..."
	@mkdir -p $(ARTIFACTS_DIR)/ubuntu-amd64
	@docker run --rm --platform linux/amd64 \
		--entrypoint /bin/sh \
		-v $(ARTIFACTS_DIR)/ubuntu-amd64:/artifacts \
		-v $(CURDIR):/testing \
		$(IMAGE_PREFIX):ubuntu-amd64 \
		-c "/bin/bash /testing/test-presets.sh --os ubuntu --artifacts /artifacts"

test-ubuntu-arm64: build-ubuntu-arm64 ## Run tests on Ubuntu arm64
	@echo "Running preset tests on Ubuntu arm64..."
	@mkdir -p $(ARTIFACTS_DIR)/ubuntu-arm64
	@docker run --rm --platform linux/arm64 \
		--entrypoint /bin/sh \
		-v $(ARTIFACTS_DIR)/ubuntu-arm64:/artifacts \
		-v $(CURDIR):/testing \
		$(IMAGE_PREFIX):ubuntu-arm64 \
		-c "/bin/bash /testing/test-presets.sh --os ubuntu --artifacts /artifacts"

test-ubuntu: ## Run tests on Ubuntu (native architecture)
	@echo "Running tests on native architecture: $(NATIVE_ARCH)"
	@$(MAKE) test-ubuntu-$(NATIVE_TAG)

# Test specific preset
PRESET ?= jq
test-preset-amd64: build-ubuntu-amd64 ## Test specific preset on amd64
	@echo "Testing preset '$(PRESET)' on Ubuntu amd64..."
	@mkdir -p $(ARTIFACTS_DIR)/ubuntu-amd64
	@docker run --rm --platform linux/amd64 \
		--entrypoint /bin/sh \
		-v $(ARTIFACTS_DIR)/ubuntu-amd64:/artifacts \
		-v $(CURDIR):/testing \
		$(IMAGE_PREFIX):ubuntu-amd64 \
		-c "/bin/bash /testing/test-presets.sh --os ubuntu --preset $(PRESET) --artifacts /artifacts"

test-preset-arm64: build-ubuntu-arm64 ## Test specific preset on arm64
	@echo "Testing preset '$(PRESET)' on Ubuntu arm64..."
	@mkdir -p $(ARTIFACTS_DIR)/ubuntu-arm64
	@docker run --rm --platform linux/arm64 \
		--entrypoint /bin/sh \
		-v $(ARTIFACTS_DIR)/ubuntu-arm64:/artifacts \
		-v $(CURDIR):/testing \
		$(IMAGE_PREFIX):ubuntu-arm64 \
		-c "/bin/bash /testing/test-presets.sh --os ubuntu --preset $(PRESET) --artifacts /artifacts"

test-preset: ## Test specific preset (native arch)
	@$(MAKE) test-preset-$(NATIVE_TAG) PRESET=$(PRESET)

# Cleanup targets
clean-binaries: ## Remove compiled binaries
	@echo "Cleaning binaries..."
	@rm -f $(BINARY_AMD64) $(BINARY_ARM64)
	@echo "Binaries removed"

clean-docker: ## Remove Docker images
	@echo "Cleaning Docker images..."
	@docker rmi -f $(IMAGE_PREFIX):ubuntu-amd64 2>/dev/null || true
	@docker rmi -f $(IMAGE_PREFIX):ubuntu-arm64 2>/dev/null || true
	@docker builder prune -f
	@echo "Docker images removed"

clean-all: clean-binaries clean-docker ## Remove everything
	@echo "Cleaning all artifacts..."
	@rm -rf $(ARTIFACTS_DIR)
	@echo "All cleaned!"

# Interactive shell
shell-ubuntu-amd64: build-ubuntu-amd64 ## Interactive shell in Ubuntu amd64 container
	@docker run --rm -it --platform linux/amd64 \
		-v $(ARTIFACTS_DIR)/ubuntu-amd64:/artifacts \
		-v $(CURDIR):/testing \
		$(IMAGE_PREFIX):ubuntu-amd64

shell-ubuntu-arm64: build-ubuntu-arm64 ## Interactive shell in Ubuntu arm64 container
	@docker run --rm -it --platform linux/arm64 \
		-v $(ARTIFACTS_DIR)/ubuntu-arm64:/artifacts \
		-v $(CURDIR):/testing \
		$(IMAGE_PREFIX):ubuntu-arm64

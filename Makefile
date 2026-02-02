build-binaries:
	bash ./scripts/build_cli_binary.sh
	sudo cp ./out/mooncake /usr/local/bin/mooncake
	sudo chmod +x /usr/local/bin/mooncake

build-ubuntu-binary:
	env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake ./cmd 
	sudo cp ./out/mooncake /usr/local/bin/mooncake
	sudo chmod +x /usr/local/bin/mooncake

build-darwin-binary:
	env GOOS=darwin GOARCH=amd64 go build -v -o out/mooncake ./cmd 

build-arm:
	env GOOS=darwin GOARCH=arm64 go build -v -o out/mooncake ./cmd

install-local:
	sudo cp ./out/mooncake /usr/local/bin/mooncake

local-arm:
	make build-arm;
	make install-local;

run-basic-test-in-ubuntu:
	docker build -f basic.Dockerfile -t mooncake-basic-test . --progress=plain

run-test-in-ubuntu:
	docker build -t mooncake-test . --progress=plain

run-ubuntu:
	./out/mooncake run -c ./mooncake-automation/main.yml -v ./mooncake-automation/global_variables.yml

release-latest:
	bash ./scripts/release_latest.sh

test-essentials:
	docker build -t mooncake-essential-test -f ./testing/essentials/Dockerfile .

serve-docs:
	pipenv run mkdocs serve

# CI targets
.PHONY: lint test scan ci

lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

test:
	@echo "Running tests..."
	@go test -v ./...

test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

test-ubuntu-docker:
	@echo "Running tests in Ubuntu Docker container (matches CI)..."
	@./scripts/test-ubuntu-quick.sh

test-ubuntu-docker-full:
	@echo "Running full test suite in Ubuntu Docker container..."
	@./scripts/test-ubuntu-docker.sh

scan: lint
	@echo "Running gosec security scan..."
	@gosec -exclude-generated ./...
	@echo ""
	@echo "Running govulncheck..."
	@govulncheck ./...

ci: lint test-race scan
	@echo ""
	@echo "✓ All CI checks passed!"

# Multi-platform testing targets
.PHONY: test-quick test-smoke test-integration test-docker-ubuntu test-docker-alpine test-docker-debian test-docker-fedora test-docker-all test-all-platforms

test-quick:
	@echo "Running quick smoke tests on Ubuntu 22.04..."
	@./scripts/test-docker.sh ubuntu-22.04 smoke

test-smoke:
	@echo "Running smoke tests on all distros..."
	@./scripts/test-docker-all.sh smoke

test-integration:
	@echo "Running integration tests..."
	@./scripts/run-integration-tests.sh

test-docker-ubuntu:
	@./scripts/test-docker.sh ubuntu-22.04

test-docker-alpine:
	@./scripts/test-docker.sh alpine-3.19

test-docker-debian:
	@./scripts/test-docker.sh debian-12

test-docker-fedora:
	@./scripts/test-docker.sh fedora-39

test-docker-all:
	@./scripts/test-docker-all.sh all

test-all-platforms:
	@./scripts/test-all-platforms.sh



build-binaries:
	bash ./scripts/build_cli_binary.sh

build-ubuntu-binary:
	env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake ./cmd 

build-darwin-binary:
	env GOOS=darwin GOARCH=amd64 go build -v -o out/mooncake ./cmd 

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

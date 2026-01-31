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

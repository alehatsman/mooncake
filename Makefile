build-ubuntu-binary:
	env GOOS=linux GOARCH=amd64 go build -o out/mooncake -v

run-test-in-ubuntu:
	docker build -t mooncake-test . --progress=plain

go-build-ubuntu:
	env GOOS=linux GOARCH=amd64 go build -o out/mooncake -v

docker-build-ubuntu:
	docker build -t mooncake-test . --progress=plain

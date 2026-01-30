build-ubuntu-binary:
	env GOOS=linux GOARCH=amd64 go build -o out/mooncake -v

build-darwin-binary:
	env GOOS=darwin GOARCH=amd64 go build -o out/mooncake -v

run-basic-test-in-ubuntu:
	docker build -f basic.Dockerfile -t mooncake-basic-test . --progress=plain

run-test-in-ubuntu:
	docker build -t mooncake-test . --progress=plain

run-ubuntu:
	./out/mooncake run -c ./mooncake-automation/main.yml -v ./mooncake-automation/global_variables.yml

release-latest:
	git tag --delete -f latest
	git push --delete origin latest
	gh release delete latest -y

	git tag latest
	git push origin latest

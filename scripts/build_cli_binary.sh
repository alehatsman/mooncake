#!/bin/bash

set -e

pairs=('darwin/amd64' 'darwin/arm64'
       'linux/386' 'linux/amd64' 'linux/arm' 'linux/arm64'
       'windows/386' 'windows/amd64' 'windows/arm')


for pair in "${pairs[@]}"; do
  os=$(echo $pair | cut -d'/' -f1)
  arch=$(echo $pair | cut -d'/' -f2)
  echo "Building for $os/$arch"
  GOOS=$os GOARCH=$arch go build -o ./out/mooncake-${os}-${arch} ./cmd/mooncake.go
done

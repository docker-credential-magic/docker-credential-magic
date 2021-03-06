#!/bin/bash -ex

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../

rm -rf .cover/
mkdir .cover/

export CGO_ENABLED=0
for pkg in `go list ./pkg/... | grep -v /vendor/`; do
    go test -v -covermode=atomic \
        -coverprofile=".cover/$(echo $pkg | sed 's/\//_/g').cover.out" $pkg
done

echo "mode: set" > .cover/cover.out && cat .cover/*.cover.out | grep -v mode: | sort -r | \
   awk '{if($1 != last) {print $0;last=$1}}' >> .cover/cover.out

go tool cover -html=.cover/cover.out -o=.cover/coverage.html

.PHONE: fetch-helpers
fetch-helpers:
	scripts/fetch-helper-gcr.sh

.PHONY: build
build:
	go build -o bin/docker-credential-magic .../cmd/docker-credential-magic

.PHONY: run
run:
	go run .../cmd/docker-credential-magic $(REF)
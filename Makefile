.PHONY: fetch-helpers
fetch-helpers:
	scripts/fetch-helper-acr-linux.sh
	scripts/fetch-helper-ecr-login.sh
	scripts/fetch-helper-gcr.sh

.PHONY: pkger-gen
pkger-gen:
	go run cmd/pkger-gen/main.go

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: build
build:
	go build -o bin/docker-credential-magic .../cmd/docker-credential-magic

.PHONY: run
run:
	go run .../cmd/docker-credential-magic $(REF)

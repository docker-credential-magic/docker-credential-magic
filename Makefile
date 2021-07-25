.PHONY: fetch-helpers
fetch-helpers:
	scripts/helpers/fetch-helper-acr-env.sh
	scripts/helpers/fetch-helper-ecr-login.sh
	scripts/helpers/fetch-helper-gcr.sh

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: build-helper
build-helper: vendor
build-helper:
	GOOS=linux GOARCH=amd64 \
		go build -o credential-helpers/docker-credential-magic \
			.../cmd/docker-credential-magic

.PHONY: pkger-gen
pkger-gen:
	go run cmd/pkger-gen/main.go

.PHONY: build
build: vendor
build:
	go build -o bin/docker-credential-magician .../cmd/docker-credential-magician

.PHONY: run
run:
	go run .../cmd/docker-credential-magician $(REF)

.PHONY: test
test: vendor
	scripts/test.sh

.PHONY: clean
clean:
	rm -rf bin/ tmp/ vendor/ credential-helpers/ \
		cmd/docker-credential-magician/pkged.go

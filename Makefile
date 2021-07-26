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
		go build -o pkg/magician/credential-helpers/docker-credential-magic \
			.../cmd/docker-credential-magic

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

.PHONY: acceptance
acceptance:
	scripts/acceptance.sh

.PHONY: clean
clean:
	rm -rf .venv/ .cover/ .robot/ bin/ tmp/ vendor/ pkg/magician/credential-helpers/

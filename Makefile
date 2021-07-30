SHELL   = /usr/bin/env bash
GIT_SHA = $(shell git rev-parse --short HEAD)
GIT_TAG = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)

VERSION = ${GIT_TAG}
ifeq ($(VERSION),)
	VERSION = ${GIT_SHA}-devel
endif

.PHONY: fetch-helpers
fetch-helpers:
	for i in $(shell find mappings -name '*.yml' -exec basename {} .yml \;); do \
		scripts/helpers/fetch-helper-$$i.sh; \
	done

.PHONY: copy-mappings
copy-mappings:
	cp -r mappings internal/embedded/mappings/embedded

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: build-magic
build-magic:
	go build -ldflags="-X main.Version=$(VERSION)" \
		-o bin/docker-credential-magic \
		.../cmd/docker-credential-magic

.PHONY: build-magic-embedded
build-magic-embedded:
	GOOS=linux GOARCH=amd64 \
		go build -ldflags="-X main.Version=$(VERSION)" \
			-o internal/embedded/helpers/embedded/docker-credential-magic \
			.../cmd/docker-credential-magic

.PHONY: build-magician
build-magician:
	go build -ldflags="-X main.Version=$(VERSION)" \
		-o bin/docker-credential-magician \
		.../cmd/docker-credential-magician

.PHONY: test
test:
	scripts/test.sh

.PHONY: acceptance
acceptance:
	scripts/acceptance.sh

.PHONY: clean
clean:
	rm -rf .venv/ .cover/ .robot/ bin/ tmp/ vendor/ \
		internal/embedded/helpers/embedded \
		internal/embedded/mappings/embedded

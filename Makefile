.PHONY: fetch-helpers
fetch-helpers:
	scripts/helpers/fetch-helper-acr-env.sh
	scripts/helpers/fetch-helper-ecr-login.sh
	scripts/helpers/fetch-helper-gcr.sh

.PHONY: copy-mappings
copy-mappings:
	cp default-mappings.yml pkg/magician/default-mappings.yml

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: build-magic
build-magic:
	go build -o bin/docker-credential-magic \
		.../cmd/docker-credential-magic

.PHONY: build-magic-embedded
build-magic-embedded:
	GOOS=linux GOARCH=amd64 \
		go build -o pkg/magician/credential-helpers/docker-credential-magic \
			.../cmd/docker-credential-magic

.PHONY: build-magician
build-magician:
	go build -o bin/docker-credential-magician .../cmd/docker-credential-magician

.PHONY: test
test:
	scripts/test.sh

.PHONY: acceptance
acceptance:
	scripts/acceptance.sh

.PHONY: clean
clean:
	rm -rf .venv/ .cover/ .robot/ bin/ tmp/ vendor/ pkg/magician/credential-helpers/

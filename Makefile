# All Go tooling runs inside a pinned `golang` container -- no local Go
# installation is assumed. `make image` needs nothing but docker itself,
# since the Dockerfile's own builder stage does the compilation.

GO_IMAGE    ?= golang:1.23-bookworm
IMAGE_NAME  ?= vault-aws-credential-helper
IMAGE_TAG   ?= latest

# `make image` defaults to the *host's* platform, so the result can actually
# be `docker run` here (override with e.g. PLATFORM=linux/arm64 to
# cross-build without running it). This is for local dev/testing only --
# release images are multi-arch (linux/amd64 + linux/arm64) and are built and
# published by CI directly, not via this Makefile; see
# .github/workflows/ci.yml.
PLATFORM    ?= $(shell docker version -f '{{.Server.Os}}/{{.Server.Arch}}' 2>/dev/null)

CACERT_URL  ?= https://curl.se/ca/cacert.pem
CACERT_FILE := internal/vault/cacert.pem

DOCKER_GO := docker run --rm \
	-v "$(CURDIR)":/src -w /src \
	-u "$$(id -u):$$(id -g)" \
	-e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath \
	$(GO_IMAGE)

.PHONY: all test vet fmt fmt-check image fetch-cacert

all: fmt-check vet test

# internal/vault/certs.go go:embeds this; it's fetched, not vendored, so
# test/vet always compile against the current bundle -- same as the
# Dockerfile's own ADD step, which this mirrors for non-Docker-build paths.
fetch-cacert:
	$(DOCKER_GO) curl -fsSL -o $(CACERT_FILE) $(CACERT_URL)

test: fetch-cacert
	$(DOCKER_GO) go test ./...

vet: fetch-cacert
	$(DOCKER_GO) go vet ./...

fmt:
	$(DOCKER_GO) gofmt -l -w .

fmt-check:
	$(DOCKER_GO) sh -c 'unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "not gofmt-formatted:"; echo "$$unformatted"; exit 1; fi'

image:
	docker build --platform $(PLATFORM) -t $(IMAGE_NAME):$(IMAGE_TAG) .

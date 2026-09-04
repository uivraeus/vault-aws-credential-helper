# All Go tooling runs inside a pinned `golang` container -- no local Go
# installation is assumed. `make image` needs nothing but docker itself,
# since the Dockerfile's own builder stage does the compilation.

GO_IMAGE    ?= golang:1.23-bookworm
IMAGE_NAME  ?= vault-aws-credential-helper
IMAGE_TAG   ?= latest
BIN_DIR     := bin

# Deployment target (KEP-4639 image volumes, x86_64 clusters). `docker build`
# cross-compiles fine for this regardless of host arch; only `docker run`ning
# the result needs a matching host arch or qemu -- see `make image-native`.
GOOS        ?= linux
GOARCH      ?= amd64
PLATFORM    ?= $(GOOS)/$(GOARCH)

CACERT_URL  ?= https://curl.se/ca/cacert.pem
CACERT_FILE := internal/vault/cacert.pem

DOCKER_GO := docker run --rm \
	-v "$(CURDIR)":/src -w /src \
	-u "$$(id -u):$$(id -g)" \
	-e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath \
	$(GO_IMAGE)

.PHONY: all build test vet fmt fmt-check image image-native clean fetch-cacert

all: fmt-check vet test build

# internal/vault/certs.go go:embeds this; it's fetched, not vendored, so
# build/test always compile against the current bundle -- same as the
# Dockerfile's own ADD step, which this mirrors for non-Docker-build paths.
fetch-cacert:
	$(DOCKER_GO) curl -fsSL -o $(CACERT_FILE) $(CACERT_URL)

build: fetch-cacert
	mkdir -p $(BIN_DIR)
	$(DOCKER_GO) env CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(IMAGE_NAME) ./cmd/$(IMAGE_NAME)

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

# Builds for the machine running docker, so the image can actually be
# `docker run` here without qemu -- for local smoke-testing only, not for
# release (see test.sh).
image-native:
	$(MAKE) image PLATFORM=$$(docker version -f '{{.Server.Os}}/{{.Server.Arch}}')

clean:
	rm -rf $(BIN_DIR)

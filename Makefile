# All Go tooling runs inside a pinned `golang` container -- no local Go
# installation is assumed. `make image` needs nothing but docker itself,
# since the Dockerfile's own builder stage does the compilation.

GO_IMAGE    ?= golang:1.23-bookworm
IMAGE_NAME  ?= vault-aws-credential-helper
IMAGE_TAG   ?= latest
BIN_DIR     := bin

DOCKER_GO := docker run --rm \
	-v "$(CURDIR)":/src -w /src \
	-u "$$(id -u):$$(id -g)" \
	-e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath \
	$(GO_IMAGE)

.PHONY: all build test vet fmt fmt-check image clean

all: fmt-check vet test build

build:
	mkdir -p $(BIN_DIR)
	$(DOCKER_GO) env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(IMAGE_NAME) ./cmd/$(IMAGE_NAME)

test:
	$(DOCKER_GO) go test ./...

vet:
	$(DOCKER_GO) go vet ./...

fmt:
	$(DOCKER_GO) gofmt -l -w .

fmt-check:
	$(DOCKER_GO) sh -c 'unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "not gofmt-formatted:"; echo "$$unformatted"; exit 1; fi'

image:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

clean:
	rm -rf $(BIN_DIR)

# Build stage. Runs natively on the build host's own platform (Go
# cross-compiles trivially) and produces a binary for TARGETOS/TARGETARCH --
# both set automatically by buildx from `docker build --platform`. No qemu
# emulation is needed to build a foreign-arch image this way, only to `docker
# run` one afterwards.
FROM --platform=$BUILDPLATFORM golang:1.23-bookworm AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

# Compiled directly into the binary (see internal/vault/certs.go), fetched
# fresh from a well-defined source (curl's extract of Mozilla's CA root
# store: https://curl.se/docs/caextract.html) rather than pinned in the repo.
# This can't instead be a file placed in the final image: when this binary
# runs via Kubernetes' Image Volume feature, it executes inside *another*
# container's root filesystem, not this image's, so a CA bundle living only
# at some path here would never be reachable at runtime.
ADD https://curl.se/ca/cacert.pem internal/vault/cacert.pem

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/vault-aws-credential-helper ./cmd/vault-aws-credential-helper

# Final distro-less / scratch image -- just the binary, nothing else needed.
FROM scratch

COPY --from=builder /out/vault-aws-credential-helper /vault-aws-credential-helper

ENTRYPOINT ["/vault-aws-credential-helper"]

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

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/vault-aws-credential-helper ./cmd/vault-aws-credential-helper

# Final distro-less / scratch image
FROM scratch

# A scratch image has no CA bundle of its own; without this, TLS verification
# against any publicly-trusted CA (the tool's secure default) would fail for
# everyone not using VAULT_CACERT or VAULT_TLS_SKIP_VERIFY. Vendored from repo
# rather than apt-installed at build time so the trust root is deterministic
# across builds -- see certs/SOURCE.md.
COPY certs/cacert.pem /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/vault-aws-credential-helper /vault-aws-credential-helper

ENTRYPOINT ["/vault-aws-credential-helper"]

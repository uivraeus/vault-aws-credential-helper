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
# everyone not using VAULT_CACERT or VAULT_TLS_SKIP_VERIFY. Fetched fresh on
# every build from curl's own extract of Mozilla's CA root store -- a
# well-defined source purpose-built for exactly this case (see
# https://curl.se/docs/caextract.html), rather than whatever `ca-certificates`
# apt happens to resolve. BuildKit re-fetches only when the remote changes.
ADD https://curl.se/ca/cacert.pem /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/vault-aws-credential-helper /vault-aws-credential-helper

ENTRYPOINT ["/vault-aws-credential-helper"]

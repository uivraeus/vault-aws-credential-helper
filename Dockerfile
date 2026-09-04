# Build stage
FROM golang:1.23-bookworm AS builder

# A scratch final image has no CA bundle of its own; without this, TLS
# verification against any publicly-trusted CA (the tool's secure default)
# would fail for everyone not using VAULT_CACERT or VAULT_TLS_SKIP_VERIFY.
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/vault-aws-credential-helper ./cmd/vault-aws-credential-helper

# Final distro-less / scratch image
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/vault-aws-credential-helper /vault-aws-credential-helper

ENTRYPOINT ["/vault-aws-credential-helper"]

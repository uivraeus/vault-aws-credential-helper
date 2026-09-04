# Vault AWS Credential Helper

An AWS [`credential_process`](https://docs.aws.amazon.com/sdkref/latest/guide/feature-process-credentials.html)
provider backed by HashiCorp Vault (or OpenBao), for pods authenticating via
their Kubernetes ServiceAccount JWT.

It's invoked fresh on every `credential_process` call: read the pod's own
ServiceAccount JWT, log in to Vault's Kubernetes auth method, read an AWS
credential lease from Vault's AWS secrets engine, and print the result as the
JSON AWS expects. No sidecar, no proactively-refreshing file on disk — this
only runs (and only talks to Vault) when the AWS SDK/CLI actually needs a
credential.

Distributed as a minimal, statically-linked OCI image (`FROM scratch`, real
`ENTRYPOINT`) intended to be mounted into pods via Kubernetes' native
[Image Volume](https://kubernetes.io/docs/tasks/configure-pod-container/image-volumes/)
feature (KEP-4639), so the app container it's mounted into needs nothing
installed to use it.

## Usage

```
vault-aws-credential-helper credential_process [flags]
```

Configuration is env-var-driven (the natural interface for a subprocess
invoked by the AWS SDK/CLI from a `credential_process` line in an AWS config
file), with equivalent flags available as an explicit override for manual
runs.

| Env var | Flag | Default | Required |
|---|---|---|---|
| `VAULT_ADDR` | `-vault-addr` | — | yes |
| `VAULT_ROLE` | `-vault-role` | — | yes |
| `VAULT_AWS_SECRETS_PATH` | `-aws-secrets-path` | — | yes |
| `VAULT_K8S_AUTH_MOUNT` | `-k8s-auth-mount` | `kubernetes` | no |
| `VAULT_SA_TOKEN_PATH` | `-sa-token-path` | `/var/run/secrets/kubernetes.io/serviceaccount/token` | no |
| `VAULT_TLS_SKIP_VERIFY` | `-tls-skip-verify` | `false` | no |
| `VAULT_CACERT` | `-cacert` | — | no |
| `VAULT_TIMEOUT` | `-timeout` | `10s` | no |

- `VAULT_AWS_SECRETS_PATH` is the **full** Vault path to read (e.g.
  `aws/creds/my-role`) — the AWS secrets engine mount point isn't assumed.
- TLS certificate verification is **on by default**; `VAULT_TLS_SKIP_VERIFY`
  is an explicit opt-out, not the default. `VAULT_CACERT` (path to a PEM CA
  bundle) trusts a private/internal CA without disabling verification —
  prefer it over skip-verify whenever Vault's certificate is signed by a CA
  you control. A `VAULT_ADDR` using `http://` bypasses TLS entirely (as in a
  lab with TLS disabled); the tool prints a one-line stderr warning in that
  case.
- On any failure, stdout is left empty and a diagnostic is written to stderr
  (never including the ServiceAccount JWT, Vault client token, or AWS
  credentials). Exit code `2` means a configuration/usage error; `1` means
  any other failure (reading the SA token, Vault login, reading the secret).

### Example AWS config

```ini
[profile vault]
credential_process = /mnt/vault-aws-credential-helper/vault-aws-credential-helper credential_process
```

with `VAULT_ADDR`, `VAULT_ROLE`, and `VAULT_AWS_SECRETS_PATH` set on the
container via the pod spec.

## Development

All Go tooling runs inside a pinned `golang` container — no local Go
installation is required, only Docker.

```
make test        # go vet + go test, containerized
make build       # static linux/amd64 binary in bin/
make image       # docker build the scratch-based OCI image
./test.sh        # unit tests + image build + container smoke tests
```

## Release

Tagging a commit `vX.Y.Z` (or `X.Y.Z`) publishes
`ghcr.io/uivraeus/vault-aws-credential-helper:X.Y.Z` (plus `X.Y` and
`vX.Y.Z` aliases); pushes to `main` publish `:latest`. See
[`.github/workflows/ci.yml`](.github/workflows/ci.yml).

# Vendored CA bundle

`cacert.pem` is a pinned copy of curl's [extract of Mozilla's CA root
store](https://curl.se/docs/caextract.html), used to give the `FROM scratch`
final image (which has no OS-provided trust store at all) a deterministic set
of roots for verifying Vault's TLS certificate. It's copied into the image at
`/etc/ssl/certs/ca-certificates.crt` (the same path Go's `crypto/x509` looks
for on Debian/Ubuntu-family systems), independent of whatever `ca-certificates`
package version happens to resolve via apt at build time.

This is unrelated to any future `serve` mode work: it verifies the tool's
*outbound* connection to Vault, needed identically regardless of which
subcommand is running.

- Source: `https://curl.se/ca/cacert.pem`
- Pinned snapshot date (Mozilla data as of, per the file's own header): 2026-08-13
- SHA-256: `f66dff1bdf8f96060b8177976f8b7d9254bc89bc4db933d769f7384d28480bc9`

## Updating

```sh
curl -sS https://curl.se/ca/cacert.pem -o certs/cacert.pem
curl -sS https://curl.se/ca/cacert.pem.sha256
sha256sum certs/cacert.pem   # confirm it matches, then commit
```

Update the pinned date and checksum above in the same commit.

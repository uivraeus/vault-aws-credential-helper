package vault

import _ "embed"

// embeddedCACertPEM is curl's extract of Mozilla's CA root store
// (https://curl.se/docs/caextract.html), fetched fresh at build time (see
// Dockerfile) and compiled directly into the binary.
//
// It is the default trust root -- not any OS/filesystem-provided store --
// because this binary may end up running inside a container whose root
// filesystem isn't this image's own (notably, when mounted via Kubernetes'
// Image Volume feature into another container). No particular CA bundle at
// any given filesystem path can be assumed to exist there, so trust roots
// need to travel with the binary itself.
//
//go:embed cacert.pem
var embeddedCACertPEM []byte

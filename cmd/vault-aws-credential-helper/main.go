// Command vault-aws-credential-helper is an AWS credential_process provider
// backed by HashiCorp Vault, for pods authenticating via their Kubernetes
// ServiceAccount JWT.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/uivraeus/vault-aws-credential-helper/internal/app"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: vault-aws-credential-helper credential_process [flags]")
		return 2
	}

	switch args[0] {
	case "credential_process":
		return app.RunCredentialProcess(args[1:], os.Stdout, os.Stderr, os.Getenv, time.Now)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (expected: credential_process)\n", args[0])
		return 2
	}
}

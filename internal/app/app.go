// Package app wires config, vault, and credential together into the
// credential_process subcommand, kept subprocess-argument/exit-code shaped
// (rather than calling os.Exit itself) so it's testable without spawning a
// real process.
package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/uivraeus/vault-aws-credential-helper/internal/config"
	"github.com/uivraeus/vault-aws-credential-helper/internal/credential"
	"github.com/uivraeus/vault-aws-credential-helper/internal/vault"
)

// RunCredentialProcess authenticates to Vault using the pod's ServiceAccount
// JWT, reads AWS credentials from Vault, and writes the AWS credential_process
// JSON contract to stdout. On any failure, stdout stays empty; a diagnostic
// (never including secret material) goes to stderr. It returns the process
// exit code: 0 on success, 2 for a configuration/usage error, 1 for any
// other failure.
func RunCredentialProcess(args []string, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	fs := flag.NewFlagSet("credential_process", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg, err := config.Parse(fs, args, getenv)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	if u, err := url.Parse(cfg.VaultAddr); err == nil && u.Scheme == "http" {
		fmt.Fprintln(stderr, "warning: connecting to Vault over plain HTTP (no TLS)")
	}

	jwt, err := os.ReadFile(cfg.SATokenPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading service account token: %v\n", err)
		return 1
	}

	client, err := vault.NewClient(vault.Options{
		Addr:          cfg.VaultAddr,
		TLSSkipVerify: cfg.TLSSkipVerify,
		CACertPath:    cfg.CACertPath,
		Timeout:       cfg.Timeout,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: setting up Vault client: %v\n", err)
		return 1
	}

	ctx := context.Background()

	token, err := client.Login(ctx, cfg.K8sAuthMount, cfg.VaultRole, strings.TrimSpace(string(jwt)))
	if err != nil {
		fmt.Fprintf(stderr, "error: Vault login: %v\n", err)
		return 1
	}

	secret, err := client.ReadSecret(ctx, token, cfg.AWSSecretsPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading AWS credentials from Vault: %v\n", err)
		return 1
	}

	out := credential.FromSecret(secret, now())
	if err := json.NewEncoder(stdout).Encode(out); err != nil {
		fmt.Fprintf(stderr, "error: writing credential output: %v\n", err)
		return 1
	}

	return 0
}

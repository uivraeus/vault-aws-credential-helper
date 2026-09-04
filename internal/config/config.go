// Package config parses the credential_process subcommand's configuration
// from environment variables (the primary interface, since this tool is
// normally invoked from a Kubernetes pod spec or AWS config file) with CLI
// flags available as an explicit override for manual runs.
package config

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultK8sAuthMount = "kubernetes"
	DefaultSATokenPath  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	DefaultTimeout      = 10 * time.Second
)

// Config holds everything needed to authenticate to Vault via Kubernetes auth
// and read an AWS credential lease.
type Config struct {
	VaultAddr      string
	K8sAuthMount   string
	VaultRole      string
	AWSSecretsPath string
	SATokenPath    string
	TLSSkipVerify  bool
	CACertPath     string
	Timeout        time.Duration
}

// Parse registers flags on fs (env vars supply their defaults) and parses
// args. It returns an error if required configuration is missing or an env
// var holds a value that can't be parsed.
func Parse(fs *flag.FlagSet, args []string, getenv func(string) string) (*Config, error) {
	cfg := &Config{}

	fs.StringVar(&cfg.VaultAddr, "vault-addr", getenv("VAULT_ADDR"),
		"Vault base address, e.g. https://vault.example.com:8200 (env VAULT_ADDR)")
	fs.StringVar(&cfg.K8sAuthMount, "k8s-auth-mount", envOrDefault(getenv, "VAULT_K8S_AUTH_MOUNT", DefaultK8sAuthMount),
		"Kubernetes auth mount path (env VAULT_K8S_AUTH_MOUNT)")
	fs.StringVar(&cfg.VaultRole, "vault-role", getenv("VAULT_ROLE"),
		"Vault role to authenticate as (env VAULT_ROLE)")
	fs.StringVar(&cfg.AWSSecretsPath, "aws-secrets-path", getenv("VAULT_AWS_SECRETS_PATH"),
		"Full Vault path to read AWS credentials from, e.g. aws/creds/my-role (env VAULT_AWS_SECRETS_PATH)")
	fs.StringVar(&cfg.SATokenPath, "sa-token-path", envOrDefault(getenv, "VAULT_SA_TOKEN_PATH", DefaultSATokenPath),
		"Path to the pod's ServiceAccount JWT (env VAULT_SA_TOKEN_PATH)")
	fs.StringVar(&cfg.CACertPath, "cacert", getenv("VAULT_CACERT"),
		"Path to a PEM CA bundle to trust for the Vault TLS connection (env VAULT_CACERT)")

	skipVerifyDefault, err := envOrDefaultBool(getenv, "VAULT_TLS_SKIP_VERIFY", false)
	if err != nil {
		return nil, err
	}
	fs.BoolVar(&cfg.TLSSkipVerify, "tls-skip-verify", skipVerifyDefault,
		"Skip Vault TLS certificate verification (env VAULT_TLS_SKIP_VERIFY, default false)")

	timeoutDefault, err := envOrDefaultDuration(getenv, "VAULT_TIMEOUT", DefaultTimeout)
	if err != nil {
		return nil, err
	}
	fs.DurationVar(&cfg.Timeout, "timeout", timeoutDefault,
		"Timeout for each HTTP call to Vault (env VAULT_TIMEOUT, default 10s)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	var missing []string
	if cfg.VaultAddr == "" {
		missing = append(missing, "VAULT_ADDR/-vault-addr")
	}
	if cfg.VaultRole == "" {
		missing = append(missing, "VAULT_ROLE/-vault-role")
	}
	if cfg.AWSSecretsPath == "" {
		missing = append(missing, "VAULT_AWS_SECRETS_PATH/-aws-secrets-path")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func envOrDefault(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

func envOrDefaultBool(getenv func(string) string, key string, def bool) (bool, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: %w", key, v, err)
	}
	return parsed, nil
}

func envOrDefaultDuration(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", key, v, err)
	}
	return parsed, nil
}

package config

import (
	"flag"
	"strings"
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestParse_Defaults(t *testing.T) {
	getenv := envMap(map[string]string{
		"VAULT_ADDR":             "https://vault.example.com:8200",
		"VAULT_ROLE":             "my-role",
		"VAULT_AWS_SECRETS_PATH": "aws/creds/my-role",
	})
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg, err := Parse(fs, nil, getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.VaultAddr != "https://vault.example.com:8200" {
		t.Errorf("VaultAddr = %q", cfg.VaultAddr)
	}
	if cfg.K8sAuthMount != DefaultK8sAuthMount {
		t.Errorf("K8sAuthMount = %q, want default %q", cfg.K8sAuthMount, DefaultK8sAuthMount)
	}
	if cfg.SATokenPath != DefaultSATokenPath {
		t.Errorf("SATokenPath = %q, want default %q", cfg.SATokenPath, DefaultSATokenPath)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want default %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.TLSSkipVerify {
		t.Errorf("TLSSkipVerify = true, want false by default")
	}
	if cfg.CACertPath != "" {
		t.Errorf("CACertPath = %q, want empty by default", cfg.CACertPath)
	}
}

func TestParse_FlagsOverrideEnv(t *testing.T) {
	getenv := envMap(map[string]string{
		"VAULT_ADDR":             "https://env-vault.example.com:8200",
		"VAULT_ROLE":             "env-role",
		"VAULT_AWS_SECRETS_PATH": "aws/creds/env-role",
		"VAULT_K8S_AUTH_MOUNT":   "env-k8s",
	})
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	args := []string{"-vault-addr=https://flag-vault.example.com:8200", "-k8s-auth-mount=flag-k8s"}
	cfg, err := Parse(fs, args, getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.VaultAddr != "https://flag-vault.example.com:8200" {
		t.Errorf("VaultAddr = %q, want flag value", cfg.VaultAddr)
	}
	if cfg.K8sAuthMount != "flag-k8s" {
		t.Errorf("K8sAuthMount = %q, want flag value", cfg.K8sAuthMount)
	}
	if cfg.VaultRole != "env-role" {
		t.Errorf("VaultRole = %q, want env value (no flag override given)", cfg.VaultRole)
	}
}

func TestParse_MissingRequired(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_, err := Parse(fs, nil, envMap(nil))
	if err == nil {
		t.Fatal("expected error for missing required configuration")
	}
	for _, want := range []string{"VAULT_ADDR", "VAULT_ROLE", "VAULT_AWS_SECRETS_PATH"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
}

func TestParse_InvalidSkipVerify(t *testing.T) {
	getenv := envMap(map[string]string{
		"VAULT_ADDR":             "https://vault.example.com:8200",
		"VAULT_ROLE":             "my-role",
		"VAULT_AWS_SECRETS_PATH": "aws/creds/my-role",
		"VAULT_TLS_SKIP_VERIFY":  "not-a-bool",
	})
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_, err := Parse(fs, nil, getenv)
	if err == nil {
		t.Fatal("expected error for invalid VAULT_TLS_SKIP_VERIFY")
	}
}

func TestParse_SkipVerifyFromEnv(t *testing.T) {
	getenv := envMap(map[string]string{
		"VAULT_ADDR":             "https://vault.example.com:8200",
		"VAULT_ROLE":             "my-role",
		"VAULT_AWS_SECRETS_PATH": "aws/creds/my-role",
		"VAULT_TLS_SKIP_VERIFY":  "true",
	})
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg, err := Parse(fs, nil, getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.TLSSkipVerify {
		t.Errorf("TLSSkipVerify = false, want true from env")
	}
}

func TestParse_InvalidTimeout(t *testing.T) {
	getenv := envMap(map[string]string{
		"VAULT_ADDR":             "https://vault.example.com:8200",
		"VAULT_ROLE":             "my-role",
		"VAULT_AWS_SECRETS_PATH": "aws/creds/my-role",
		"VAULT_TIMEOUT":          "not-a-duration",
	})
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_, err := Parse(fs, nil, getenv)
	if err == nil {
		t.Fatal("expected error for invalid VAULT_TIMEOUT")
	}
}

package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testEnv(vaultAddr, tokenPath string, extra map[string]string) func(string) string {
	base := map[string]string{
		"VAULT_ADDR":             vaultAddr,
		"VAULT_ROLE":             "my-role",
		"VAULT_AWS_SECRETS_PATH": "aws/creds/my-role",
		"VAULT_SA_TOKEN_PATH":    tokenPath,
	}
	for k, v := range extra {
		base[k] = v
	}
	return func(key string) string { return base[key] }
}

func writeSAToken(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("sa-jwt\n"), 0o600); err != nil {
		t.Fatalf("writing SA token: %v", err)
	}
	return path
}

func TestRunCredentialProcess_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/kubernetes/login":
			_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]string{"client_token": "vault-token"}})
		case "/v1/aws/creds/my-role":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lease_duration": 900,
				"data": map[string]string{
					"access_key":     "AKIA123",
					"secret_key":     "shh",
					"security_token": "sesh-token",
				},
			})
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	getenv := testEnv(srv.URL, writeSAToken(t), nil)

	var stdout, stderr bytes.Buffer
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	code := RunCredentialProcess(nil, &stdout, &stderr, getenv, func() time.Time { return fixedNow })

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}

	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (stdout=%q)", err, stdout.String())
	}
	if out["AccessKeyId"] != "AKIA123" || out["SecretAccessKey"] != "shh" || out["SessionToken"] != "sesh-token" {
		t.Errorf("unexpected credential fields: %+v", out)
	}
	if out["Expiration"] != "2026-01-01T00:15:00Z" {
		t.Errorf("Expiration = %v", out["Expiration"])
	}
	// httptest.NewServer is plain HTTP, so the only expected stderr output is
	// the http-scheme warning (covered more precisely by the HTTPWarning test).
	if want := "warning: connecting to Vault over plain HTTP (no TLS)\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunCredentialProcess_HTTPWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/kubernetes/login":
			_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]string{"client_token": "vault-token"}})
		case "/v1/aws/creds/my-role":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lease_duration": 900,
				"data": map[string]string{
					"access_key": "AKIA123", "secret_key": "shh", "security_token": "sesh-token",
				},
			})
		}
	}))
	defer srv.Close()

	getenv := testEnv(srv.URL, writeSAToken(t), nil)
	var stdout, stderr bytes.Buffer
	code := RunCredentialProcess(nil, &stdout, &stderr, getenv, time.Now)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("plain HTTP")) {
		t.Errorf("expected an HTTP warning on stderr, got %q", stderr.String())
	}
}

func TestRunCredentialProcess_MissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCredentialProcess(nil, &stdout, &stderr, func(string) string { return "" }, time.Now)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on config error, got %q", stdout.String())
	}
}

func TestRunCredentialProcess_MissingSAToken(t *testing.T) {
	getenv := testEnv("https://vault.example.com", "/nonexistent/token", nil)
	var stdout, stderr bytes.Buffer
	code := RunCredentialProcess(nil, &stdout, &stderr, getenv, time.Now)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on failure, got %q", stdout.String())
	}
}

func TestRunCredentialProcess_VaultLoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"permission denied"}})
	}))
	defer srv.Close()

	getenv := testEnv(srv.URL, writeSAToken(t), nil)
	var stdout, stderr bytes.Buffer
	code := RunCredentialProcess(nil, &stdout, &stderr, getenv, time.Now)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on failure, got %q", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("permission denied")) {
		t.Errorf("stderr = %q, expected it to mention Vault's error", stderr.String())
	}
	// The SA JWT must never appear in diagnostic output.
	if bytes.Contains(stderr.Bytes(), []byte("sa-jwt")) {
		t.Errorf("stderr leaked the ServiceAccount JWT: %q", stderr.String())
	}
}

func TestRunCredentialProcess_SecretReadFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/kubernetes/login":
			_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]string{"client_token": "vault-token"}})
		case "/v1/aws/creds/my-role":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"no such secret"}})
		}
	}))
	defer srv.Close()

	getenv := testEnv(srv.URL, writeSAToken(t), nil)
	var stdout, stderr bytes.Buffer
	code := RunCredentialProcess(nil, &stdout, &stderr, getenv, time.Now)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on failure, got %q", stdout.String())
	}
	// The Vault client token from the (successful) login step must never leak.
	if bytes.Contains(stderr.Bytes(), []byte("vault-token")) {
		t.Errorf("stderr leaked the Vault client token: %q", stderr.String())
	}
}

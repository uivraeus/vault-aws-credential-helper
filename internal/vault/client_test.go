package vault

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogin_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/auth/kubernetes/login" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["jwt"] != "sa-jwt" || body["role"] != "my-role" {
			t.Fatalf("unexpected login body: %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]string{"client_token": "vault-token-abc"},
		})
	}))
	defer srv.Close()

	c, err := NewClient(Options{Addr: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	token, err := c.Login(context.Background(), "kubernetes", "my-role", "sa-jwt")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token != "vault-token-abc" {
		t.Errorf("token = %q", token)
	}
}

func TestLogin_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"permission denied"}})
	}))
	defer srv.Close()

	c, err := NewClient(Options{Addr: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.Login(context.Background(), "kubernetes", "my-role", "sa-jwt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error %q does not mention Vault's error message", err.Error())
	}
}

func TestReadSecret_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/aws/creds/my-role" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Vault-Token") != "vault-token-abc" {
			t.Fatalf("missing/incorrect X-Vault-Token header: %q", r.Header.Get("X-Vault-Token"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"lease_duration": 3600,
			"data": map[string]string{
				"access_key":     "AKIA...",
				"secret_key":     "secret",
				"security_token": "token",
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(Options{Addr: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	secret, err := c.ReadSecret(context.Background(), "vault-token-abc", "aws/creds/my-role")
	if err != nil {
		t.Fatalf("ReadSecret: %v", err)
	}
	if secret.AccessKey != "AKIA..." || secret.SecretKey != "secret" || secret.SecurityToken != "token" || secret.LeaseDuration != 3600 {
		t.Errorf("unexpected secret: %+v", secret)
	}
}

func TestReadSecret_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{}})
	}))
	defer srv.Close()

	c, err := NewClient(Options{Addr: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.ReadSecret(context.Background(), "tok", "aws/creds/missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewClient_TLSSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]string{"client_token": "tok"}})
	}))
	defer srv.Close()

	// Without skip-verify, the server's self-signed test cert must be rejected.
	strict, err := NewClient(Options{Addr: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := strict.Login(context.Background(), "kubernetes", "role", "jwt"); err == nil {
		t.Fatal("expected TLS verification error without skip-verify")
	}

	lenient, err := NewClient(Options{Addr: srv.URL, Timeout: 5 * time.Second, TLSSkipVerify: true})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := lenient.Login(context.Background(), "kubernetes", "role", "jwt"); err != nil {
		t.Fatalf("unexpected error with skip-verify: %v", err)
	}
}

func TestNewClient_CACert(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]string{"client_token": "tok"}})
	}))
	defer srv.Close()

	certPath := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(certPath, pemBytes, 0o600); err != nil {
		t.Fatalf("writing CA cert: %v", err)
	}

	c, err := NewClient(Options{Addr: srv.URL, Timeout: 5 * time.Second, CACertPath: certPath})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.Login(context.Background(), "kubernetes", "role", "jwt"); err != nil {
		t.Fatalf("unexpected error trusting CA cert: %v", err)
	}
}

func TestNewClient_InvalidCACertPath(t *testing.T) {
	if _, err := NewClient(Options{Addr: "https://vault.example.com", CACertPath: "/nonexistent/ca.pem"}); err == nil {
		t.Fatal("expected error for unreadable CA cert path")
	}
}

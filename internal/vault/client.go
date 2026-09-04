// Package vault implements just enough of the Vault HTTP API for this tool:
// Kubernetes auth login and reading a dynamic secret (e.g. from the AWS
// secrets engine).
package vault

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Secret holds AWS credential material read from Vault.
type Secret struct {
	AccessKey     string
	SecretKey     string
	SecurityToken string
	LeaseDuration int
}

// Options configures a Client's connection to Vault.
type Options struct {
	Addr          string
	TLSSkipVerify bool
	CACertPath    string
	Timeout       time.Duration
}

// Client talks to a single Vault server.
type Client struct {
	addr       string
	httpClient *http.Client
}

// NewClient builds a Client. TLS certificate verification is enabled unless
// TLSSkipVerify is set; CACertPath, if set, trusts an additional CA bundle
// instead of (or alongside) the system roots.
func NewClient(opts Options) (*Client, error) {
	tlsConfig := &tls.Config{}
	if opts.TLSSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	if opts.CACertPath != "" {
		pemBytes, err := os.ReadFile(opts.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("no valid certificates found in %s", opts.CACertPath)
		}
		tlsConfig.RootCAs = pool
	}

	return &Client{
		addr: strings.TrimRight(opts.Addr, "/"),
		httpClient: &http.Client{
			Timeout:   opts.Timeout,
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
	}, nil
}

type loginRequest struct {
	JWT  string `json:"jwt"`
	Role string `json:"role"`
}

type loginResponse struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
}

// Login authenticates to Vault's Kubernetes auth method at the given mount
// and returns a Vault client token.
func (c *Client) Login(ctx context.Context, mount, role, jwt string) (string, error) {
	body, err := json.Marshal(loginRequest{JWT: jwt, Role: role})
	if err != nil {
		return "", fmt.Errorf("encoding login request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/auth/%s/login", c.addr, mount)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Vault login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Vault login failed: %s", describeError(resp))
	}

	var out loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding Vault login response: %w", err)
	}
	if out.Auth.ClientToken == "" {
		return "", fmt.Errorf("Vault login response did not include a client token")
	}
	return out.Auth.ClientToken, nil
}

type secretResponse struct {
	Data struct {
		AccessKey     string `json:"access_key"`
		SecretKey     string `json:"secret_key"`
		SecurityToken string `json:"security_token"`
	} `json:"data"`
	LeaseDuration int `json:"lease_duration"`
}

// ReadSecret reads a dynamic secret from the given full Vault path (e.g.
// "aws/creds/my-role"), authenticated with token.
func (c *Client) ReadSecret(ctx context.Context, token, path string) (*Secret, error) {
	url := fmt.Sprintf("%s/v1/%s", c.addr, strings.TrimLeft(path, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building secret request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Vault secrets endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading AWS secret from Vault failed: %s", describeError(resp))
	}

	var out secretResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding Vault secret response: %w", err)
	}
	if out.Data.AccessKey == "" || out.Data.SecretKey == "" {
		return nil, fmt.Errorf("Vault secret response missing access_key/secret_key")
	}

	return &Secret{
		AccessKey:     out.Data.AccessKey,
		SecretKey:     out.Data.SecretKey,
		SecurityToken: out.Data.SecurityToken,
		LeaseDuration: out.LeaseDuration,
	}, nil
}

// describeError summarizes a failed response using Vault's structured
// "errors" field. It deliberately never echoes the raw response body: on a
// successful (200) response that body could in principle carry a client
// token or secret material, so no code path here should print it verbatim.
func describeError(resp *http.Response) string {
	var parsed struct {
		Errors []string `json:"errors"`
	}
	limited := io.LimitReader(resp.Body, 64*1024)
	if err := json.NewDecoder(limited).Decode(&parsed); err == nil && len(parsed.Errors) > 0 {
		return fmt.Sprintf("%s (%s)", resp.Status, strings.Join(parsed.Errors, "; "))
	}
	return resp.Status
}

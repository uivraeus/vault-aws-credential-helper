package credential

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/uivraeus/vault-aws-credential-helper/internal/vault"
)

func TestFromSecret(t *testing.T) {
	s := &vault.Secret{
		AccessKey:     "AKIA123",
		SecretKey:     "secret",
		SecurityToken: "token",
		LeaseDuration: 3600,
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	out := FromSecret(s, now)

	if out.Version != 1 {
		t.Errorf("Version = %d, want 1", out.Version)
	}
	if out.AccessKeyId != "AKIA123" || out.SecretAccessKey != "secret" || out.SessionToken != "token" {
		t.Errorf("unexpected credential fields: %+v", out)
	}
	if want := "2026-01-01T01:00:00Z"; out.Expiration != want {
		t.Errorf("Expiration = %q, want %q", out.Expiration, want)
	}
}

func TestFromSecret_JSONFieldNames(t *testing.T) {
	s := &vault.Secret{AccessKey: "AKIA123", SecretKey: "secret", SecurityToken: "token", LeaseDuration: 60}
	out := FromSecret(s, time.Now())

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, field := range []string{"Version", "AccessKeyId", "SecretAccessKey", "SessionToken", "Expiration"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("JSON output missing required field %q: %s", field, b)
		}
	}
}

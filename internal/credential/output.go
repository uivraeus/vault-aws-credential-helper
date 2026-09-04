// Package credential formats AWS credentials as the JSON contract expected
// by the AWS SDK/CLI's credential_process integration.
package credential

import (
	"time"

	"github.com/uivraeus/vault-aws-credential-helper/internal/vault"
)

// ProcessOutput is the credential_process stdout contract:
// https://docs.aws.amazon.com/sdkref/latest/guide/feature-process-credentials.html
type ProcessOutput struct {
	Version         int    `json:"Version"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"`
}

// FromSecret builds the credential_process output for a Vault AWS secret.
// Expiration is set to now plus the secret's lease duration, in RFC3339.
func FromSecret(s *vault.Secret, now time.Time) ProcessOutput {
	return ProcessOutput{
		Version:         1,
		AccessKeyId:     s.AccessKey,
		SecretAccessKey: s.SecretKey,
		SessionToken:    s.SecurityToken,
		Expiration:      now.Add(time.Duration(s.LeaseDuration) * time.Second).UTC().Format(time.RFC3339),
	}
}

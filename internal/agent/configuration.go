package agent

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// Configuration contains only control-plane identity and liveness settings.
// It deliberately contains no target addresses or executable work.
type Configuration struct {
	Version                  int          `json:"version"`
	AgentID                  int64        `json:"agentId"`
	ProjectID                int64        `json:"projectId"`
	EnvironmentID            int64        `json:"environmentId"`
	HeartbeatIntervalSeconds int          `json:"heartbeatIntervalSeconds"`
	Assignments              []Assignment `json:"assignments"`
	IssuedAt                 time.Time    `json:"issuedAt"`
	ExpiresAt                time.Time    `json:"expiresAt"`
}

// Assignment is the agent-facing projection of an editor-approved private
// check. It excludes request headers, credentials, bodies, and all arbitrary
// command fields.
type Assignment struct {
	ID           int64  `json:"id"`
	Method       string `json:"method"`
	Target       string `json:"target"`
	IntervalSecs int    `json:"intervalSeconds"`
	TimeoutMS    int    `json:"timeoutMs"`
}

type SignedConfiguration struct {
	Configuration Configuration `json:"configuration"`
	KeyID         string        `json:"keyId"`
	Signature     string        `json:"signature"`
}

type ConfigurationSigner struct{ private ed25519.PrivateKey }

func NewConfigurationSigner(privateKey []byte) (*ConfigurationSigner, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("agent configuration signing key must be an Ed25519 private key")
	}
	return &ConfigurationSigner{private: ed25519.PrivateKey(append([]byte(nil), privateKey...))}, nil
}

func (s *ConfigurationSigner) Sign(config Configuration) (SignedConfiguration, error) {
	body, err := json.Marshal(config)
	if err != nil {
		return SignedConfiguration{}, err
	}
	sig := ed25519.Sign(s.private, body)
	pub := s.private.Public().(ed25519.PublicKey)
	hash := sha256.Sum256(pub)
	return SignedConfiguration{Configuration: config, KeyID: base64.RawURLEncoding.EncodeToString(hash[:8]), Signature: base64.RawURLEncoding.EncodeToString(sig)}, nil
}

func VerifyConfiguration(publicKey []byte, signed SignedConfiguration, now time.Time) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return errors.New("agent configuration verification key must be an Ed25519 public key")
	}
	if signed.Configuration.ExpiresAt.Before(now) {
		return errors.New("agent configuration has expired")
	}
	body, err := json.Marshal(signed.Configuration)
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(signed.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), body, sig) {
		return errors.New("agent configuration signature is invalid")
	}
	return nil
}

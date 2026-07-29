package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestConfigurationSignatureAndExpiry(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewConfigurationSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	signed, err := signer.Sign(Configuration{Version: 1, AgentID: 4, ProjectID: 5, EnvironmentID: 6, HeartbeatIntervalSeconds: 60, IssuedAt: now, ExpiresAt: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyConfiguration(pub, signed, now); err != nil {
		t.Fatal(err)
	}
	signed.Configuration.ProjectID = 7
	if err = VerifyConfiguration(pub, signed, now); err == nil {
		t.Fatal("tampered configuration verified")
	}
}

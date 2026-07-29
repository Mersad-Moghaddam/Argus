package secrets

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestAEADRoundTripAndWrongKeyRejection(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	sealed, err := Seal(key, `{"Authorization":"Bearer secret"}`)
	if err != nil || sealed == "" || sealed == `{"Authorization":"Bearer secret"}` {
		t.Fatalf("seal: %q %v", sealed, err)
	}
	opened, err := Open(key, sealed)
	if err != nil || opened != `{"Authorization":"Bearer secret"}` {
		t.Fatalf("open: %q %v", opened, err)
	}
	if _, err = Open(bytes.Repeat([]byte{8}, 32), sealed); err == nil {
		t.Fatal("wrong key must be rejected")
	}
	if _, err = ParseKey(base64.RawStdEncoding.EncodeToString(key)); err != nil {
		t.Fatal(err)
	}
}

func TestRewrapRejectsWrongCurrentKey(t *testing.T) {
	oldKey, newKey := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	sealed, err := Seal(oldKey, "secret")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := Rewrap(oldKey, newKey, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Open(newKey, rotated); err != nil || got != "secret" {
		t.Fatalf("rewrap: %q %v", got, err)
	}
	if _, err := Rewrap(newKey, oldKey, sealed); err == nil {
		t.Fatal("wrong current key accepted")
	}
}

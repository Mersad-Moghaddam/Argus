// Package secrets provides the small explicit key-management boundary used
// for encrypting control-plane secrets before MySQL persistence.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

const prefix = "v1:"

func ParseKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	key, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, errors.New("route secret encryption key must be base64-encoded 32 bytes")
	}
	return key, nil
}
func Seal(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if len(key) != 32 {
		return "", errors.New("route secret encryption is not configured")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return prefix + base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}
func Open(key []byte, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if len(key) != 32 || len(ciphertext) < len(prefix) || ciphertext[:len(prefix)] != prefix {
		return "", errors.New("invalid encrypted route secret")
	}
	raw, err := base64.RawStdEncoding.DecodeString(ciphertext[len(prefix):])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted route secret")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	return string(plain), err
}

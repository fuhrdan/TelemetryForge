package incidentarchive

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

const encryptionMagic = "TFINENC1"

var encryptionAAD = []byte("TelemetryForge Incident Archive AES-256-GCM v1")

// LoadKeyFile reads one 256-bit archive key encoded as exactly 64 hex digits.
func LoadKeyFile(filename string) ([]byte, error) {
	payload, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read archive key file: %w", err)
	}
	raw := strings.TrimSpace(string(payload))
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, errors.New("archive key file must contain exactly 64 hexadecimal characters (32 bytes)")
	}
	return key, nil
}

// Encrypt wraps one ZIP payload in an authenticated AES-256-GCM envelope.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("archive encryption requires a 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate archive nonce: %w", err)
	}

	result := make([]byte, 0, len(encryptionMagic)+len(nonce)+len(plaintext)+gcm.Overhead())
	result = append(result, []byte(encryptionMagic)...)
	result = append(result, nonce...)
	result = gcm.Seal(result, nonce, plaintext, encryptionAAD)
	return result, nil
}

// Decrypt opens an authenticated archive envelope.
func Decrypt(payload, key []byte) ([]byte, error) {
	if len(payload) < len(encryptionMagic) ||
		string(payload[:len(encryptionMagic)]) != encryptionMagic {
		return nil, errors.New("payload is not an encrypted TelemetryForge incident archive")
	}
	if len(key) != 32 {
		return nil, errors.New("archive decryption requires a 32-byte key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	offset := len(encryptionMagic)
	if len(payload) < offset+gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("encrypted archive is truncated")
	}
	nonce := payload[offset : offset+gcm.NonceSize()]
	ciphertext := payload[offset+gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, encryptionAAD)
	if err != nil {
		return nil, errors.New("archive authentication failed: wrong key or modified ciphertext")
	}
	return plaintext, nil
}

// IsEncrypted reports whether payload starts with the v1 encryption envelope.
func IsEncrypted(payload []byte) bool {
	return len(payload) >= len(encryptionMagic) &&
		string(payload[:len(encryptionMagic)]) == encryptionMagic
}

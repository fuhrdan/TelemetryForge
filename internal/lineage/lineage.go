// Package lineage provides cryptographic primitives for TelemetryForge WAL lineage.
package lineage

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SealFormat  = "telemetryforge-lineage-seal"
	SealVersion = 1
	Algorithm   = "Ed25519"
)

// Signer owns the edge identity used to sign immutable segment seals.
type Signer struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	keyID   string
}

// Seal attests to the ordered record set in one closed WAL segment.
type Seal struct {
	Format              string    `json:"format"`
	FormatVersion       int       `json:"format_version"`
	EdgeID              string    `json:"edge_id"`
	Segment             string    `json:"segment"`
	FirstSequence       uint64    `json:"first_sequence"`
	LastSequence        uint64    `json:"last_sequence"`
	RecordCount         uint64    `json:"record_count"`
	FirstRecordHash     string    `json:"first_record_hash"`
	LastRecordHash      string    `json:"last_record_hash"`
	MerkleRoot          string    `json:"merkle_root"`
	PreviousSegmentRoot string    `json:"previous_segment_root,omitempty"`
	SealedAt            time.Time `json:"sealed_at"`
	Algorithm           string    `json:"algorithm"`
	KeyID               string    `json:"key_id"`
	PublicKey           string    `json:"public_key"`
	Signature           string    `json:"signature"`
}

type sealPayload struct {
	Format              string    `json:"format"`
	FormatVersion       int       `json:"format_version"`
	EdgeID              string    `json:"edge_id"`
	Segment             string    `json:"segment"`
	FirstSequence       uint64    `json:"first_sequence"`
	LastSequence        uint64    `json:"last_sequence"`
	RecordCount         uint64    `json:"record_count"`
	FirstRecordHash     string    `json:"first_record_hash"`
	LastRecordHash      string    `json:"last_record_hash"`
	MerkleRoot          string    `json:"merkle_root"`
	PreviousSegmentRoot string    `json:"previous_segment_root,omitempty"`
	SealedAt            time.Time `json:"sealed_at"`
	Algorithm           string    `json:"algorithm"`
	KeyID               string    `json:"key_id"`
	PublicKey           string    `json:"public_key"`
}

// LoadOrCreateSigner loads an Ed25519 private key or atomically creates one.
// Auto-generation is intended for local/dev use; production should mount a
// persistent key from a secret manager and protect the private-key path.
func LoadOrCreateSigner(privatePath, publicPath string) (*Signer, error) {
	privatePath = strings.TrimSpace(privatePath)
	if privatePath == "" {
		return nil, errors.New("lineage private key path is required")
	}
	payload, err := os.ReadFile(privatePath)
	if errors.Is(err, os.ErrNotExist) {
		if _, err := GenerateKeyPair(privatePath, publicPath, false); err != nil {
			return nil, err
		}
		payload, err = os.ReadFile(privatePath)
	}
	if err != nil {
		return nil, fmt.Errorf("read lineage private key: %w", err)
	}
	private, err := parsePrivateKey(payload)
	if err != nil {
		return nil, err
	}
	public := private.Public().(ed25519.PublicKey)
	if strings.TrimSpace(publicPath) != "" {
		if err := writePublicIfMissing(publicPath, public); err != nil {
			return nil, err
		}
	}
	return &Signer{private: private, public: public, keyID: KeyID(public)}, nil
}

// GenerateKeyPair creates an Ed25519 key pair in PEM form.
func GenerateKeyPair(privatePath, publicPath string, overwrite bool) (string, error) {
	if strings.TrimSpace(privatePath) == "" || strings.TrimSpace(publicPath) == "" {
		return "", errors.New("private and public key paths are required")
	}
	if !overwrite {
		if _, err := os.Stat(privatePath); err == nil {
			return "", fmt.Errorf("private key already exists: %s", privatePath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if _, err := os.Stat(publicPath); err == nil {
			return "", fmt.Errorf("public key already exists: %s", publicPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return "", err
	}
	if err := writePEMAtomic(privatePath, 0o600, "PRIVATE KEY", privateDER, overwrite); err != nil {
		return "", err
	}
	if err := writePEMAtomic(publicPath, 0o644, "PUBLIC KEY", publicDER, overwrite); err != nil {
		return "", err
	}
	return KeyID(public), nil
}

func (signer *Signer) KeyID() string { return signer.keyID }

func (signer *Signer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), signer.public...)
}

// SignSeal signs the canonical seal metadata and embeds the public key.
func (signer *Signer) SignSeal(seal Seal) (Seal, error) {
	seal.Format = SealFormat
	seal.FormatVersion = SealVersion
	seal.Algorithm = Algorithm
	seal.KeyID = signer.keyID
	seal.PublicKey = base64.StdEncoding.EncodeToString(signer.public)
	seal.Signature = ""
	payload, err := canonicalSealPayload(seal)
	if err != nil {
		return Seal{}, err
	}
	seal.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(signer.private, payload))
	return seal, nil
}

// VerifySeal verifies structure, key identity, and Ed25519 signature. If trusted
// is non-nil, the embedded key must exactly match that trust anchor.
func VerifySeal(seal Seal, trusted ed25519.PublicKey) error {
	if seal.Format != SealFormat || seal.FormatVersion != SealVersion {
		return errors.New("unsupported lineage seal format/version")
	}
	if seal.Algorithm != Algorithm {
		return fmt.Errorf("unsupported lineage signature algorithm %q", seal.Algorithm)
	}
	if strings.TrimSpace(seal.EdgeID) == "" || strings.TrimSpace(seal.Segment) == "" {
		return errors.New("lineage seal edge_id and segment are required")
	}
	if seal.FirstSequence == 0 || seal.LastSequence < seal.FirstSequence || seal.RecordCount == 0 {
		return errors.New("invalid lineage seal sequence range")
	}
	for name, value := range map[string]string{"first_record_hash": seal.FirstRecordHash, "last_record_hash": seal.LastRecordHash, "merkle_root": seal.MerkleRoot} {
		if !validHash(value) {
			return fmt.Errorf("invalid %s", name)
		}
	}
	if seal.PreviousSegmentRoot != "" && !validHash(seal.PreviousSegmentRoot) {
		return errors.New("invalid previous_segment_root")
	}
	if seal.SealedAt.IsZero() {
		return errors.New("sealed_at is required")
	}
	publicBytes, err := base64.StdEncoding.DecodeString(seal.PublicKey)
	if err != nil || len(publicBytes) != ed25519.PublicKeySize {
		return errors.New("invalid embedded Ed25519 public key")
	}
	public := ed25519.PublicKey(publicBytes)
	if KeyID(public) != seal.KeyID {
		return errors.New("lineage key ID does not match embedded public key")
	}
	if trusted != nil && !public.Equal(trusted) {
		return errors.New("lineage seal signer does not match trusted public key")
	}
	signature, err := base64.StdEncoding.DecodeString(seal.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid Ed25519 signature encoding")
	}
	payload, err := canonicalSealPayload(seal)
	if err != nil {
		return err
	}
	if !ed25519.Verify(public, payload, signature) {
		return errors.New("lineage seal signature verification failed")
	}
	return nil
}

func canonicalSealPayload(seal Seal) ([]byte, error) {
	return json.Marshal(sealPayload{
		Format: seal.Format, FormatVersion: seal.FormatVersion, EdgeID: seal.EdgeID,
		Segment: seal.Segment, FirstSequence: seal.FirstSequence, LastSequence: seal.LastSequence,
		RecordCount: seal.RecordCount, FirstRecordHash: seal.FirstRecordHash,
		LastRecordHash: seal.LastRecordHash, MerkleRoot: seal.MerkleRoot,
		PreviousSegmentRoot: seal.PreviousSegmentRoot, SealedAt: seal.SealedAt.UTC(),
		Algorithm: seal.Algorithm, KeyID: seal.KeyID, PublicKey: seal.PublicKey,
	})
}

// MerkleRoot computes a deterministic binary SHA-256 Merkle root from hex leaves.
// An odd leaf is duplicated at each level, matching common audit-tree behavior.
func MerkleRoot(leaves []string) (string, error) {
	if len(leaves) == 0 {
		return "", errors.New("cannot build Merkle root for zero leaves")
	}
	level := make([][]byte, 0, len(leaves))
	for _, leaf := range leaves {
		decoded, err := hex.DecodeString(leaf)
		if err != nil || len(decoded) != sha256.Size {
			return "", errors.New("Merkle leaf is not a SHA-256 hash")
		}
		level = append(level, decoded)
	}
	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for index := 0; index < len(level); index += 2 {
			left := level[index]
			right := left
			if index+1 < len(level) {
				right = level[index+1]
			}
			material := make([]byte, 0, len(left)+len(right))
			material = append(material, left...)
			material = append(material, right...)
			sum := sha256.Sum256(material)
			next = append(next, sum[:])
		}
		level = next
	}
	return hex.EncodeToString(level[0]), nil
}

func KeyID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return "ed25519:" + hex.EncodeToString(sum[:8])
}

func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(payload)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	public, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not Ed25519")
	}
	return public, nil
}

func ReadSeal(path string) (Seal, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Seal{}, err
	}
	var seal Seal
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&seal); err != nil {
		return Seal{}, err
	}
	return seal, nil
}

func WriteSeal(path string, seal Seal) error {
	payload, err := json.MarshalIndent(seal, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return writeAtomic(path, payload, 0o640, false)
}

func parsePrivateKey(payload []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(payload)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("invalid private key PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not Ed25519")
	}
	return private, nil
}

func writePublicIfMissing(path string, public ed25519.PublicKey) error {
	if existing, err := LoadPublicKey(path); err == nil {
		if !existing.Equal(public) {
			return errors.New("existing lineage public key does not match private key")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(path); statErr == nil {
			return err
		}
	}
	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return err
	}
	return writePEMAtomic(path, 0o644, "PUBLIC KEY", der, false)
}

func writePEMAtomic(path string, mode os.FileMode, blockType string, der []byte, overwrite bool) error {
	payload := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	return writeAtomic(path, payload, mode, overwrite)
}

func writeAtomic(path string, payload []byte, mode os.FileMode, overwrite bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file already exists: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if overwrite {
		_ = os.Remove(path)
	}
	return os.Rename(temporary, path)
}

func validHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

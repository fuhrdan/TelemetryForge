package lineage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSignVerifySealAndTrustAnchor(t *testing.T) {
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "edge.key")
	publicPath := filepath.Join(dir, "edge.pub")
	signer, err := LoadOrCreateSigner(privatePath, publicPath)
	if err != nil {
		t.Fatal(err)
	}
	root := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	seal, err := signer.SignSeal(Seal{EdgeID: "edge-a", Segment: "0001.tfwal", FirstSequence: 1, LastSequence: 2, RecordCount: 2, FirstRecordHash: root, LastRecordHash: root, MerkleRoot: root, SealedAt: time.Unix(1700000000, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := LoadPublicKey(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySeal(seal, trusted); err != nil {
		t.Fatal(err)
	}
	seal.MerkleRoot = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := VerifySeal(seal, trusted); err == nil {
		t.Fatal("expected tampered seal to fail verification")
	}
}

func TestMerkleRootDeterministic(t *testing.T) {
	leaves := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000002",
		"0000000000000000000000000000000000000000000000000000000000000003",
	}
	first, err := MerkleRoot(leaves)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MerkleRoot(leaves)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first == leaves[0] {
		t.Fatalf("unexpected root: %s %s", first, second)
	}
}

func TestGenerateKeyPairRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "edge.key")
	publicPath := filepath.Join(dir, "edge.pub")
	if _, err := GenerateKeyPair(privatePath, publicPath, false); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateKeyPair(privatePath, publicPath, false); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	if info, err := os.Stat(privatePath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected private key permissions: %v %v", info, err)
	}
}

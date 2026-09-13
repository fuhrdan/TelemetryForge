package wasmplugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

type fakeBackend struct {
	output []byte
	err    error
}

func (b fakeBackend) Invoke(context.Context, []byte, string, []byte) ([]byte, error) {
	return b.output, b.err
}
func moduleBytes() []byte {
	return []byte{0, 97, 115, 109, 1, 0, 0, 0, 1, 4, 1, 96, 0, 0, 3, 2, 1, 0, 7, 11, 1, 7, 112, 114, 101, 100, 105, 99, 116, 0, 0, 10, 4, 1, 2, 0, 11}
}
func manifestFor(module []byte) Manifest {
	m := DefaultManifest()
	m.Name = "predictor"
	m.Version = "1.0.0"
	sum := sha256.Sum256(module)
	m.ModuleSHA256 = hex.EncodeToString(sum[:])
	return m
}
func TestModuleDigestAndInvocation(t *testing.T) {
	mod := moduleBytes()
	host, err := NewHost(manifestFor(mod), mod, fakeBackend{output: []byte(`{"predicted_pressure":0.91}`)})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Predicted float64 `json:"predicted_pressure"`
	}
	if err := host.Invoke(context.Background(), map[string]any{"pressure": .8}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Predicted != .91 {
		t.Fatalf("unexpected result %v", out.Predicted)
	}
}
func TestTamperedModuleRejected(t *testing.T) {
	mod := moduleBytes()
	m := manifestFor(mod)
	mod = append(mod, 1)
	if _, err := NewHost(m, mod, fakeBackend{}); err == nil {
		t.Fatal("expected digest failure")
	}
}
func TestCapabilitiesRejected(t *testing.T) {
	m := manifestFor(moduleBytes())
	m.Capabilities = []string{"network"}
	if m.Validate() == nil {
		t.Fatal("capabilities must fail")
	}
}
func TestBackendFailurePropagates(t *testing.T) {
	mod := moduleBytes()
	host, _ := NewHost(manifestFor(mod), mod, fakeBackend{err: errors.New("sandbox trap")})
	var out any
	if host.Invoke(context.Background(), map[string]int{"x": 1}, &out) == nil {
		t.Fatal("expected trap")
	}
}
func TestTimeoutBound(t *testing.T) {
	m := manifestFor(moduleBytes())
	m.Timeout = 2 * time.Second
	if m.Validate() == nil {
		t.Fatal("expected timeout validation")
	}
}

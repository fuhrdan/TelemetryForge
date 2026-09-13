// Package wasmplugin defines a bounded WebAssembly plugin boundary for
// TelemetryForge autonomous predictors. v2.7 validates immutable modules and
// delegates execution to an explicitly configured sandbox backend; plugins
// never receive direct storage, network, lifecycle, or mutation capabilities.
package wasmplugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const FormatVersion = 1

type Manifest struct {
	FormatVersion  int           `json:"format_version"`
	Name           string        `json:"name"`
	Version        string        `json:"version"`
	ModuleSHA256   string        `json:"module_sha256"`
	Entrypoint     string        `json:"entrypoint"`
	Timeout        time.Duration `json:"timeout"`
	MaxInputBytes  int           `json:"max_input_bytes"`
	MaxOutputBytes int           `json:"max_output_bytes"`
	Capabilities   []string      `json:"capabilities,omitempty"`
}

func DefaultManifest() Manifest {
	return Manifest{FormatVersion: FormatVersion, Entrypoint: "predict", Timeout: 50 * time.Millisecond, MaxInputBytes: 64 << 10, MaxOutputBytes: 64 << 10}
}
func (m Manifest) Validate() error {
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported plugin manifest version %d", m.FormatVersion)
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return errors.New("plugin name and version are required")
	}
	if len(m.ModuleSHA256) != 64 {
		return errors.New("plugin module_sha256 must be 64 hex characters")
	}
	if _, err := hex.DecodeString(m.ModuleSHA256); err != nil {
		return errors.New("plugin module_sha256 is invalid")
	}
	if strings.TrimSpace(m.Entrypoint) == "" {
		return errors.New("plugin entrypoint is required")
	}
	if m.Timeout <= 0 || m.Timeout > time.Second {
		return errors.New("plugin timeout must be between 0 and 1 second")
	}
	if m.MaxInputBytes < 1 || m.MaxInputBytes > 1<<20 || m.MaxOutputBytes < 1 || m.MaxOutputBytes > 1<<20 {
		return errors.New("plugin input/output limits must be between 1 byte and 1 MiB")
	}
	// v2.7 predictor plugins are intentionally capability-free.
	if len(m.Capabilities) > 0 {
		return errors.New("v2.7 predictor plugins must declare no capabilities")
	}
	return nil
}

func LoadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, err
	}
	return m, m.Validate()
}

func VerifyModule(module []byte, expected string) error {
	if len(module) < 8 || string(module[:4]) != "\x00asm" || module[4] != 1 || module[5] != 0 || module[6] != 0 || module[7] != 0 {
		return errors.New("module is not WebAssembly binary format version 1")
	}
	sum := sha256.Sum256(module)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(expected)) {
		return errors.New("WebAssembly module SHA-256 does not match manifest")
	}
	return nil
}

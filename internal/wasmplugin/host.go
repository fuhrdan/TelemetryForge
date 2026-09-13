package wasmplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Backend is the deliberately narrow execution boundary. A production backend
// must enforce its own memory/instruction sandbox; the host supplies timeout and
// byte limits and gives the plugin no TelemetryForge object handles.
type Backend interface {
	Invoke(context.Context, []byte, string, []byte) ([]byte, error)
}

type Host struct {
	manifest Manifest
	module   []byte
	backend  Backend
}

func NewHost(manifest Manifest, module []byte, backend Backend) (*Host, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	if err := VerifyModule(module, manifest.ModuleSHA256); err != nil {
		return nil, err
	}
	if err := VerifyEntrypoint(module, manifest.Entrypoint); err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, errors.New("Wasm execution backend is required")
	}
	return &Host{manifest: manifest, module: append([]byte(nil), module...), backend: backend}, nil
}

func (h *Host) Invoke(ctx context.Context, input any, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if len(raw) > h.manifest.MaxInputBytes {
		return fmt.Errorf("plugin input exceeds %d bytes", h.manifest.MaxInputBytes)
	}
	callCtx, cancel := context.WithTimeout(ctx, h.manifest.Timeout)
	defer cancel()
	result, err := h.backend.Invoke(callCtx, h.module, h.manifest.Entrypoint, raw)
	if err != nil {
		return err
	}
	if len(result) > h.manifest.MaxOutputBytes {
		return fmt.Errorf("plugin output exceeds %d bytes", h.manifest.MaxOutputBytes)
	}
	if !json.Valid(result) {
		return errors.New("plugin output is not valid JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(result))
	dec.DisallowUnknownFields()
	if err := dec.Decode(output); err != nil {
		return fmt.Errorf("decode plugin output: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("plugin output contains trailing JSON")
	}
	return nil
}

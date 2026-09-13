package autonomy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Auditor receives immutable action-state snapshots.
type Auditor interface{ Record(Action) error }

type NopAuditor struct{}

func (NopAuditor) Record(Action) error { return nil }

// JSONLAuditor is a local append-only audit sink. Each record is fsynced before
// Record returns so an automatic action is never intentionally unaudited.
type JSONLAuditor struct {
	mu   sync.Mutex
	file *os.File
}

func OpenJSONLAuditor(path string) (*JSONLAuditor, error) {
	if path == "" {
		return nil, fmt.Errorf("autonomy audit path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}
	return &JSONLAuditor{file: file}, nil
}
func (a *JSONLAuditor) Close() error {
	if a == nil || a.file == nil {
		return nil
	}
	return a.file.Close()
}
func (a *JSONLAuditor) Record(action Action) error {
	if a == nil || a.file == nil {
		return fmt.Errorf("autonomy audit sink is unavailable")
	}
	payload, err := json.Marshal(action)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err = a.file.Write(append(payload, '\n')); err != nil {
		return err
	}
	return a.file.Sync()
}

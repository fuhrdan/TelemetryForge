package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Load reads and validates a versioned JSON policy file.
func Load(filename string) (Policy, error) {
	payload, err := os.ReadFile(filename)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy %q: %w", filename, err)
	}

	var result Policy
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Policy{}, fmt.Errorf("decode policy %q: %w", filename, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Policy{}, fmt.Errorf("decode policy %q: trailing JSON value", filename)
		}
		return Policy{}, fmt.Errorf("decode policy %q trailing data: %w", filename, err)
	}
	if err := result.Validate(); err != nil {
		return Policy{}, fmt.Errorf("validate policy %q: %w", filename, err)
	}
	return result, nil
}

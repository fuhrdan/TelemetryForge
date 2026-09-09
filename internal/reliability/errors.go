// Package reliability defines processing failure policy.
//
// Keeping retry classification explicit prevents every worker from inventing
// its own interpretation of "try again" versus "send this to the DLQ".
package reliability

import (
	"errors"
	"fmt"
)

// Class identifies whether repeating an operation can reasonably succeed.
type Class string

const (
	// Transient describes failures such as a temporarily unavailable database.
	Transient Class = "transient"
	// Permanent describes failures where repeating the same input will not help.
	Permanent Class = "permanent"
)

// Error wraps a processing error with its retry classification.
type Error struct {
	Class Class
	Op    string
	Err   error
}

// Error returns the operation-aware failure message.
func (err *Error) Error() string {
	if err.Op == "" {
		return err.Err.Error()
	}
	return fmt.Sprintf("%s: %v", err.Op, err.Err)
}

// Unwrap exposes the underlying error for errors.Is/errors.As traversal.
func (err *Error) Unwrap() error { return err.Err }

// New creates a classified processing error.
func New(class Class, op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Class: class, Op: op, Err: err}
}

// IsTransient reports whether an error is explicitly retryable.
//
// Unknown errors are treated as permanent. This conservative default prevents
// programming/validation bugs from causing unbounded retry loops.
func IsTransient(err error) bool {
	var classified *Error
	return errors.As(err, &classified) && classified.Class == Transient
}

// Classification returns a stable string for logs, DLQ records, and tooling.
func Classification(err error) Class {
	if IsTransient(err) {
		return Transient
	}
	return Permanent
}

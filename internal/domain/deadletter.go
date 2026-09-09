package domain

import (
	"time"
)

// DeadLetter preserves failed work and enough source metadata to investigate or
// replay it without guessing where the record originally came from.
type DeadLetter struct {
	Event         *Event    `json:"event,omitempty"`
	RawPayload    []byte    `json:"raw_payload_base64,omitempty"`
	OriginalTopic string    `json:"original_topic"`
	Partition     int32     `json:"partition"`
	Offset        int64     `json:"offset"`
	FailureClass  string    `json:"failure_class"`
	Error         string    `json:"error"`
	Attempts      int       `json:"attempts"`
	FailedAt      time.Time `json:"failed_at"`
}

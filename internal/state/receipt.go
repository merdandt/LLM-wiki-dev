package state

import "time"

type Fingerprint struct {
	BaseCommit string `json:"base_commit"`
	Evidence   string `json:"evidence"`
	Wiki       string `json:"wiki"`
	Schema     int    `json:"schema"`
}

type ReceiptKind string

const (
	ReceiptSynced   ReceiptKind = "synced"
	ReceiptNoUpdate ReceiptKind = "no-update"
)

type Receipt struct {
	Kind        ReceiptKind `json:"kind"`
	Fingerprint Fingerprint `json:"fingerprint"`
	Reason      string      `json:"reason,omitempty"`
	SessionID   string      `json:"session_id,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

func (r Receipt) Matches(want Fingerprint) bool {
	return r.Fingerprint == want
}

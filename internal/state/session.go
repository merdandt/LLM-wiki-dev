package state

import "time"

type Session struct {
	ID               string      `json:"id"`
	WorktreeID       string      `json:"worktree_id"`
	Baseline         Fingerprint `json:"baseline"`
	LastObservedHead string      `json:"last_observed_head"`
	RecoveryPasses   int         `json:"recovery_passes"`
	StartupAudit     bool        `json:"startup_audit"`
	StartedAt        time.Time   `json:"started_at"`
}

type Observation struct {
	WorktreeID string    `json:"worktree_id"`
	Head       string    `json:"head"`
	ObservedAt time.Time `json:"observed_at"`
}

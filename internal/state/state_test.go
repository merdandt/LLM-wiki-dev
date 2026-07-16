package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestReceiptRoundTripAndMatch(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), ".llm-wiki-state"))
	receipt := Receipt{
		Kind: ReceiptSynced,
		Fingerprint: Fingerprint{
			BaseCommit: "abc",
			Evidence:   "sha256:evidence",
			Wiki:       "sha256:wiki",
			Schema:     1,
		},
		SessionID: "session-1",
		CreatedAt: time.Unix(10, 0).UTC(),
	}

	if err := layout.WriteReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	got, err := layout.ReadReceipt(receipt.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Matches(receipt.Fingerprint) || got.Kind != ReceiptSynced {
		t.Fatalf("unexpected receipt: %#v", got)
	}
}

func TestLatestSessionFiltersWorktreeAndLatestReceiptUsesTimestamp(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), ".llm-wiki-state"))
	for _, session := range []Session{
		{ID: "old", WorktreeID: "worktree-a", StartedAt: time.Unix(10, 0).UTC()},
		{ID: "new", WorktreeID: "worktree-a", StartedAt: time.Unix(20, 0).UTC()},
		{ID: "other", WorktreeID: "worktree-b", StartedAt: time.Unix(30, 0).UTC()},
	} {
		if err := layout.WriteSession(session); err != nil {
			t.Fatal(err)
		}
	}
	latestSession, err := layout.LatestSession("worktree-a")
	if err != nil {
		t.Fatal(err)
	}
	if latestSession.ID != "new" {
		t.Fatalf("LatestSession = %#v", latestSession)
	}

	for _, receipt := range []Receipt{
		{Kind: ReceiptNoUpdate, Fingerprint: Fingerprint{BaseCommit: "a"}, CreatedAt: time.Unix(40, 0).UTC()},
		{Kind: ReceiptSynced, Fingerprint: Fingerprint{BaseCommit: "b"}, CreatedAt: time.Unix(50, 0).UTC()},
	} {
		if err := layout.WriteReceipt(receipt); err != nil {
			t.Fatal(err)
		}
	}
	latestReceipt, err := layout.LatestReceipt()
	if err != nil {
		t.Fatal(err)
	}
	if latestReceipt.Kind != ReceiptSynced || latestReceipt.Fingerprint.BaseCommit != "b" {
		t.Fatalf("LatestReceipt = %#v", latestReceipt)
	}
}

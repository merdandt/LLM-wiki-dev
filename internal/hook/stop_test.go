package hook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/lock"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

func startSession(t *testing.T, root string) {
	t.Helper()
	if _, err := SessionStart(context.Background(), Input{
		SessionID: "s1", CWD: root, EventName: "SessionStart",
	}); err != nil {
		t.Fatal(err)
	}
}

func materialChange(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "internal", "orders", "service.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package orders\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMatchingReceipt(t *testing.T, root string) {
	t.Helper()
	repo := discoverRepo(t, root)
	cfg, err := config.Load(filepath.Join(root, "llm-wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	current, err := BuildFingerprint(repo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	layout := state.NewLayout(filepath.Join(root, cfg.StatePath))
	if err := layout.WriteReceipt(state.Receipt{
		Kind: state.ReceiptSynced, Fingerprint: current, SessionID: "s1", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
}

func exhaustRecovery(t *testing.T, root string) {
	t.Helper()
	layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
	session, err := layout.ReadSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	session.RecoveryPasses = 1
	if err := layout.WriteSession(session); err != nil {
		t.Fatal(err)
	}
}

func TestStopOutcomes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		input Input
		want  Outcome
	}{
		{name: "stop hook active is silent", setup: func(t *testing.T, root string) {
			startSession(t, root)
			materialChange(t, root)
		}, input: Input{SessionID: "s1", EventName: "Stop", StopHookActive: true}, want: OutcomeClean},
		{name: "no session is silent", setup: func(t *testing.T, root string) {},
			input: Input{SessionID: "ghost", EventName: "Stop"}, want: OutcomeClean},
		{name: "no changes", setup: startSession,
			input: Input{SessionID: "s1", EventName: "Stop"}, want: OutcomeClean},
		{name: "material drift blocks", setup: func(t *testing.T, root string) {
			startSession(t, root)
			materialChange(t, root)
		}, input: Input{SessionID: "s1", EventName: "Stop"}, want: OutcomeDrift},
		{name: "matching receipt is synchronized", setup: func(t *testing.T, root string) {
			startSession(t, root)
			materialChange(t, root)
			writeMatchingReceipt(t, root)
		}, input: Input{SessionID: "s1", EventName: "Stop"}, want: OutcomeSynchronized},
		{name: "recovery exhausted warns", setup: func(t *testing.T, root string) {
			startSession(t, root)
			materialChange(t, root)
			exhaustRecovery(t, root)
		}, input: Input{SessionID: "s1", EventName: "Stop"}, want: OutcomeFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initializedRepoFixture(t)
			tt.setup(t, root)
			input := tt.input
			input.CWD = root
			got, err := Stop(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != tt.want {
				t.Fatalf("Outcome = %q, want %q (reason %q)", got.Outcome, tt.want, got.Reason)
			}
		})
	}
}

func TestStopDriftKeepsLeaseAndIncrementsRecovery(t *testing.T) {
	root := initializedRepoFixture(t)
	startSession(t, root)
	materialChange(t, root)
	result, err := Stop(context.Background(), Input{SessionID: "s1", CWD: root, EventName: "Stop"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeDrift || result.Reason != DriftReason {
		t.Fatalf("unexpected result: %#v", result)
	}
	layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
	session, err := layout.ReadSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	if session.RecoveryPasses != 1 {
		t.Fatalf("RecoveryPasses = %d, want 1", session.RecoveryPasses)
	}
	repo := discoverRepo(t, root)
	worktreeID, err := repo.WorktreeID()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := lockOwner(root, worktreeID)
	if err != nil || owner != "s1" {
		t.Fatalf("lease owner = %q err=%v, want s1 held", owner, err)
	}
}

func TestStopLeaseConflictWarnsWithoutStealing(t *testing.T) {
	root := initializedRepoFixture(t)
	startSession(t, root)
	materialChange(t, root)

	repo := discoverRepo(t, root)
	worktreeID, err := repo.WorktreeID()
	if err != nil {
		t.Fatal(err)
	}
	layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
	lockPath := layout.LockPath(worktreeID)
	lease, err := lock.Acquire(context.Background(), lockPath, "other-session", time.Second, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// Intentionally not released: this simulates a concurrent session
	// holding the synchronization lease across Stop's call.

	result, err := Stop(context.Background(), Input{SessionID: "s1", CWD: root, EventName: "Stop"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeFailure {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFailure)
	}
	if result.Reason == "" {
		t.Fatal("Reason is empty, want a warning explaining the lease conflict")
	}

	owner, err := lock.CurrentOwner(context.Background(), lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "other-session" {
		t.Fatalf("lease owner = %q, want it unchanged (no steal)", owner)
	}

	if err := lease.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStopCorruptSessionSurfacesError(t *testing.T) {
	root := initializedRepoFixture(t)
	startSession(t, root)
	stateRoot := filepath.Join(root, ".llm-wiki-state")
	sum := sha256.Sum256([]byte("s1"))
	sessionPath := filepath.Join(stateRoot, "sessions", hex.EncodeToString(sum[:])+".json")
	if err := os.WriteFile(sessionPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Stop(context.Background(), Input{SessionID: "s1", CWD: root, EventName: "Stop"}); err == nil {
		t.Fatal("expected an error for a corrupted session file")
	}
}

func TestStopDormantWhileUninitialized(t *testing.T) {
	root := initializedRepoFixture(t)
	data, err := os.ReadFile(filepath.Join(root, "llm-wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "initialized: true", "initialized: false", 1)
	if err := os.WriteFile(filepath.Join(root, "llm-wiki.yaml"), []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	materialChange(t, root)
	result, err := Stop(context.Background(), Input{SessionID: "s1", CWD: root, EventName: "Stop"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeClean {
		t.Fatalf("Outcome = %q, want clean while uninitialized", result.Outcome)
	}
}

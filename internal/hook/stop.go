package hook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/lock"
	"github.com/merdandt/LLM-wiki-dev/internal/materiality"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

// DriftReason is the maintenance instruction returned to the agent on a
// blocked stop. Kept under 1200 characters per the design spec.
const DriftReason = "Durable project changes are not yet reflected in team memory. Before finishing: " +
	"(1) review your diff and update the affected pages under the wiki path from llm-wiki.yaml " +
	"(components/flows/contracts/decisions/quality/operations; update index.md and append one log.md entry); " +
	"(2) if the changes affect anything the project README documents - features, usage, install steps, API - " +
	"update those README sections too; " +
	"(3) run `.llm-wiki/llm-wiki validate` and fix any errors; " +
	"(4) finish with `.llm-wiki/llm-wiki receipt write --kind synced` " +
	"(or `--kind no-update --reason \"<why>\"` if nothing durable changed). " +
	"Do not modify application code in this pass."

func Stop(ctx context.Context, input Input) (Result, error) {
	if input.StopHookActive {
		return Result{Outcome: OutcomeClean}, nil
	}
	repo, err := gitrepo.Discover(input.CWD)
	if err != nil {
		return Result{Outcome: OutcomeClean}, nil
	}
	cfg, err := config.Load(filepath.Join(repo.Root, "llm-wiki.yaml"))
	if errors.Is(err, os.ErrNotExist) || (err == nil && !cfg.Initialized) {
		return Result{Outcome: OutcomeClean}, nil
	}
	if err != nil {
		return Result{}, err
	}
	layout := state.NewLayout(filepath.Join(repo.Root, cfg.StatePath))
	session, err := layout.ReadSession(input.SessionID)
	if errors.Is(err, os.ErrNotExist) {
		return Result{Outcome: OutcomeClean}, nil
	}
	if err != nil {
		return Result{}, err
	}
	paths, err := repo.ChangedPaths(session.Baseline.BaseCommit)
	if err != nil {
		return Result{}, err
	}
	if !requiresReview(paths, cfg.WikiPath, session.StartupAudit) {
		return Result{Outcome: OutcomeClean}, nil
	}

	lease, err := lock.Acquire(ctx, layout.LockPath(session.WorktreeID), input.SessionID,
		time.Duration(cfg.LockWaitSeconds)*time.Second,
		time.Duration(cfg.SyncLeaseSeconds)*time.Second)
	if err != nil {
		return Result{Outcome: OutcomeFailure,
			Reason: "LLM Wiki: another session holds the synchronization lease; skipping maintenance."}, nil
	}
	keepLease := false
	defer func() {
		if !keepLease {
			_ = lease.Release(context.Background())
		}
	}()

	current, err := BuildFingerprint(repo, cfg)
	if err != nil {
		return Result{}, err
	}
	if current == session.Baseline && !session.StartupAudit {
		return Result{Outcome: OutcomeClean}, nil
	}
	if receipt, err := layout.ReadReceipt(current); err == nil && receipt.Matches(current) {
		session.Baseline = current
		session.StartupAudit = false
		session.RecoveryPasses = 0
		session.LastObservedHead = current.BaseCommit
		if err := layout.WriteSession(session); err != nil {
			return Result{}, err
		}
		if err := layout.WriteObservation(state.Observation{
			WorktreeID: session.WorktreeID, Head: current.BaseCommit, ObservedAt: time.Now().UTC(),
		}); err != nil {
			return Result{}, err
		}
		return Result{Outcome: OutcomeSynchronized}, nil
	}
	if session.RecoveryPasses >= cfg.Maintenance.MaxRecoveryPasses {
		return Result{Outcome: OutcomeFailure,
			Reason: "LLM Wiki: team memory was not synchronized after the allowed maintenance pass; run `.llm-wiki/llm-wiki status` to review."}, nil
	}
	session.RecoveryPasses++
	if err := layout.WriteSession(session); err != nil {
		return Result{}, err
	}
	keepLease = true
	return Result{Outcome: OutcomeDrift, Reason: DriftReason}, nil
}

func requiresReview(paths []string, wikiPath string, startupAudit bool) bool {
	if startupAudit {
		return true
	}
	if materiality.ClassifyPaths(paths, wikiPath) != materiality.HintNone {
		return true
	}
	for _, path := range paths {
		if inside(path, wikiPath) || filepath.ToSlash(path) == "llm-wiki.yaml" {
			return true
		}
	}
	return false
}

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/hook"
	"github.com/merdandt/LLM-wiki-dev/internal/lock"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
	"github.com/merdandt/LLM-wiki-dev/internal/wiki"
)

func runReceipt(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "write" {
		fmt.Fprintln(stderr, "usage: llm-wiki receipt write --kind <synced|no-update> [--reason TEXT] [--root PATH]")
		return 2
	}
	flags := flag.NewFlagSet("receipt write", flag.ContinueOnError)
	flags.SetOutput(stderr)
	kindFlag := flags.String("kind", "", "synced or no-update")
	reasonFlag := flags.String("reason", "", "why no update was needed")
	rootFlag := flags.String("root", ".", "Git repository root")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	var kind state.ReceiptKind
	switch *kindFlag {
	case "synced":
		kind = state.ReceiptSynced
	case "no-update":
		kind = state.ReceiptNoUpdate
		if *reasonFlag == "" || len(*reasonFlag) > 500 {
			fmt.Fprintln(stderr, "llm-wiki: --kind no-update requires --reason (1..500 chars)")
			return 2
		}
	default:
		fmt.Fprintln(stderr, "llm-wiki: --kind must be synced or no-update")
		return 2
	}

	repo, err := gitrepo.Discover(*rootFlag)
	if err != nil {
		return commandError(stderr, err)
	}
	cfg, err := config.Load(filepath.Join(repo.Root, "llm-wiki.yaml"))
	if err != nil {
		return commandError(stderr, err)
	}
	worktreeID, err := repo.WorktreeID()
	if err != nil {
		return commandError(stderr, err)
	}
	layout := state.NewLayout(filepath.Join(repo.Root, cfg.StatePath))
	ctx := context.Background()
	owner, err := lock.CurrentOwner(ctx, layout.LockPath(worktreeID))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stderr, "llm-wiki: no active synchronization lease; the Stop hook opens one on drift")
		return 5
	}
	if err != nil {
		return commandError(stderr, err)
	}
	lease, err := lock.Acquire(ctx, layout.LockPath(worktreeID), owner,
		time.Duration(cfg.LockWaitSeconds)*time.Second,
		time.Duration(cfg.SyncLeaseSeconds)*time.Second)
	if err != nil {
		// Reachable only if the lease changes owners between CurrentOwner and Acquire.
		fmt.Fprintln(stderr, "llm-wiki: synchronization lease is held by another session")
		return 6
	}

	report := wiki.Validate(wiki.Options{
		Root: repo.Root, WikiPath: cfg.WikiPath, IndexEntryLimit: cfg.IndexEntryLimit,
	})
	if len(report.Errors) > 0 {
		fmt.Fprintf(stderr, "llm-wiki: refusing receipt while %d validation errors remain; run `llm-wiki validate`\n", len(report.Errors))
		return 4
	}
	current, err := hook.BuildFingerprint(repo, cfg)
	if err != nil {
		return commandError(stderr, err)
	}
	if err := layout.WriteReceipt(state.Receipt{
		Kind: kind, Fingerprint: current, Reason: *reasonFlag,
		SessionID: owner, CreatedAt: time.Now().UTC(),
	}); err != nil {
		return commandError(stderr, err)
	}
	session, err := layout.ReadSession(owner)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return commandError(stderr, err)
	}
	if err == nil {
		session.Baseline = current
		session.StartupAudit = false
		session.RecoveryPasses = 0
		session.LastObservedHead = current.BaseCommit
		if err := layout.WriteSession(session); err != nil {
			return commandError(stderr, err)
		}
	}
	if err := layout.WriteObservation(state.Observation{
		WorktreeID: worktreeID, Head: current.BaseCommit, ObservedAt: time.Now().UTC(),
	}); err != nil {
		return commandError(stderr, err)
	}
	if err := lease.Release(ctx); err != nil {
		return commandError(stderr, err)
	}
	return 0
}

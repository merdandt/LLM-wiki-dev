package hook

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/materiality"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
	"github.com/merdandt/LLM-wiki-dev/internal/wiki"
)

const packetLimit = 1024

func SessionStart(_ context.Context, input Input) (Result, error) {
	repo, err := gitrepo.Discover(input.CWD)
	if err != nil {
		return Result{Outcome: OutcomeClean}, nil
	}
	cfg, err := config.Load(filepath.Join(repo.Root, "llm-wiki.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return Result{Outcome: OutcomeClean}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if !cfg.Initialized {
		packet := fmt.Sprintf(
			"LLM Wiki: %[1]s/ holds a scaffold that is not yet compiled into project memory.\n"+
				"When convenient this session: read %[1]s/schema.md, replace scaffold pages with evidence-backed content, set each page's verification via `.llm-wiki/llm-wiki fingerprint --page <page>`, run `.llm-wiki/llm-wiki validate`, then `.llm-wiki/llm-wiki finalize-init`.\n"+
				"Maintenance hooks stay quiet until the wiki is finalized.\n",
			cfg.WikiPath,
		)
		if len(packet) > packetLimit {
			packet = packet[:packetLimit]
		}
		return Result{Outcome: OutcomeClean, Context: packet}, nil
	}
	head, err := repo.Head()
	if err != nil {
		return Result{}, err
	}
	worktreeID, err := repo.WorktreeID()
	if err != nil {
		return Result{}, err
	}
	report := wiki.Validate(wiki.Options{
		Root: repo.Root, WikiPath: cfg.WikiPath, IndexEntryLimit: cfg.IndexEntryLimit,
	})
	layout := state.NewLayout(filepath.Join(repo.Root, cfg.StatePath))

	startupAudit := len(report.Errors) > 0
	observation, observationErr := layout.ReadObservation(worktreeID)
	if observationErr == nil && observation.Head != head {
		paths, diffErr := repo.ChangedPaths(observation.Head)
		if diffErr != nil || materiality.ClassifyPaths(paths, cfg.WikiPath) != materiality.HintNone {
			startupAudit = true
		}
	} else if observationErr != nil && !errors.Is(observationErr, os.ErrNotExist) {
		return Result{}, observationErr
	}

	baseline, err := BuildFingerprint(repo, cfg)
	if err != nil {
		return Result{}, err
	}
	session, sessionErr := layout.ReadSession(input.SessionID)
	switch {
	case sessionErr == nil:
		if session.WorktreeID != worktreeID {
			return Result{}, errors.New("session ID is already bound to another worktree")
		}
		session.LastObservedHead = head
		session.StartupAudit = session.StartupAudit || startupAudit
	case errors.Is(sessionErr, os.ErrNotExist):
		session = state.Session{
			ID: input.SessionID, WorktreeID: worktreeID, Baseline: baseline,
			LastObservedHead: head, StartupAudit: startupAudit, StartedAt: time.Now().UTC(),
		}
	default:
		return Result{}, sessionErr
	}
	if err := layout.WriteSession(session); err != nil {
		return Result{}, err
	}
	if !session.StartupAudit {
		if err := layout.WriteObservation(state.Observation{
			WorktreeID: worktreeID, Head: head, ObservedAt: time.Now().UTC(),
		}); err != nil {
			return Result{}, err
		}
	}

	audit := "no"
	if session.StartupAudit {
		audit = "yes"
	}
	packet := fmt.Sprintf(
		"LLM Wiki: this repo has team memory at %[1]s/.\n"+
			"Before exploring code, read %[1]s/index.md and follow links to relevant pages (components/, flows/, contracts/, decisions/, quality/, operations/).\n"+
			"Shortcuts: `.llm-wiki/llm-wiki status --json` (health), `.llm-wiki/llm-wiki validate` (structure).\n"+
			"After durable changes the Stop hook may request one quiet maintenance pass.\n"+
			"[health: %d issues | schema %d | startup audit: %s]\n",
		cfg.WikiPath, len(report.Errors), cfg.SchemaVersion, audit,
	)
	if len(packet) > packetLimit {
		packet = packet[:packetLimit]
	}
	return Result{Outcome: OutcomeClean, Context: packet}, nil
}

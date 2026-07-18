package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/fingerprint"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/initrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/lock"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
	"github.com/merdandt/LLM-wiki-dev/internal/wiki"
)

var Version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	return RunWithStdin(os.Stdin, args, stdout, stderr)
}

func RunWithStdin(stdin io.Reader, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "llm-wiki %s\n", Version)
		return 0
	}
	if len(args) > 0 {
		switch args[0] {
		case "validate":
			return runValidate(args[1:], stdout, stderr)
		case "fingerprint":
			return runFingerprint(args[1:], stdout, stderr)
		case "status":
			return runStatus(args[1:], stdout, stderr)
		case "init":
			return runInit(args[1:], stdout, stderr)
		case "finalize-init":
			return runFinalizeInit(args[1:], stdout, stderr)
		case "hook":
			return runHook(stdin, args[1:], stdout, stderr)
		case "receipt":
			return runReceipt(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintln(stderr, "usage: llm-wiki <version|init|finalize-init|status|validate|fingerprint|hook|receipt>")
	return 2
}

func runInit(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootFlag := flags.String("root", ".", "Git repository root")
	templateFlag := flags.String("template", "", "template directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if err := initrepo.Initialize(*rootFlag, *templateFlag); err != nil {
		return commandError(stderr, err)
	}
	repo, err := gitrepo.Discover(*rootFlag)
	if err != nil {
		return commandError(stderr, err)
	}
	warnings, err := initrepo.WriteHookConfigs(repo.Root)
	if err != nil {
		return commandError(stderr, err)
	}
	for _, warning := range warnings {
		fmt.Fprintln(stderr, warning)
	}
	fmt.Fprintln(stdout, "llm-wiki: initialized repository template")
	return 0
}

type Status struct {
	Initialized      bool   `json:"initialized"`
	Schema           int    `json:"schema"`
	WikiPath         string `json:"wiki_path"`
	HealthItems      int    `json:"health_items"`
	ValidationErrors int    `json:"validation_errors"`
	StartupAudit     bool   `json:"startup_audit"`
	ContextBudget    int    `json:"context_budget_bytes"`
	LeaseActive      bool   `json:"sync_lease_active"`
	LeaseOwner       string `json:"sync_lease_owner,omitempty"`
	LastReceipt      string `json:"last_receipt_kind,omitempty"`
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootFlag := flags.String("root", ".", "Git repository root")
	jsonFlag := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
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
	report := wiki.Validate(wiki.Options{Root: repo.Root, WikiPath: cfg.WikiPath, AllowUninitialized: true, IndexEntryLimit: cfg.IndexEntryLimit})
	healthItems := 0
	if data, err := os.ReadFile(filepath.Join(repo.Root, filepath.FromSlash(cfg.WikiPath), "health.md")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "## ") {
				healthItems++
			}
		}
	}
	status := Status{
		Initialized:      cfg.Initialized,
		Schema:           cfg.SchemaVersion,
		WikiPath:         cfg.WikiPath,
		HealthItems:      healthItems,
		ValidationErrors: len(report.Errors),
		ContextBudget:    cfg.ContextBudgetBytes,
	}
	worktreeID, err := repo.WorktreeID()
	if err != nil {
		return commandError(stderr, err)
	}
	layout := state.NewLayout(filepath.Join(repo.Root, cfg.StatePath))
	if session, err := layout.LatestSession(worktreeID); err == nil {
		status.StartupAudit = session.StartupAudit
	}
	if receipt, err := layout.LatestReceipt(); err == nil {
		status.LastReceipt = string(receipt.Kind)
	}
	owner, err := lock.CurrentOwner(context.Background(), layout.LockPath(worktreeID))
	if err == nil {
		status.LeaseActive = true
		status.LeaseOwner = owner
	} else if !errors.Is(err, os.ErrNotExist) {
		return commandError(stderr, err)
	}
	if *jsonFlag {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(status); err != nil {
			return commandError(stderr, err)
		}
		return 0
	}
	lease := "inactive"
	if status.LeaseActive {
		lease = "active"
	}
	startup := "no"
	if status.StartupAudit {
		startup = "yes"
	}
	lastReceipt := "none"
	if status.LastReceipt != "" {
		lastReceipt = status.LastReceipt
	}
	fmt.Fprintf(stdout, "LLM Wiki: ready\nSchema: %d\nHealth items: %d\nValidation errors: %d\nStartup audit: %s\nSync lease: %s\nLast receipt: %s\n", status.Schema, status.HealthItems, status.ValidationErrors, startup, lease, lastReceipt)
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootFlag := flags.String("root", ".", "Git repository root")
	allowUninitialized := flags.Bool("allow-uninitialized", false, "allow scaffold fingerprints")
	if err := flags.Parse(args); err != nil {
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
	report := wiki.Validate(wiki.Options{
		Root:               repo.Root,
		WikiPath:           cfg.WikiPath,
		AllowUninitialized: *allowUninitialized,
		IndexEntryLimit:    cfg.IndexEntryLimit,
	})
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return commandError(stderr, err)
	}
	if len(report.Errors) > 0 {
		return 4
	}
	return 0
}

func runFingerprint(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("fingerprint", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootFlag := flags.String("root", ".", "Git repository root")
	pageFlag := flags.String("page", "", "repository-relative Markdown page")
	jsonFlag := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*pageFlag) == "" {
		return commandError(stderr, errors.New("--page is required"))
	}
	repo, err := gitrepo.Discover(*rootFlag)
	if err != nil {
		return commandError(stderr, err)
	}
	cfg, err := config.Load(filepath.Join(repo.Root, "llm-wiki.yaml"))
	if err != nil {
		return commandError(stderr, err)
	}
	pagePath, page, err := loadFingerprintPage(repo.Root, cfg.WikiPath, *pageFlag)
	if err != nil {
		return commandError(stderr, err)
	}
	evidencePaths := make([]string, 0, len(page.Evidence))
	for _, evidence := range page.Evidence {
		clean := filepath.Clean(filepath.FromSlash(evidence.Path))
		if evidence.Path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return commandError(stderr, fmt.Errorf("unsafe evidence path in %s: %s", pagePath, evidence.Path))
		}
		info, err := os.Lstat(filepath.Join(repo.Root, clean))
		if err != nil {
			return commandError(stderr, fmt.Errorf("evidence path missing: %s", evidence.Path))
		}
		if !info.Mode().IsRegular() {
			return commandError(stderr, fmt.Errorf("evidence path is not a regular file: %s", evidence.Path))
		}
		evidencePaths = append(evidencePaths, evidence.Path)
	}
	fp, err := fingerprint.Files(repo.Root, evidencePaths)
	if err != nil {
		return commandError(stderr, err)
	}
	baseCommit, err := repo.Head()
	if err != nil {
		return commandError(stderr, err)
	}
	result := struct {
		BaseCommit          string `json:"base_commit"`
		EvidenceFingerprint string `json:"evidence_fingerprint"`
	}{BaseCommit: baseCommit, EvidenceFingerprint: fp}
	if *jsonFlag {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return commandError(stderr, err)
		}
		return 0
	}
	fmt.Fprintf(stdout, "base_commit: %s\nevidence_fingerprint: %s\n", result.BaseCommit, result.EvidenceFingerprint)
	return 0
}

func loadFingerprintPage(root, wikiPath, requested string) (string, wiki.Page, error) {
	clean := filepath.Clean(filepath.FromSlash(requested))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.Ext(clean) != ".md" {
		return "", wiki.Page{}, errors.New("page must be a repository-relative Markdown file")
	}
	wikiRoot := filepath.Join(root, filepath.FromSlash(wikiPath))
	full := filepath.Join(root, clean)
	relativeToWiki, err := filepath.Rel(wikiRoot, full)
	if err != nil || relativeToWiki == ".." || strings.HasPrefix(relativeToWiki, ".."+string(filepath.Separator)) {
		return "", wiki.Page{}, errors.New("page must be inside the configured wiki path")
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", wiki.Page{}, err
	}
	if !info.Mode().IsRegular() {
		return "", wiki.Page{}, errors.New("page must be a regular non-symlink file")
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", wiki.Page{}, err
	}
	page, err := wiki.ParsePage(filepath.ToSlash(relativeToWiki), data)
	return filepath.ToSlash(clean), page, err
}

func commandError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "llm-wiki: %v\n", err)
	return 3
}

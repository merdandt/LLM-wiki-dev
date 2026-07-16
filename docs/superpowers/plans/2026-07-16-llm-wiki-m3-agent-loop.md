# LLM Wiki Milestone 3 Agent Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add portable skills and lifecycle hooks that keep the active coding agent inside a quiet, one-pass wiki-maintenance loop.

**Architecture:** Hooks perform deterministic detection and protocol adaptation only. The active agent performs semantic recall and synchronization through four shared skills. Clean and synchronized outcomes emit no output; drift blocks one stop attempt and returns maintenance instructions to the active agent.

**Tech Stack:** Go 1.26.5, JSON hook protocols, POSIX shell, PowerShell, Markdown Agent Skills.

---

## Task 1: Parse hook input and render platform-specific output

**Files:**

- Create: `internal/hook/input.go`
- Create: `internal/hook/result.go`
- Create: `internal/hook/platform.go`
- Test: `internal/hook/hook_test.go`
- Create: `testdata/hooks/codex-session-start.json`
- Create: `testdata/hooks/codex-stop.json`
- Create: `testdata/hooks/claude-session-start.json`
- Create: `testdata/hooks/claude-stop.json`

- [ ] **Step 1: Create representative hook fixtures**

`codex-stop.json`:

```json
{
  "session_id": "codex-session",
  "turn_id": "turn-1",
  "transcript_path": null,
  "cwd": "/tmp/project",
  "hook_event_name": "Stop",
  "model": "gpt-5.4",
  "permission_mode": "default"
}
```

`claude-stop.json`:

```json
{
  "session_id": "claude-session",
  "transcript_path": "/tmp/transcript.jsonl",
  "cwd": "/tmp/project",
  "hook_event_name": "Stop",
  "permission_mode": "default",
  "stop_hook_active": false
}
```

Create corresponding `SessionStart` fixtures with `hook_event_name: "SessionStart"` and `source: "startup"`.

- [ ] **Step 2: Write failing protocol tests**

```go
package hook

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	if got := DetectPlatform(Input{Model: "gpt-5.4"}); got != PlatformCodex {
		t.Fatalf("DetectPlatform() = %q, want codex", got)
	}
	if got := DetectPlatform(Input{}); got != PlatformClaude {
		t.Fatalf("DetectPlatform() = %q, want claude", got)
	}
}

func TestEncodeCleanIsSilent(t *testing.T) {
	var out bytes.Buffer
	if err := Encode(PlatformCodex, "Stop", Result{Outcome: OutcomeClean}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("output = %q, want empty", out.String())
	}
}

func TestEncodeDriftUsesHostProtocol(t *testing.T) {
	tests := []struct {
		platform Platform
		key      string
	}{
		{platform: PlatformCodex, key: "continue"},
		{platform: PlatformClaude, key: "decision"},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		err := Encode(tt.platform, "Stop", Result{
			Outcome: OutcomeDrift,
			Reason:  "Run the wiki-sync workflow.",
		}, &out)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if _, ok := got[tt.key]; !ok {
			t.Fatalf("output %s lacks %q", out.String(), tt.key)
		}
	}
}
```

- [ ] **Step 3: Run tests and verify they fail**

```bash
go test ./internal/hook -run 'TestDetectPlatform|TestEncode' -v
```

Expected: FAIL because hook types do not exist.

- [ ] **Step 4: Implement neutral hook types**

`internal/hook/input.go`:

```go
package hook

import (
	"encoding/json"
	"errors"
)

type Input struct {
	SessionID      string `json:"session_id"`
	TurnID         string `json:"turn_id,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`
	CWD            string `json:"cwd"`
	EventName      string `json:"hook_event_name"`
	Model          string `json:"model,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	Source         string `json:"source,omitempty"`
	StopHookActive bool   `json:"stop_hook_active,omitempty"`
}

func Decode(data []byte) (Input, error) {
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return Input{}, err
	}
	if input.SessionID == "" || input.CWD == "" || input.EventName == "" {
		return Input{}, errors.New("session_id, cwd, and hook_event_name are required")
	}
	switch input.EventName {
	case "SessionStart", "Stop":
		return input, nil
	default:
		return Input{}, errors.New("unsupported hook event")
	}
}
```

`internal/hook/result.go`:

```go
package hook

type Outcome string

const (
	OutcomeClean        Outcome = "clean"
	OutcomeSynchronized Outcome = "synchronized"
	OutcomeDrift        Outcome = "drift"
	OutcomeFailure      Outcome = "failure"
)

type Result struct {
	Outcome           Outcome
	Reason            string
	AdditionalContext string
}
```

- [ ] **Step 5: Implement platform output adapters**

`internal/hook/platform.go`:

```go
package hook

import (
	"encoding/json"
	"io"
)

type Platform string

const (
	PlatformCodex  Platform = "codex"
	PlatformClaude Platform = "claude"
)

func DetectPlatform(input Input) Platform {
	if input.Model != "" || input.TurnID != "" {
		return PlatformCodex
	}
	return PlatformClaude
}

func Encode(platform Platform, event string, result Result, out io.Writer) error {
	if result.Outcome == OutcomeClean || result.Outcome == OutcomeSynchronized {
		return nil
	}
	var payload any
	switch event {
	case "SessionStart":
		payload = map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":    "SessionStart",
				"additionalContext": result.AdditionalContext,
			},
		}
	case "Stop":
		if result.Outcome == OutcomeDrift {
			if platform == PlatformCodex {
				payload = map[string]any{
					"continue":      false,
					"stopReason":    result.Reason,
					"systemMessage": "LLM Wiki maintenance is running before completion.",
				}
			} else {
				payload = map[string]any{
					"decision": "block",
					"reason":   result.Reason,
				}
			}
		} else if platform == PlatformCodex {
			payload = map[string]any{"continue": true, "systemMessage": result.Reason}
		} else {
			payload = map[string]any{"systemMessage": result.Reason}
		}
	}
	if payload == nil {
		return nil
	}
	return json.NewEncoder(out).Encode(payload)
}
```

- [ ] **Step 6: Run tests and commit**

```bash
gofmt -w internal/hook
go test ./internal/hook -v
git add internal/hook testdata/hooks
git commit -m "feat: adapt agent hook protocols"
```

## Task 2: Implement the quiet `SessionStart` hook

**Files:**

- Create: `internal/hook/session_start.go`
- Create: `internal/hook/fingerprint.go`
- Modify: `internal/hook/hook_test.go`
- Create: `internal/hook/test_helpers_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

- [ ] **Step 1: Write failing initialized and uninitialized tests**

```go
func TestSessionStartUninitializedIsClean(t *testing.T) {
	result, err := SessionStart(context.Background(), Input{
		SessionID: "s1",
		CWD:       t.TempDir(),
		EventName: "SessionStart",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeClean || result.AdditionalContext != "" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSessionStartStoresBaselineAndReturnsBoundedContext(t *testing.T) {
	root := initializedRepoFixture(t)
	result, err := SessionStart(context.Background(), Input{
		SessionID: "s1",
		CWD:       root,
		EventName: "SessionStart",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeDrift && result.Outcome != OutcomeSynchronized {
		t.Fatalf("unexpected outcome: %q", result.Outcome)
	}
	if len(result.AdditionalContext) > 1024 {
		t.Fatalf("context is %d bytes, want <=1024", len(result.AdditionalContext))
	}
}
```

`internal/hook/test_helpers_test.go`:

```go
package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initializedRepoFixture(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs("../../testdata/wiki/valid")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "add", ".")
	runGit(t, root,
		"-c", "user.name=Hook Test",
		"-c", "user.email=hook@example.com",
		"commit", "-qm", "baseline",
	)
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
go test ./internal/hook -run TestSessionStart -v
```

Expected: FAIL because `SessionStart` does not exist.

- [ ] **Step 3: Implement repository-state fingerprinting and baseline creation**

`internal/hook/fingerprint.go`:

```go
package hook

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/fingerprint"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

func BuildFingerprint(repo gitrepo.Repo, cfg config.Config) (state.Fingerprint, error) {
	head, err := repo.Head()
	if err != nil {
		return state.Fingerprint{}, err
	}
	patch, err := repo.WorktreePatch(cfg.WikiPath, cfg.StatePath, "llm-wiki.yaml")
	if err != nil {
		return state.Fingerprint{}, err
	}
	untracked, err := repo.UntrackedPaths(cfg.WikiPath, cfg.StatePath, "llm-wiki.yaml")
	if err != nil {
		return state.Fingerprint{}, err
	}
	evidenceRecords := []fingerprint.Record{{
		Path: "@worktree-patch",
		Kind: "git-diff",
		Data: []byte(patch),
	}}
	for _, relative := range untracked {
		record, err := repositoryRecord(repo, relative)
		if err != nil {
			return state.Fingerprint{}, err
		}
		evidenceRecords = append(evidenceRecords, record)
	}
	wikiOutput, err := repo.Output(
		"ls-files", "-co", "--exclude-standard", "--",
		"llm-wiki.yaml", cfg.WikiPath,
	)
	if err != nil {
		return state.Fingerprint{}, err
	}
	var wikiRecords []fingerprint.Record
	for _, relative := range strings.Split(wikiOutput, "\n") {
		if relative == "" ||
			(relative != "llm-wiki.yaml" && !strings.EqualFold(filepath.Ext(relative), ".md")) {
			continue
		}
		record, err := repositoryRecord(repo, relative)
		if err != nil {
			return state.Fingerprint{}, err
		}
		wikiRecords = append(wikiRecords, record)
	}
	return state.Fingerprint{
		BaseCommit: head,
		Evidence:   fingerprint.Records(evidenceRecords),
		Wiki:       fingerprint.Records(wikiRecords),
		Schema:     cfg.SchemaVersion,
	}, nil
}

func repositoryRecord(repo gitrepo.Repo, relative string) (fingerprint.Record, error) {
	slash := filepath.ToSlash(filepath.Clean(relative))
	full := filepath.Join(repo.Root, filepath.FromSlash(slash))
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return fingerprint.Record{Path: slash, Kind: "missing"}, nil
	}
	if err != nil {
		return fingerprint.Record{}, err
	}
	switch {
	case info.Mode().IsRegular():
		data, err := os.ReadFile(full)
		return fingerprint.Record{Path: slash, Kind: "file", Data: data}, err
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(full)
		return fingerprint.Record{
			Path: slash,
			Kind: "symlink",
			Data: []byte(filepath.ToSlash(target)),
		}, err
	case info.IsDir():
		nested, err := gitrepo.Discover(full)
		if err != nil {
			return fingerprint.Record{Path: slash, Kind: "directory"}, nil
		}
		commit, err := nested.Head()
		return fingerprint.Record{Path: slash, Kind: "gitlink", Data: []byte(commit)}, err
	default:
		return fingerprint.Record{Path: slash, Kind: info.Mode().String()}, nil
	}
}

func inside(relative, directory string) bool {
	path := filepath.ToSlash(filepath.Clean(relative))
	root := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(directory)), "/")
	return path == root || strings.HasPrefix(path, root+"/")
}
```

`internal/hook/session_start.go`:

```go
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
	"github.com/merdandt/LLM-wiki-dev/internal/lock"
	"github.com/merdandt/LLM-wiki-dev/internal/materiality"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
	"github.com/merdandt/LLM-wiki-dev/internal/wiki"
)

func SessionStart(_ context.Context, input Input) (Result, error) {
	repo, err := gitrepo.Discover(input.CWD)
	if err != nil {
		return Result{Outcome: OutcomeClean}, nil
	}
	configPath := filepath.Join(repo.Root, "llm-wiki.yaml")
	cfg, err := config.Load(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return Result{Outcome: OutcomeClean}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if !cfg.Initialized {
		return Result{Outcome: OutcomeClean}, nil
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
		Root:               repo.Root,
		WikiPath:           cfg.WikiPath,
		AllowUninitialized: !cfg.Initialized,
		IndexEntryLimit:    cfg.IndexEntryLimit,
	})
	layout := state.NewLayout(filepath.Join(repo.Root, cfg.StatePath))
	_, leaseErr := lock.CurrentOwner(
		context.Background(),
		layout.LockPath(worktreeID),
	)
	leaseActive := leaseErr == nil
	if leaseErr != nil && !errors.Is(leaseErr, os.ErrNotExist) {
		return Result{}, leaseErr
	}
	startupAudit := len(report.Errors)+len(report.Warnings) > 0
	automaticNoUpdate := false
	observation, observationErr := layout.ReadObservation(worktreeID)
	switch {
	case observationErr == nil && observation.Head != head:
		paths, diffErr := repo.ChangedPaths(observation.Head)
		if diffErr != nil {
			startupAudit = true
		} else if materiality.ClassifyPaths(paths, cfg.WikiPath) == materiality.HintNone && !startupAudit {
			automaticNoUpdate = true
		} else {
			startupAudit = true
		}
	case observationErr != nil && !errors.Is(observationErr, os.ErrNotExist):
		return Result{}, observationErr
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
			ID:               input.SessionID,
			WorktreeID:       worktreeID,
			Baseline:         state.Fingerprint{BaseCommit: head, Schema: cfg.SchemaVersion},
			LastObservedHead: head,
			StartupAudit:     startupAudit,
			StartedAt:        time.Now().UTC(),
		}
	default:
		return Result{}, sessionErr
	}
	if err := layout.WriteSession(session); err != nil {
		return Result{}, err
	}
	if automaticNoUpdate && !session.StartupAudit {
		current, err := BuildFingerprint(repo, cfg)
		if err != nil {
			return Result{}, err
		}
		if err := layout.WriteReceipt(state.Receipt{
			Kind:        state.ReceiptNoUpdate,
			Fingerprint: current,
			Reason:      "unseen commit changed only deterministically non-material paths",
			SessionID:   input.SessionID,
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			return Result{}, err
		}
	}
	if !session.StartupAudit {
		if err := layout.WriteObservation(state.Observation{
			WorktreeID: worktreeID,
			Head:       head,
			ObservedAt: time.Now().UTC(),
		}); err != nil {
			return Result{}, err
		}
	}
	context := fmt.Sprintf(
		"LLM Wiki: %s; schema=%d; health=%d; startup_audit=%t; sync_lease_active=%t. Use wiki-recall only when relevant.",
			cfg.WikiPath,
			cfg.SchemaVersion,
			len(report.Errors)+len(report.Warnings),
			session.StartupAudit,
			leaseActive,
	)
	if len(context) > 1024 {
		context = context[:1024]
	}
	return Result{Outcome: OutcomeDrift, AdditionalContext: context}, nil
}
```

Add `TestSessionStartDetectsUnseenCommits` as a two-case table test. Each case writes an earlier `state.Observation`, commits either `internal/orders/service.go` or `README.md`, calls `SessionStart`, reads the stored session with `layout.ReadSession("s1")`, and asserts `StartupAudit` is respectively `true` and `false`. For the README case, build the current fingerprint and assert a matching `ReceiptNoUpdate` exists. Also add a fixture with `initialized: false` and assert that no session file is created.

Add `TestBuildFingerprintPartitionsWikiAndEvidence`: changing a source file changes only `Evidence`, changing a wiki page changes only `Wiki`, changing `llm-wiki.yaml` changes only `Wiki`, and writing under `.llm-wiki-state` changes neither.

- [ ] **Step 4: Route `hook session-start`**

The CLI must:

1. Read all stdin.
2. Decode `hook.Input`.
3. Call `hook.SessionStart`.
4. Detect the platform.
5. Encode the result.
6. Return `0` for clean/success and `4` for hook-internal validation failure.

- [ ] **Step 5: Run tests and commit**

```bash
gofmt -w internal/hook internal/cli
go test ./internal/hook ./internal/cli -v
git add internal/hook internal/cli
git commit -m "feat: load bounded wiki session context"
```

## Task 3: Implement the one-pass `Stop` state machine

**Files:**

- Create: `internal/hook/stop.go`
- Modify: `internal/hook/hook_test.go`
- Modify: `internal/cli/run.go`

- [ ] **Step 1: Write failing clean, synced, drift, and loop-guard tests**

```go
func TestStopOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, root string)
		wantOutcome Outcome
	}{
		{name: "no changes", setup: noChanges, wantOutcome: OutcomeClean},
		{name: "matching receipt", setup: matchingReceipt, wantOutcome: OutcomeSynchronized},
		{name: "material drift", setup: materialDrift, wantOutcome: OutcomeDrift},
		{name: "recovery exhausted", setup: exhaustedRecovery, wantOutcome: OutcomeFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initializedRepoFixture(t)
			tt.setup(t, root)
			got, err := Stop(context.Background(), Input{
				SessionID: "s1",
				CWD:       root,
				EventName: "Stop",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != tt.wantOutcome {
				t.Fatalf("Outcome = %q, want %q", got.Outcome, tt.wantOutcome)
			}
		})
	}
}
```

Extend the `internal/hook/hook_test.go` imports with `context`, `os`, `path/filepath`, `time`, `internal/config`, `internal/gitrepo`, and `internal/state`, then add:

```go
func noChanges(t *testing.T, root string) {
	t.Helper()
	if _, err := SessionStart(context.Background(), Input{
		SessionID: "s1",
		CWD:       root,
		EventName: "SessionStart",
	}); err != nil {
		t.Fatal(err)
	}
}

func materialDrift(t *testing.T, root string) {
	t.Helper()
	noChanges(t, root)
	path := filepath.Join(root, "internal", "newfeature", "feature.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package newfeature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func matchingReceipt(t *testing.T, root string) {
	t.Helper()
	materialDrift(t, root)
	repo, err := gitrepo.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
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
		Kind:        state.ReceiptSynced,
		Fingerprint: current,
		SessionID:   "s1",
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
}

func exhaustedRecovery(t *testing.T, root string) {
	t.Helper()
	materialDrift(t, root)
	cfg, err := config.Load(filepath.Join(root, "llm-wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	layout := state.NewLayout(filepath.Join(root, cfg.StatePath))
	session, err := layout.ReadSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	session.RecoveryPasses = cfg.Maintenance.MaxRecoveryPasses
	if err := layout.WriteSession(session); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
go test ./internal/hook -run TestStopOutcomes -v
```

Expected: FAIL because `Stop` does not exist.

- [ ] **Step 3: Implement stop evaluation**

`internal/hook/stop.go`:

```go
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

func Stop(ctx context.Context, input Input) (Result, error) {
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
		return Result{Outcome: OutcomeClean}, nil
	}
	layout := state.NewLayout(filepath.Join(repo.Root, cfg.StatePath))
	session, err := layout.ReadSession(input.SessionID)
	if err != nil {
		return Result{Outcome: OutcomeFailure, Reason: "LLM Wiki session state is missing."}, nil
	}
	paths, err := repo.ChangedPaths(session.Baseline.BaseCommit)
	if err != nil {
		return Result{}, err
	}
	if !requiresWikiReview(paths, cfg.WikiPath, session.StartupAudit) {
		return Result{Outcome: OutcomeClean}, nil
	}
	lease, err := lock.Acquire(
		ctx,
		layout.LockPath(session.WorktreeID),
		input.SessionID,
		time.Duration(cfg.LockWaitSeconds)*time.Second,
		time.Duration(cfg.SyncLeaseSeconds)*time.Second,
	)
	if err != nil {
		return Result{
			Outcome: OutcomeFailure,
			Reason:  "LLM Wiki synchronization is already active in this worktree.",
		}, nil
	}
	releaseLease := true
	defer func() {
		if releaseLease {
			_ = lease.Release(context.Background())
		}
	}()
	session, err = layout.ReadSession(input.SessionID)
	if err != nil {
		return Result{Outcome: OutcomeFailure, Reason: "LLM Wiki session state changed unexpectedly."}, nil
	}
	paths, err = repo.ChangedPaths(session.Baseline.BaseCommit)
	if err != nil {
		return Result{}, err
	}
	if !requiresWikiReview(paths, cfg.WikiPath, session.StartupAudit) {
		return Result{Outcome: OutcomeClean}, nil
	}
	current, err := BuildFingerprint(repo, cfg)
	if err != nil {
		return Result{}, err
	}
	if receipt, err := layout.ReadReceipt(current); err == nil && receipt.Matches(current) {
		session.StartupAudit = false
		session.Baseline = current
		session.LastObservedHead = current.BaseCommit
		session.RecoveryPasses = 0
		if err := layout.WriteSession(session); err != nil {
			return Result{}, err
		}
		if err := layout.WriteObservation(state.Observation{
			WorktreeID: session.WorktreeID,
			Head:       current.BaseCommit,
			ObservedAt: time.Now().UTC(),
		}); err != nil {
			return Result{}, err
		}
		return Result{Outcome: OutcomeSynchronized}, nil
	}
	if session.RecoveryPasses >= cfg.Maintenance.MaxRecoveryPasses {
		return Result{
			Outcome: OutcomeFailure,
			Reason:  "LLM Wiki could not synchronize after one recovery pass; review wiki health.",
		}, nil
	}
	session.RecoveryPasses++
	if err := layout.WriteSession(session); err != nil {
		return Result{}, err
	}
	releaseLease = false
	return Result{
		Outcome: OutcomeDrift,
		Reason:  "Use the wiki-sync workflow now while this session owns the worktree lease. Update durable project memory or record a no-update receipt, validate, then finish.",
	}, nil
}

func requiresWikiReview(paths []string, wikiPath string, startupAudit bool) bool {
	if startupAudit || materiality.ClassifyPaths(paths, wikiPath) != materiality.HintNone {
		return true
	}
	for _, path := range paths {
		if inside(path, wikiPath) || filepath.ToSlash(path) == "llm-wiki.yaml" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Route `hook stop` and verify protocol output**

Add CLI tests asserting:

- Clean emits zero stdout and zero stderr.
- Drift returns valid Codex JSON for a Codex fixture.
- Drift returns valid Claude JSON for a Claude fixture.
- Failure allows completion but emits one warning.

- [ ] **Step 5: Run tests and commit**

```bash
gofmt -w internal/hook internal/cli
go test ./internal/hook ./internal/cli -v
git add internal/hook internal/cli
git commit -m "feat: enforce one-pass wiki synchronization"
```

## Task 4: Add `receipt write`

**Files:**

- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

- [ ] **Step 1: Write a failing receipt-command test**

The test initializes a fixture, starts a hook session, applies a whitespace-only formatting change to an existing source file, runs:

```text
receipt write --kind no-update --reason "formatting only"
```

Then it runs `hook stop` and asserts silent synchronized completion.

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/cli -run TestReceiptWrite -v
```

Expected: FAIL because the command is not routed.

- [ ] **Step 3: Implement receipt writing**

Parse:

```text
--kind synced|no-update
--reason TEXT
--root PATH
--session-id ID
```

The command:

1. Discovers the repository.
2. Resolves the worktree ID. When `--session-id` is omitted, reads the active owner with `lock.CurrentOwner(layout.LockPath(worktreeID))`; when it is present, requires it to match that owner.
3. Loads config and the owning session.
4. Reacquires the persistent lease as the same owner to renew `sync_lease_seconds`.
5. Runs strict validation and rejects either receipt kind while validation errors remain.
6. Requires a non-empty reason of at most 500 characters for `no-update`.
7. Calls `hook.BuildFingerprint` after validation.
8. Writes `state.Receipt` atomically with that fingerprint.
9. Releases the persistent lease only after the receipt is durable.
10. Produces no output on success.

Return exit code `5` when no active synchronization lease exists and `6` when another owner holds it.

- [ ] **Step 4: Run tests and commit**

```bash
gofmt -w internal/cli
go test ./internal/cli -run TestReceiptWrite -v
git add internal/cli
git commit -m "feat: record wiki synchronization receipts"
```

## Task 5: Add shared hook wrappers and configuration

**Files:**

- Create: `plugins/llm-wiki/hooks/hooks.json`
- Create: `plugins/llm-wiki/scripts/run-hook.sh`
- Create: `plugins/llm-wiki/scripts/run-hook.ps1`
- Create: `plugins/llm-wiki/bin/.gitkeep`
- Test: `internal/hook/wrapper_test.go`

- [ ] **Step 1: Write a failing hook-config validation test**

The test loads `hooks.json`, asserts only `SessionStart` and `Stop` exist, and checks every handler has:

- `type: "command"`.
- A POSIX command.
- A Windows command override.
- Timeout no longer than 10 seconds.
- A quoted plugin-root command path so installation directories containing spaces work.

- [ ] **Step 2: Create the shared hook configuration**

`plugins/llm-wiki/hooks/hooks.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "\"${CLAUDE_PLUGIN_ROOT}/scripts/run-hook.sh\" session-start",
            "commandWindows": "powershell -NoProfile -ExecutionPolicy Bypass -File \"${CLAUDE_PLUGIN_ROOT}/scripts/run-hook.ps1\" session-start",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "\"${CLAUDE_PLUGIN_ROOT}/scripts/run-hook.sh\" stop",
            "commandWindows": "powershell -NoProfile -ExecutionPolicy Bypass -File \"${CLAUDE_PLUGIN_ROOT}/scripts/run-hook.ps1\" stop",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

- [ ] **Step 3: Create the POSIX wrapper**

`plugins/llm-wiki/scripts/run-hook.sh`:

```sh
#!/bin/sh
set -eu

event="${1:?hook event is required}"
script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
root="$(dirname "$script_dir")"
os="$(uname -s)"
arch="$(uname -m)"

case "$os/$arch" in
  Darwin/arm64) target="darwin-arm64" ;;
  Darwin/x86_64) target="darwin-amd64" ;;
  Linux/aarch64|Linux/arm64) target="linux-arm64" ;;
  Linux/x86_64) target="linux-amd64" ;;
  *)
    echo "LLM Wiki: unsupported hook platform $os/$arch" >&2
    exit 1
    ;;
esac

binary="$root/bin/$target/llm-wiki"
[ -x "$binary" ] ||
  { echo "LLM Wiki: packaged helper is missing for $target" >&2; exit 1; }
exec "$binary" hook "$event"
```

- [ ] **Step 4: Create the PowerShell wrapper**

`plugins/llm-wiki/scripts/run-hook.ps1`:

```powershell
param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("session-start", "stop")]
  [string]$Event
)

$root = Split-Path -Parent $PSScriptRoot

$binary = Join-Path $root "bin/windows-amd64/llm-wiki.exe"
if (-not (Test-Path $binary)) {
  [Console]::Error.WriteLine("LLM Wiki: packaged Windows helper is missing.")
  exit 1
}

& $binary hook $Event
exit $LASTEXITCODE
```

- [ ] **Step 5: Run config and wrapper tests**

The wrapper test copies the plugin scripts under a temporary path containing spaces, installs a fake target binary that records its arguments and stdin, invokes both events, and asserts exact forwarding. It also asserts an unsupported platform or missing binary exits nonzero with exactly one concise stderr warning.

```bash
chmod +x plugins/llm-wiki/scripts/run-hook.sh
go test ./internal/hook -run 'TestHookConfig|TestWrapper' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add plugins/llm-wiki/hooks plugins/llm-wiki/scripts plugins/llm-wiki/bin internal/hook
git commit -m "feat: add portable wiki lifecycle hooks"
```

## Task 6: Write the four portable Agent Skills

**Files:**

- Create: `plugins/llm-wiki/skills/wiki-init/SKILL.md`
- Create: `plugins/llm-wiki/skills/wiki-recall/SKILL.md`
- Create: `plugins/llm-wiki/skills/wiki-sync/SKILL.md`
- Create: `plugins/llm-wiki/skills/wiki-audit/SKILL.md`
- Test: `internal/hook/skills_test.go`

- [ ] **Step 1: Create `wiki-init`**

```markdown
---
name: wiki-init
description: Initialize or finish initializing the team-shared software-development LLM Wiki in a Git repository. Use when llm-wiki.yaml is missing or initialized is false.
---

# Initialize the Software Wiki

1. Find the Git root and inspect existing `AGENTS.md`, `CLAUDE.md`, `.gitignore`, and documentation.
2. Run `llm-wiki init --root <git-root> --non-interactive`.
3. Read `wiki_path` from `llm-wiki.yaml`, then read `<wiki-path>/schema.md`.
4. Inspect repository entry points, build manifests, source layout, tests, schemas, migrations, deployment files, and existing ADRs.
5. Replace scaffold claims with evidence-backed pages. Create component, flow, contract, decision, quality, and operations pages only when supported by evidence.
6. Keep `index.md` bounded and link every substantive page.
7. Remove the initial-synthesis item from `health.md`.
8. Append one `init` entry to `log.md`.
9. For every changed non-template page, run `llm-wiki fingerprint --root <git-root> --page <repository-relative-page>` and copy the returned verification fields into frontmatter.
10. Run `llm-wiki validate --root <git-root>`.
11. Fix all validation errors, then run `llm-wiki finalize-init --root <git-root>`.

Do not modify application code, create commits, or store secrets and failed hypotheses.
```

- [ ] **Step 2: Create `wiki-recall`**

```markdown
---
name: wiki-recall
description: Recall bounded architecture, contracts, flows, invariants, decisions, operations, and failure modes before planning or changing software behavior.
---

# Recall Project Knowledge

1. Run `llm-wiki status --json`.
2. Read `wiki_path` from the status output and open `<wiki-path>/index.md` first.
3. Search page summaries, stable IDs, relations, and evidence paths for the task's concepts and files.
4. Read only matching pages, then verify stale or material claims against code and tests.
5. Return a context packet no larger than `context_budget_bytes` from `llm-wiki.yaml`.

The packet must contain:

- Relevant responsibilities and boundaries.
- Affected flows and state transitions.
- Contracts and invariants.
- Current decisions and confirmed failure modes.
- Evidence paths and explicit unknowns.

Never load the complete wiki or full maintenance log.
```

- [ ] **Step 3: Create `wiki-sync`**

```markdown
---
name: wiki-sync
description: Synchronize durable software-project knowledge after a verified feature, bug fix, refactor, architecture pivot, dependency upgrade, contract change, deployment change, or removal.
---

# Synchronize Project Knowledge

1. Run `llm-wiki status --json` and require `sync_lease_active: true`; if it is false, do not edit the wiki and let the lifecycle Stop hook open the synchronization transaction.
2. Inspect the verified task outcome and the current Git diff.
3. Decide whether the change affects durable behavior, boundaries, dependencies, contracts, invariants, operations, decisions, or confirmed failure modes.
4. If it does not, run `llm-wiki receipt write --kind no-update --reason "<specific reason>"` and stop.
5. Read the affected existing pages before creating new ones.
6. Merge evidence into current component, flow, contract, quality, operation, and glossary pages.
7. For architectural pivots, create a new ADR and preserve superseded ADRs.
8. For bug fixes, retain only confirmed root cause, violated invariant, corrected behavior, regression evidence, and reusable diagnostics.
9. Update `index.md` and append one parseable `sync` entry to `log.md`.
10. For every changed page, run `llm-wiki fingerprint --page <repository-relative-page>` and copy the returned verification fields into frontmatter.
11. Run `llm-wiki validate`.
12. Fix all validation errors.
13. Run `llm-wiki receipt write --kind synced`.

Modify only canonical wiki files and local LLM Wiki state. Never modify application code or Git history.
```

- [ ] **Step 4: Create `wiki-audit`**

```markdown
---
name: wiki-audit
description: Audit or repair LLM Wiki health, stale evidence, contradictions, broken links, orphan pages, duplicate concepts, oversized indexes, logs, or schema migrations.
---

# Audit Project Knowledge

1. Run `llm-wiki status --json` and `llm-wiki validate`.
2. Before repairing files, require `sync_lease_active: true`; if it is false, report the audit result without editing and let the next Stop hook open the transaction.
3. Verify stale page evidence against current repository files.
4. Find contradictions, missing relationships, orphan pages, duplicate concepts, and important recurring concepts without pages.
5. Keep unresolved authoritative conflicts in `health.md`; do not guess.
6. Split the root index when it exceeds `index_entry_limit`.
7. Archive old log entries by year without changing their heading format.
8. Run `llm-wiki migrate` when the configured schema is older than the helper schema.
9. Update each affected verification block with `llm-wiki fingerprint --page <repository-relative-page>`.
10. Append one `audit` entry to `log.md`.
11. Run validation again and write a synced receipt when clean.

Stay silent when no repair is needed.
```

- [ ] **Step 5: Add skill structure tests**

The test scans `plugins/llm-wiki/skills/*/SKILL.md` and asserts:

- Exactly four skills exist.
- Frontmatter contains only `name` and `description`.
- Names match folder names.
- Descriptions are non-empty and under 300 characters.
- Each body names the deterministic commands it depends on.
- No `TODO`, `TBD`, or placeholder tokens exist.

- [ ] **Step 6: Run tests and commit**

```bash
go test ./internal/hook -run TestSkills -v
git add plugins/llm-wiki/skills internal/hook
git commit -m "feat: add software wiki agent skills"
```

## Task 7: Verify the complete in-loop workflow

**Files:**

- Create: `testdata/repos/hook-feature/`
- Create: `testdata/repos/hook-noop/`
- Create: `internal/hook/integration_test.go`
- Modify: `Makefile`

- [ ] **Step 1: Write end-to-end hook tests**

Cover:

1. Uninitialized repository: both hooks are silent.
2. Initialized repository with no changes: stop is silent.
3. Formatting-only change plus no-update receipt: stop is silent.
4. Material code change without receipt: stop blocks once.
5. Material change plus valid synced receipt: stop is silent.
6. Second unsatisfied stop: task may finish with exactly one warning.
7. Unseen material commit: startup audit forces one sync pass.
8. Unseen non-material commit: SessionStart writes a matching local no-update receipt and advances the observation silently.
9. Session-start context is at most 1 KiB.

- [ ] **Step 2: Build a test binary into the plugin**

In `integration_test.go`, map `runtime.GOOS` and `runtime.GOARCH` to the same target names used by the wrapper, create that target directory, and run `go build -o <target-binary> ../../cmd/llm-wiki` before wrapper-level scenarios. Use `llm-wiki.exe` on Windows and register `t.Cleanup` to remove only the generated binary.

- [ ] **Step 3: Run the focused integration tests**

```bash
go test ./internal/hook -run TestInLoop -v
```

Expected: all nine scenarios pass.

- [ ] **Step 4: Keep generated local binaries out of ordinary commits**

Add development binary paths to the root `.gitignore`:

```text
plugins/llm-wiki/bin/*/llm-wiki
plugins/llm-wiki/bin/*/llm-wiki.exe
```

Keep only `.gitkeep` files until the release-packaging milestone intentionally stages binaries.

- [ ] **Step 5: Update and run the milestone gate**

Add:

```make
.PHONY: hook-test

hook-test:
	go test ./internal/hook ./internal/materiality ./internal/state

verify: fmt test vet build brick hook-test
```

Run:

```bash
make verify
```

Expected: PASS with no hook output on successful cases.

- [ ] **Step 6: Commit**

```bash
git add .gitignore internal/hook testdata/repos Makefile
git commit -m "test: verify quiet wiki maintenance loop"
```

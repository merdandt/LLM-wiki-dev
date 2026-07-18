# LLM Wiki Lifecycle Hooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `llm-wiki hook session-start`, `llm-wiki hook stop`, and `llm-wiki receipt write`, wire them into target projects via `.claude/settings.json` and `.codex/hooks.json` written by init, and ship as v0.2.0.

**Architecture:** One protocol serves both platforms (Codex hooks are Claude-Code-compatible): session-start prints an orientation packet to stdout; stop prints `{"decision":"block","reason":...}` on drift and nothing otherwise. Deterministic detection uses the existing fingerprint/materiality/lease/receipt primitives from `internal/`. Hooks never block completion, never touch the network, never modify application code.

**Tech Stack:** Go 1.26 (stdlib + existing internal packages), JSON hook protocol, POSIX shell guard commands.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-18-llm-wiki-hooks-design.md`. The archived M3 plan's Codex protocol (`continue`/`stopReason`, platform detection via `model`) is obsolete — do NOT implement it.
- Session-start stdout packet hard cap: 1024 bytes. Stop drift `reason` ≤1200 chars.
- Hook subcommands always exit 0 (internal errors → one stderr line). `receipt write` exit codes: 0 success, 2 usage, 3 command error, 4 validation errors, 5 no active lease, 6 lease timeout/other owner.
- All state writes go through `internal/state` / `internal/atomicfile`; never write outside `.llm-wiki-state/`, `.claude/settings.json`, `.codex/hooks.json`.
- Every commit message ends with: `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`
- Gate for every task: `gofmt -w` on touched packages, focused `go test`, and `make verify` before the final commit of the milestone.

---

### Task 1: Hook input decoding and single-protocol output

**Files:**
- Create: `internal/hook/input.go`
- Create: `internal/hook/result.go`
- Test: `internal/hook/protocol_test.go`

**Interfaces:**
- Produces: `hook.Input{SessionID, CWD, EventName, Source, StopHookActive}`, `hook.Decode([]byte) (Input, error)`, `hook.Result{Outcome, Reason, Context}`, `hook.Outcome` constants (`OutcomeClean`, `OutcomeSynchronized`, `OutcomeDrift`, `OutcomeFailure`), `hook.Encode(event string, result Result, out io.Writer) error`. Tasks 3–5 rely on these exact names.

- [ ] **Step 1: Write the failing tests**

```go
package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	valid := []byte(`{"session_id":"s1","cwd":"/tmp/p","hook_event_name":"Stop","stop_hook_active":true}`)
	input, err := Decode(valid)
	if err != nil {
		t.Fatal(err)
	}
	if input.SessionID != "s1" || input.CWD != "/tmp/p" || input.EventName != "Stop" || !input.StopHookActive {
		t.Fatalf("unexpected input: %#v", input)
	}
	for name, data := range map[string]string{
		"missing session": `{"cwd":"/tmp/p","hook_event_name":"Stop"}`,
		"unknown event":   `{"session_id":"s1","cwd":"/tmp/p","hook_event_name":"Notification"}`,
		"malformed":       `{`,
	} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestEncodeSessionStartPrintsContext(t *testing.T) {
	var out bytes.Buffer
	err := Encode("SessionStart", Result{Outcome: OutcomeClean, Context: "LLM Wiki: memory at docs/llm-wiki/."}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "LLM Wiki") {
		t.Fatalf("context not printed: %q", out.String())
	}
}

func TestEncodeSilentOutcomes(t *testing.T) {
	for _, result := range []Result{
		{Outcome: OutcomeClean},
		{Outcome: OutcomeSynchronized},
		{Outcome: OutcomeFailure, Reason: "warning goes to stderr, not stdout"},
	} {
		var out bytes.Buffer
		if err := Encode("Stop", result, &out); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Fatalf("outcome %q wrote stdout: %q", result.Outcome, out.String())
		}
	}
}

func TestEncodeDriftBlocksWithReason(t *testing.T) {
	var out bytes.Buffer
	if err := Encode("Stop", Result{Outcome: OutcomeDrift, Reason: "sync the wiki"}, &out); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["decision"] != "block" || payload["reason"] != "sync the wiki" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/hook -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement**

`internal/hook/input.go`:

```go
package hook

import (
	"encoding/json"
	"errors"
)

type Input struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	EventName      string `json:"hook_event_name"`
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

import (
	"encoding/json"
	"io"
)

type Outcome string

const (
	OutcomeClean        Outcome = "clean"
	OutcomeSynchronized Outcome = "synchronized"
	OutcomeDrift        Outcome = "drift"
	OutcomeFailure      Outcome = "failure"
)

type Result struct {
	Outcome Outcome
	Reason  string
	Context string
}

// Encode writes the single cross-platform hook protocol: SessionStart
// context goes to stdout as plain text; Stop drift emits a block decision.
// Everything else is silent on stdout (failure warnings are the CLI's
// stderr concern).
func Encode(event string, result Result, out io.Writer) error {
	switch event {
	case "SessionStart":
		if result.Context == "" {
			return nil
		}
		_, err := io.WriteString(out, result.Context)
		return err
	case "Stop":
		if result.Outcome != OutcomeDrift {
			return nil
		}
		return json.NewEncoder(out).Encode(map[string]string{
			"decision": "block",
			"reason":   result.Reason,
		})
	}
	return nil
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `gofmt -w internal/hook && go test ./internal/hook -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/hook
git commit -m "feat: decode hook input and encode the shared protocol

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 2: Repository fingerprint partitioning

**Files:**
- Create: `internal/hook/fingerprint.go`
- Test: `internal/hook/fingerprint_test.go`
- Create: `internal/hook/helpers_test.go`

**Interfaces:**
- Consumes: `gitrepo.Repo` (`Head`, `WorktreePatch`, `UntrackedPaths`, `Output`), `fingerprint.Record{Path, Kind, Data}`, `fingerprint.Records`, `config.Config`, `state.Fingerprint{BaseCommit, Evidence, Wiki, Schema}`.
- Produces: `hook.BuildFingerprint(repo gitrepo.Repo, cfg config.Config) (state.Fingerprint, error)` and test helper `initializedRepoFixture(t *testing.T) string`. Tasks 3–5 use both.

- [ ] **Step 1: Write the failing test and the shared fixture helper**

`internal/hook/helpers_test.go`:

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
	source, err := filepath.Abs(filepath.Join("..", "..", "testdata", "wiki", "valid"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "add", ".")
	runGit(t, root,
		"-c", "user.name=Hook Test", "-c", "user.email=hook@example.com",
		"commit", "-qm", "baseline")
	// t.TempDir may sit behind a symlink on macOS; match gitrepo.Discover.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
```

`internal/hook/fingerprint_test.go`:

```go
package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

func buildFor(t *testing.T, root string) state.Fingerprint {
	t.Helper()
	repo, err := gitrepo.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "llm-wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fp, err := BuildFingerprint(repo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func TestBuildFingerprintPartitions(t *testing.T) {
	root := initializedRepoFixture(t)
	base := buildFor(t, root)

	// Source change moves Evidence only.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := buildFor(t, root)
	if after.Evidence == base.Evidence {
		t.Fatal("source change did not move the evidence fingerprint")
	}
	if after.Wiki != base.Wiki {
		t.Fatal("source change moved the wiki fingerprint")
	}

	// Wiki change moves Wiki only.
	if err := os.WriteFile(filepath.Join(root, "docs", "llm-wiki", "glossary.md"),
		[]byte("# Glossary\n\nUpdated.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	final := buildFor(t, root)
	if final.Wiki == after.Wiki {
		t.Fatal("wiki change did not move the wiki fingerprint")
	}
	if final.Evidence != after.Evidence {
		t.Fatal("wiki change moved the evidence fingerprint")
	}

	// State writes move neither.
	if err := os.MkdirAll(filepath.Join(root, ".llm-wiki-state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".llm-wiki-state", "scratch.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	unchanged := buildFor(t, root)
	if unchanged != final {
		t.Fatal("state write changed the fingerprint")
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

Run: `go test ./internal/hook -run TestBuildFingerprint -v`
Expected: FAIL — `BuildFingerprint` undefined.

- [ ] **Step 3: Implement**

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
	evidence := fingerprint.Records([]fingerprint.Record{{
		Path: "@worktree-patch",
		Kind: "git-diff",
		Data: []byte(patch),
	}})

	wikiOutput, err := repo.Output(
		"ls-files", "-co", "--exclude-standard", "--", "llm-wiki.yaml", cfg.WikiPath)
	if err != nil {
		return state.Fingerprint{}, err
	}
	var wikiRecords []fingerprint.Record
	for _, relative := range strings.Split(wikiOutput, "\n") {
		if relative == "" {
			continue
		}
		if relative != "llm-wiki.yaml" && !strings.EqualFold(filepath.Ext(relative), ".md") {
			continue
		}
		record, err := fileRecord(repo.Root, relative)
		if err != nil {
			return state.Fingerprint{}, err
		}
		wikiRecords = append(wikiRecords, record)
	}
	return state.Fingerprint{
		BaseCommit: head,
		Evidence:   evidence,
		Wiki:       fingerprint.Records(wikiRecords),
		Schema:     cfg.SchemaVersion,
	}, nil
}

func fileRecord(root, relative string) (fingerprint.Record, error) {
	slash := filepath.ToSlash(filepath.Clean(relative))
	full := filepath.Join(root, filepath.FromSlash(slash))
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
		return fingerprint.Record{Path: slash, Kind: "symlink", Data: []byte(filepath.ToSlash(target))}, err
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

Note: `WorktreePatch` already embeds untracked file contents, so the single `@worktree-patch` record covers modified-tracked and untracked evidence. The wiki partition hashes actual file contents so agent wiki edits change `Wiki` even before staging.

- [ ] **Step 4: Run tests, verify they pass**

Run: `gofmt -w internal/hook && go test ./internal/hook -v`
Expected: PASS. (`inside` is unused until Task 4; if `go vet` flags it, move the `inside` function into Task 4's `stop.go` instead — decide by running `make verify`. Unused private functions are allowed by vet, so it should be fine.)

- [ ] **Step 5: Commit**

```bash
git add internal/hook
git commit -m "feat: partition repository fingerprints into evidence and wiki

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 3: The session-start hook

**Files:**
- Create: `internal/hook/session_start.go`
- Test: `internal/hook/session_start_test.go`

**Interfaces:**
- Consumes: Task 1 types, Task 2 `BuildFingerprint` + fixture, `state.Layout` (`ReadObservation`, `WriteObservation`, `ReadSession`, `WriteSession`, `WriteReceipt`), `materiality.ClassifyPaths`, `wiki.Validate`, `lock.CurrentOwner`.
- Produces: `hook.SessionStart(ctx context.Context, input Input) (Result, error)`. Task 5 routes it.

- [ ] **Step 1: Write the failing tests**

`internal/hook/session_start_test.go`:

```go
package hook

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

func TestSessionStartUninitializedIsSilent(t *testing.T) {
	result, err := SessionStart(context.Background(), Input{
		SessionID: "s1", CWD: t.TempDir(), EventName: "SessionStart",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Context != "" {
		t.Fatalf("expected silence, got %q", result.Context)
	}
}

func TestSessionStartWritesBaselineAndPacket(t *testing.T) {
	root := initializedRepoFixture(t)
	result, err := SessionStart(context.Background(), Input{
		SessionID: "s1", CWD: root, EventName: "SessionStart", Source: "startup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Context) == 0 || len(result.Context) > 1024 {
		t.Fatalf("packet size %d, want 1..1024", len(result.Context))
	}
	for _, want := range []string{"docs/llm-wiki", "index.md", "status", "validate"} {
		if !strings.Contains(result.Context, want) {
			t.Fatalf("packet lacks %q:\n%s", want, result.Context)
		}
	}
	layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
	session, err := layout.ReadSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	if session.Baseline.Evidence == "" || session.Baseline.Wiki == "" {
		t.Fatalf("baseline fingerprint incomplete: %#v", session.Baseline)
	}
}

func TestSessionStartUnseenCommits(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		content   string
		wantAudit bool
	}{
		{name: "material commit", path: "internal/orders/service.go", content: "package orders\n", wantAudit: true},
		{name: "non-material commit", path: "NOTES.txt", content: "notes\n", wantAudit: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initializedRepoFixture(t)
			repo := discoverRepo(t, root)
			head, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			worktreeID, err := repo.WorktreeID()
			if err != nil {
				t.Fatal(err)
			}
			layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
			if err := layout.WriteObservation(state.Observation{WorktreeID: worktreeID, Head: head}); err != nil {
				t.Fatal(err)
			}
			writeAndCommit(t, root, tt.path, tt.content)
			if _, err := SessionStart(context.Background(), Input{
				SessionID: "s1", CWD: root, EventName: "SessionStart",
			}); err != nil {
				t.Fatal(err)
			}
			session, err := layout.ReadSession("s1")
			if err != nil {
				t.Fatal(err)
			}
			if session.StartupAudit != tt.wantAudit {
				t.Fatalf("StartupAudit = %v, want %v", session.StartupAudit, tt.wantAudit)
			}
		})
	}
}
```

Add to `internal/hook/helpers_test.go`:

```go
func discoverRepo(t *testing.T, root string) gitrepo.Repo {
	t.Helper()
	repo, err := gitrepo.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func writeAndCommit(t *testing.T, root, relative, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root,
		"-c", "user.name=Hook Test", "-c", "user.email=hook@example.com",
		"commit", "-qm", "update")
}
```

(Import `"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"` in the helpers file.)

Materiality note: `NOTES.txt` classifies as `HintNone` only if `ClassifyPaths` treats `.txt` as non-material; verify with `go test ./internal/materiality -v` and read `classify.go` — if `.txt` yields `HintReview`, use `README.md` as the non-material fixture path instead (the classifier's test table shows `README.md` → `HintNone`).

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/hook -run TestSessionStart -v`
Expected: FAIL — `SessionStart` undefined.

- [ ] **Step 3: Implement**

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
	if errors.Is(err, os.ErrNotExist) || (err == nil && !cfg.Initialized) {
		return Result{Outcome: OutcomeClean}, nil
	}
	if err != nil {
		return Result{}, err
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
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `gofmt -w internal/hook && go test ./internal/hook -v`
Expected: PASS. If `wiki.Validate` requires `AllowUninitialized` for the fixture, check `internal/wiki` options against how `runStatus` calls it (`internal/cli/run.go:90`) and mirror that call shape.

- [ ] **Step 5: Commit**

```bash
git add internal/hook
git commit -m "feat: orient sessions with a bounded wiki packet

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 4: The stop state machine

**Files:**
- Create: `internal/hook/stop.go`
- Test: `internal/hook/stop_test.go`

**Interfaces:**
- Consumes: Tasks 1–3 plus `lock.Acquire`, `state.Receipt`, `layout.ReadReceipt`.
- Produces: `hook.Stop(ctx context.Context, input Input) (Result, error)` and `hook.DriftReason` (exported const so the CLI test can assert it). Task 5 routes it.

- [ ] **Step 1: Write the failing tests**

`internal/hook/stop_test.go`:

```go
package hook

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
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
```

Add to `internal/hook/helpers_test.go`:

```go
func lockOwner(root, worktreeID string) (string, error) {
	layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
	return lock.CurrentOwner(context.Background(), layout.LockPath(worktreeID))
}
```

(Imports: `"context"`, `"github.com/merdandt/LLM-wiki-dev/internal/lock"`, `"github.com/merdandt/LLM-wiki-dev/internal/state"`.)

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/hook -run TestStop -v`
Expected: FAIL — `Stop`, `DriftReason` undefined.

- [ ] **Step 3: Implement**

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
	if err != nil {
		return Result{Outcome: OutcomeClean}, nil
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
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `gofmt -w internal/hook && go test ./internal/hook -v`
Expected: PASS, including the lease-held assertion.

- [ ] **Step 5: Commit**

```bash
git add internal/hook
git commit -m "feat: enforce one quiet maintenance pass on stop

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 5: CLI routing for `hook` and `receipt write`

**Files:**
- Create: `internal/cli/hook.go`
- Create: `internal/cli/receipt.go`
- Modify: `internal/cli/run.go` (dispatch + usage line)
- Test: `internal/cli/hook_test.go`

**Interfaces:**
- Consumes: `hook.Decode`, `hook.SessionStart`, `hook.Stop`, `hook.Encode`, `hook.BuildFingerprint`, `hook.DriftReason`, `lock`, `state`, `wiki`, `config`, `gitrepo`.
- Produces: CLI commands `llm-wiki hook session-start`, `llm-wiki hook stop` (stdin JSON), `llm-wiki receipt write --kind ... --reason ... --root ...`. Exit codes per Global Constraints.

- [ ] **Step 1: Write the failing CLI tests**

`internal/cli/hook_test.go` — use the same fixture pattern as `internal/hook` (copy the fixture helper; CLI package cannot import test helpers across packages):

```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func hookFixture(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "testdata", "wiki", "valid"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "."},
		{"-c", "user.name=T", "-c", "user.email=t@e.c", "commit", "-qm", "baseline"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func runHookCommand(t *testing.T, root, event, sessionID string, extra map[string]any) (string, string, int) {
	t.Helper()
	payload := map[string]any{"session_id": sessionID, "cwd": root,
		"hook_event_name": map[string]string{"session-start": "SessionStart", "stop": "Stop"}[event]}
	for k, v := range extra {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithStdin(bytes.NewReader(data), []string{"hook", event}, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestHookLifecycleEndToEnd(t *testing.T) {
	root := hookFixture(t)

	// 1. Session start prints the packet.
	stdout, stderr, code := runHookCommand(t, root, "session-start", "s1", nil)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "team memory") {
		t.Fatalf("session-start: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}

	// 2. Stop with no changes is byte-silent.
	stdout, stderr, code = runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("clean stop: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	// 3. Material change drifts with block JSON.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code = runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 {
		t.Fatalf("drift stop exit = %d", code)
	}
	var block map[string]string
	if err := json.Unmarshal([]byte(stdout), &block); err != nil {
		t.Fatalf("drift stdout not JSON: %q", stdout)
	}
	if block["decision"] != "block" || block["reason"] == "" {
		t.Fatalf("unexpected block payload: %v", block)
	}

	// 4. receipt write closes the loop (agent pass simulated by a wiki edit).
	if err := os.WriteFile(filepath.Join(root, "docs", "llm-wiki", "log.md"),
		appendLine(t, filepath.Join(root, "docs", "llm-wiki", "log.md"), "\n## 2026-07-18 sync\n\nUpdated for main.go change.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	code = Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdoutBuf, &stderrBuf)
	if code != 0 {
		t.Fatalf("receipt write: code=%d stderr=%q", code, stderrBuf.String())
	}

	// 5. Next stop is silent again.
	stdout, stderr, code = runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("post-receipt stop: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func appendLine(t *testing.T, path, line string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, []byte(line)...)
}

func TestReceiptWriteWithoutLease(t *testing.T) {
	root := hookFixture(t)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdout, &stderr)
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (no active lease); stderr=%q", code, stderr.String())
	}
}

func TestHookNeverBreaksSession(t *testing.T) {
	// Garbage stdin: one stderr line, exit 0.
	var stdout, stderr bytes.Buffer
	code := RunWithStdin(strings.NewReader("not json"), []string{"hook", "stop"}, &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "llm-wiki") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/cli -run 'TestHook|TestReceipt' -v`
Expected: FAIL — `RunWithStdin` and the commands do not exist.

- [ ] **Step 3: Implement routing**

`internal/cli/run.go` — replace the dispatch block and usage line, and add a stdin-aware entry point (keep `Run` for compatibility):

```go
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
		case "hook":
			return runHook(stdin, args[1:], stdout, stderr)
		case "receipt":
			return runReceipt(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintln(stderr, "usage: llm-wiki <version|init|status|validate|fingerprint|hook|receipt>")
	return 2
}
```

(`cmd/llm-wiki/main.go` keeps calling `cli.Run` — check with `cat cmd/llm-wiki/main.go` and leave unchanged.)

`internal/cli/hook.go`:

```go
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/merdandt/LLM-wiki-dev/internal/hook"
)

// runHook adapts stdin/stdout to the hook state machines. It always exits 0:
// a broken hook must never break the agent session.
func runHook(stdin io.Reader, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || (args[0] != "session-start" && args[0] != "stop") {
		fmt.Fprintln(stderr, "usage: llm-wiki hook <session-start|stop>")
		return 2
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook input unreadable: %v\n", err)
		return 0
	}
	input, err := hook.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook input rejected: %v\n", err)
		return 0
	}
	wantEvent := map[string]string{"session-start": "SessionStart", "stop": "Stop"}[args[0]]
	if input.EventName != wantEvent {
		fmt.Fprintf(stderr, "llm-wiki: hook event %q does not match subcommand %q\n", input.EventName, args[0])
		return 0
	}
	var result hook.Result
	if wantEvent == "SessionStart" {
		result, err = hook.SessionStart(context.Background(), input)
	} else {
		result, err = hook.Stop(context.Background(), input)
	}
	if err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook error: %v\n", err)
		return 0
	}
	if result.Outcome == hook.OutcomeFailure && result.Reason != "" {
		fmt.Fprintln(stderr, result.Reason)
	}
	if err := hook.Encode(wantEvent, result, stdout); err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook output failed: %v\n", err)
	}
	return 0
}
```

`internal/cli/receipt.go`:

```go
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
	if session, err := layout.ReadSession(owner); err == nil {
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
```

Note on the validation options: mirror exactly how `runStatus` builds `wiki.Options` (`internal/cli/run.go:90`) including `AllowUninitialized: true` if the fixture requires it — run the tests to decide; strictness for receipts means NOT passing `AllowUninitialized`.

Note on the lifecycle test's wiki edit: before choosing the appended `log.md` text, read `testdata/wiki/valid/docs/llm-wiki/log.md` and match its heading format exactly — the validator may parse the maintenance log, and a mismatched heading would make `receipt write` exit 4.

- [ ] **Step 4: Run tests, verify they pass**

Run: `gofmt -w internal/cli && go test ./internal/cli -v`
Expected: PASS, including the full lifecycle test.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat: route hook and receipt commands

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 6: Hook config generation and merge in init

**Files:**
- Create: `internal/initrepo/hookconfig.go`
- Test: `internal/initrepo/hookconfig_test.go`
- Modify: `internal/cli/run.go` (`runInit` calls the writer and prints warnings)

**Interfaces:**
- Produces: `initrepo.WriteHookConfigs(root string) (warnings []string, err error)`. Files written: `.claude/settings.json`, `.codex/hooks.json`.

- [ ] **Step 1: Write the failing tests**

`internal/initrepo/hookconfig_test.go`:

```go
package initrepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readHooks(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func hookEntries(t *testing.T, doc map[string]any, event string) []any {
	t.Helper()
	hooks, _ := doc["hooks"].(map[string]any)
	entries, _ := hooks[event].([]any)
	return entries
}

func TestWriteHookConfigsFresh(t *testing.T) {
	root := t.TempDir()
	warnings, err := WriteHookConfigs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	for _, path := range []string{
		filepath.Join(root, ".claude", "settings.json"),
		filepath.Join(root, ".codex", "hooks.json"),
	} {
		doc := readHooks(t, path)
		if len(hookEntries(t, doc, "SessionStart")) != 1 || len(hookEntries(t, doc, "Stop")) != 1 {
			t.Fatalf("%s: missing hook entries: %v", path, doc)
		}
	}
}

func TestWriteHookConfigsIdempotent(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		if _, err := WriteHookConfigs(root); err != nil {
			t.Fatal(err)
		}
	}
	doc := readHooks(t, filepath.Join(root, ".claude", "settings.json"))
	if got := len(hookEntries(t, doc, "Stop")); got != 1 {
		t.Fatalf("Stop entries = %d, want 1 after rerun", got)
	}
}

func TestWriteHookConfigsPreservesForeignContent(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{
  "permissions": {"allow": ["Bash(go test:*)"]},
  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "echo done"}]}]}
}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHookConfigs(root); err != nil {
		t.Fatal(err)
	}
	doc := readHooks(t, filepath.Join(claudeDir, "settings.json"))
	if _, ok := doc["permissions"]; !ok {
		t.Fatal("foreign permissions key was dropped")
	}
	if got := len(hookEntries(t, doc, "Stop")); got != 2 {
		t.Fatalf("Stop entries = %d, want foreign + ours", got)
	}
}

func TestWriteHookConfigsSkipsMalformed(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err := WriteHookConfigs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one about the malformed file", warnings)
	}
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{broken" {
		t.Fatal("malformed file was modified")
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/initrepo -run TestWriteHookConfigs -v`
Expected: FAIL — `WriteHookConfigs` undefined.

- [ ] **Step 3: Implement**

`internal/initrepo/hookconfig.go`:

```go
package initrepo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
)

const hookMarker = ".llm-wiki/llm-wiki\" hook"

func claudeCommand(event string) string {
	return `[ -x "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" ] && "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" hook ` + event + ` || exit 0`
}

func codexCommand(event string) string {
	return `root="$(git rev-parse --show-toplevel 2>/dev/null)" && [ -x "$root/.llm-wiki/llm-wiki" ] && "$root/.llm-wiki/llm-wiki" hook ` + event + ` || exit 0`
}

func hookEvents(command func(string) string) map[string][]any {
	sessionStart := map[string]any{
		"matcher": "startup|resume|clear|compact",
		"hooks": []any{map[string]any{
			"type": "command", "command": command("session-start"), "timeout": 10,
		}},
	}
	stop := map[string]any{
		"hooks": []any{map[string]any{
			"type": "command", "command": command("stop"), "timeout": 15,
		}},
	}
	return map[string][]any{"SessionStart": {sessionStart}, "Stop": {stop}}
}

// WriteHookConfigs writes or conservatively merges the LLM Wiki lifecycle
// hooks into .claude/settings.json and .codex/hooks.json. Foreign hooks and
// settings keys are never modified; malformed files are left untouched and
// reported as warnings.
func WriteHookConfigs(root string) ([]string, error) {
	var warnings []string
	targets := []struct {
		path    string
		command func(string) string
	}{
		{filepath.Join(root, ".claude", "settings.json"), claudeCommand},
		{filepath.Join(root, ".codex", "hooks.json"), codexCommand},
	}
	for _, target := range targets {
		warning, err := mergeHookFile(target.path, hookEvents(target.command))
		if err != nil {
			return warnings, err
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

func mergeHookFile(path string, events map[string][]any) (string, error) {
	doc := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return "", err
	default:
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Sprintf("llm-wiki: %s is not valid JSON; add the LLM Wiki hooks manually", path), nil
		}
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	changed := false
	for event, entries := range events {
		existing, _ := hooks[event].([]any)
		if containsMarker(existing) {
			continue
		}
		hooks[event] = append(existing, entries...)
		changed = true
	}
	if !changed {
		return "", nil
	}
	doc["hooks"] = hooks
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return "", atomicfile.Write(path, append(encoded, '\n'), 0o644)
}

func containsMarker(entries []any) bool {
	encoded, err := json.Marshal(entries)
	if err != nil {
		return false
	}
	return strings.Contains(string(encoded), hookMarker)
}
```

- [ ] **Step 4: Wire into `runInit`**

In `internal/cli/run.go`, extend `runInit` after the successful `initrepo.Initialize` call:

```go
	if err := initrepo.Initialize(*rootFlag, *templateFlag); err != nil {
		return commandError(stderr, err)
	}
	warnings, err := initrepo.WriteHookConfigs(*rootFlag)
	if err != nil {
		return commandError(stderr, err)
	}
	for _, warning := range warnings {
		fmt.Fprintln(stderr, warning)
	}
	fmt.Fprintln(stdout, "llm-wiki: initialized repository template")
	return 0
```

Note: `*rootFlag` may be a subdirectory; `WriteHookConfigs` must receive the Git root. Match `Initialize`'s discovery: call `gitrepo.Discover(*rootFlag)` in `runInit` and pass `repo.Root` (add the import).

- [ ] **Step 5: Run tests, verify they pass**

Run: `gofmt -w internal/initrepo internal/cli && go test ./internal/initrepo ./internal/cli -v`
Expected: PASS, including the pre-existing init idempotency test (`git status` in the fixture now also shows `.claude/` and `.codex/` — the idempotency snapshot in `init_test.go` hashes the whole tree, and reruns must produce identical bytes; `json.MarshalIndent` with sorted map keys is deterministic in Go).

- [ ] **Step 6: Commit**

```bash
git add internal/initrepo internal/cli
git commit -m "feat: install lifecycle hook configs for Claude Code and Codex

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 7: Template instructions and documentation

**Files:**
- Modify: `template/AGENTS.md`
- Modify: `README.md`
- Modify: `docs/installation.md`

- [ ] **Step 1: Update the template's managed instruction block**

Replace `template/AGENTS.md` content (inside the existing markers) with:

```markdown
<!-- llm-wiki:start -->
## LLM Wiki

Keep `{{wiki_path}}` as the team-shared, evidence-backed memory for this repository. Read `{{wiki_path}}/index.md` before exploring code and follow links to only the relevant pages. Hooks are quiet and local-only; they never call a network service during normal agent sessions.

If the Stop hook requests a maintenance pass: update the wiki pages affected by your diff, refresh README sections the changes invalidate (features, usage, install steps, API), run `.llm-wiki/llm-wiki validate`, then finish with `.llm-wiki/llm-wiki receipt write --kind synced` (or `--kind no-update --reason "<why>"` if nothing durable changed).
<!-- llm-wiki:end -->
```

- [ ] **Step 2: Add the hooks section to README.md**

In `README.md`, replace the paragraph starting `**So what should you expect right now?**` and the following interim-hook paragraph (inside "What works today, what to expect") with:

```markdown
**So what should you expect?** After install, both hooks are wired automatically: a session-start hook injects a small orientation packet (where the wiki is, how to search it), and a stop hook runs the sticky-note check after every agent turn — silent unless durable knowledge drifted, in which case the agent gets exactly one quiet maintenance pass to update the wiki (and README sections the change invalidated) before finishing. Codex users approve the project hooks once with `/hooks`.
```

And update the "Not shipped yet" list to remove hooks (skills + plugin packaging remain).

- [ ] **Step 3: Document the Codex trust step in docs/installation.md**

Append after the existing team-ownership paragraph:

```markdown
Installation wires quiet lifecycle hooks for both Claude Code (`.claude/settings.json`) and Codex (`.codex/hooks.json`); commit both so the whole team gets them. Codex asks each developer to approve the project hooks once via the `/hooks` command. The hook commands are no-ops when `.llm-wiki/llm-wiki` is absent, so teammates who have not run the installer see no errors.
```

- [ ] **Step 4: Verify and commit**

Run: `make verify`
Expected: PASS (docs changes cannot break it; this catches stray template regressions via init tests).

```bash
git add template/AGENTS.md README.md docs/installation.md
git commit -m "docs: describe the wired lifecycle hooks

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 8: Binary-level integration test

**Files:**
- Create: `internal/cli/integration_test.go`

- [ ] **Step 1: Write the test**

```go
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBuiltBinaryHookFlow drives the real binary the way Claude Code and
// Codex do: JSON on stdin, silence or block JSON on stdout.
func TestBuiltBinaryHookFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell guard commands are not exercised on Windows")
	}
	binary := filepath.Join(t.TempDir(), "llm-wiki")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/llm-wiki")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	root := hookFixture(t)

	run := func(event, name string) (string, string, int) {
		payload, err := json.Marshal(map[string]any{
			"session_id": "it1", "cwd": root, "hook_event_name": name,
		})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "hook", event)
		cmd.Stdin = bytes.NewReader(payload)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return stdout.String(), stderr.String(), code
	}

	stdout, stderr, code := run("session-start", "SessionStart")
	if code != 0 || !strings.Contains(stdout, "team memory") || stderr != "" {
		t.Fatalf("session-start: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = run("stop", "Stop")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("clean stop: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code = run("stop", "Stop")
	if code != 0 || !strings.Contains(stdout, `"decision"`) {
		t.Fatalf("drift stop: code=%d stdout=%q", code, stdout)
	}

	// The guard command is a silent no-op without the binary.
	guard := exec.Command("sh", "-c",
		`[ -x "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" ] && "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" hook stop || exit 0`)
	guard.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+root)
	var guardOut bytes.Buffer
	guard.Stdout = &guardOut
	if err := guard.Run(); err != nil || guardOut.Len() != 0 {
		t.Fatalf("guard without binary: err=%v out=%q", err, guardOut.String())
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/cli -run TestBuiltBinaryHookFlow -v`
Expected: PASS.

- [ ] **Step 3: Full gate and commit**

Run: `make verify`
Expected: PASS.

```bash
git add internal/cli/integration_test.go
git commit -m "test: drive the built binary through the hook protocol

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

### Task 9: Release v0.2.0 and live verification

**Files:**
- Modify: `release/release-manifest.json`
- Create: `cloudflare/site/releases/0.2.0/release-manifest.json`
- Modify: `cloudflare/site/releases/latest/release-manifest.json`

- [ ] **Step 1: Build, manifest, tag, publish** (the proven pipeline; commit + tag BEFORE `gh release create` so archives match the tag)

```bash
rm -rf dist
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64 windows-amd64; do
  scripts/package-release.sh --version 0.2.0 --target "$target" --output dist
done
scripts/verify-release.sh dist
find dist -maxdepth 1 -type f -name 'llm-wiki-*.tar.gz' -print0 | sort -z | xargs -0 shasum -a 256 > dist/SHA256SUMS
python3 - <<'EOF'
import json
sums = {}
for line in open('dist/SHA256SUMS'):
    digest, path = line.split()
    sums[path.replace('dist/', '')] = digest
manifest = {
    "version": "0.2.0",
    "release_base_url": "https://github.com/merdandt/LLM-wiki-dev/releases/download/v0.2.0",
    "artifacts": {},
}
for t in ['darwin-arm64', 'darwin-amd64', 'linux-arm64', 'linux-amd64', 'windows-amd64']:
    name = f"llm-wiki-{t}.tar.gz"
    manifest["artifacts"][f"{t}.archive"] = name
    manifest["artifacts"][f"{t}.sha256"] = sums[name]
open('release/release-manifest.json', 'w').write(json.dumps(manifest, indent=2) + "\n")
EOF
mkdir -p cloudflare/site/releases/0.2.0
cp release/release-manifest.json cloudflare/site/releases/0.2.0/release-manifest.json
cp release/release-manifest.json cloudflare/site/releases/latest/release-manifest.json
make verify && node --test cloudflare/worker.test.js
git add release/release-manifest.json cloudflare/site/releases/
git commit -m "release: v0.2.0 lifecycle hooks

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin main v0.2.0
env -u GITHUB_TOKEN -u GH_TOKEN gh release create v0.2.0 dist/llm-wiki-*.tar.gz dist/SHA256SUMS \
  --repo merdandt/LLM-wiki-dev --title "LLM Wiki v0.2.0" --generate-notes
cd cloudflare && CLOUDFLARE_ACCOUNT_ID=7d5610881fa4e1ad9022c4ffef7a2f7b npx -y wrangler@latest deploy
```

- [ ] **Step 2: Live end-to-end check in a disposable repo**

```bash
tmp=$(mktemp -d) && git -C "$tmp" init -q && git -C "$tmp" config user.email t@e.c && git -C "$tmp" config user.name t
cd "$tmp" && curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash
./.llm-wiki/llm-wiki version                          # expect 0.2.0
ls .claude/settings.json .codex/hooks.json            # expect both present
printf '{"session_id":"live","cwd":"%s","hook_event_name":"SessionStart"}' "$tmp" | ./.llm-wiki/llm-wiki hook session-start
# expect the orientation packet
printf '{"session_id":"live","cwd":"%s","hook_event_name":"Stop"}' "$tmp" | ./.llm-wiki/llm-wiki hook stop
# expect silence
```

Rerun the installer and confirm idempotency (`git status` fingerprint unchanged).

- [ ] **Step 3: Report** — summarize live results; hand the dummy-project scenario matrix (feature/bugfix/no-op/teammate-pull in real Claude Code + Codex sessions) back to the user for interactive testing.

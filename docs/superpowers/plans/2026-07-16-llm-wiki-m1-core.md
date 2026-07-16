# LLM Wiki Milestone 1 Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the deterministic `llm-wiki` core with configuration, repository fingerprints, state, persistent synchronization leases, materiality hints, and structural validation.

**Architecture:** Use a small standard-library CLI and focused internal Go packages. Git remains the repository engine, YAML config is parsed with `gopkg.in/yaml.v3`, Markdown links are parsed with Goldmark, and a short-lived `gofrs/flock` mutex protects persistent cross-process lease records.

**Tech Stack:** Go 1.26.5, `gopkg.in/yaml.v3`, `github.com/yuin/goldmark`, `github.com/gofrs/flock`, `github.com/natefinch/atomic`, Git.

---

## Task 1: Bootstrap the Go module and CLI shell

**Files:**

- Create: `.tool-versions`
- Create: `go.mod`
- Create: `Makefile`
- Create: `cmd/llm-wiki/main.go`
- Create: `internal/cli/run.go`
- Test: `internal/cli/run_test.go`

- [ ] **Step 1: Write the failing version-command test**

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "llm-wiki dev" {
		t.Fatalf("stdout = %q, want %q", got, "llm-wiki dev")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not exist**

Run:

```bash
go test ./internal/cli -run TestRunVersion -v
```

Expected: FAIL because `internal/cli` and `Run` do not exist.

- [ ] **Step 3: Create the pinned tool and module files**

`.tool-versions`:

```text
golang 1.26.5
dart 3.12.2
```

`go.mod`:

```go
module github.com/merdandt/LLM-wiki-dev

go 1.26.0
```

`internal/cli/run.go`:

```go
package cli

import (
	"fmt"
	"io"
)

var Version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "llm-wiki %s\n", Version)
		return 0
	}
	fmt.Fprintln(stderr, "usage: llm-wiki <version|validate|status|init|finalize-init|migrate|hook|receipt|plugin>")
	return 2
}
```

`cmd/llm-wiki/main.go`:

```go
package main

import (
	"os"

	"github.com/merdandt/LLM-wiki-dev/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

`Makefile`:

```make
.PHONY: test vet build verify

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/llm-wiki

verify: test vet build
```

- [ ] **Step 4: Run the test and binary**

Run:

```bash
go test ./internal/cli -run TestRunVersion -v
go run ./cmd/llm-wiki version
```

Expected:

```text
PASS
llm-wiki dev
```

- [ ] **Step 5: Commit**

```bash
git add .tool-versions go.mod Makefile cmd/llm-wiki internal/cli
git commit -m "feat: bootstrap llm-wiki helper"
```

## Task 2: Define and load `llm-wiki.yaml`

**Files:**

- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Create: `schemas/llm-wiki-config-v1.schema.json`
- Modify: `go.mod`
- Create through Go tooling: `go.sum`

- [ ] **Step 1: Write failing tests for defaults and YAML loading**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	got := Default()
	if got.SchemaVersion != 1 || got.WikiPath != "docs/llm-wiki" {
		t.Fatalf("unexpected defaults: %#v", got)
	}
	if got.ContextBudgetBytes != 12*1024 || got.IndexEntryLimit != 200 {
		t.Fatalf("unexpected budgets: %#v", got)
	}
	if got.LockWaitSeconds != 5 || got.SyncLeaseSeconds != 600 ||
		got.Maintenance.MaxRecoveryPasses != 1 {
		t.Fatalf("unexpected maintenance defaults: %#v", got)
	}
}

func TestLoadRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "llm-wiki.yaml")
	err := os.WriteFile(path, []byte("schema_version: 1\nwiki_path: ../outside\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want path traversal error")
	}
}
```

Add table cases for `wiki_path: .git/hooks`, `state_path: .git/llm-wiki`, identical wiki/state paths, and one path nested inside the other; every case must return an error.

Add a symlinked-config test and assert `Load` refuses to follow it; skip only when the operating system denies test symlink creation.

- [ ] **Step 2: Run the tests and verify they fail**

Run:

```bash
go test ./internal/config -v
```

Expected: FAIL because `Default` and `Load` do not exist.

- [ ] **Step 3: Add YAML and implement the config model**

Run:

```bash
go get gopkg.in/yaml.v3@v3.0.1
```

`internal/config/config.go`:

```go
package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Maintenance struct {
	MaxRecoveryPasses int `yaml:"max_recovery_passes" json:"max_recovery_passes"`
}

type Config struct {
	SchemaVersion     int         `yaml:"schema_version" json:"schema_version"`
	Initialized       bool        `yaml:"initialized" json:"initialized"`
	WikiPath          string      `yaml:"wiki_path" json:"wiki_path"`
	StatePath         string      `yaml:"state_path" json:"state_path"`
	ContextBudgetBytes int        `yaml:"context_budget_bytes" json:"context_budget_bytes"`
	IndexEntryLimit   int         `yaml:"index_entry_limit" json:"index_entry_limit"`
	LockWaitSeconds   int         `yaml:"lock_wait_seconds" json:"lock_wait_seconds"`
	SyncLeaseSeconds  int         `yaml:"sync_lease_seconds" json:"sync_lease_seconds"`
	Maintenance       Maintenance `yaml:"maintenance" json:"maintenance"`
}

func Default() Config {
	return Config{
		SchemaVersion:      1,
		WikiPath:           "docs/llm-wiki",
		StatePath:          ".llm-wiki-state",
		ContextBudgetBytes: 12 * 1024,
		IndexEntryLimit:    200,
		LockWaitSeconds:    5,
		SyncLeaseSeconds:   600,
		Maintenance:        Maintenance{MaxRecoveryPasses: 1},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	info, err := os.Lstat(path)
	if err != nil {
		return Config{}, err
	}
	if !info.Mode().IsRegular() {
		return Config{}, errors.New("llm-wiki config must be a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, err
	}
	for _, key := range []string{"schema_version", "initialized", "wiki_path", "state_path"} {
		if _, ok := raw[key]; !ok {
			return Config{}, errors.New("missing required config key: " + key)
		}
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.SchemaVersion != 1 {
		return Config{}, errors.New("unsupported schema_version")
	}
	if err := validateRelative(cfg.WikiPath); err != nil {
		return Config{}, err
	}
	if err := validateRelative(cfg.StatePath); err != nil {
		return Config{}, err
	}
	cfg.WikiPath = filepath.ToSlash(filepath.Clean(cfg.WikiPath))
	cfg.StatePath = filepath.ToSlash(filepath.Clean(cfg.StatePath))
	if pathsOverlap(cfg.WikiPath, cfg.StatePath) {
		return Config{}, errors.New("wiki_path and state_path must not overlap")
	}
	if cfg.ContextBudgetBytes < 1024 ||
		cfg.IndexEntryLimit < 10 ||
		cfg.LockWaitSeconds < 0 || cfg.LockWaitSeconds > 60 ||
		cfg.SyncLeaseSeconds < 30 || cfg.SyncLeaseSeconds > 3600 ||
		cfg.Maintenance.MaxRecoveryPasses < 0 || cfg.Maintenance.MaxRecoveryPasses > 3 {
		return Config{}, errors.New("config value is outside the supported range")
	}
	return cfg, nil
}

var safeRepoPath = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

func validateRelative(path string) error {
	clean := filepath.Clean(path)
	slash := filepath.ToSlash(clean)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		slash == ".git" || strings.HasPrefix(slash, ".git/") ||
		!safeRepoPath.MatchString(slash) {
		return errors.New("path must stay inside repository")
	}
	return nil
}

func pathsOverlap(first, second string) bool {
	a := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(first)), "/")
	b := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(second)), "/")
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
```

- [ ] **Step 4: Add the machine-readable config schema**

`schemas/llm-wiki-config-v1.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://llm-wiki.dev/schemas/config-v1.json",
  "type": "object",
  "required": ["schema_version", "initialized", "wiki_path", "state_path"],
  "properties": {
    "schema_version": { "const": 1 },
    "initialized": { "type": "boolean" },
    "wiki_path": { "type": "string", "minLength": 1 },
    "state_path": { "type": "string", "minLength": 1 },
    "context_budget_bytes": { "type": "integer", "minimum": 1024 },
    "index_entry_limit": { "type": "integer", "minimum": 10 },
    "lock_wait_seconds": { "type": "integer", "minimum": 0, "maximum": 60 },
    "sync_lease_seconds": { "type": "integer", "minimum": 30, "maximum": 3600 },
    "maintenance": {
      "type": "object",
      "required": ["max_recovery_passes"],
      "properties": {
        "max_recovery_passes": { "type": "integer", "minimum": 0, "maximum": 3 }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 5: Run tests and formatting**

Run:

```bash
gofmt -w internal/config
go test ./internal/config -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config schemas/llm-wiki-config-v1.schema.json
git commit -m "feat: add llm-wiki configuration"
```

## Task 3: Discover Git repositories and calculate fingerprints

**Files:**

- Create: `internal/gitrepo/repo.go`
- Test: `internal/gitrepo/repo_test.go`
- Create: `internal/fingerprint/fingerprint.go`
- Test: `internal/fingerprint/fingerprint_test.go`

- [ ] **Step 1: Write failing repository-discovery tests**

```go
package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDiscoverFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	repo, err := Discover(nested)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Root != root {
		t.Fatalf("Root = %q, want %q", repo.Root, root)
	}
}
```

- [ ] **Step 2: Write the failing deterministic-fingerprint test**

```go
package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilesIsOrderIndependent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Files(root, []string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Files(root, []string{"b.txt", "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("fingerprints differ: %q != %q", first, second)
	}
}
```

- [ ] **Step 3: Run both tests and verify they fail**

```bash
go test ./internal/gitrepo ./internal/fingerprint -v
```

Expected: FAIL because the packages are missing.

- [ ] **Step 4: Implement Git discovery and command execution**

`internal/gitrepo/repo.go`:

```go
package gitrepo

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Repo struct {
	Root string
}

func Discover(start string) (Repo, error) {
	cmd := exec.Command("git", "-C", start, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return Repo{}, errors.New("not inside a Git repository")
	}
	root, err := filepath.Abs(strings.TrimSpace(string(out)))
	if err != nil {
		return Repo{}, err
	}
	return Repo{Root: root}, nil
}

func (r Repo) Output(args ...string) (string, error) {
	return r.OutputInput(nil, args...)
}

func (r Repo) OutputInput(input []byte, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.Root}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r Repo) Head() (string, error) {
	head, err := r.Output("rev-parse", "--verify", "HEAD")
	if err == nil {
		return head, nil
	}
	return r.OutputInput(nil, "hash-object", "-t", "tree", "--stdin")
}

func (r Repo) WorktreeID() (string, error) {
	path, err := r.Output("rev-parse", "--git-path", ".")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.Root, path)
	}
	return filepath.Abs(path)
}

func (r Repo) ChangedPaths(base string) ([]string, error) {
	committedAndTracked, err := r.Output("diff", "--name-only", base, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := r.Output("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	unique := map[string]struct{}{}
	for _, output := range []string{committedAndTracked, untracked} {
		for _, path := range strings.Split(output, "\n") {
			if path != "" {
				unique[filepath.ToSlash(path)] = struct{}{}
			}
		}
	}
	paths := make([]string, 0, len(unique))
	for path := range unique {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func (r Repo) WorktreePatch(excludedPaths ...string) (string, error) {
	head, err := r.Head()
	if err != nil {
		return "", err
	}
	args := []string{
		"diff", "--binary", "--no-ext-diff", "--no-textconv", "--submodule=diff",
		head, "--", ".",
	}
	args = append(args, exclusionPathspecs(excludedPaths...)...)
	return r.Output(args...)
}

func (r Repo) UntrackedPaths(excludedPaths ...string) ([]string, error) {
	args := []string{"ls-files", "--others", "--exclude-standard", "--", "."}
	args = append(args, exclusionPathspecs(excludedPaths...)...)
	out, err := r.Output(args...)
	if err != nil || out == "" {
		return nil, err
	}
	paths := strings.Split(out, "\n")
	sort.Strings(paths)
	return paths, nil
}

func exclusionPathspecs(paths ...string) []string {
	var result []string
	for _, path := range paths {
		slash := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(path)), "/")
		result = append(
			result,
			":(top,exclude,literal)"+slash,
			":(top,exclude,glob)"+slash+"/**",
		)
	}
	return result
}
```

This contract includes tracked changes since the session baseline and new untracked files, so a newly created source module cannot bypass materiality detection.

Add tests that modify, stage, delete, and create files, then assert `WorktreePatch` and `UntrackedPaths` include source changes while excluding custom wiki/state paths and `llm-wiki.yaml`.

Add an unborn-repository test: initialize Git without a commit, assert `Head` returns the repository's empty-tree object ID, and assert `ChangedPaths` plus `WorktreePatch` still describe staged and untracked files.

- [ ] **Step 5: Implement stable file fingerprints**

`internal/fingerprint/fingerprint.go`:

```go
package fingerprint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

type Record struct {
	Path string
	Kind string
	Data []byte
}

func Files(root string, paths []string) (string, error) {
	records := make([]Record, 0, len(paths))
	for _, relative := range paths {
		clean := filepath.Clean(relative)
		full := filepath.Join(root, clean)
		info, err := os.Lstat(full)
		if err != nil {
			return "", err
		}
		var kind string
		var data []byte
		switch {
		case info.Mode().IsRegular():
			kind = "file"
			data, err = os.ReadFile(full)
		case info.Mode()&os.ModeSymlink != 0:
			kind = "symlink"
			var target string
			target, err = os.Readlink(full)
			data = []byte(filepath.ToSlash(target))
		default:
			err = errors.New("unsupported evidence file type")
		}
		if err != nil {
			return "", err
		}
		records = append(records, Record{
			Path: filepath.ToSlash(clean),
			Kind: kind,
			Data: data,
		})
	}
	return Records(records), nil
}

func Records(records []Record) string {
	ordered := append([]Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Path == ordered[j].Path {
			return ordered[i].Kind < ordered[j].Kind
		}
		return ordered[i].Path < ordered[j].Path
	})
	hash := sha256.New()
	for _, record := range ordered {
		data := record.Data
		if utf8.Valid(data) && !bytes.ContainsRune(data, '\x00') {
			data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		}
		fmt.Fprintf(hash, "%s\x00%s\x00%d\x00", record.Path, record.Kind, len(data))
		_, _ = hash.Write(data)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
```

Add tests proving symlinks hash their link target without reading the target file and that `Records` changes when a record becomes `missing` or a submodule record changes commit. Skip only the symlink case when the operating system denies test symlink creation.

Add a line-ending test proving UTF-8 text with LF and CRLF produces the same fingerprint while binary records containing NUL bytes remain byte-sensitive.

- [ ] **Step 6: Run tests**

```bash
gofmt -w internal/gitrepo internal/fingerprint
go test ./internal/gitrepo ./internal/fingerprint -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/gitrepo internal/fingerprint
git commit -m "feat: add repository fingerprints"
```

## Task 4: Persist sessions and synchronization receipts atomically

**Files:**

- Create: `internal/state/layout.go`
- Create: `internal/state/session.go`
- Create: `internal/state/receipt.go`
- Test: `internal/state/state_test.go`
- Create: `internal/atomicfile/write.go`
- Test: `internal/atomicfile/write_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write failing receipt round-trip and matching tests**

```go
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
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/state -v
```

Expected: FAIL because the state package does not exist.

- [ ] **Step 3: Add cross-platform atomic replacement support**

```bash
go get github.com/natefinch/atomic@v1.0.1
```

`internal/atomicfile/write.go`:

```go
package atomicfile

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/natefinch/atomic"
)

func Write(path string, data []byte, permission fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(permission); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return atomic.ReplaceFile(temporary, path)
}
```

`internal/atomicfile/write_test.go` writes `"first"` and then `"second"` to the same path, asserts the final file is exactly `"second"`, and asserts no `.tmp-` sibling remains. On non-Windows systems, also assert the requested mode is preserved.

- [ ] **Step 4: Define the shared state types**

`internal/state/receipt.go`:

```go
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
```

`internal/state/session.go`:

```go
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
```

- [ ] **Step 5: Implement atomic JSON persistence**

`internal/state/layout.go`:

```go
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
)

type Layout struct {
	Root string
}

func NewLayout(root string) Layout {
	return Layout{Root: root}
}

func (l Layout) WriteReceipt(receipt Receipt) error {
	return writeJSON(l.receiptPath(receipt.Fingerprint), receipt)
}

func (l Layout) ReadReceipt(fp Fingerprint) (Receipt, error) {
	var receipt Receipt
	err := readJSON(l.receiptPath(fp), &receipt)
	return receipt, err
}

func (l Layout) WriteSession(session Session) error {
	return writeJSON(l.sessionPath(session.ID), session)
}

func (l Layout) ReadSession(id string) (Session, error) {
	var session Session
	err := readJSON(l.sessionPath(id), &session)
	return session, err
}

func (l Layout) WriteObservation(observation Observation) error {
	return writeJSON(l.observationPath(observation.WorktreeID), observation)
}

func (l Layout) ReadObservation(worktreeID string) (Observation, error) {
	var observation Observation
	err := readJSON(l.observationPath(worktreeID), &observation)
	return observation, err
}

func (l Layout) LatestSession(worktreeID string) (Session, error) {
	entries, err := os.ReadDir(filepath.Join(l.Root, "sessions"))
	if err != nil {
		return Session{}, err
	}
	var latest Session
	found := false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var candidate Session
		if err := readJSON(filepath.Join(l.Root, "sessions", entry.Name()), &candidate); err != nil {
			return Session{}, err
		}
		if candidate.WorktreeID == worktreeID &&
			(!found || candidate.StartedAt.After(latest.StartedAt)) {
			latest = candidate
			found = true
		}
	}
	if !found {
		return Session{}, os.ErrNotExist
	}
	return latest, nil
}

func (l Layout) LatestReceipt() (Receipt, error) {
	entries, err := os.ReadDir(filepath.Join(l.Root, "receipts"))
	if err != nil {
		return Receipt{}, err
	}
	var latest Receipt
	found := false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var candidate Receipt
		if err := readJSON(filepath.Join(l.Root, "receipts", entry.Name()), &candidate); err != nil {
			return Receipt{}, err
		}
		if !found || candidate.CreatedAt.After(latest.CreatedAt) {
			latest = candidate
			found = true
		}
	}
	if !found {
		return Receipt{}, os.ErrNotExist
	}
	return latest, nil
}

func (l Layout) receiptPath(fp Fingerprint) string {
	data, _ := json.Marshal(fp)
	sum := sha256.Sum256(data)
	return filepath.Join(l.Root, "receipts", hex.EncodeToString(sum[:])+".json")
}

func (l Layout) observationPath(worktreeID string) string {
	sum := sha256.Sum256([]byte(worktreeID))
	return filepath.Join(l.Root, "observations", hex.EncodeToString(sum[:])+".json")
}

func (l Layout) LockPath(worktreeID string) string {
	sum := sha256.Sum256([]byte(worktreeID))
	return filepath.Join(l.Root, "locks", hex.EncodeToString(sum[:])+".lock")
}

func (l Layout) sessionPath(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(l.Root, "sessions", hex.EncodeToString(sum[:])+".json")
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(data, '\n'), 0o600)
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.ErrNotExist
		}
		return err
	}
	return json.Unmarshal(data, value)
}
```

Add tests that write sessions for two worktrees and receipts with distinct timestamps, then assert `LatestSession` filters by worktree and `LatestReceipt` selects the newest receipt.

- [ ] **Step 6: Run tests**

```bash
gofmt -w internal/atomicfile internal/state
go test ./internal/atomicfile ./internal/state -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/atomicfile internal/state
git commit -m "feat: persist wiki session receipts"
```

## Task 5: Add a cross-process worktree synchronization lease

**Files:**

- Create: `internal/lock/lock.go`
- Test: `internal/lock/lock_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write the failing persistent-lease test**

```go
package lock

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLeasePersistsUntilOwnerReleasesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wiki.lock")
	first, err := Acquire(
		context.Background(),
		path,
		"session-1",
		100*time.Millisecond,
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Acquire(
		context.Background(),
		path,
		"session-2",
		20*time.Millisecond,
		time.Minute,
	)
	if err == nil {
		t.Fatal("Acquire() error = nil, want timeout")
	}

	sameOwner, err := Acquire(
		context.Background(),
		path,
		"session-1",
		20*time.Millisecond,
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if sameOwner.Owner() != "session-1" {
		t.Fatalf("Owner() = %q", sameOwner.Owner())
	}

	if err := first.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(
		context.Background(),
		path,
		"session-2",
		100*time.Millisecond,
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release(context.Background())
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/lock -v
```

Expected: FAIL because the lease API does not exist.

- [ ] **Step 3: Add `gofrs/flock` and implement the persistent lease**

```bash
go get github.com/gofrs/flock@v0.13.0
```

`internal/lock/lock.go`:

```go
package lock

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
)

type record struct {
	Owner      string    `json:"owner"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type Lease struct {
	path  string
	owner string
}

func Acquire(
	ctx context.Context,
	path string,
	owner string,
	wait time.Duration,
	ttl time.Duration,
) (*Lease, error) {
	if owner == "" {
		return nil, errors.New("lease owner is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		mutex, err := acquireMutex(ctx, path+".mutex", time.Until(deadline))
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		current, readErr := readRecord(path)
		switch {
		case errors.Is(readErr, os.ErrNotExist):
		case readErr != nil:
			_ = mutex.Unlock()
			return nil, readErr
		case current.Owner == owner:
			current.ExpiresAt = now.Add(ttl)
			if err := writeRecord(path, current); err != nil {
				_ = mutex.Unlock()
				return nil, err
			}
			_ = mutex.Unlock()
			return &Lease{path: path, owner: owner}, nil
		case current.ExpiresAt.After(now):
			_ = mutex.Unlock()
			if time.Now().After(deadline) {
				return nil, errors.New("wiki synchronization lease timeout")
			}
			if err := waitForRetry(ctx, 25*time.Millisecond); err != nil {
				return nil, err
			}
			continue
		}
		next := record{Owner: owner, AcquiredAt: now, ExpiresAt: now.Add(ttl)}
		if err := writeRecord(path, next); err != nil {
			_ = mutex.Unlock()
			return nil, err
		}
		_ = mutex.Unlock()
		return &Lease{path: path, owner: owner}, nil
	}
}

func (l *Lease) Owner() string {
	return l.owner
}

func CurrentOwner(ctx context.Context, path string) (string, error) {
	mutex, err := acquireMutex(ctx, path+".mutex", 250*time.Millisecond)
	if err != nil {
		return "", err
	}
	defer mutex.Unlock()
	current, err := readRecord(path)
	if err != nil {
		return "", err
	}
	if !current.ExpiresAt.After(time.Now().UTC()) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "", os.ErrNotExist
	}
	return current.Owner, nil
}

func (l *Lease) Release(ctx context.Context) error {
	mutex, err := acquireMutex(ctx, l.path+".mutex", 5*time.Second)
	if err != nil {
		return err
	}
	defer mutex.Unlock()
	current, err := readRecord(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.Owner != l.owner {
		return errors.New("wiki synchronization lease owner changed")
	}
	return os.Remove(l.path)
}

func acquireMutex(ctx context.Context, path string, wait time.Duration) (*flock.Flock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if wait < 0 {
		wait = 0
	}
	deadline := time.Now().Add(wait)
	mutex := flock.New(path)
	for {
		locked, err := mutex.TryLock()
		if err != nil {
			return nil, err
		}
		if locked {
			return mutex, nil
		}
		if time.Now().After(deadline) {
			return nil, errors.New("wiki lease mutex timeout")
		}
		if err := waitForRetry(ctx, 10*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

func waitForRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func readRecord(path string) (record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return record{}, err
	}
	var value record
	err = json.Unmarshal(data, &value)
	return value, err
}

func writeRecord(path string, value record) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(data, '\n'), 0o600)
}
```

The short-lived OS mutex protects lease-file reads and writes. The JSON lease remains after the hook process exits, so it spans the active agent's semantic wiki-editing pass. An expired lease may be replaced only while holding the mutex.

- [ ] **Step 4: Add stale-lease and wrong-owner tests**

Write one expired record directly and assert a new owner replaces it. Then acquire as `session-1`, construct `Lease{path: path, owner: "session-2"}`, and assert `Release` refuses to delete the first owner's lease.

- [ ] **Step 5: Run tests**

```bash
gofmt -w internal/lock
go test ./internal/lock -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/lock
git commit -m "feat: add persistent wiki sync leases"
```

## Task 6: Classify deterministic materiality hints

**Files:**

- Create: `internal/materiality/classify.go`
- Test: `internal/materiality/classify_test.go`

- [ ] **Step 1: Write the failing path-classification test**

```go
package materiality

import "testing"

func TestClassifyPaths(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  Hint
	}{
		{name: "formatting docs only", paths: []string{"README.md"}, want: HintNone},
		{name: "source code", paths: []string{"src/auth/service.ts"}, want: HintPossible},
			{name: "schema", paths: []string{"api/openapi.yaml"}, want: HintPossible},
			{name: "lockfile only", paths: []string{"package-lock.json"}, want: HintReview},
			{name: "extensionless build file", paths: []string{"Dockerfile"}, want: HintReview},
			{name: "wiki only", paths: []string{"docs/llm-wiki/system.md"}, want: HintNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyPaths(tt.paths, "docs/llm-wiki"); got != tt.want {
				t.Fatalf("ClassifyPaths() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/materiality -v
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement conservative hints**

`internal/materiality/classify.go`:

```go
package materiality

import (
	"path/filepath"
	"strings"
)

type Hint string

const (
	HintNone     Hint = "none"
	HintReview   Hint = "review"
	HintPossible Hint = "possible"
)

func ClassifyPaths(paths []string, wikiPath string) Hint {
	result := HintNone
	for _, path := range paths {
		slash := filepath.ToSlash(path)
		if slash == wikiPath || strings.HasPrefix(slash, strings.TrimSuffix(wikiPath, "/")+"/") {
			continue
		}
		base := filepath.Base(slash)
		switch base {
		case "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "go.sum", "Cargo.lock", "pubspec.lock":
			if result == HintNone {
				result = HintReview
			}
			continue
		}
		ext := strings.ToLower(filepath.Ext(base))
			switch ext {
			case ".go", ".rs", ".py", ".js", ".jsx", ".ts", ".tsx", ".java", ".kt", ".swift",
				".dart", ".rb", ".php", ".cs", ".sql", ".proto", ".graphql", ".yaml", ".yml",
				".toml", ".json":
				return HintPossible
			case ".md", ".mdx", ".txt", ".png", ".jpg", ".jpeg", ".gif", ".svg":
				continue
			default:
				if result == HintNone {
					result = HintReview
				}
			}
	}
	return result
}
```

- [ ] **Step 4: Run tests**

```bash
gofmt -w internal/materiality
go test ./internal/materiality -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/materiality
git commit -m "feat: classify possible wiki drift"
```

## Task 7: Parse wiki frontmatter and Markdown links

**Files:**

- Create: `internal/wiki/frontmatter.go`
- Create: `internal/wiki/links.go`
- Test: `internal/wiki/validator_test.go`
- Create: `schemas/llm-wiki-page-v1.schema.json`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write failing page-parser tests**

```go
package wiki

import "testing"

func TestParsePage(t *testing.T) {
	input := []byte(`---
id: component.auth
kind: component
status: current
summary: Auth component.
verification:
  base_commit: abc
  evidence_fingerprint: sha256:def
evidence:
  - path: src/auth.go
relations:
  - flow.login
---
# Auth

See [Login](../flows/login.md).
`)

	page, err := ParsePage("components/auth.md", input)
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "component.auth" || page.Kind != "component" {
		t.Fatalf("unexpected page: %#v", page)
	}
	if len(page.Links) != 1 || page.Links[0] != "../flows/login.md" {
		t.Fatalf("unexpected links: %#v", page.Links)
	}
}
```

Add a second case with CRLF line endings and assert it parses to the same `Page`.

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/wiki -run TestParsePage -v
```

Expected: FAIL because `ParsePage` does not exist.

- [ ] **Step 3: Add Goldmark**

```bash
go get github.com/yuin/goldmark@v1.7.13
```

- [ ] **Step 4: Implement frontmatter parsing**

`internal/wiki/frontmatter.go`:

```go
package wiki

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"
)

type Verification struct {
	BaseCommit         string `yaml:"base_commit" json:"base_commit"`
	EvidenceFingerprint string `yaml:"evidence_fingerprint" json:"evidence_fingerprint"`
}

type Evidence struct {
	Path   string `yaml:"path" json:"path"`
	Symbol string `yaml:"symbol,omitempty" json:"symbol,omitempty"`
}

type Page struct {
	Path         string       `yaml:"-" json:"path"`
	ID           string       `yaml:"id" json:"id"`
	Kind         string       `yaml:"kind" json:"kind"`
	Status       string       `yaml:"status" json:"status"`
	Summary      string       `yaml:"summary" json:"summary"`
	Verification Verification `yaml:"verification" json:"verification"`
	Evidence     []Evidence   `yaml:"evidence" json:"evidence"`
	Relations    []string     `yaml:"relations,omitempty" json:"relations,omitempty"`
	Supersedes   []string     `yaml:"supersedes,omitempty" json:"supersedes,omitempty"`
	Links        []string     `yaml:"-" json:"links"`
	Body         []byte       `yaml:"-" json:"-"`
}

func ParsePage(path string, data []byte) (Page, error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return Page{}, errors.New("missing YAML frontmatter")
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return Page{}, errors.New("unterminated YAML frontmatter")
	}
	frontmatter := data[4 : 4+end]
	body := data[4+end+5:]
	page := Page{Path: path}
	decoder := yaml.NewDecoder(bytes.NewReader(frontmatter))
	decoder.KnownFields(true)
	if err := decoder.Decode(&page); err != nil {
		return Page{}, err
	}
	page.Links = ExtractLinks(body)
	page.Body = append([]byte(nil), body...)
	return page, nil
}
```

- [ ] **Step 5: Implement Markdown link extraction**

`internal/wiki/links.go`:

```go
package wiki

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func ExtractLinks(markdown []byte) []string {
	root := goldmark.DefaultParser().Parse(text.NewReader(markdown))
	var links []string
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if link, ok := node.(*ast.Link); ok {
			links = append(links, string(link.Destination))
		}
		return ast.WalkContinue, nil
	})
	return links
}
```

- [ ] **Step 6: Add the page schema**

`schemas/llm-wiki-page-v1.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://llm-wiki.dev/schemas/page-v1.json",
  "type": "object",
  "required": [
    "id",
    "kind",
    "status",
    "summary",
    "verification",
    "evidence"
  ],
  "properties": {
    "id": {
      "type": "string",
      "pattern": "^[a-z0-9]+(?:[._-][a-z0-9]+)*$"
    },
    "kind": {
      "enum": [
        "system",
        "component",
        "flow",
        "contract",
        "decision",
        "quality",
        "operation",
        "glossary",
        "health",
        "index",
        "log"
      ]
    },
    "status": {
      "enum": [
        "current",
        "deprecated",
        "superseded",
        "planned"
      ]
    },
    "summary": {
      "type": "string",
      "minLength": 1,
      "maxLength": 300
    },
    "verification": {
      "type": "object",
      "required": [
        "base_commit",
        "evidence_fingerprint"
      ],
      "properties": {
        "base_commit": {
          "type": "string",
          "minLength": 1
        },
        "evidence_fingerprint": {
          "type": "string",
          "pattern": "^(?:sha256:[a-f0-9]{64}|uninitialized)$"
        }
      },
      "additionalProperties": false
    },
    "evidence": {
      "type": "array",
      "items": {
        "type": "object",
        "required": [
          "path"
        ],
        "properties": {
          "path": {
            "type": "string",
            "minLength": 1
          },
          "symbol": {
            "type": "string",
            "minLength": 1
          }
        },
        "additionalProperties": false
      }
    },
    "relations": {
      "type": "array",
      "items": {
        "type": "string",
        "pattern": "^[a-z0-9]+(?:[._-][a-z0-9]+)*$"
      },
      "uniqueItems": true
    },
    "supersedes": {
      "type": "array",
      "items": {
        "type": "string",
        "pattern": "^[a-z0-9]+(?:[._-][a-z0-9]+)*$"
      },
      "uniqueItems": true
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 7: Run tests**

```bash
gofmt -w internal/wiki
go test ./internal/wiki -run TestParsePage -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/wiki schemas/llm-wiki-page-v1.schema.json
git commit -m "feat: parse llm-wiki pages"
```

## Task 8: Validate wiki structure, evidence, links, ADRs, and secrets

**Files:**

- Create: `internal/wiki/secrets.go`
- Create: `internal/wiki/validator.go`
- Modify: `internal/wiki/validator_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Create: `testdata/wiki/valid/`
- Create: `testdata/wiki/invalid-duplicate-id/`
- Create: `testdata/wiki/invalid-broken-link/`

- [ ] **Step 1: Add failing validator tests**

```go
func TestValidateValidWiki(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	report := Validate(Options{Root: root, WikiPath: "docs/llm-wiki", AllowUninitialized: false})
	if len(report.Errors) != 0 {
		t.Fatalf("errors = %#v", report.Errors)
	}
}

func TestValidateDuplicateID(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/invalid-duplicate-id")
	report := Validate(Options{Root: root, WikiPath: "docs/llm-wiki"})
	if !report.ContainsCode("duplicate-id") {
		t.Fatalf("errors = %#v, want duplicate-id", report.Errors)
	}
}
```

Implement `copyFixture` in the test using `os.ReadDir`, `os.MkdirAll`, and `os.WriteFile`; never mutate files under `testdata`.

Add focused tests named:

```text
TestValidateMissingRequiredFile
TestValidateUnknownFrontmatterField
TestValidateInvalidKindAndStatus
TestValidateBrokenLinkAndOrphanPage
TestValidateStaleEvidenceFingerprint
TestValidateUnsafeEvidencePathAndSymlink
TestValidateSymlinkedWikiRootAndPage
TestValidateSupersessionCycleAndStatuses
TestValidateInvalidLogHeading
TestValidateIndexEntryLimit
TestValidateLikelySecret
```

Each test mutates a copied valid fixture, asserts its exact issue code, and leaves all unrelated checks valid so failures identify one contract at a time.

- [ ] **Step 2: Run the tests and verify they fail**

```bash
go test ./internal/wiki -run TestValidate -v
```

Expected: FAIL because `Validate` does not exist.

- [ ] **Step 3: Implement secret-pattern scanning**

`internal/wiki/secrets.go`:

```go
package wiki

import "regexp"

var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "private-key", re: regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`)},
	{name: "github-token", re: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{30,}\b`)},
	{name: "aws-access-key", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{name: "openai-api-key", re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)},
	{name: "slack-token", re: regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{20,}\b`)},
}

func SecretMatches(data []byte) []string {
	var matches []string
	for _, pattern := range secretPatterns {
		if pattern.re.Match(data) {
			matches = append(matches, pattern.name)
		}
	}
	return matches
}
```

- [ ] **Step 4: Implement validator result types and checks**

`internal/wiki/validator.go`:

```go
package wiki

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/fingerprint"
)

type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type Report struct {
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
}

func (r Report) ContainsCode(code string) bool {
	for _, issue := range r.Errors {
		if issue.Code == code {
			return true
		}
	}
	return false
}

type Options struct {
	Root               string
	WikiPath           string
	AllowUninitialized bool
	IndexEntryLimit    int
}

func Validate(options Options) Report {
	var report Report
	if options.IndexEntryLimit == 0 {
		options.IndexEntryLimit = 200
	}
	wikiRoot := filepath.Join(options.Root, options.WikiPath)
	pages := map[string]Page{}
	ids := map[string]string{}
	var pagePaths []string
	if err := validateWikiDirectoryChain(options.Root, options.WikiPath); err != nil {
		report.Errors = append(report.Errors, Issue{
			Code: "unsafe-wiki-root", Path: options.WikiPath, Message: err.Error(),
		})
		return report
	}
	validateRequiredFiles(&report, wikiRoot)
	if data, err := os.ReadFile(filepath.Join(options.Root, "llm-wiki.yaml")); err == nil {
		if matches := SecretMatches(data); len(matches) > 0 {
			report.Errors = append(report.Errors, Issue{
				Code: "likely-secret", Path: "llm-wiki.yaml", Message: strings.Join(matches, ", "),
			})
		}
	}

	_ = filepath.WalkDir(wikiRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			report.Errors = append(report.Errors, Issue{Code: "walk", Path: path, Message: walkErr.Error()})
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			report.Errors = append(report.Errors, Issue{Code: "stat", Path: path, Message: err.Error()})
			return nil
		}
		if !info.Mode().IsRegular() {
			report.Errors = append(report.Errors, Issue{Code: "unsafe-wiki-file", Path: path})
			return nil
		}
		relative, _ := filepath.Rel(wikiRoot, path)
		relative = filepath.ToSlash(relative)
		data, err := os.ReadFile(path)
		if err != nil {
			report.Errors = append(report.Errors, Issue{Code: "read", Path: relative, Message: err.Error()})
			return nil
		}
		if matches := SecretMatches(data); len(matches) > 0 {
			report.Errors = append(report.Errors, Issue{Code: "likely-secret", Path: relative, Message: strings.Join(matches, ", ")})
		}
		if strings.HasPrefix(entry.Name(), "_") {
			return nil
		}
		page, err := ParsePage(relative, data)
		if err != nil {
			report.Errors = append(report.Errors, Issue{Code: "frontmatter", Path: relative, Message: err.Error()})
			return nil
		}
		if previous, exists := ids[page.ID]; exists {
			report.Errors = append(report.Errors, Issue{Code: "duplicate-id", Path: relative, Message: fmt.Sprintf("also used by %s", previous)})
		}
		ids[page.ID] = relative
		pages[relative] = page
		pagePaths = append(pagePaths, relative)
		return nil
	})

	sort.Strings(pagePaths)
	for _, relative := range pagePaths {
		page := pages[relative]
		validatePageMetadata(&report, page, options.AllowUninitialized)
		for _, evidence := range page.Evidence {
			clean := filepath.Clean(filepath.FromSlash(evidence.Path))
			if evidence.Path == "" || filepath.IsAbs(clean) || clean == ".." ||
				strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				report.Errors = append(report.Errors, Issue{Code: "unsafe-evidence-path", Path: relative, Message: evidence.Path})
				continue
			}
			info, err := os.Lstat(filepath.Join(options.Root, clean))
			if err != nil {
				report.Errors = append(report.Errors, Issue{Code: "missing-evidence", Path: relative, Message: evidence.Path})
			} else if !info.Mode().IsRegular() {
				report.Errors = append(report.Errors, Issue{Code: "unsupported-evidence-type", Path: relative, Message: evidence.Path})
			}
		}
		for _, link := range page.Links {
			linkPath, local := localLinkPath(link)
			if !local {
				continue
			}
			target := filepath.ToSlash(filepath.Clean(filepath.Join(
				filepath.Dir(relative),
				filepath.FromSlash(linkPath),
			)))
			if _, ok := pages[target]; !ok {
				report.Errors = append(report.Errors, Issue{Code: "broken-link", Path: relative, Message: link})
			}
		}
		for _, supersededID := range page.Supersedes {
			if page.Status != "current" {
				report.Errors = append(report.Errors, Issue{Code: "superseding-status-mismatch", Path: relative, Message: page.Status})
			}
			supersededPath, ok := ids[supersededID]
			if !ok {
				report.Errors = append(report.Errors, Issue{Code: "missing-superseded-id", Path: relative, Message: supersededID})
			} else if page.Kind != "decision" || pages[supersededPath].Kind != "decision" {
				report.Errors = append(report.Errors, Issue{Code: "invalid-supersession-kind", Path: relative, Message: supersededID})
			} else if pages[supersededPath].Status != "superseded" {
				report.Errors = append(report.Errors, Issue{Code: "superseded-status-mismatch", Path: supersededPath, Message: page.ID})
			}
		}
		for _, relatedID := range page.Relations {
			if _, ok := ids[relatedID]; !ok {
				report.Errors = append(report.Errors, Issue{Code: "missing-relation-id", Path: relative, Message: relatedID})
			}
		}
		validateEvidenceFingerprint(&report, options.Root, page)
		if page.Kind == "log" {
			validateLogHeadings(&report, page)
		}
	}
	validateIndexCoverage(&report, pages, pagePaths, options.IndexEntryLimit)
	validateSupersessionCycles(&report, pages, ids)
	return report
}

var validKinds = map[string]struct{}{
	"system": {}, "component": {}, "flow": {}, "contract": {}, "decision": {},
	"quality": {}, "operation": {}, "glossary": {}, "health": {}, "index": {}, "log": {},
}

var validStatuses = map[string]struct{}{
	"current": {}, "deprecated": {}, "superseded": {}, "planned": {},
}

var stableID = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)
var evidenceFingerprint = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func validatePageMetadata(report *Report, page Page, allowUninitialized bool) {
	required := []struct {
		value string
		code  string
	}{
		{page.ID, "missing-id"},
		{page.Kind, "missing-kind"},
		{page.Status, "missing-status"},
		{page.Summary, "missing-summary"},
		{page.Verification.BaseCommit, "missing-base-commit"},
		{page.Verification.EvidenceFingerprint, "missing-evidence-fingerprint"},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			report.Errors = append(report.Errors, Issue{Code: field.code, Path: page.Path})
		}
	}
	if page.Evidence == nil {
		report.Errors = append(report.Errors, Issue{Code: "missing-evidence-list", Path: page.Path})
	}
	if page.ID != "" && !stableID.MatchString(page.ID) {
		report.Errors = append(report.Errors, Issue{Code: "invalid-id", Path: page.Path, Message: page.ID})
	}
	if len(page.Summary) > 300 {
		report.Errors = append(report.Errors, Issue{Code: "summary-too-long", Path: page.Path})
	}
	if _, ok := validKinds[page.Kind]; page.Kind != "" && !ok {
		report.Errors = append(report.Errors, Issue{Code: "invalid-kind", Path: page.Path, Message: page.Kind})
	}
	if _, ok := validStatuses[page.Status]; page.Status != "" && !ok {
		report.Errors = append(report.Errors, Issue{Code: "invalid-status", Path: page.Path, Message: page.Status})
	}
	if page.Verification.EvidenceFingerprint == "uninitialized" {
		issue := Issue{Code: "uninitialized-evidence", Path: page.Path}
		if allowUninitialized {
			report.Warnings = append(report.Warnings, issue)
		} else {
			report.Errors = append(report.Errors, issue)
		}
	} else if page.Verification.EvidenceFingerprint != "" &&
		!evidenceFingerprint.MatchString(page.Verification.EvidenceFingerprint) {
		report.Errors = append(report.Errors, Issue{
			Code:    "invalid-evidence-fingerprint",
			Path:    page.Path,
			Message: page.Verification.EvidenceFingerprint,
		})
	}
	for _, id := range append(append([]string(nil), page.Relations...), page.Supersedes...) {
		if !stableID.MatchString(id) {
			report.Errors = append(report.Errors, Issue{Code: "invalid-related-id", Path: page.Path, Message: id})
		}
	}
}

func validateEvidenceFingerprint(report *Report, root string, page Page) {
	if page.Verification.EvidenceFingerprint == "" ||
		page.Verification.EvidenceFingerprint == "uninitialized" ||
		len(page.Evidence) == 0 {
		return
	}
	paths := make([]string, 0, len(page.Evidence))
	for _, item := range page.Evidence {
		clean := filepath.Clean(filepath.FromSlash(item.Path))
		if item.Path == "" || filepath.IsAbs(clean) || clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return
		}
		info, err := os.Lstat(filepath.Join(root, clean))
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		paths = append(paths, item.Path)
	}
	got, err := fingerprint.Files(root, paths)
	if err != nil {
		report.Errors = append(report.Errors, Issue{Code: "fingerprint-error", Path: page.Path, Message: err.Error()})
		return
	}
	if got != page.Verification.EvidenceFingerprint {
		report.Errors = append(report.Errors, Issue{
			Code:    "stale-evidence",
			Path:    page.Path,
			Message: fmt.Sprintf("got %s, want %s", got, page.Verification.EvidenceFingerprint),
		})
	}
}

func validateWikiDirectoryChain(root, wikiPath string) error {
	current := root
	for _, segment := range strings.Split(filepath.ToSlash(wikiPath), "/") {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe path component: %s", current)
		}
	}
	return nil
}

func validateRequiredFiles(report *Report, wikiRoot string) {
	required := []string{
		"index.md",
		"system.md",
		"schema.md",
		"glossary.md",
		"health.md",
		"log.md",
		"quality/invariants.md",
		"quality/testing.md",
		"quality/failure-modes.md",
	}
	for _, relative := range required {
		info, err := os.Lstat(filepath.Join(wikiRoot, filepath.FromSlash(relative)))
		if err != nil || !info.Mode().IsRegular() {
			report.Errors = append(report.Errors, Issue{Code: "missing-required-file", Path: relative})
		}
	}
}

var logHeading = regexp.MustCompile(
	`^## \[\d{4}-\d{2}-\d{2}\] (init|sync|audit|migrate) \| .+$`,
)

func validateLogHeadings(report *Report, page Page) {
	for _, line := range strings.Split(string(page.Body), "\n") {
		if strings.HasPrefix(line, "## ") && !logHeading.MatchString(line) {
			report.Errors = append(report.Errors, Issue{
				Code:    "invalid-log-heading",
				Path:    page.Path,
				Message: line,
			})
		}
	}
}

func validateIndexCoverage(
	report *Report,
	pages map[string]Page,
	pagePaths []string,
	entryLimit int,
) {
	linked := map[string]struct{}{}
	for _, relative := range pagePaths {
		page := pages[relative]
		if page.Kind != "index" {
			continue
		}
		if len(page.Links) > entryLimit {
			report.Errors = append(report.Errors, Issue{
				Code:    "index-entry-limit",
				Path:    relative,
				Message: fmt.Sprintf("%d links exceeds %d", len(page.Links), entryLimit),
			})
		}
		for _, link := range page.Links {
			linkPath, local := localLinkPath(link)
			if !local {
				continue
			}
			target := filepath.ToSlash(filepath.Clean(filepath.Join(
				filepath.Dir(relative),
				filepath.FromSlash(linkPath),
			)))
			linked[target] = struct{}{}
		}
	}
	for _, relative := range pagePaths {
		switch pages[relative].Kind {
		case "index", "log":
			continue
		}
		if _, ok := linked[filepath.ToSlash(relative)]; !ok {
			report.Errors = append(report.Errors, Issue{Code: "orphan-page", Path: relative})
		}
	}
}

func localLinkPath(link string) (string, bool) {
	if strings.HasPrefix(link, "#") || strings.HasPrefix(link, "//") {
		return "", false
	}
	parsed, err := url.Parse(link)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Path == "" {
		return "", false
	}
	path, err := url.PathUnescape(parsed.Path)
	return path, err == nil
}

func validateSupersessionCycles(report *Report, pages map[string]Page, ids map[string]string) {
	byID := make(map[string]Page, len(pages))
	var orderedIDs []string
	for _, page := range pages {
		byID[page.ID] = page
		orderedIDs = append(orderedIDs, page.ID)
	}
	sort.Strings(orderedIDs)
	state := map[string]uint8{}
	var visit func(string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}
		state[id] = 1
		for _, older := range byID[id].Supersedes {
			if _, exists := ids[older]; exists && visit(older) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, id := range orderedIDs {
		if visit(id) {
			report.Errors = append(report.Errors, Issue{
				Code:    "supersession-cycle",
				Path:    ids[id],
				Message: id,
			})
			return
		}
	}
}
```

- [ ] **Step 5: Wire the `validate` CLI command**

Add a `runValidate` function to `internal/cli/run.go` that:

1. Parses `--root` and `--allow-uninitialized` with `flag.NewFlagSet`.
2. Discovers the Git root.
3. Loads `llm-wiki.yaml`.
4. Calls `wiki.Validate`.
5. Prints indented JSON to stdout.
6. Returns `4` when `report.Errors` is non-empty.

Add tests that assert valid returns `0` and invalid returns `4`.

In the same task, route:

```text
llm-wiki fingerprint --root PATH --page WIKI_PAGE [--json]
```

`WIKI_PAGE` is repository-relative, must resolve inside the configured wiki path, and must be a regular non-symlink Markdown file. The command parses the page's current evidence list, rejects unsafe or non-regular evidence, calls `fingerprint.Files`, reads `repo.Head()`, and prints either:

```yaml
base_commit: 0123456789abcdef
evidence_fingerprint: sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

or the equivalent JSON object. It never edits the page. Add tests for a valid page, an escaping page path, a missing evidence path, and deterministic JSON output.

- [ ] **Step 6: Run focused and full tests**

```bash
gofmt -w internal/wiki internal/cli
go test ./internal/wiki ./internal/cli -v
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/wiki internal/cli testdata/wiki
git commit -m "feat: validate llm-wiki structure"
```

## Task 9: Add deterministic status output and the Milestone 1 gate

**Files:**

- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `Makefile`

- [ ] **Step 1: Write a failing JSON-status test**

```go
func TestRunStatusJSON(t *testing.T) {
	root := copyRepoFixture(t, "../../testdata/wiki/valid")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status", "--root", root, "--json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var got struct {
		Initialized bool   `json:"initialized"`
		Schema      int    `json:"schema"`
		WikiPath    string `json:"wiki_path"`
		LeaseActive bool   `json:"sync_lease_active"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Initialized || got.Schema != 1 || got.WikiPath != "docs/llm-wiki" || got.LeaseActive {
		t.Fatalf("unexpected status: %#v", got)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/cli -run TestRunStatusJSON -v
```

Expected: FAIL because `status` is not routed.

- [ ] **Step 3: Implement status routing**

Add:

```go
type Status struct {
	Initialized    bool   `json:"initialized"`
	Schema         int    `json:"schema"`
	WikiPath       string `json:"wiki_path"`
	HealthItems    int    `json:"health_items"`
	ValidationErrors int  `json:"validation_errors"`
	StartupAudit   bool   `json:"startup_audit"`
	ContextBudget int    `json:"context_budget_bytes"`
	LeaseActive    bool   `json:"sync_lease_active"`
	LeaseOwner     string `json:"sync_lease_owner,omitempty"`
	LastReceipt    string `json:"last_receipt_kind,omitempty"`
}
```

`runStatus` loads config, counts unresolved `health.md` headings matching `^## `, checks page fingerprints through the validator, resolves the Git worktree ID, reads `layout.LatestSession(worktreeID)` and `layout.LatestReceipt()`, calls `lock.CurrentOwner` on `layout.LockPath(worktreeID)`, and prints either JSON or these text lines:

```text
LLM Wiki: ready
Schema: 1
Health items: 0
Validation errors: 0
Startup audit: no
Sync lease: inactive
Last receipt: none
```

Keep JSON field names exactly as declared above.

`status` remains a diagnostic command and returns `0` after successfully loading the repository even when `validation_errors` is nonzero; `validate` remains the enforcing command that returns `4`.

- [ ] **Step 4: Update `Makefile`**

```make
.PHONY: fmt test vet build verify

fmt:
	test -z "$$(gofmt -l cmd internal)"

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/llm-wiki

verify: fmt test vet build
```

- [ ] **Step 5: Run the complete milestone gate**

```bash
make verify
```

Expected:

```text
No gofmt output.
All tests pass.
Go vet reports no findings.
The llm-wiki binary builds.
```

- [ ] **Step 6: Commit**

```bash
git add internal/cli Makefile
git commit -m "feat: report llm-wiki status"
```

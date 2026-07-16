# LLM Wiki Milestone 2 Brick and Initialization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the Mason Brick and an idempotent repository initializer that installs Mason when possible, generates the wiki skeleton, and safely integrates `AGENTS.md`, `CLAUDE.md`, `.gitignore`, and `llm-wiki.yaml`.

**Architecture:** Keep file generation in the Brick and orchestration in the Go helper. The Go initializer owns prerequisite detection and safe merges; the Brick pre-generation hook validates variables but does not mutate files outside Mason's output.

**Tech Stack:** Go 1.26.5, Dart 3.12.2, Mason CLI 0.1.3, Mason package 0.1.2, YAML, Git.

---

## Task 1: Scaffold and validate the Mason Brick

**Files:**

- Create: `bricks/software-wiki/brick.yaml`
- Create: `bricks/software-wiki/README.md`
- Create: `bricks/software-wiki/CHANGELOG.md`
- Create: `bricks/software-wiki/LICENSE`
- Create: `bricks/software-wiki/hooks/pubspec.yaml`
- Create: `bricks/software-wiki/hooks/pre_gen.dart`

- [ ] **Step 1: Create the Brick manifest**

`bricks/software-wiki/brick.yaml`:

```yaml
name: llm_wiki
description: Initialize a team-shared LLM-maintained software-development wiki.
repository: https://github.com/merdandt/LLM-wiki-dev
version: 0.1.0+1

environment:
  mason: ">=0.1.0 <0.2.0"

vars:
  project_name:
    type: string
    description: Human-readable project name.
    default: Software Project
    prompt: What is the project name?
  wiki_path:
    type: string
    description: Repository-relative canonical wiki path.
    default: docs/llm-wiki
    prompt: Where should the wiki be created?
  context_budget_kib:
    type: number
    description: Maximum task recall packet size in KiB.
    default: 12
    prompt: What recall context budget should be used?
  include_codex:
    type: boolean
    description: Add the AGENTS.md integration.
    default: true
    prompt: Configure Codex?
  include_claude:
    type: boolean
    description: Add the CLAUDE.md integration.
    default: true
    prompt: Configure Claude Code?
```

- [ ] **Step 2: Create Brick metadata files**

`bricks/software-wiki/README.md`:

```markdown
# LLM Wiki Brick

Generates the canonical Markdown skeleton and `llm-wiki.yaml` used by the LLM Wiki plugin.

Use the plugin's `wiki-init` workflow for complete initialization, instruction merging, baseline compilation, and validation.
```

`bricks/software-wiki/CHANGELOG.md`:

```markdown
# Changelog

## 0.1.0+1

- Initial software-development wiki skeleton.
```

`bricks/software-wiki/LICENSE`:

```text
MIT License

Copyright (c) 2026 LLM Wiki contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 3: Add a pre-generation hook that rejects unsafe paths**

`bricks/software-wiki/hooks/pubspec.yaml`:

```yaml
name: llm_wiki_hooks
environment:
  sdk: ">=3.5.0 <4.0.0"
dependencies:
  mason: ^0.1.2
```

`bricks/software-wiki/hooks/pre_gen.dart`:

```dart
import 'package:mason/mason.dart';

void run(HookContext context) {
  final rawPath = (context.vars['wiki_path'] as String).trim();
  final segments = rawPath.replaceAll('\\', '/').split('/');
  final normalized =
      segments.where((segment) => segment != '.').toList(growable: false);
  final unsafe = rawPath.isEmpty ||
      rawPath.startsWith('/') ||
      RegExp(r'^[A-Za-z]:').hasMatch(rawPath) ||
      segments.any((segment) => segment.isEmpty || segment == '..') ||
      normalized.isEmpty ||
      normalized.first == '.git' ||
      normalized.first == '.llm-wiki-state' ||
      !RegExp(r'^[A-Za-z0-9._/-]+$').hasMatch(normalized.join('/'));

  if (unsafe) {
    throw StateError(
      'wiki_path must be a non-empty repository-relative path without "..".',
    );
  }

  final budget = context.vars['context_budget_kib'] as num;
  if (budget < 1 || budget > 128) {
    throw StateError('context_budget_kib must be between 1 and 128.');
  }

  context.vars = {
    ...context.vars,
    'wiki_path': normalized.join('/'),
    'context_budget_bytes': budget.toInt() * 1024,
  };
}
```

- [ ] **Step 4: Bundle the empty Brick and verify metadata**

Run:

```bash
bundle_dir="$(mktemp -d)"
trap 'rm -rf "$bundle_dir"' EXIT
mason bundle bricks/software-wiki -o "$bundle_dir"
```

Expected: command succeeds and reports a bundled `llm_wiki` Brick.

- [ ] **Step 5: Commit**

```bash
git add bricks/software-wiki
git commit -m "feat: scaffold llm-wiki Mason Brick"
```

## Task 2: Add the complete generated wiki skeleton

**Files:**

- Create: `bricks/software-wiki/__brick__/llm-wiki.yaml`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/index.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/system.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/schema.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/glossary.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/health.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/log.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/quality/invariants.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/quality/testing.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/quality/failure-modes.md`
- Create: `bricks/software-wiki/__brick__/{{wiki_path}}/decisions/_template.md`
- Create: `.gitkeep` files in empty generated directories.

- [ ] **Step 1: Create the generated configuration**

`bricks/software-wiki/__brick__/llm-wiki.yaml`:

```yaml
schema_version: 1
initialized: false
wiki_path: "{{wiki_path}}"
state_path: .llm-wiki-state
context_budget_bytes: {{context_budget_bytes}}
index_entry_limit: 200
lock_wait_seconds: 5
sync_lease_seconds: 600
maintenance:
  max_recovery_passes: 1
```

- [ ] **Step 2: Create navigation and system pages**

`index.md`:

```markdown
---
id: index.root
kind: index
status: planned
summary: Navigation index for {{project_name}} project knowledge.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence: []
relations: []
---
# {{project_name}} LLM Wiki

The initial repository synthesis is pending.

## System

- [System overview](system.md)
- [Wiki schema](schema.md)
- [Glossary](glossary.md)

## Components

No component pages have been compiled.

## Flows

No flow pages have been compiled.

## Contracts

No contract pages have been compiled.

## Decisions

No decisions have been compiled.

## Quality

- [Invariants](quality/invariants.md)
- [Testing strategy](quality/testing.md)
- [Failure modes](quality/failure-modes.md)

## Maintenance

- [Wiki health](health.md)
- [Maintenance log](log.md)
```

`system.md`:

```markdown
---
id: system.overview
kind: system
status: planned
summary: Initial system synthesis for {{project_name}} is pending.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence: []
relations: []
---
# System

The `wiki-init` workflow replaces this scaffold with an evidence-backed system overview.
```

- [ ] **Step 3: Create the schema page**

`schema.md`:

```markdown
---
id: system.wiki-schema
kind: system
status: current
summary: Schema and maintenance rules for the project wiki.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence:
  - path: llm-wiki.yaml
relations: []
---
# Wiki Schema

## Authority

1. Code, tests, schemas, migrations, and configuration describe current behavior.
2. Approved specifications and ADRs describe intended behavior and rationale.
3. Wiki pages are derived synthesis.
4. Conversations and failed hypotheses are not evidence.

## Required page metadata

- `id`
- `kind`
- `status`
- `summary`
- `verification.base_commit`
- `verification.evidence_fingerprint`
- `evidence`

## Material updates

Update the wiki for behavior, boundaries, dependencies, contracts, invariants, operations, architectural decisions, and confirmed reusable failure modes.

Do not record formatting, transient debugging, failed hypotheses, or unchanged implementation details.
```

- [ ] **Step 4: Create quality and maintenance pages**

`glossary.md`:

```markdown
---
id: glossary.root
kind: glossary
status: planned
summary: Project terminology compiled from repository evidence.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence: []
relations: []
---
# Glossary

The initial terminology synthesis is pending.
```

`health.md`:

```markdown
---
id: health.current
kind: health
status: planned
summary: Unresolved wiki contradictions, stale evidence, and knowledge gaps.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence: []
relations: []
---
# Wiki Health

## Initial synthesis pending

Run the `wiki-init` workflow to compile repository knowledge and clear this item.
```

`log.md`:

```markdown
---
id: log.maintenance
kind: log
status: current
summary: Append-only LLM Wiki maintenance history.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence:
  - path: llm-wiki.yaml
relations: []
---
# Maintenance Log

## [{{current_date}}] init | Wiki scaffold generated
```

`quality/invariants.md`:

```markdown
---
id: quality.invariants
kind: quality
status: planned
summary: System-wide correctness, security, and reliability invariants.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence: []
relations: []
---
# Invariants

The initial invariant synthesis is pending.
```

`quality/testing.md`:

```markdown
---
id: quality.testing
kind: quality
status: planned
summary: Project test strategy and verification boundaries.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence: []
relations: []
---
# Testing

The initial testing synthesis is pending.
```

`quality/failure-modes.md`:

```markdown
---
id: quality.failure-modes
kind: quality
status: planned
summary: Confirmed reusable project failure modes and regression evidence.
verification:
  base_commit: uninitialized
  evidence_fingerprint: uninitialized
evidence: []
relations: []
---
# Failure Modes

No confirmed reusable failure modes have been compiled.
```

- [ ] **Step 5: Create the ADR template and empty directories**

`decisions/_template.md`:

```markdown
---
id: decision.NNNN
kind: decision
status: planned
summary: Replace with a one-sentence decision summary.
verification:
  base_commit: replace-with-base-commit
  evidence_fingerprint: replace-with-evidence-fingerprint
evidence: []
relations: []
supersedes: []
---
# NNNN: Decision title

## Context

Describe the evidence-backed forces that require a decision.

## Decision

State the selected direction.

## Consequences

List positive, negative, and migration consequences.
```

Create empty marker files:

```text
components/.gitkeep
flows/.gitkeep
contracts/.gitkeep
operations/.gitkeep
```

- [ ] **Step 6: Add `current_date` to the Mason hook**

Add:

```dart
final now = DateTime.now().toUtc();
final currentDate =
    '${now.year.toString().padLeft(4, '0')}-'
    '${now.month.toString().padLeft(2, '0')}-'
    '${now.day.toString().padLeft(2, '0')}';
```

Include `'current_date': currentDate` in `context.vars`.

- [ ] **Step 7: Generate a golden fixture**

Run:

```bash
golden_dir="$(mktemp -d)"
trap 'rm -rf "$golden_dir"' EXIT
mason make llm_wiki \
  --project_name Example \
  --wiki_path docs/llm-wiki \
  --context_budget_kib 12 \
  --include_codex true \
  --include_claude true \
  -o "$golden_dir"
```

Expected: configuration and the complete wiki skeleton are generated under the temporary directory.

- [ ] **Step 8: Commit**

```bash
git add bricks/software-wiki
git commit -m "feat: add software wiki template"
```

## Task 3: Detect and install Mason deterministically

**Files:**

- Create: `internal/mason/runner.go`
- Create: `internal/mason/installer.go`
- Test: `internal/mason/mason_test.go`

- [ ] **Step 1: Write failing detection tests with a fake runner**

```go
package mason

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	results map[string][]Result
	calls   []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) Result {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if results := f.results[key]; len(results) > 0 {
		f.results[key] = results[1:]
		return results[0]
	}
	return Result{Err: errors.New("not found")}
}

func TestEnsureUsesCompatibleExistingMason(t *testing.T) {
	runner := &fakeRunner{results: map[string][]Result{
		"mason --version": {{Stdout: "mason_cli 0.1.3"}},
	}}
	got, err := Ensure(context.Background(), runner, "0.1.3")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "0.1.3" || got.Command != "mason" || len(runner.calls) != 1 {
		t.Fatalf("unexpected result=%#v calls=%#v", got, runner.calls)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/mason -v
```

Expected: FAIL because `Runner`, `Result`, and `Ensure` do not exist.

- [ ] **Step 3: Implement the runner**

`internal/mason/runner.go`:

```go
package mason

import (
	"bytes"
	"context"
	"os/exec"
)

type Result struct {
	Stdout string
	Stderr string
	Err    error
}

type Runner interface {
	Run(context.Context, string, ...string) Result
}

type ExecRunner struct {
	Dir string
}

func (r ExecRunner) Run(ctx context.Context, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), Err: err}
}
```

- [ ] **Step 4: Implement installer fallback order**

`internal/mason/installer.go`:

```go
package mason

import (
	"context"
	"errors"
	"regexp"
	"runtime"
)

type Installation struct {
	Version string
	Method  string
	Command string
	Prefix  []string
}

var versionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

func Ensure(ctx context.Context, runner Runner, want string) (Installation, error) {
	return ensureForOS(ctx, runner, want, runtime.GOOS)
}

func ensureForOS(ctx context.Context, runner Runner, want, goos string) (Installation, error) {
	if version := detectVersion(runner.Run(ctx, "mason", "--version")); version == want {
		return Installation{Version: version, Method: "existing", Command: "mason"}, nil
	}
	if runner.Run(ctx, "dart", "--version").Err == nil {
		result := runner.Run(ctx, "dart", "pub", "global", "activate", "mason_cli", want)
		if result.Err == nil {
			invocation := Installation{
				Method:  "dart-pub",
				Command: "dart",
				Prefix:  []string{"pub", "global", "run", "mason_cli:mason"},
			}
			if version := detectVersion(invocation.Run(ctx, runner, "--version")); version == want {
				invocation.Version = version
				return invocation, nil
			}
		}
	}
	if goos != "windows" && runner.Run(ctx, "brew", "--version").Err == nil {
		if runner.Run(ctx, "brew", "tap", "felangel/mason").Err != nil {
			return Installation{}, errors.New("failed to add Mason Homebrew tap")
		}
		if runner.Run(ctx, "brew", "install", "mason").Err != nil {
			return Installation{}, errors.New("failed to install Mason with Homebrew")
		}
		if version := detectVersion(runner.Run(ctx, "mason", "--version")); version == want {
			return Installation{Version: version, Method: "homebrew", Command: "mason"}, nil
		}
	}
	return Installation{}, errors.New("compatible Mason CLI unavailable; install Dart SDK 3.5 or newer")
}

func (i Installation) Run(ctx context.Context, runner Runner, args ...string) Result {
	commandArgs := append([]string(nil), i.Prefix...)
	commandArgs = append(commandArgs, args...)
	return runner.Run(ctx, i.Command, commandArgs...)
}

func detectVersion(result Result) string {
	if result.Err != nil {
		return ""
	}
	match := versionPattern.FindStringSubmatch(result.Stdout + "\n" + result.Stderr)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}
```

- [ ] **Step 5: Add tests for Dart, Homebrew, and missing-prerequisite paths**

Add these table cases to `internal/mason/mason_test.go`:

```go
func TestEnsureFallbackOrder(t *testing.T) {
	notFound := Result{Err: errors.New("not found")}
	tests := []struct {
		name        string
		goos        string
		results     map[string][]Result
		wantMethod  string
		wantCalls   []string
		wantErr     bool
	}{
		{
			name: "dart pub",
			goos: "linux",
			results: map[string][]Result{
				"mason --version": {notFound},
				"dart --version": {{Stderr: "Dart SDK version: 3.12.2"}},
				"dart pub global activate mason_cli 0.1.3": {{Stdout: "Activated mason_cli 0.1.3"}},
				"dart pub global run mason_cli:mason --version": {{Stdout: "mason_cli 0.1.3"}},
			},
			wantMethod: "dart-pub",
			wantCalls: []string{
				"mason --version",
				"dart --version",
				"dart pub global activate mason_cli 0.1.3",
				"dart pub global run mason_cli:mason --version",
			},
		},
		{
			name: "homebrew",
			goos: "darwin",
			results: map[string][]Result{
				"mason --version": {
					notFound,
					{Stdout: "mason_cli 0.1.3"},
				},
				"dart --version":           {notFound},
				"brew --version":           {{Stdout: "Homebrew 5.0.0"}},
				"brew tap felangel/mason":  {{Stdout: "ok"}},
				"brew install mason":       {{Stdout: "ok"}},
			},
			wantMethod: "homebrew",
			wantCalls: []string{
				"mason --version",
				"dart --version",
				"brew --version",
				"brew tap felangel/mason",
				"brew install mason",
				"mason --version",
			},
		},
		{
			name: "missing prerequisites",
			goos: "linux",
			results: map[string][]Result{
				"mason --version": {notFound},
				"dart --version":  {notFound},
				"brew --version":  {notFound},
			},
			wantCalls: []string{
				"mason --version",
				"dart --version",
				"brew --version",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{results: tt.results}
			got, err := ensureForOS(context.Background(), runner, "0.1.3", tt.goos)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr=%t", err, tt.wantErr)
			}
			if got.Method != tt.wantMethod {
				t.Fatalf("Method = %q, want %q", got.Method, tt.wantMethod)
			}
			if !reflect.DeepEqual(runner.calls, tt.wantCalls) {
				t.Fatalf("calls = %#v, want %#v", runner.calls, tt.wantCalls)
			}
		})
	}
}
```

Add `reflect` to the test imports. The existing-Mason test already asserts the `existing` method and a single detection call.

Add `TestEnsureFallsBackToHomebrewAfterDartActivationFailure`, with successful `dart --version`, failed activation, successful Homebrew installation, and exact call-order assertions.

- [ ] **Step 6: Run tests and commit**

```bash
gofmt -w internal/mason
go test ./internal/mason -v
git add internal/mason
git commit -m "feat: install compatible Mason CLI"
```

## Task 4: Merge instruction blocks and `.gitignore` idempotently

**Files:**

- Create: `internal/initrepo/instructions.go`
- Create: `internal/initrepo/gitignore.go`
- Test: `internal/initrepo/init_test.go`

- [ ] **Step 1: Write failing idempotency tests**

```go
package initrepo

import (
	"strings"
	"testing"
)

func TestMergeManagedBlockIsIdempotent(t *testing.T) {
	original := "# Existing\n\nKeep this.\n"
	first, err := MergeManagedBlock(original, AgentsBlock("docs/llm-wiki"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := MergeManagedBlock(first, AgentsBlock("docs/llm-wiki"))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("second merge changed content:\n%s", second)
	}
	if !strings.Contains(second, "Keep this.") {
		t.Fatal("existing content was lost")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/initrepo -run TestMergeManagedBlockIsIdempotent -v
```

Expected: FAIL because merge functions do not exist.

- [ ] **Step 3: Implement exact managed blocks**

`internal/initrepo/instructions.go`:

```go
package initrepo

import (
	"errors"
	"strings"
)

const startMarker = "<!-- llm-wiki:start -->"
const endMarker = "<!-- llm-wiki:end -->"

func AgentsBlock(wikiPath string) string {
	return "<!-- llm-wiki:start -->\n" +
		"## LLM Wiki\n\n" +
		"- Treat `" + wikiPath + "/` as derived project memory; code, tests, schemas, migrations, and configuration remain authoritative.\n" +
		"- Before planning a material change, use the installed `wiki-recall` workflow.\n" +
		"- After a verified material change, use the installed `wiki-sync` workflow before completion.\n" +
		"- Do not store secrets, failed hypotheses, or transient debugging notes.\n" +
		"- Ordinary LLM Wiki maintenance may modify only `" + wikiPath + "/**` and `.llm-wiki-state/**`.\n" +
		"<!-- llm-wiki:end -->"
}

const ClaudeBlock = `<!-- llm-wiki:start -->
@AGENTS.md
<!-- llm-wiki:end -->`

func MergeClaudeInstructions(original string) (string, error) {
	if !strings.Contains(original, startMarker) {
		for _, line := range strings.Split(original, "\n") {
			if strings.TrimSpace(line) == "@AGENTS.md" {
				return original, nil
			}
		}
	}
	return MergeManagedBlock(original, ClaudeBlock)
}

func MergeManagedBlock(original, block string) (string, error) {
	start := strings.Index(original, startMarker)
	end := strings.Index(original, endMarker)
	if (start >= 0) != (end >= 0) {
		return "", errors.New("incomplete llm-wiki managed block")
	}
	if start >= 0 {
		end += len(endMarker)
		return original[:start] + block + original[end:], nil
	}
	trimmed := strings.TrimRight(original, "\n")
	if trimmed == "" {
		return block + "\n", nil
	}
	return trimmed + "\n\n" + block + "\n", nil
}
```

- [ ] **Step 4: Implement `.gitignore` merging**

`internal/initrepo/gitignore.go`:

```go
package initrepo

import "strings"

func MergeGitignore(original string) string {
	for _, line := range strings.Split(original, "\n") {
		if strings.TrimSpace(line) == ".llm-wiki-state/" {
			return original
		}
	}
	trimmed := strings.TrimRight(original, "\n")
	if trimmed == "" {
		return ".llm-wiki-state/\n"
	}
	return trimmed + "\n.llm-wiki-state/\n"
}
```

- [ ] **Step 5: Add tests for missing, existing, and malformed blocks**

Assert:

- Existing content is preserved.
- Repeated merges are byte-identical.
- A pre-existing `@AGENTS.md` import is not duplicated.
- An unmatched marker returns an error without changing the file.
- `.llm-wiki-state/` appears exactly once.

- [ ] **Step 6: Run tests and commit**

```bash
gofmt -w internal/initrepo
go test ./internal/initrepo -v
git add internal/initrepo
git commit -m "feat: merge agent wiki instructions"
```

## Task 5: Implement the `init` command

**Files:**

- Create: `internal/initrepo/init.go`
- Modify: `internal/initrepo/init_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

- [ ] **Step 1: Write a failing orchestration test**

Create a fake Mason runner and assert that `Initialize`:

1. Calls `mason --version`.
2. Calls `mason add -g llm_wiki --path <brick-path>` for development fixtures, or `mason add -g llm_wiki 0.1.0+1` in release mode, so initialization never creates a project-local `mason.yaml`.
3. Calls `mason make llm_wiki --project_name Example --wiki_path docs/llm-wiki --context_budget_kib 12 --include_codex true --include_claude true --on-conflict skip -o <root>`.
4. Retries a failed BrickHub registration with `mason add -g llm_wiki --path <bundled-brick-path>` when a bundled path is available.
5. Merges all three instruction files.
6. Does not create `mason.yaml` in the target repository.
7. Rejects symlinked or non-regular `llm-wiki.yaml`, wiki path components, `AGENTS.md`, `CLAUDE.md`, and `.gitignore` paths without following them.
8. Produces no diff on a second run.

Use a temporary Git repository and fixture Brick path.

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/initrepo -run TestInitialize -v
```

Expected: FAIL because `Initialize` does not exist.

- [ ] **Step 3: Implement initialization orchestration**

`internal/initrepo/init.go`:

```go
package initrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/mason"
	"github.com/merdandt/LLM-wiki-dev/internal/wiki"
)

type Options struct {
	Root             string
	ProjectName      string
	WikiPath         string
	ContextBudgetKiB int
	IncludeCodex     bool
	IncludeClaude    bool
	BrickPath        string
	BundledBrickPath string
	BrickVersion     string
	MasonVersion     string
}

func Initialize(ctx context.Context, runner mason.Runner, options Options) error {
	installation, err := mason.Ensure(ctx, runner, options.MasonVersion)
	if err != nil {
		return err
	}
	addArgs := []string{"add", "-g", "llm_wiki", options.BrickVersion}
	if options.BrickPath != "" {
		addArgs = []string{"add", "-g", "llm_wiki", "--path", options.BrickPath}
	}
	result := installation.Run(ctx, runner, addArgs...)
	if result.Err != nil && options.BrickPath == "" && options.BundledBrickPath != "" {
		addArgs = []string{
			"add", "-g", "llm_wiki", "--path", options.BundledBrickPath,
		}
		result = installation.Run(ctx, runner, addArgs...)
	}
	if result.Err != nil {
		return fmt.Errorf("mason add: %s", result.Stderr)
	}
	if err := validateWikiDestination(options.Root, options.WikiPath); err != nil {
		return err
	}
	makeArgs := []string{
		"make", "llm_wiki",
		"--project_name", options.ProjectName,
		"--wiki_path", options.WikiPath,
		"--context_budget_kib", fmt.Sprint(options.ContextBudgetKiB),
		"--include_codex", fmt.Sprint(options.IncludeCodex),
		"--include_claude", fmt.Sprint(options.IncludeClaude),
		"--on-conflict", "skip",
		"-o", options.Root,
	}
	if result := installation.Run(ctx, runner, makeArgs...); result.Err != nil {
		return fmt.Errorf("mason make: %s", result.Stderr)
	}
	if options.IncludeCodex {
		if err := mergeFile(
			filepath.Join(options.Root, "AGENTS.md"),
			AgentsBlock(options.WikiPath),
		); err != nil {
			return err
		}
	}
	if options.IncludeClaude {
		if err := mergeClaudeFile(filepath.Join(options.Root, "CLAUDE.md")); err != nil {
			return err
		}
	}
	if err := mergeGitignoreFile(filepath.Join(options.Root, ".gitignore")); err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(options.Root, "llm-wiki.yaml"))
	if err != nil {
		return err
	}
	report := wiki.Validate(wiki.Options{
		Root:               options.Root,
		WikiPath:           cfg.WikiPath,
		AllowUninitialized: true,
		IndexEntryLimit:    cfg.IndexEntryLimit,
	})
	if len(report.Errors) > 0 {
		return fmt.Errorf("generated wiki is invalid: %v", report.Errors)
	}
	return nil
}

func mergeFile(path, block string) error {
	original, err := readManagedFile(path)
	if err != nil {
		return err
	}
	merged, err := MergeManagedBlock(string(original), block)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, []byte(merged), 0o644)
}

func mergeClaudeFile(path string) error {
	original, err := readManagedFile(path)
	if err != nil {
		return err
	}
	merged, err := MergeClaudeInstructions(string(original))
	if err != nil {
		return err
	}
	return atomicfile.Write(path, []byte(merged), 0o644)
}

func mergeGitignoreFile(path string) error {
	original, err := readManagedFile(path)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, []byte(MergeGitignore(string(original))), 0o644)
}

func readManagedFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("managed path is not a regular file: %s", path)
	}
	return os.ReadFile(path)
}

func validateWikiDestination(root, wikiPath string) error {
	configPath := filepath.Join(root, "llm-wiki.yaml")
	if info, err := os.Lstat(configPath); err == nil && !info.Mode().IsRegular() {
		return errors.New("llm-wiki.yaml is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
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
			return fmt.Errorf("wiki output component is unsafe: %s", current)
		}
	}
	return nil
}
```

- [ ] **Step 4: Route `llm-wiki init`**

Parse:

```text
--root
--project-name
--wiki-path
--context-budget-kib
--non-interactive
--brick-path
```

Defaults:

```text
root: current Git root
project-name: repository directory name
wiki-path: docs/llm-wiki
context-budget-kib: 12
brick-version: 0.1.0+1
mason-version: 0.1.3
include-codex: true
include-claude: true
bundled-brick-path: auto-detected from `bin/<target>/llm-wiki` when packaged
```

Add:

```go
func bundledBrickPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return ""
	}
	pluginRoot := filepath.Dir(filepath.Dir(filepath.Dir(executable)))
	candidate := filepath.Join(pluginRoot, "assets", "bricks", "software-wiki")
	if _, err := os.Stat(filepath.Join(candidate, "brick.yaml")); err != nil {
		return ""
	}
	return candidate
}
```

The CLI passes this value as `Options.BundledBrickPath`. An explicit `--brick-path` always wins; BrickHub is attempted next; the bundled source Brick is the offline fallback.

Before calling `Initialize`, `runInit` creates a temporary empty directory, defers its removal, and uses `mason.ExecRunner{Dir: temporaryDirectory}`. This ensures a target repository's existing `mason.yaml` cannot shadow the globally registered `llm_wiki` Brick and no Mason workspace file is written into the repository.

Return exit code `8` for missing external prerequisites and `4` for an invalid generated skeleton.

- [ ] **Step 5: Run focused tests**

```bash
gofmt -w internal/initrepo internal/cli
go test ./internal/initrepo ./internal/cli -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/initrepo internal/cli
git commit -m "feat: initialize repository wiki"
```

## Task 6: Finalize initial semantic compilation

**Files:**

- Modify: `internal/config/config.go`
- Create: `internal/initrepo/finalize.go`
- Test: `internal/initrepo/finalize_test.go`
- Modify: `internal/cli/run.go`

- [ ] **Step 1: Write a failing finalization test**

The test must:

- Create a scaffolded fixture with `initialized: false`.
- Replace all `uninitialized` page verification values with valid evidence fingerprints.
- Run `Finalize`.
- Assert `initialized: true`.
- Assert strict validation passes.

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/initrepo -run TestFinalize -v
```

Expected: FAIL because `Finalize` does not exist.

- [ ] **Step 3: Add atomic config saving**

Add `github.com/merdandt/LLM-wiki-dev/internal/atomicfile` to `internal/config/config.go`, then add:

```go
func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, data, 0o644)
}
```

- [ ] **Step 4: Implement finalization**

`internal/initrepo/finalize.go`:

```go
package initrepo

import (
	"errors"
	"path/filepath"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/wiki"
)

func Finalize(root string) error {
	configPath := filepath.Join(root, "llm-wiki.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	report := wiki.Validate(wiki.Options{
		Root:            root,
		WikiPath:        cfg.WikiPath,
		IndexEntryLimit: cfg.IndexEntryLimit,
	})
	if len(report.Errors) > 0 {
		return errors.New("cannot finalize an invalid wiki")
	}
	cfg.Initialized = true
	return config.Save(configPath, cfg)
}
```

- [ ] **Step 5: Route `finalize-init` and run tests**

```bash
gofmt -w internal/config internal/initrepo internal/cli
go test ./internal/config ./internal/initrepo ./internal/cli -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config internal/initrepo internal/cli
git commit -m "feat: finalize initial wiki compilation"
```

## Task 7: Add ordered schema migration infrastructure

**Files:**

- Create: `internal/migrate/migrate.go`
- Test: `internal/migrate/migrate_test.go`
- Modify: `internal/cli/run.go`

- [ ] **Step 1: Write failing tests for no-op and unsupported migrations**

```go
package migrate

import "testing"

func TestRunNoOpAtCurrentSchema(t *testing.T) {
	result, err := Run(t.TempDir(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 0 {
		t.Fatalf("Applied = %d, want 0", result.Applied)
	}
}

func TestRunRejectsDowngrade(t *testing.T) {
	if _, err := Run(t.TempDir(), 2, 1); err == nil {
		t.Fatal("Run() error = nil, want downgrade error")
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
go test ./internal/migrate -v
```

Expected: FAIL because `Run` does not exist.

- [ ] **Step 3: Implement the migration registry**

`internal/migrate/migrate.go`:

```go
package migrate

import (
	"errors"
	"fmt"
)

type Migration func(root string) error

type Result struct {
	From    int
	To      int
	Applied int
}

var registry = map[int]Migration{}

func Run(root string, from, to int) (Result, error) {
	if from > to {
		return Result{}, errors.New("schema downgrade is not supported")
	}
	result := Result{From: from, To: to}
	for version := from; version < to; version++ {
		migration, ok := registry[version]
		if !ok {
			return Result{}, fmt.Errorf("no migration from schema %d", version)
		}
		if err := migration(root); err != nil {
			return Result{}, err
		}
		result.Applied++
	}
	return result, nil
}
```

- [ ] **Step 4: Route `migrate`**

The CLI requires an active synchronization lease, loads the current config, targets the helper's supported schema, and writes a byte-for-byte backup of config plus the canonical wiki under `.llm-wiki-state/backups/<timestamp>/`. It runs ordered migrations, validates, and only then saves the new `schema_version`. If any migration or validation step fails, it restores every backed-up file with `atomicfile.Write`, removes newly created migration files, leaves the original schema version unchanged, and retains the backup for diagnosis.

Return exit code `7` for a missing or unsafe migration.

Add a failing migration fixture that changes one page and then returns an error; assert the repository files are byte-identical to their pre-migration state afterward.

- [ ] **Step 5: Run tests and commit**

```bash
gofmt -w internal/migrate internal/cli
go test ./internal/migrate ./internal/cli -v
git add internal/migrate internal/cli
git commit -m "feat: add wiki schema migrations"
```

## Task 8: Prove Brick generation and initialization idempotency

**Files:**

- Create: `testdata/repos/empty-git/README.md`
- Create: `testdata/repos/existing-instructions/AGENTS.md`
- Create: `testdata/repos/existing-instructions/CLAUDE.md`
- Create: `testdata/repos/existing-instructions/.gitignore`
- Create: `internal/initrepo/golden_test.go`
- Modify: `Makefile`

- [ ] **Step 1: Create fixture content**

`existing-instructions/AGENTS.md`:

```markdown
# Team Instructions

Run the full test suite before merging.
```

`existing-instructions/CLAUDE.md`:

```markdown
# Claude-specific notes

Use concise explanations.
```

`existing-instructions/.gitignore`:

```text
node_modules/
```

- [ ] **Step 2: Write a golden idempotency test**

The test:

1. Copies the fixture to a temporary directory.
2. Initializes Git.
3. Calls the real Mason CLI with `--brick-path`.
4. Captures every generated file and SHA-256 digest.
5. Runs initialization again.
6. Asserts the digest map is identical.
7. Asserts original instruction text is preserved.
8. Runs strict validation only after replacing scaffold pages with a small evidence-backed baseline fixture and calling `Finalize`.

- [ ] **Step 3: Run the focused test**

```bash
go test ./internal/initrepo -run TestGoldenInitializationIsIdempotent -v
```

Expected: PASS.

- [ ] **Step 4: Update the milestone gate**

Add to `Makefile`:

```make
.PHONY: brick

brick:
	tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	mason bundle bricks/software-wiki -o "$$tmp"

verify: fmt test vet build brick
```

- [ ] **Step 5: Run the complete milestone gate**

```bash
make verify
```

Expected:

```text
All Go tests pass.
Go vet reports no findings.
The helper builds.
The llm_wiki Brick bundles successfully.
```

- [ ] **Step 6: Commit**

```bash
git add testdata/repos internal/initrepo Makefile
git commit -m "test: verify wiki initialization lifecycle"
```

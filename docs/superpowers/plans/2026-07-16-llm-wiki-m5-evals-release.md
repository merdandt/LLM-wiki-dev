# LLM Wiki Milestone 5 Evaluations and Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the product across representative software projects, enforce token and silence budgets, run cross-platform CI, and produce reproducible plugin and installer release artifacts.

**Architecture:** Separate deterministic CI tests from authenticated agent forward-evaluations. Go tests validate fixtures, hooks, generated artifacts, and budgets on every change; release candidates additionally run Codex and Claude scenario prompts against disposable fixture copies.

**Tech Stack:** Go 1.26.5, GitHub Actions, GitHub Releases, Cloudflare, Markdown scenario fixtures, shell.

---

## Task 1: Build representative repository fixtures

**Files:**

- Create: `internal/eval/fixtures.go`
- Test: `internal/eval/fixtures_test.go`
- Create: `testdata/repos/frontend/`
- Create: `testdata/repos/backend/`
- Create: `testdata/repos/cli/`
- Create: `testdata/repos/multi-service/`
- Create: `testdata/repos/library/`
- Create: `testdata/repos/infrastructure/`
- Create: `testdata/repos/monorepo/`

- [ ] **Step 1: Write the failing fixture-contract test**

```go
package eval

import "testing"

func TestFixtureContracts(t *testing.T) {
	fixtures := []string{
		"frontend",
		"backend",
		"cli",
		"multi-service",
		"library",
		"infrastructure",
		"monorepo",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			fixture, err := LoadFixture("../../testdata/repos/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.EntryPoints == 0 {
				t.Fatal("fixture has no detectable entry point")
			}
			if fixture.Tests == 0 {
				t.Fatal("fixture has no test evidence")
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/eval -run TestFixtureContracts -v
```

Expected: FAIL because fixtures and `LoadFixture` do not exist.

- [ ] **Step 3: Create the frontend fixture**

Create:

```text
frontend/
├── .gitignore
├── package.json
├── package-lock.json
├── src/
│   ├── App.tsx
│   └── auth/session.ts
└── tests/
    └── session.test.ts
```

`.gitignore` contains:

```gitignore
node_modules/
```

`package.json`:

```json
{
  "name": "fixture-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "test": "vitest run"
  },
  "devDependencies": {
    "typescript": "5.8.3",
    "vitest": "3.2.4"
  }
}
```

Create a lockfile-v3 `package-lock.json` for exactly these pinned development dependencies and verify `npm ci && npm test` succeeds before committing the fixture.

`src/auth/session.ts`:

```ts
export type Session = { userId: string; expiresAt: number };

export function isSessionActive(session: Session, now: number): boolean {
  return session.expiresAt > now;
}
```

`tests/session.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { isSessionActive } from "../src/auth/session";

describe("isSessionActive", () => {
  it("expires at the boundary", () => {
    expect(isSessionActive({ userId: "u1", expiresAt: 10 }, 10)).toBe(false);
  });
});
```

- [ ] **Step 4: Create the backend and CLI fixtures**

The backend fixture contains:

```text
backend/
├── go.mod
├── cmd/server/main.go
├── internal/orders/service.go
├── internal/orders/service_test.go
└── api/openapi.yaml
```

The CLI fixture contains:

```text
cli/
├── go.mod
├── cmd/acme/main.go
├── internal/config/config.go
└── internal/config/config_test.go
```

Use valid, buildable Go code. The backend service must expose `CreateOrder`, reject empty customer IDs, and have one test for that invariant. The CLI must load a YAML config path and have one test for a missing file.

- [ ] **Step 5: Create the system-scale fixtures**

Create these exact minimum layouts:

```text
multi-service/
├── go.mod
├── services/api/main.go
├── services/api/main_test.go
├── services/worker/main.go
├── services/worker/main_test.go
└── contracts/order-created.json

library/
├── go.mod
├── client/client.go
├── client/client_test.go
└── compatibility/public_api_test.go

infrastructure/
├── deploy/service.yaml
├── scripts/health-check.sh
├── scripts/rollback.sh
└── tests/deployment_test.go

monorepo/
├── go.mod
├── apps/web/main.go
├── services/api/main.go
├── packages/contracts/order.go
└── integration/contract_test.go
```

The multi-service test must prove that the API-produced event is accepted by the worker. The library compatibility test must compile against the exported API. The infrastructure test must parse the deployment manifest and assert health and rollback commands exist. The monorepo integration test must exercise the shared contract through both producer and consumer packages.

Keep each fixture under 40 source files and under 100 KiB so evaluation copies remain fast.

- [ ] **Step 6: Implement fixture inspection**

`internal/eval/fixtures.go`:

```go
package eval

import (
	"io/fs"
	"path/filepath"
)

type Fixture struct {
	Root        string
	EntryPoints int
	Tests       int
	Schemas     int
}

func LoadFixture(root string) (Fixture, error) {
	fixture := Fixture{Root: root}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		slash := filepath.ToSlash(path)
		base := filepath.Base(path)
		switch {
			case base == "main.go" || base == "App.tsx" || base == "index.ts" ||
				base == "client.go" || (base == "service.yaml" && contains(slash, "/deploy/")):
			fixture.EntryPoints++
		case isTestPath(slash):
			fixture.Tests++
		case base == "openapi.yaml" || filepath.Ext(base) == ".proto" || filepath.Ext(base) == ".graphql":
			fixture.Schemas++
		}
		return nil
	})
	return fixture, err
}

func isTestPath(path string) bool {
	base := filepath.Base(path)
	return filepath.Ext(base) == ".go" && len(base) > len("_test.go") && base[len(base)-len("_test.go"):] == "_test.go" ||
		filepath.Ext(base) == ".ts" && (contains(base, ".test.") || contains(base, ".spec."))
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
```

- [ ] **Step 7: Run tests and commit**

```bash
gofmt -w internal/eval
go test ./internal/eval -run TestFixtureContracts -v
git add internal/eval testdata/repos
git commit -m "test: add software project fixtures"
```

## Task 2: Define behavior scenarios and expected wiki effects

**Files:**

- Create: `evals/scenarios/feature.md`
- Create: `evals/scenarios/bug-fix.md`
- Create: `evals/scenarios/refactor.md`
- Create: `evals/scenarios/architecture-pivot.md`
- Create: `evals/scenarios/package-upgrade.md`
- Create: `evals/scenarios/contract-change.md`
- Create: `evals/scenarios/deployment-change.md`
- Create: `evals/scenarios/removal.md`
- Create: `evals/scenarios/no-op.md`
- Create: `evals/scenarios/unseen-commit.md`
- Create: `evals/mutations/unseen-commit.sh`
- Create: `internal/eval/scenario.go`
- Test: `internal/eval/scenario_test.go`

- [ ] **Step 1: Define the scenario frontmatter contract**

Every file uses:

```yaml
---
id: feature-session-refresh
fixture: frontend
expected_outcome: synced
expected_page_kinds:
  - component
  - flow
  - contract
forbidden_phrases:
  - maybe
  - attempted fix
max_context_bytes: 12288
---
```

`pre_session_script` is optional and is permitted only for a controlled harness mutation under `evals/mutations/`.

The body contains:

- Setup mutation.
- Agent prompt.
- Required repository verification.
- Expected canonical wiki effects.
- Explicit forbidden effects.

- [ ] **Step 2: Create the feature scenario**

`evals/scenarios/feature.md`:

```markdown
---
id: feature-session-refresh
fixture: frontend
expected_outcome: synced
expected_page_kinds:
  - component
  - flow
  - contract
forbidden_phrases:
  - attempted fix
  - temporary workaround
max_context_bytes: 12288
---
# Session refresh feature

## Mutation

Add a refresh-token flow that replaces an expired access token without requiring a new login.

## Prompt

Implement session refresh, verify it with tests, and finish the task.

## Expected wiki effects

- The authentication component describes refresh responsibility.
- A login/session flow documents access-token expiry and refresh.
- A token contract records compatibility and failure behavior.
- Index and log are updated.
- A synced receipt exists.

## Forbidden effects

- No debugging attempts appear in canonical pages.
- No unrelated components are created.
```

- [ ] **Step 3: Create the remaining scenarios**

Use these exact expectations:

| Scenario | Fixture | Outcome | Required page kinds | Expected result |
|---|---|---|---|---|
| Bug fix | backend | `synced` | `quality`, `component` | Failure mode, invariant, regression evidence; no failed hypotheses |
| Refactor | library | `no-update` | none | Internal-only refactor preserves public responsibility and creates no canonical wiki diff |
| Architecture pivot | multi-service | `synced` | `decision`, `system`, `flow` | New ADR with `supersedes`; old ADR retained |
| Package upgrade | frontend | `no-update` | none | Lockfile-only compatible upgrade produces a justified no-update receipt |
| Contract change | monorepo | `synced` | `contract`, `component` | Contract plus all producers and consumers |
| Deployment change | infrastructure | `synced` | `operation`, `quality` | Operations, health, rollback, observability |
| Removal | cli | `synced` | `component`, `decision` | Current pages and links updated; decision history retained |
| No-op | frontend | `clean` | none | No canonical wiki diff, no current receipt, and silent completion |
| Unseen commit | backend | `synced` | `component`, `quality` | Startup audit followed by a synchronized wiki receipt |

Write each Markdown body with the same five sections as `feature.md`.

Set this field in `unseen-commit.md`:

```yaml
pre_session_script: evals/mutations/unseen-commit.sh
```

`evals/mutations/unseen-commit.sh`:

```sh
#!/bin/sh
set -eu

workdir="${1:?workdir is required}"
mkdir -p "$workdir/internal/orders"
cat > "$workdir/internal/orders/priority.go" <<'EOF'
package orders

func IsPriority(totalCents int) bool {
	return totalCents >= 10000
}
EOF
cat > "$workdir/internal/orders/priority_test.go" <<'EOF'
package orders

import "testing"

func TestIsPriority(t *testing.T) {
	if !IsPriority(10000) || IsPriority(9999) {
		t.Fatal("priority boundary is incorrect")
	}
}
EOF
(
	cd "$workdir"
	go test ./...
)
git -C "$workdir" add internal/orders/priority.go internal/orders/priority_test.go
git -C "$workdir" \
	-c user.name=Eval \
	-c user.email=eval@example.com \
	commit -qm "feat: add priority order classification"
```

- [ ] **Step 4: Implement scenario parsing**

`internal/eval/scenario.go`:

```go
package eval

import (
	"bytes"
	"errors"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Scenario struct {
	ID                string   `yaml:"id"`
	Fixture           string   `yaml:"fixture"`
	ExpectedOutcome   string   `yaml:"expected_outcome"`
	ExpectedPageKinds []string `yaml:"expected_page_kinds"`
	ForbiddenPhrases  []string `yaml:"forbidden_phrases"`
	MaxContextBytes   int      `yaml:"max_context_bytes"`
	PreSessionScript  string   `yaml:"pre_session_script,omitempty"`
	Body              string   `yaml:"-"`
}

func LoadScenario(path string) (Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return Scenario{}, errors.New("missing scenario frontmatter")
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return Scenario{}, errors.New("unterminated scenario frontmatter")
	}
	var scenario Scenario
	if err := yaml.Unmarshal(data[4:4+end], &scenario); err != nil {
		return Scenario{}, err
	}
	scenario.Body = string(data[4+end+5:])
	if scenario.ID == "" || scenario.Fixture == "" || scenario.ExpectedOutcome == "" {
		return Scenario{}, errors.New("id, fixture, and expected_outcome are required")
	}
	switch scenario.ExpectedOutcome {
	case "clean", "synced", "no-update":
	default:
		return Scenario{}, errors.New("unsupported expected_outcome")
	}
	if scenario.MaxContextBytes < 0 || scenario.MaxContextBytes > 12*1024 {
		return Scenario{}, errors.New("invalid max_context_bytes")
	}
	if scenario.PreSessionScript != "" {
		clean := path.Clean(scenario.PreSessionScript)
		if clean != scenario.PreSessionScript ||
			!strings.HasPrefix(clean, "evals/mutations/") ||
			!strings.HasSuffix(clean, ".sh") {
			return Scenario{}, errors.New("unsafe pre_session_script")
		}
	}
	return scenario, nil
}
```

- [ ] **Step 5: Add scenario completeness tests**

Assert:

- All ten scenario IDs are unique.
- Every fixture exists.
- `expected_outcome` is `clean`, `synced`, or `no-update`.
- Synced scenarios list at least one expected page kind.
- Every scenario defines forbidden effects.
- Context budgets do not exceed 12 KiB.
- `pre_session_script`, when present, is a repository-relative path under `evals/mutations`.

- [ ] **Step 6: Run tests and commit**

```bash
gofmt -w internal/eval
go test ./internal/eval -run TestScenarios -v
git add evals/scenarios internal/eval
git commit -m "test: define wiki behavior scenarios"
```

## Task 3: Add deterministic artifact and token-budget evaluation

**Files:**

- Create: `internal/eval/report.go`
- Test: `internal/eval/report_test.go`
- Create: `internal/eval/collect.go`
- Test: `internal/eval/collect_test.go`
- Create: `cmd/llm-wiki-eval/main.go`
- Create: `scripts/evaluate-artifacts.sh`
- Modify: `Makefile`

- [ ] **Step 1: Write failing report tests**

```go
package eval

import "testing"

func TestEvaluateArtifacts(t *testing.T) {
	result := EvaluateArtifacts(ArtifactInput{
		Scenario: Scenario{
			ExpectedOutcome:   "synced",
			ExpectedPageKinds: []string{"component", "flow"},
			ForbiddenPhrases:  []string{"attempted fix"},
			MaxContextBytes:   12288,
		},
			PageKinds:    []string{"component", "flow"},
			Canonical:    "Confirmed behavior.",
			CanonicalChanged: true,
			ContextBytes: 2048,
			ReceiptKind:  "synced",
			LeaseActive:  false,
	})
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/eval -run TestEvaluateArtifacts -v
```

Expected: FAIL because `EvaluateArtifacts` does not exist.

- [ ] **Step 3: Implement artifact checks**

`internal/eval/report.go`:

```go
package eval

import (
	"fmt"
	"strings"
)

type ArtifactInput struct {
	Scenario        Scenario
	PageKinds      []string
	Canonical      string
	CanonicalChanged bool
	ContextBytes   int
	ReceiptKind    string
	LeaseActive    bool
	UnexpectedCommits bool
}

type Result struct {
	Failures []string `json:"failures"`
}

func EvaluateArtifacts(input ArtifactInput) Result {
	var result Result
	wantReceipt := input.Scenario.ExpectedOutcome
	if wantReceipt == "clean" {
		wantReceipt = ""
	}
	if input.ReceiptKind != wantReceipt {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"receipt=%s want=%s",
			input.ReceiptKind,
			wantReceipt,
		))
	}
	if input.Scenario.ExpectedOutcome == "no-update" && input.CanonicalChanged {
		result.Failures = append(result.Failures, "no-update scenario changed canonical wiki")
	}
	if input.Scenario.ExpectedOutcome == "synced" && !input.CanonicalChanged {
		result.Failures = append(result.Failures, "synced scenario left canonical wiki unchanged")
	}
	if input.Scenario.ExpectedOutcome == "clean" && input.CanonicalChanged {
		result.Failures = append(result.Failures, "clean scenario changed canonical wiki")
	}
	if input.LeaseActive {
		result.Failures = append(result.Failures, "synchronization lease is still active")
	}
	if input.UnexpectedCommits {
		result.Failures = append(result.Failures, "agent created an unauthorized commit")
	}
	for _, want := range input.Scenario.ExpectedPageKinds {
		if !containsString(input.PageKinds, want) {
			result.Failures = append(result.Failures, "missing page kind: "+want)
		}
	}
	lower := strings.ToLower(input.Canonical)
	for _, phrase := range input.Scenario.ForbiddenPhrases {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			result.Failures = append(result.Failures, "forbidden phrase: "+phrase)
		}
	}
	if input.ContextBytes > input.Scenario.MaxContextBytes {
		result.Failures = append(result.Failures, "context budget exceeded")
	}
	return result
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Create the deterministic evaluation script**

Implement `eval.Collect(root, baseline, expectedHead string, scenario Scenario) (ArtifactInput, error)`:

1. Load `llm-wiki.yaml`.
2. Combine `git diff --name-only <baseline> -- <wiki-path>` with untracked Markdown paths under the wiki.
3. Parse only those changed canonical pages and collect unique changed page kinds plus concatenated changed-page text; for a deleted page, parse its baseline content with `git show <baseline>:<path>`. Set `CanonicalChanged` from whether the changed-path set is empty.
4. Call `hook.BuildFingerprint`, then `state.NewLayout(<state-path>).ReadReceipt(currentFingerprint)`; set `ReceiptKind` only when a matching current receipt exists.
5. Resolve the worktree ID and set `LeaseActive` from `lock.CurrentOwner`.
6. Compare `repo.Head()` with the supplied expected pre-agent head and set `UnexpectedCommits`.
7. Set `ContextBytes` from the captured SessionStart context file when provided, or zero for artifact-only evaluation.
8. Reject paths outside the supplied disposable repository.

`cmd/llm-wiki-eval/main.go` parses:

```text
--scenario PATH
--root PATH
--baseline COMMIT
--expected-head COMMIT
--context-file PATH
--output PATH
```

It calls `LoadScenario`, `Collect`, and `EvaluateArtifacts`, writes an indented JSON report, and exits nonzero when `Failures` is non-empty. `collect_test.go` creates a temporary Git repository with a baseline wiki, modifies one page, writes a receipt, and asserts every collected field.

`scripts/evaluate-artifacts.sh`:

```sh
#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$repo_root"

go test ./internal/eval -v
go test ./internal/hook -run 'TestInLoop|TestSkills' -v
go build ./cmd/llm-wiki-eval
```

- [ ] **Step 5: Add budget assertions**

Tests must additionally enforce:

- Managed `AGENTS.md` block is at most 30 lines.
- Session-start additional context is at most 1 KiB.
- Default recall packet is at most 12 KiB.
- Root index fixture has at most 200 entries.
- Clean and synchronized hooks write zero stdout bytes.

- [ ] **Step 6: Run and commit**

```bash
chmod +x scripts/evaluate-artifacts.sh
./scripts/evaluate-artifacts.sh
git add internal/eval internal/hook scripts/evaluate-artifacts.sh Makefile
git commit -m "test: enforce wiki quality budgets"
```

## Task 4: Add authenticated forward-evaluation scripts

**Files:**

- Create: `scripts/run-agent-evals.sh`
- Create: `evals/README.md`
- Create: `evals/results/.gitkeep`

- [ ] **Step 1: Create the forward-evaluation runner**

`scripts/run-agent-evals.sh`:

```sh
#!/bin/sh
set -eu

host="${1:?usage: run-agent-evals.sh <codex|claude>}"
scenario="${2:?scenario markdown path is required}"
repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
scenario_dir="$(CDPATH= cd -- "$(dirname -- "$scenario")" 2>/dev/null && pwd)" ||
  { echo "scenario does not exist: $scenario" >&2; exit 2; }
scenario_abs="$scenario_dir/$(basename -- "$scenario")"
[ ! -L "$scenario_abs" ] || { echo "scenario symlinks are not allowed" >&2; exit 2; }
case "$scenario_abs" in
  "$repo_root"/evals/scenarios/*.md) ;;
  *) echo "scenario must be under evals/scenarios" >&2; exit 2 ;;
esac
case "$host" in
  codex|claude) ;;
  *) echo "unsupported host: $host" >&2; exit 2 ;;
esac

scenario_id="$(awk '/^id:/ { print $2; exit }' "$scenario_abs")"
fixture="$(awk '/^fixture:/ { print $2; exit }' "$scenario_abs")"
pre_session_script="$(awk '/^pre_session_script:/ { print $2; exit }' "$scenario_abs")"
case "$fixture" in
  ""|*[!A-Za-z0-9_-]*) echo "unsafe fixture name: $fixture" >&2; exit 2 ;;
esac
fixture_root="$repo_root/testdata/repos/$fixture"
[ -d "$fixture_root" ] || { echo "fixture does not exist: $fixture" >&2; exit 2; }

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

cp -R "$fixture_root/." "$workdir/"
git -C "$workdir" init -q
git -C "$workdir" add .
git -C "$workdir" -c user.name=Eval -c user.email=eval@example.com commit -qm baseline

prompt="$(awk '/^## Prompt$/ { capture=1; next } /^## / && capture { exit } capture { print }' "$scenario_abs")"
init_prompt="Invoke wiki-init for this repository. Build an evidence-backed baseline wiki, validate it, and finalize initialization. Do not change application code."

case "$host" in
  codex)
    codex exec --cwd "$workdir" "$init_prompt"
    ;;
  claude)
    claude -p --cwd "$workdir" "$init_prompt"
    ;;
esac

case "$(uname -s)/$(uname -m)" in
  Darwin/arm64) target="darwin-arm64/llm-wiki" ;;
  Darwin/x86_64) target="darwin-amd64/llm-wiki" ;;
  Linux/aarch64|Linux/arm64) target="linux-arm64/llm-wiki" ;;
  Linux/x86_64) target="linux-amd64/llm-wiki" ;;
  *) echo "unsupported evaluation platform" >&2; exit 8 ;;
esac
helper="$repo_root/plugins/llm-wiki/bin/$target"
"$helper" validate --root "$workdir"
git -C "$workdir" add .
git -C "$workdir" -c user.name=Eval -c user.email=eval@example.com commit -qm wiki-baseline
wiki_baseline="$(git -C "$workdir" rev-parse HEAD)"
rm -rf "$workdir/.llm-wiki-state"
if [ -n "$pre_session_script" ]; then
  pre_session_dir="$(
    CDPATH= cd -- "$(dirname -- "$repo_root/$pre_session_script")" 2>/dev/null &&
      pwd
  )" || { echo "pre-session script does not exist" >&2; exit 2; }
  pre_session_abs="$pre_session_dir/$(basename -- "$pre_session_script")"
  [ ! -L "$pre_session_abs" ] ||
    { echo "pre-session script symlinks are not allowed" >&2; exit 2; }
  case "$pre_session_abs" in
    "$repo_root"/evals/mutations/*.sh) ;;
    *) echo "unsafe pre-session script" >&2; exit 2 ;;
  esac
  printf '{"session_id":"eval-observer","cwd":"%s","hook_event_name":"SessionStart","source":"startup"}\n' "$workdir" |
    "$helper" hook session-start >/dev/null
  "$pre_session_abs" "$workdir"
fi
scenario_start_head="$(git -C "$workdir" rev-parse HEAD)"

case "$host" in
  codex)
    codex exec --cwd "$workdir" "$prompt"
    ;;
  claude)
    claude -p --cwd "$workdir" "$prompt"
    ;;
esac
"$helper" validate --root "$workdir"

result_dir="$repo_root/evals/results/$host/$scenario_id"
rm -rf "$result_dir"
mkdir -p "$result_dir"
"$helper" status --root "$workdir" --json > "$result_dir/status.json"
evaluation_status=0
(
  cd "$repo_root"
  go run ./cmd/llm-wiki-eval \
    --scenario "$scenario_abs" \
    --root "$workdir" \
    --baseline "$wiki_baseline" \
    --expected-head "$scenario_start_head" \
    --output "$result_dir/report.json"
) ||
  evaluation_status=$?
git -C "$workdir" add -N .
git -C "$workdir" diff --binary "$wiki_baseline" -- > "$result_dir/changes.patch"
cp -R "$workdir/docs/llm-wiki" "$result_dir/wiki"
exit "$evaluation_status"
```

Before running, install the candidate plugin from this checkout for the selected host. The script itself performs the `wiki-init` baseline pass in the disposable fixture before running the scenario prompt.

- [ ] **Step 2: Document the evaluation contract**

`evals/README.md`:

````markdown
# Agent forward evaluations

Forward evaluations require authenticated Codex or Claude Code installations and are not part of unauthenticated CI.

For each release candidate:

1. Install the candidate plugin locally.
2. Run every scenario once with Codex and once with Claude Code.
3. Validate the resulting wiki with the candidate helper.
4. Run deterministic artifact evaluation.
5. Store only generated wiki artifacts, diffs, and machine-readable reports. Do not commit transcripts.

Commands:

```bash
./scripts/run-agent-evals.sh codex evals/scenarios/feature.md
./scripts/run-agent-evals.sh claude evals/scenarios/feature.md
```
````

- [ ] **Step 3: Add safety checks**

The runner must refuse to operate when:

- The fixture path escapes `testdata/repos`.
- The scenario is missing.
- The host is not `codex` or `claude`.
- A real repository path is supplied.

- [ ] **Step 4: Shell-check syntax and commit**

```bash
sh -n scripts/run-agent-evals.sh
sh -n evals/mutations/unseen-commit.sh
chmod +x scripts/run-agent-evals.sh evals/mutations/unseen-commit.sh
git add scripts/run-agent-evals.sh evals
git commit -m "test: add agent forward evaluation harness"
```

## Task 5: Add cross-platform continuous integration

**Files:**

- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create the CI workflow**

`.github/workflows/ci.yml`:

```yaml
name: CI

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

jobs:
  verify:
    strategy:
      fail-fast: false
      matrix:
        os:
          - ubuntu-latest
          - macos-latest
          - windows-latest
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: 1.26.5
          cache: true
      - name: Install GNU Make on Windows
        if: runner.os == 'Windows'
        shell: pwsh
        run: choco install make --no-progress -y
      - name: Verify
        shell: bash
        run: make verify

  package:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: 1.26.5
          cache: true
      - name: Package plugin
        run: ./scripts/package-release.sh
      - name: Verify release package
        run: ./scripts/verify-release.sh
      - uses: actions/upload-artifact@v4
        with:
          name: llm-wiki-plugin
          path: plugins/llm-wiki
```

- [ ] **Step 2: Make Makefile commands Windows-compatible**

All commands invoked under `shell: bash` may use POSIX syntax. Go tests must use `t.TempDir()` and `filepath`, not hard-coded `/tmp` or slash assumptions.

- [ ] **Step 3: Validate workflow syntax**

Run:

```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/ci.yml"); puts "valid yaml"'
```

Expected:

```text
valid yaml
```

- [ ] **Step 4: Run local verification and commit**

```bash
make verify
git add .github/workflows/ci.yml
git commit -m "ci: verify llm-wiki across platforms"
```

## Task 6: Add reproducible release workflow and installer checks

**Files:**

- Create: `.github/workflows/release.yml`
- Create: `docs/release-checklist.md`
- Modify: `scripts/verify-release.sh`

- [ ] **Step 1: Create release workflow**

`.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: 1.26.5
          cache: true
      - name: Verify source
        run: make verify
      - name: Package plugin
        run: ./scripts/package-release.sh
      - name: Build installer release archives
        run: ./scripts/package-release.sh --dist dist
      - name: Archive plugin
        run: |
          mkdir -p dist
          git archive \
            --format=tar.gz \
            --prefix=llm-wiki/ \
            -o "dist/llm-wiki-plugin-${GITHUB_REF_NAME}.tar.gz" \
            HEAD:plugins/llm-wiki
      - name: Create checksums
        run: |
          cd dist
          find . -maxdepth 1 -type f ! -name SHA256SUMS -print0 |
            sort -z |
            xargs -0 sha256sum > SHA256SUMS
      - name: Publish GitHub release
        env:
          GH_TOKEN: ${{ github.token }}
        run: gh release create "$GITHUB_REF_NAME" dist/* --generate-notes
```

- [ ] **Step 2: Create release checklist**

`docs/release-checklist.md`:

```markdown
# LLM Wiki release checklist

1. Confirm `release/plugin-metadata.yaml`, template version, helper version, and schema compatibility.
2. Run `make verify` from a clean checkout.
3. Run all Codex and Claude forward-evaluation scenarios.
4. Confirm no transcripts or secrets are present in `evals/results`.
5. Generate plugin binaries and verify checksums.
6. Validate Claude and Codex manifests with installed official clients.
7. Build the five helper archives and generate sorted SHA-256 manifests.
8. Verify `release/install.sh` through the Cloudflare subdomain and a disposable repository.
9. Push a signed `vX.Y.Z` tag and verify GitHub release artifacts.
10. Publish the bootstrap installer and release manifest through Cloudflare.
```

- [ ] **Step 3: Extend release verification**

Extend `verify-release.sh` with an optional `--dist PATH` argument. The ordinary no-argument verification remains usable by `make verify`; when `--dist` is present, additionally assert:

- Metadata version equals the Git tag when `GITHUB_REF_TYPE=tag`.
- Template version matches metadata.
- The release directory contains one universal source template archive.
- The release archive includes both manifests, hooks, skills, the source template, helper binaries, and checksums.

Add this release-workflow step after the plugin archive and checksum are created:

```yaml
      - name: Verify release artifacts
        run: ./scripts/verify-release.sh --dist dist
```

- [ ] **Step 4: Validate and commit**

```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/release.yml"); puts "valid yaml"'
./scripts/verify-release.sh
git add .github/workflows/release.yml docs/release-checklist.md scripts/verify-release.sh
git commit -m "build: add reproducible release workflow"
```

## Task 7: Add installation and maintenance documentation

**Files:**

- Modify: `README.md`
- Create: `docs/installation.md`
- Create: `docs/maintenance.md`

- [ ] **Step 1: Write installation documentation**

First, merge the user's existing README draft into a release-quality project README without discarding its intent. Correct the Karpathy gist link, then add concise sections for the compiled-wiki idea, quiet in-loop lifecycle, installation from `https://llm-wiki-dev.salesshortcut.ai/install.sh`, generated repository files, safety boundaries, development commands, and links to the detailed installation and maintenance documents. Do not replace unrelated user-authored content silently.

`docs/installation.md` must contain:

````markdown
# Installing LLM Wiki

## Prerequisites

- Git
- Codex or Claude Code
- A Git repository

The installer downloads a pinned helper and source template release, verifies its checksum, and initializes the current repository. End users do not need Go, Dart, Mason, or another package manager.

## Claude Code

```bash
claude plugin marketplace add merdandt/LLM-wiki-dev
claude plugin install llm-wiki@llm-wiki
```

Open the target repository and invoke the `wiki-init` skill.

## Codex

```bash
codex plugin marketplace add merdandt/LLM-wiki-dev
codex plugin marketplace list
```

Install `llm-wiki` from the `llm-wiki` marketplace, open the target repository, and invoke `wiki-init`.

## Repository output

- `llm-wiki.yaml`
- `docs/llm-wiki/`
- A managed block in `AGENTS.md`
- An `@AGENTS.md` managed block in `CLAUDE.md`
- `.llm-wiki-state/` in `.gitignore`

Normal maintenance does not commit, push, open pull requests, or change application code.
````

- [ ] **Step 2: Write maintenance documentation**

`docs/maintenance.md`:

```markdown
# Maintaining LLM Wiki

## Ordinary operation

The active coding agent recalls relevant pages, performs the requested software task, synchronizes durable knowledge, validates the wiki, and records a receipt.

Clean and synchronized hooks are silent.

## Health warnings

A warning is shown only for contradictory evidence, repeated validation failure, an unsafe migration, irreconcilable concurrent edits, or corrupted state.

Run the `wiki-audit` skill to repair health issues.

## Updating the plugin

Update through the host marketplace, restart or reload plugins, and run `llm-wiki migrate` when `status` reports an older schema.

## Removing local state

It is safe to delete `.llm-wiki-state/` while no agent is synchronizing. The next session recreates it and runs an unseen-commit audit.
```

- [ ] **Step 3: Verify documentation links and commit**

```bash
rg -n 'TODO|TBD|FIXME' docs plugins/llm-wiki
git diff --check
git add README.md docs/installation.md docs/maintenance.md
git commit -m "docs: add plugin installation and maintenance"
```

Expected: `rg` finds no placeholders and `git diff --check` exits successfully.

## Task 8: Run the final acceptance gate

**Files:**

- Create: `scripts/acceptance.sh`
- Modify: `Makefile`

- [ ] **Step 1: Create the acceptance script**

`scripts/acceptance.sh`:

```sh
#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$repo_root"

make verify
./scripts/evaluate-artifacts.sh
./scripts/package-release.sh
dist="$(mktemp -d)"
trap 'rm -rf "$dist"' EXIT HUP INT TERM
./scripts/package-release.sh --output "$dist"
git archive \
  --format=tar.gz \
  --prefix=llm-wiki/ \
  -o "$dist/llm-wiki-plugin-local.tar.gz" \
  HEAD:plugins/llm-wiki
./scripts/verify-release.sh --dist "$dist"
git diff --check
git diff --exit-code -- \
  .claude-plugin \
  .agents/plugins \
  plugins/llm-wiki/.claude-plugin \
  plugins/llm-wiki/.codex-plugin \
  plugins/llm-wiki/assets/release-checksums.json

test -f plugins/llm-wiki/.claude-plugin/plugin.json
test -f plugins/llm-wiki/.codex-plugin/plugin.json
test -f .claude-plugin/marketplace.json
test -f .agents/plugins/marketplace.json
test "$(find plugins/llm-wiki/skills -name SKILL.md | wc -l | tr -d ' ')" = "4"
```

- [ ] **Step 2: Add acceptance target**

```make
.PHONY: acceptance

acceptance:
	./scripts/acceptance.sh
```

- [ ] **Step 3: Run final verification**

```bash
chmod +x scripts/acceptance.sh
make acceptance
```

Expected:

```text
All unit, integration, budget, package, and release checks pass.
The release archives and installer verify.
Both plugin adapters are current.
Five helper binaries match their checksums.
No unexpected diff or placeholder remains.
```

- [ ] **Step 4: Review the diff against the approved design**

Run:

```bash
git diff --stat "$(git merge-base HEAD origin/main)"..HEAD
git diff --check
```

Check every acceptance criterion in the approved design specification against test evidence.

- [ ] **Step 5: Commit**

```bash
git add scripts/acceptance.sh Makefile
git commit -m "test: add final llm-wiki acceptance gate"
```

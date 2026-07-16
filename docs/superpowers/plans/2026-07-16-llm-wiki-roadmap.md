# LLM Wiki Delivery Roadmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a cross-platform, Mason-initialized, team-shared software-development wiki that Codex and Claude Code maintain autonomously through quiet in-loop hooks.

**Architecture:** Build one deterministic Go helper, one Mason Brick, four portable Agent Skills, one shared hook configuration, and thin Claude/Codex packaging adapters. Deliver them in five sequential milestones so each milestone leaves a working, tested artifact and establishes the contracts used by the next milestone.

**Tech Stack:** Go 1.26.5, Dart 3.12.2, Mason CLI 0.1.3, Markdown, YAML, JSON, POSIX shell, PowerShell, GitHub Actions.

---

## Approved design

Implementation must conform to:

`docs/superpowers/specs/2026-07-16-llm-wiki-software-development-design.md`

If implementation pressure suggests changing an approved product invariant, update and re-review the design specification before changing code.

## Toolchain preflight

The current workspace does not have a working Go or Dart installation. Before Milestone 1, verify:

```bash
go version
dart --version
mason --version
```

Expected versions:

```text
go version go1.26.5 ...
Dart SDK version: 3.12.2 ...
mason_cli 0.1.3
```

On macOS with Homebrew:

```bash
brew install go
brew tap dart-lang/dart
brew install dart
dart pub global activate mason_cli 0.1.3
```

Do not modify the user's uncommitted `README.md` while setting up the toolchain.

## Repository file map

The completed source tree should have these responsibilities:

```text
.
├── .agents/
│   └── plugins/
│       └── marketplace.json
├── .claude-plugin/
│   └── marketplace.json
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── .gitattributes
├── .gitignore
├── .tool-versions
├── Makefile
├── go.mod
├── go.sum
├── cmd/
│   ├── llm-wiki/
│   │   └── main.go
│   └── llm-wiki-eval/
│       └── main.go
├── docs/
│   ├── installation.md
│   ├── maintenance.md
│   └── release-checklist.md
├── evals/
│   ├── README.md
│   ├── results/
│   │   └── .gitkeep
│   └── scenarios/
│       ├── architecture-pivot.md
│       ├── bug-fix.md
│       ├── contract-change.md
│       ├── deployment-change.md
│       ├── feature.md
│       ├── no-op.md
│       ├── package-upgrade.md
│       ├── refactor.md
│       ├── removal.md
│       └── unseen-commit.md
├── internal/
│   ├── atomicfile/
│   │   ├── write.go
│   │   └── write_test.go
│   ├── cli/
│   │   ├── run.go
│   │   └── run_test.go
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── eval/
│   │   ├── collect.go
│   │   ├── collect_test.go
│   │   ├── fixtures.go
│   │   ├── fixtures_test.go
│   │   ├── report.go
│   │   ├── report_test.go
│   │   ├── scenario.go
│   │   └── scenario_test.go
│   ├── fingerprint/
│   │   ├── fingerprint.go
│   │   └── fingerprint_test.go
│   ├── gitrepo/
│   │   ├── repo.go
│   │   └── repo_test.go
│   ├── hook/
│   │   ├── input.go
│   │   ├── platform.go
│   │   ├── result.go
│   │   ├── session_start.go
│   │   ├── stop.go
│   │   └── hook_test.go
│   ├── initrepo/
│   │   ├── gitignore.go
│   │   ├── init.go
│   │   ├── instructions.go
│   │   └── init_test.go
│   ├── lock/
│   │   ├── lock.go
│   │   └── lock_test.go
│   ├── mason/
│   │   ├── installer.go
│   │   ├── runner.go
│   │   └── mason_test.go
│   ├── materiality/
│   │   ├── classify.go
│   │   └── classify_test.go
│   ├── migrate/
│   │   ├── migrate.go
│   │   └── migrate_test.go
│   ├── pluginmeta/
│   │   ├── metadata.go
│   │   ├── render.go
│   │   └── render_test.go
│   ├── state/
│   │   ├── layout.go
│   │   ├── receipt.go
│   │   ├── session.go
│   │   └── state_test.go
│   └── wiki/
│       ├── frontmatter.go
│       ├── links.go
│       ├── secrets.go
│       ├── validator.go
│       └── validator_test.go
├── schemas/
│   ├── llm-wiki-config-v1.schema.json
│   └── llm-wiki-page-v1.schema.json
├── bricks/
│   └── software-wiki/
│       ├── brick.yaml
│       ├── CHANGELOG.md
│       ├── LICENSE
│       ├── README.md
│       ├── __brick__/
│       │   ├── llm-wiki.yaml
│       │   └── {{wiki_path}}/
│       └── hooks/
│           ├── pre_gen.dart
│           └── pubspec.yaml
├── plugins/
│   └── llm-wiki/
│       ├── .claude-plugin/
│       │   └── plugin.json
│       ├── .codex-plugin/
│       │   └── plugin.json
│       ├── assets/
│       │   ├── bricks/
│       │   │   └── software-wiki/
│       │   └── release-checksums.json
│       ├── bin/
│       │   ├── darwin-amd64/
│       │   ├── darwin-arm64/
│       │   ├── linux-amd64/
│       │   ├── linux-arm64/
│       │   └── windows-amd64/
│       ├── hooks/
│       │   └── hooks.json
│       ├── scripts/
│       │   ├── run-hook.ps1
│       │   └── run-hook.sh
│       └── skills/
│           ├── wiki-audit/
│           │   └── SKILL.md
│           ├── wiki-init/
│           │   └── SKILL.md
│           ├── wiki-recall/
│           │   └── SKILL.md
│           └── wiki-sync/
│               └── SKILL.md
├── release/
│   └── plugin-metadata.yaml
├── scripts/
│   ├── acceptance.sh
│   ├── evaluate-artifacts.sh
│   ├── package-plugin.sh
│   ├── run-agent-evals.sh
│   └── verify-release.sh
└── testdata/
    ├── hooks/
    ├── plugins/
    ├── repos/
    └── wiki/
```

## Public command contract

The completed helper exposes:

```text
llm-wiki version
llm-wiki validate [--root PATH] [--allow-uninitialized]
llm-wiki status [--root PATH] [--json]
llm-wiki fingerprint --root PATH --page WIKI_PAGE [--json]
llm-wiki init [--root PATH] [--wiki-path PATH] [--non-interactive]
llm-wiki finalize-init [--root PATH]
llm-wiki migrate [--root PATH]
llm-wiki hook session-start
llm-wiki hook stop
llm-wiki receipt write --kind synced|no-update [--reason TEXT]
llm-wiki plugin render
llm-wiki plugin checksums --root PATH --output PATH
```

Exit-code contract:

```text
0  command succeeded
2  invalid command-line usage
3  repository is not initialized
4  validation failed
5  synchronization is required
6  concurrent synchronization could not be reconciled
7  unsupported schema or migration
8  missing external prerequisite
```

## Shared type contract

All milestone plans use these names consistently:

```go
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

type Outcome string

const (
	OutcomeClean        Outcome = "clean"
	OutcomeSynchronized Outcome = "synchronized"
	OutcomeDrift        Outcome = "drift"
	OutcomeFailure      Outcome = "failure"
)
```

Do not rename these fields in later milestones without updating all earlier tests and both platform adapters.

## Milestone plans

### Milestone 1: Deterministic core and validator

Plan:

`docs/superpowers/plans/2026-07-16-llm-wiki-m1-core.md`

Produces:

- A working `llm-wiki` Go binary.
- Config parsing and defaults.
- Git/worktree discovery.
- Evidence and wiki fingerprints.
- Local session state, receipts, and persistent synchronization leases.
- Structural wiki validation.
- `version`, `status`, `validate`, and read-only page-fingerprint commands.

Gate:

```bash
go test ./...
go vet ./...
go build ./cmd/llm-wiki
```

### Milestone 2: Mason Brick and repository initialization

Plan:

`docs/superpowers/plans/2026-07-16-llm-wiki-m2-brick-init.md`

Produces:

- The versioned Mason Brick.
- Mason detection and installation.
- Idempotent instruction-file merging.
- `init`, `finalize-init`, and `migrate` commands.
- Golden initialization tests.

Gate:

```bash
make brick
go test ./internal/mason ./internal/initrepo ./internal/migrate
go test ./...
```

### Milestone 3: Agent skills and in-loop hooks

Plan:

`docs/superpowers/plans/2026-07-16-llm-wiki-m3-agent-loop.md`

Produces:

- Four portable skills.
- Shared `SessionStart` and `Stop` hooks.
- Codex and Claude hook-output adapters.
- Quiet clean/no-op behavior.
- One-pass recovery behavior.

Gate:

```bash
go test ./internal/hook ./internal/materiality ./internal/state
go test ./...
```

### Milestone 4: Dual-platform plugin packaging

Plan:

`docs/superpowers/plans/2026-07-16-llm-wiki-m4-plugin-packaging.md`

Produces:

- Claude and Codex plugin manifests.
- Claude and Codex marketplace catalogs.
- Cross-platform binary packaging.
- Release metadata generation.
- Local installation smoke tests.

Gate:

```bash
go run ./cmd/llm-wiki plugin render
./scripts/package-plugin.sh
./scripts/verify-release.sh
go test ./internal/pluginmeta ./...
```

### Milestone 5: Evaluations, CI, and release

Plan:

`docs/superpowers/plans/2026-07-16-llm-wiki-m5-evals-release.md`

Produces:

- Representative software-project fixtures.
- Behavioral and token-budget evaluations.
- macOS, Linux, and Windows CI.
- Reproducible release artifacts and checksums.
- BrickHub publication verification.

Gate:

```bash
make acceptance
```

## Global implementation rules

- Use test-driven development for every behavior change.
- Run the named failing test before writing implementation.
- Keep ordinary hook success paths completely silent.
- Never let hook code edit canonical wiki files.
- Keep semantic synthesis in skills and the active agent.
- Keep deterministic behavior in Go.
- Never edit application-source fixtures in-place during tests; copy them to temporary directories.
- Preserve the user's existing uncommitted `README.md` until a dedicated documentation task explicitly incorporates it.
- Commit after every completed task using the commit message shown in that task's milestone plan.

## Final completion gate

Do not call the complete project finished until all of these pass from a clean checkout:

```bash
make acceptance
```

Expected result:

```text
All Go tests pass.
Go vet reports no findings.
The helper builds.
The Mason Brick bundles.
Both plugin adapters render deterministically.
All release artifacts match their checksums.
The worktree contains no unexpected generated diff.
```

## Roadmap execution

- [ ] **Step 1: Complete Milestone 1 and its gate**

Follow `2026-07-16-llm-wiki-m1-core.md`.

- [ ] **Step 2: Complete Milestone 2 and its gate**

Follow `2026-07-16-llm-wiki-m2-brick-init.md`.

- [ ] **Step 3: Complete Milestone 3 and its gate**

Follow `2026-07-16-llm-wiki-m3-agent-loop.md`.

- [ ] **Step 4: Complete Milestone 4 and its gate**

Follow `2026-07-16-llm-wiki-m4-plugin-packaging.md`.

- [ ] **Step 5: Complete Milestone 5 and the final completion gate**

Follow `2026-07-16-llm-wiki-m5-evals-release.md`.

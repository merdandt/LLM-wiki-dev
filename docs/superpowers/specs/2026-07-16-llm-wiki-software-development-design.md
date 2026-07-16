# LLM Wiki for Software Development

Status: Approved design

Date: 2026-07-16

## 1. Summary

LLM Wiki is a local-first, Git-versioned project memory system maintained autonomously by coding agents.

It adapts Karpathy's LLM Wiki pattern to software development:

- The repository and its accepted engineering artifacts are the evidence layer.
- A structured Markdown wiki is the compiled knowledge layer.
- Skills, hooks, rules, and deterministic tooling form the maintenance control plane.

The product is distributed as one shared plugin implementation with thin adapters for Codex and Claude Code. A signed release bundle initializes the wiki through a small bootstrap installer at `https://llm-wiki-dev.salesshortcut.ai/install.sh`. After initialization, quiet in-loop hooks ensure the active coding agent evaluates and applies any necessary wiki updates before finishing its work.

The helper modifies project-memory files only. It never edits application code, creates commits, pushes branches, or opens pull requests.

During ordinary sessions, canonical writes are limited to `docs/llm-wiki/**`; local writes are limited to `.llm-wiki-state/**`. Explicit initialization and migration workflows may also update `llm-wiki.yaml`, `.gitignore`, `AGENTS.md`, and `CLAUDE.md`.

## 2. Product principles

### 2.1 Compiled knowledge, not a conversation archive

The wiki is a maintained synthesis of durable project knowledge. It must not become a dump of prompts, transcripts, debugging attempts, or file summaries.

### 2.2 Repository evidence remains authoritative

The wiki is derived. It never overrides executable evidence or accepted engineering decisions.

The authority order is:

1. Code, tests, schemas, migrations, configuration, and deployed-system evidence determine current behavior.
2. Approved specifications and Architecture Decision Records determine intended behavior and historical rationale.
3. The wiki summarizes and connects that evidence.
4. Conversations and intermediate hypotheses are unverified candidates.

When authoritative sources conflict, the agent records the conflict and does not invent a resolution.

### 2.3 Team-owned and Git-versioned

Canonical wiki files are committed with the repository and reviewed through the team's normal code-review process. Local hook state, fingerprints, locks, and temporary context packets are ignored by Git.

### 2.4 Autonomous but bounded

Wiki maintenance requires no separate human approval. The active coding agent updates it as part of completing a task.

Autonomy is bounded by:

- A strict write allowlist.
- Evidence requirements.
- Deterministic validation.
- A single automatic recovery pass.
- Explicit handling for contradictions and uncertainty.

### 2.5 Quiet by default

Successful checks and no-op checks produce no user-visible output.

The helper reports only:

- Contradictory authoritative evidence.
- Validation that remains broken after recovery.
- Unsafe schema migrations.
- Concurrent wiki edits that cannot be reconciled.
- Unsupported or corrupted wiki state.

### 2.6 Progressive disclosure

Agents load only the knowledge needed for the current task. The system must save more context than it consumes.

## 3. Goals

The first release must:

- Work in Git repositories containing frontends, backends, services, libraries, modules, CLIs, infrastructure, or monorepos.
- Initialize a consistent software-development wiki with one HTTPS installer command.
- Publish versioned release bundles and checksums through GitHub Releases, with the branded subdomain serving only the bootstrap installer and release metadata.
- Integrate with both Codex and Claude Code.
- Share skills, hooks, scripts, and templates between both platforms.
- Maintain the wiki autonomously within the active agent loop.
- Handle features, bug fixes, refactors, architectural pivots, dependency upgrades, contract changes, deployments, and removals.
- Preserve architectural history while keeping current-state pages accurate.
- Detect stale, contradictory, orphaned, or invalid knowledge.
- Keep always-loaded instructions and hook output very small.
- Avoid runtime network calls during ordinary coding sessions.
- Remain useful without embeddings, an MCP server, a daemon, or a cloud account.

## 4. Non-goals

The first release will not include:

- A separate background daemon.
- A second LLM call from hooks.
- A hosted synchronization service.
- Automatic Git commits, pushes, or pull requests.
- An MCP server.
- Embedding-based retrieval.
- A graphical management interface.
- A CI bot that rewrites the wiki.
- Transcript ingestion as an authoritative source.
- Automatic modification of application code.
- Mason or Dart as a required installation prerequisite.
- Multiple federated wikis inside one Git repository.

## 5. System architecture

### 5.1 Evidence layer

The evidence layer already exists in a software repository. The product must reference it rather than duplicate it.

Typical evidence includes:

- Source code.
- Tests and fixtures.
- API, event, database, and configuration schemas.
- Database migrations.
- Package and lock manifests.
- Deployment configuration.
- Observability configuration.
- Existing documentation.
- Specifications.
- Architecture Decision Records.
- Git commits and diffs.
- Checked-in incident or postmortem records.

The wiki helper reads this layer but does not modify it.

### 5.2 Compiled project memory

The default canonical location is:

```text
docs/llm-wiki/
```

The default structure is:

```text
docs/llm-wiki/
├── index.md
├── system.md
├── schema.md
├── components/
├── flows/
├── contracts/
├── decisions/
├── quality/
│   ├── invariants.md
│   ├── testing.md
│   └── failure-modes.md
├── operations/
├── glossary.md
├── health.md
└── log.md
```

The root configuration is:

```text
llm-wiki.yaml
```

Local runtime state is:

```text
.llm-wiki-state/
```

The initializer adds `.llm-wiki-state/` to `.gitignore`.

### 5.3 Control plane

The control plane consists of:

- A concise `AGENTS.md` integration block.
- A `CLAUDE.md` import or integration block.
- Four reusable agent skills.
- Shared lifecycle hooks.
- A deterministic cross-platform helper.
- The versioned repository template and release bundle.
- Wiki schema migrations.

The control plane tells agents when to recall project knowledge, when a change is material, how to synchronize the wiki, and how to validate the result.

## 6. Wiki information model

### 6.1 Standard page metadata

Substantive wiki pages use YAML frontmatter:

```yaml
---
id: component.auth-service
kind: component
status: current
summary: Authenticates users and issues session credentials.
verification:
  base_commit: 0123456789abcdef
  evidence_fingerprint: sha256:89abcdef01234567
evidence:
  - path: src/auth/service.ts
    symbol: AuthService
  - path: tests/auth/session.test.ts
relations:
  - contract.session-token
  - flow.user-login
---
```

Required fields are:

- `id`: Stable identifier that does not depend on the filename.
- `kind`: Page type.
- `status`: `current`, `deprecated`, `superseded`, or `planned`.
- `summary`: A short retrieval-oriented description.
- `verification.base_commit`: Commit checked out when the evidence was verified.
- `verification.evidence_fingerprint`: Aggregate content fingerprint for the referenced evidence.
- `evidence`: Repository paths with optional symbols, headings, test names, or schema identifiers.

Reserved navigation files such as `index.md`, `schema.md`, and `log.md` may use specialized metadata.

The evidence fingerprint is the freshness authority because code and wiki updates are commonly prepared in the same uncommitted working tree. The base commit provides historical context but is not required to contain the verified changes.

Valid `kind` values in the first schema are `system`, `component`, `flow`, `contract`, `decision`, `quality`, `operation`, `glossary`, `health`, `index`, and `log`.

### 6.2 Page types

#### System

`system.md` explains:

- Product purpose.
- Major actors.
- System boundaries.
- External dependencies.
- Top-level runtime topology.
- High-level navigation into component and flow pages.

#### Components

Component pages describe services, packages, modules, frontends, CLIs, workers, and infrastructure units.

Each page covers:

- Responsibility.
- Public surface.
- Dependencies and dependents.
- State and data ownership.
- Important invariants.
- Operational behavior.
- Test and observability evidence.

#### Flows

Flow pages describe behavior crossing component boundaries.

Each page covers:

- Trigger and actors.
- Ordered steps.
- Inputs and outputs.
- State transitions.
- Failure and recovery paths.
- Contracts and invariants involved.

#### Contracts

Contract pages cover:

- HTTP or RPC APIs.
- Events and queues.
- Database schemas.
- CLI interfaces.
- Configuration contracts.
- File formats.
- Compatibility and migration rules.

#### Decisions

Decision files use:

```text
decisions/NNNN-short-title.md
```

Decisions are append-only historical records. A new decision may supersede an earlier decision, but the earlier file is retained and linked.

#### Quality

Quality pages capture:

- System-wide invariants.
- Test strategy and verification boundaries.
- Confirmed reusable failure modes.
- Security or reliability properties that cross components.

#### Operations

Operations pages capture:

- Deployment topology.
- Configuration and environment behavior.
- Observability.
- Recovery and rollback.
- Maintenance playbooks.

#### Health

`health.md` contains unresolved:

- Evidence contradictions.
- Stale pages.
- Broken ownership or dependency assumptions.
- Knowledge gaps.
- Pages needing a safe migration.

Resolved items leave `health.md` and receive a maintenance-log entry.

### 6.3 Linking

The wiki uses standard relative Markdown links for compatibility with GitHub, editors, and Markdown tooling. Obsidian may be used as a viewer, but Obsidian-specific syntax is not required.

### 6.4 Index and log

`index.md` is content-oriented. It groups pages by type or domain and includes a link plus one-line summary.

When the primary index grows beyond 200 entries, the audit workflow creates sharded indexes and keeps the root index as a bounded directory.

`log.md` is chronological and append-only. Entries use:

```text
## [2026-07-16] sync | Updated authentication flow
```

Agents read the tail of the log rather than loading the entire file. Large logs may be archived by year while preserving the same parseable format.

## 7. Materiality policy

The agent updates the wiki when a completed change affects durable knowledge.

Material changes include:

- Externally observable behavior.
- Component responsibility or boundary.
- Dependency direction.
- Public API, event, schema, CLI, file, or configuration contracts.
- State ownership or data lifecycle.
- Security, correctness, or reliability invariants.
- Deployment, observability, recovery, or rollback behavior.
- An accepted architectural decision.
- A confirmed recurring failure mode.
- A renamed or removed concept that other pages reference.

Changes that normally require no canonical wiki update include:

- Formatting.
- Comment wording.
- Mechanical code movement with no boundary change.
- Temporary debugging instrumentation.
- Failed hypotheses.
- Test refactoring that preserves the same behavioral contract.
- Lockfile churn with no compatibility, security, behavioral, or operational consequence.

For a no-update decision, the active agent writes a local receipt keyed to the current diff fingerprint. The rationale is not committed unless it represents durable project knowledge.

## 8. Development lifecycle integration

### 8.1 Session start

The `SessionStart` hook:

1. Locates the Git root and `llm-wiki.yaml`.
2. Exits silently if the repository is not initialized.
3. Records the current commit, dirty-file hashes, wiki hashes, worktree identity, and session identifier.
4. Checks schema compatibility, unresolved health warnings, and indexed-page evidence fingerprints.
5. Detects commits not previously observed in local state and marks a bounded startup audit when their changed paths may contain durable knowledge.
6. Injects a status packet no larger than 1 KiB.

The packet contains only:

- Wiki path.
- Schema status.
- Freshness status.
- Count of unresolved health items.
- Whether an unseen-commit audit is pending.
- Whether targeted recall is recommended.

It never injects complete wiki pages.

### 8.2 Recall before and during work

The agent invokes or implicitly triggers `wiki-recall` when a task can benefit from existing project knowledge.

Recall proceeds in this order:

1. Read `index.md`.
2. Search summaries, stable IDs, links, and evidence paths.
3. Read only matching wiki pages.
4. Consult the underlying code or tests when evidence needs verification.
5. Produce a bounded task-context packet.

The default packet budget is 12 KiB and is configurable in `llm-wiki.yaml`.

The packet prioritizes:

- Relevant boundaries and responsibilities.
- Flows and state transitions.
- Contracts.
- Invariants.
- Current decisions.
- Confirmed failure modes.
- Evidence and freshness.
- Explicit unknowns.

### 8.3 Stop-time synchronization

The `Stop` hook compares the session baseline, current repository state, wiki state, startup-audit state, and any local receipt.

It returns one of three outcomes:

#### Clean

No material repository change occurred, or a matching no-update receipt exists. The hook emits nothing.

#### Synchronized

Material knowledge changed and a matching validated wiki-update receipt exists. The hook emits nothing.

#### Possible drift

Material knowledge may have changed without a matching receipt, or an unseen-commit audit remains unresolved. The hook returns control to the active agent for one maintenance pass.

The active agent:

1. Runs the `wiki-sync` workflow.
2. Determines the actual material impact.
3. Updates affected wiki pages or records a no-update receipt.
4. Updates the index and log when canonical pages changed.
5. Runs validation.
6. Writes a validated receipt.

The next stop check finishes silently when validation succeeds.

The hook permits only one automatic recovery cycle. If synchronization still cannot complete, the coding task may finish and the user receives a concise warning.

## 9. Change-type behavior

### 9.1 Feature development

Update the affected:

- Components.
- End-to-end flows.
- Contracts.
- Invariants.
- Operational behavior.
- Decisions, when the feature introduces a lasting architectural choice.

### 9.2 Bug fixing

Retain only:

- Confirmed root cause.
- Violated invariant.
- Corrected behavior.
- Regression-test evidence.
- Reusable failure or diagnostic knowledge.

Do not retain failed hypotheses or transient debugging steps.

### 9.3 Refactoring and polishing

Update the wiki only when responsibilities, boundaries, dependencies, symbols used as evidence, performance characteristics, or operational behavior changed.

If behavior and boundaries remain the same, create a no-update receipt.

### 9.4 Architectural pivots

For a pivot:

1. Create a new ADR.
2. Link it with `supersedes`.
3. Mark the previous decision as superseded without deleting it.
4. Update current component, flow, contract, and operations pages.
5. Record migration state as `planned`, `in-progress`, or `complete`.

### 9.5 Package upgrades

Do not duplicate package manifests.

Update the wiki only for:

- Compatibility changes.
- Security consequences.
- Behavior changes.
- Runtime or deployment changes.
- New constraints imposed on consumers.

### 9.6 Contract changes

Update:

- Contract definition.
- Producers and consumers.
- Compatibility expectations.
- Migration or rollout behavior.
- Tests proving the new contract.

### 9.7 Deployment changes

Update:

- Runtime topology.
- Configuration.
- Health and readiness behavior.
- Observability.
- Rollback and recovery.

### 9.8 Removals

Update current-state pages and links. Retain historical decisions and meaningful migration records.

## 10. Agent skills

The plugin provides four shared skills using cross-platform Agent Skills frontmatter.

### 10.1 `wiki-init`

Use when initializing LLM Wiki in a repository.

Responsibilities:

- Preflight Git and workspace state.
- Download and verify the pinned release bundle.
- Install the versioned repository template and helper assets.
- Merge instruction blocks idempotently.
- Analyze the repository.
- Compile the initial wiki.
- Validate the result.
- Initialize local state.

### 10.2 `wiki-recall`

Use before planning or changing behavior that may depend on existing project knowledge.

Responsibilities:

- Navigate through the bounded index.
- Retrieve relevant pages.
- Verify stale evidence as needed.
- Produce a bounded context packet.

### 10.3 `wiki-sync`

Use after a material, verified change.

Responsibilities:

- Classify the change.
- Identify affected pages.
- Merge new evidence into existing pages.
- Avoid duplicate concepts.
- Preserve historical decisions.
- Update index and log.
- Validate and write a receipt.

### 10.4 `wiki-audit`

Use for explicit health checks and schema maintenance.

Responsibilities:

- Detect stale evidence.
- Find broken links and orphan pages.
- Find duplicate concepts.
- Find contradictions.
- Find missing pages for important recurring concepts.
- Shard oversized indexes.
- Archive oversized logs.
- Apply safe schema migrations.

## 11. Hooks

The shared hook configuration uses only events and command handlers supported by both Codex and Claude Code.

The first release uses:

- `SessionStart`.
- `Stop`.

The hook process reads the host-provided JSON from standard input and delegates to the deterministic helper.

Platform adapters translate the helper's neutral result into the host's supported hook output.

Hooks:

- Run from the repository context.
- Resolve the Git root instead of trusting the current subdirectory.
- Never parse an unstable transcript format.
- Never call an LLM.
- Never make normal-session network requests.
- Never modify canonical wiki files directly.
- Exit successfully with no output on clean and synchronized states.

## 12. Plugin and marketplace structure

The product repository contains:

```text
.
├── .claude-plugin/
│   └── marketplace.json
├── .agents/
│   └── plugins/
│       └── marketplace.json
├── plugins/
│   └── llm-wiki/
│       ├── .claude-plugin/
│       │   └── plugin.json
│       ├── .codex-plugin/
│       │   └── plugin.json
│       ├── skills/
│       ├── hooks/
│       │   └── hooks.json
│       ├── scripts/
│       ├── bin/
│       └── assets/
└── template/
    ├── llm-wiki.yaml
    ├── AGENTS.md
    ├── CLAUDE.md
    └── docs/llm-wiki/
```

The Claude and Codex manifests are generated from shared release metadata but validated independently.

Shared skills use only portable frontmatter fields. Shared hooks use the common command-hook subset. Platform-specific behavior stays in adapters rather than being embedded in the wiki workflow.

## 13. Installation and initialization

### 13.1 Plugin installation

Plugin installation is user-scoped or project-scoped according to the host platform. Installing the plugin does not mutate the current repository.

This separation is required because plugin installation may occur without an active project or may apply to many projects.

### 13.2 Repository initialization

Inside a target repository, the user explicitly invokes `wiki-init`.

Initialization:

1. Verifies the workspace is a Git repository.
2. Detects the operating system and architecture.
3. Verifies the deterministic helper.
4. Downloads the platform-specific helper and the repository template from a pinned GitHub Release.
5. Verifies the archive against a signed checksum manifest before extracting it.
6. Merges the template, skills, hooks, rules, and instruction blocks into the current Git repository.
7. Merges marked integration blocks.
8. Compiles the initial wiki.
9. Validates the complete result.

Network downloads occur only during explicit installation or upgrade workflows. Ordinary agent hooks never access the network.

The installer supports macOS, Linux, and Windows helper archives. It refuses an unverified or unsupported archive and reports a concise recovery command. A later release may export the same template for other package managers, but no package manager is part of the critical path.

### 13.3 Instruction-file merging

Managed sections use markers:

```text
<!-- llm-wiki:start -->
...
<!-- llm-wiki:end -->
```

The merge operation:

- Creates `AGENTS.md` when absent.
- Updates only the managed block when present.
- Preserves all user-authored content.
- Creates `CLAUDE.md` with an `@AGENTS.md` import when absent.
- Adds the import in the managed block when an existing `CLAUDE.md` does not already import `AGENTS.md`.
- Produces no diff when repeated with the same version.

The always-loaded block remains concise and points to skills for detailed workflows.

## 14. Deterministic helper

The recommended implementation is a small Go CLI distributed as native binaries for supported operating systems and architectures.

Go is selected because it provides:

- No Python or Node runtime dependency.
- Fast startup for hooks.
- Straightforward cross-platform builds.
- Safe JSON and filesystem handling.
- A single testable implementation for both platforms.

The helper owns deterministic behavior only:

- Git root and worktree discovery.
- Baseline and diff fingerprints.
- State storage.
- Locking.
- Receipt storage.
- Hook input and output adaptation.
- Configuration parsing.
- Wiki structural validation.
- Secret-pattern checks.
- Schema migration execution.
- Status packet generation.

Semantic synthesis remains the responsibility of the active coding agent.

The initial binary targets are:

- macOS ARM64 and x86-64.
- Linux ARM64 and x86-64.
- Windows x86-64.

## 15. State, transactions, and concurrency

### 15.1 Session state

State is stored by Git worktree and session:

```text
.llm-wiki-state/
├── sessions/
├── receipts/
├── locks/
├── context/
└── backups/
```

State files are machine-readable and not committed.

### 15.2 Fingerprints

A receipt is keyed by:

- Git worktree identity.
- Base commit.
- Relevant evidence fingerprint.
- Resulting wiki fingerprint.
- Wiki schema version.

This prevents a receipt for one change from satisfying a later unrelated change.

The agent session is retained as diagnostic metadata but is not part of receipt identity, allowing an interrupted synchronization to be recovered by a later session.

### 15.3 Synchronization transaction

A wiki synchronization:

1. Acquires the worktree wiki lock.
2. Records hashes for affected wiki pages.
3. Applies semantic updates through the active agent.
4. Runs validation.
5. Writes the receipt only after successful validation.
6. Releases the lock.

If the session ends before step 5, the next check treats the synchronization as incomplete.

### 15.4 Concurrent sessions

Only one session may synchronize the canonical wiki for a worktree at a time.

If another session holds the lock:

- The current session waits for up to five seconds.
- It re-reads the wiki after the lock is released.
- It recalculates affected pages before editing.
- It warns only when the edits cannot be reconciled within the recovery cycle.

## 16. Validation

Validation checks:

- `llm-wiki.yaml` syntax and supported schema version.
- Required wiki files.
- Frontmatter syntax.
- Required metadata fields.
- Unique stable IDs.
- Valid `kind` and `status` values.
- Relative Markdown links.
- Index-to-page consistency.
- Existing evidence paths.
- Matching evidence fingerprints.
- Valid ADR supersession chains.
- Parseable log headings.
- Repository-local wiki and state paths.
- No writes outside the configured allowlist.
- Common secret patterns.

Semantic validation is performed by the active agent:

- Does the page match the evidence?
- Does the update preserve current versus historical truth?
- Did the change create a duplicate concept?
- Are affected flows, contracts, and invariants consistent?

## 17. Security and privacy

The default system:

- Keeps all canonical knowledge in the repository.
- Keeps runtime state locally.
- Makes no normal-session network calls.
- Uses no separate API key.
- Does not send source code to an additional service.
- Avoids `.env`, credential stores, generated secrets, and ignored secret files as evidence.
- Runs secret-pattern validation before accepting a wiki receipt.
- Pins plugin, helper, template, and schema versions.
- Verifies packaged helper checksums and release metadata.

Hooks are still executable code and remain subject to each host's trust and workspace policies.

## 18. Token and scale budgets

The default budgets are:

- `AGENTS.md` managed block: at most 30 lines.
- Session-start context: at most 1 KiB.
- Recall context packet: at most 12 KiB.
- Root index: at most 200 page entries before sharding.
- Full log reads: prohibited during ordinary recall.

The first release uses:

- Markdown summaries.
- Stable IDs and links.
- `rg`.
- Git.

Local full-text search may be added after index sharding. Embeddings or MCP search require measured evidence that deterministic retrieval is insufficient.

## 19. Versioning and migrations

Four versions are independent:

- Plugin version: installable product release.
- Template version: initialization files and managed integrations.
- Wiki schema version: on-disk knowledge contract.
- Helper version: deterministic runtime.

`llm-wiki.yaml` records all required compatibility information.

The versioned template is used to initialize new repositories. Existing wikis are upgraded through ordered, idempotent schema migrations.

A safe migration:

- Changes structure without inventing semantic content.
- Is repeatable.
- Validates before recording the new schema version.
- Preserves a local backup until validation succeeds.

If a migration requires semantic judgment, the active agent performs it through `wiki-audit`. If it cannot establish the result from evidence, it records a health item and reports the problem.

## 20. Monorepo behavior

The first release uses one wiki and one `llm-wiki.yaml` per Git root.

Packages and services are modeled as components. Domain-specific indexes may be added under the root wiki as it grows.

Nested `AGENTS.md` files may point agents toward relevant component or domain pages, but nested independent wikis are outside the first-release scope.

## 21. Testing strategy

### 21.1 Unit tests

Test:

- Hook input parsing.
- Platform output adapters.
- Git and dirty-state fingerprints.
- Worktree identity.
- Lock acquisition and expiry.
- Receipt matching.
- Configuration parsing.
- Frontmatter and link validation.
- ADR supersession validation.
- Secret-pattern detection.
- Status and context budgets.

### 21.2 Golden tests

Golden tests cover:

- Default template output.
- Configurable wiki path.
- Existing `AGENTS.md`.
- Existing `CLAUDE.md`.
- Existing `@AGENTS.md` import.
- Repeated initialization.
- Schema migration output.

### 21.3 Integration fixtures

Maintain representative fixture repositories for:

- Frontend application.
- Backend service.
- Multi-service system.
- Shared library.
- CLI.
- Infrastructure project.
- Monorepo.

Each fixture exercises:

- Feature implementation.
- Bug fix.
- Refactor.
- Architectural pivot.
- Package upgrade.
- Contract change.
- Deployment change.
- Removal.
- Formatting-only no-op.
- Unseen material commit from another contributor.
- Unseen non-material commit from another contributor.

### 21.4 Platform tests

CI validates:

- macOS.
- Linux.
- Windows.
- Claude manifest and marketplace.
- Codex manifest and marketplace.
- Shared hook configuration.
- Installer archive and checksum validation.
- Supported helper architectures.

### 21.5 Behavioral evaluations

Evaluations measure:

- Correct pages selected for recall.
- Recall packet remains within budget.
- Durable changes update the expected page classes.
- No-op changes create no canonical diff.
- Bug fixes retain confirmed causes but not failed hypotheses.
- Architectural pivots preserve superseded decisions.
- Successful hooks emit no output.
- Failed validation receives one recovery pass.
- The helper never writes outside its allowlist.

## 22. Acceptance criteria

The MVP is complete when:

1. A user can install the plugin in either Codex or Claude Code.
2. `install.sh` initializes a Git repository using a pinned, verified release bundle.
3. Running initialization twice produces no second diff.
4. Existing instruction files are preserved.
5. An agent can recall a bounded project context before a task.
6. A material feature change updates the appropriate wiki pages before the agent stops.
7. A formatting-only change produces no wiki diff and no visible hook output.
8. A bug fix records only confirmed durable knowledge.
9. An architectural pivot creates a superseding ADR and preserves history.
10. An unresolved contradiction appears in `health.md` and produces the permitted warning.
11. Validation catches broken links, duplicate IDs, stale evidence paths, invalid ADR chains, and likely secrets.
12. Hooks have no normal-session network dependency.
13. The helper never changes source code or Git history.
14. The same shared skills and hook logic pass both platform test suites.
15. A newly observed material commit triggers a bounded audit, while a newly observed non-material commit remains silent after a local no-update receipt.

## 23. Implementation sequence

Implementation proceeds in five milestones:

1. Core schema, configuration, helper state model, and validator.
2. Signed release bundle and idempotent repository initialization.
3. Agent skills, instruction integration, and lifecycle behavior.
4. Claude and Codex plugin adapters and marketplaces.
5. Cross-platform fixtures, evaluations, release packaging, and installer publication.

Each milestone must pass its own tests before the next milestone depends on it.

## 24. Source references

- [Karpathy: LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)
- [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases)
- [Claude Code plugins](https://code.claude.com/docs/en/plugins)
- [Claude Code plugin reference](https://code.claude.com/docs/en/plugins-reference)
- [Claude Code hooks](https://code.claude.com/docs/en/hooks)
- [Claude Code project memory](https://code.claude.com/docs/en/memory)
- [Codex build plugins](https://learn.chatgpt.com/docs/build-plugins)
- [Codex build skills](https://learn.chatgpt.com/docs/build-skills)
- [Codex hooks](https://learn.chatgpt.com/docs/hooks)
- [Codex AGENTS.md guidance](https://learn.chatgpt.com/docs/agent-configuration/agents-md)

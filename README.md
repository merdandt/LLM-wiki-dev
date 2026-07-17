# LLM Wiki

Team-shared, Git-versioned project memory for coding agents (Claude Code, Codex), applying [Karpathy's LLM Wiki idea](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) to software repositories: keep an agent-maintained, evidence-backed *compiled* wiki next to the code, instead of re-deriving the same architectural knowledge in every session.

## Install

From the root of any Git repository:

```sh
curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash
```

The installer detects your OS/architecture, downloads a pinned release archive from GitHub Releases, verifies its SHA-256 checksum, rejects unsafe archive entries, installs the helper under `.llm-wiki/`, and initializes the repository. It is idempotent — rerunning it changes nothing. It requires only Git, a POSIX shell, `curl`, `tar`, and a SHA-256 utility.

Options (pass after `bash -s --`):

| Flag | Effect |
| --- | --- |
| `--version X.Y.Z` | Install a specific immutable release instead of latest |
| `--no-init` | Install the helper without touching repository files |
| `--global` | Install the binary to `~/.local/bin` instead of `.llm-wiki/` |
| `--dry-run` | Download and verify only |
| `--root PATH` | Target a repository other than the current directory |

## What gets installed

```
.llm-wiki/               # helper binary + template (gitignored; each machine reruns the installer)
.llm-wiki-state/         # local fingerprints, leases, receipts (gitignored)
llm-wiki.yaml            # committed configuration and schema version
AGENTS.md / CLAUDE.md    # managed LLM Wiki section merged in, existing content preserved
docs/llm-wiki/           # the committed wiki:
├── index.md             #   bounded navigation map
├── system.md            #   purpose, boundaries, actors, major dependencies
├── schema.md            #   page format and maintenance policy
├── components/          #   services, modules, packages, frontends, CLIs
├── flows/               #   end-to-end behavior across components
├── contracts/           #   APIs, events, schemas, CLI and config contracts
├── decisions/           #   append-only ADRs with supersession links
├── quality/             #   invariants, testing, failure modes
├── operations/          #   deployment, observability, recovery
├── glossary.md
├── health.md            #   contradictions, stale pages, unresolved gaps
└── log.md               #   append-only maintenance history
```

`docs/llm-wiki/`, `llm-wiki.yaml`, and the instruction files are team-owned and committed. The helper directories are per-machine and gitignored.

## How it works

Three strict layers:

1. **Evidence** (read-only): code, tests, schemas, migrations, configuration, Git history, ADRs.
2. **Compiled memory** (agent-owned, committed): durable knowledge synthesized from evidence — boundaries, contracts, invariants, decisions, failure modes. Not a conversation dump; formatting changes and implementation trivia produce no updates.
3. **Control plane**: the helper CLI, schema validation, and (planned) hooks/skills governing recall and synchronization.

Every substantive wiki page carries frontmatter with a stable ID, type, status, summary, and evidence links, verified against content fingerprints so staleness is detectable.

### Helper CLI

| Command | Purpose |
| --- | --- |
| `llm-wiki init` | Install the template into a repository (idempotent, never overwrites project files) |
| `llm-wiki status [--json]` | Schema, health items, validation errors, sync-lease and receipt state |
| `llm-wiki validate` | Structural validation: frontmatter, unique IDs, resolvable links, index bounds, evidence paths (exit 4 on errors) |
| `llm-wiki fingerprint --page <path>` | Base commit + evidence fingerprint for one page's cited evidence |
| `llm-wiki version` | Installed helper version |

## Agent integration status

**Working today:** the installer merges a concise LLM Wiki section into `AGENTS.md`/`CLAUDE.md`, so agents that read those files know to treat `docs/llm-wiki/` as project memory and to run the helper for validation. The underlying primitives for the autonomous lifecycle — evidence fingerprints, materiality classification of changed paths, per-worktree sync leases, and synchronization receipts — are implemented and unit-tested in `internal/`.

**Not shipped yet (designed, see `docs/superpowers/specs/`):** the `hook`, `receipt`, `finalize-init`, and `migrate` subcommands; SessionStart/Stop hook wiring; the recall/sync/audit skills; and plugin packaging for Claude Code and Codex. There is intentionally no `.claude/` directory in consuming projects — the design places hooks and skills in a once-per-user plugin, not in every repository. Until that plugin ships, nothing fires automatically; agents follow the instruction block.

Interim manual wiring for Claude Code (optional): a project can add a Stop hook to `.claude/settings.json` that runs `./.llm-wiki/llm-wiki validate` so broken wiki structure surfaces at session end.

## Release and delivery

GitHub Releases hold the immutable versioned archives and checksums. A Cloudflare Worker at `llm-wiki-dev.salesshortcut.ai` serves `install.sh` and per-version release manifests, and redirects archive downloads to the matching GitHub release — any released version remains installable via `--version`. See `docs/maintenance.md` for the release procedure.

## Development

```sh
make verify                          # go test ./..., go vet, build
node --test cloudflare/worker.test.js  # Worker route tests
scripts/package-release.sh --version X.Y.Z --target <os-arch> --output dist
scripts/verify-release.sh dist
```

Design history and plans live in `docs/superpowers/specs/` and `docs/superpowers/plans/`.

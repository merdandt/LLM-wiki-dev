# LLM Wiki

**Give your coding agents a memory that survives the session.**

One command installs a team-shared, Git-versioned project wiki that agents like Claude Code and Codex read before working and quietly keep up to date after making changes.

```sh
curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash
```

## The problem

Coding agents have amnesia. Every new session, they re-read the same files, re-discover the same architecture, re-ask the same questions — and re-make the same mistakes. The knowledge they build up during a session (why a decision was made, which invariant must never break, what that one confusing module actually does) evaporates when the session ends.

Notes files and `CLAUDE.md` help, but they turn into unstructured dumps: half-true, never validated, and too big to load into context. And they belong to one person's machine, not the team.

## The idea (Karpathy's LLM Wiki)

[Andrej Karpathy's LLM Wiki concept](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) proposes a fix with **three strict layers**:

1. **Evidence** — the raw, immutable truth. For software: the code itself, tests, schemas, configs, migrations, Git history, accepted decisions. Nobody edits truth; it just *is*.
2. **Compiled wiki** — knowledge an LLM *synthesizes* from that evidence and maintains incrementally: what the system does, where the boundaries are, what must not break, why decisions were made. Compiled means integrated — cross-referenced and contradiction-checked — not a transcript dump.
3. **Rules** — a schema and maintenance policy governing how the wiki evolves: what counts as durable knowledge, what gets discarded, how staleness is detected.

The key property: knowledge is **integrated once, then maintained incrementally** — instead of being re-derived from scratch in every session.

## Our solution

We apply those layers to software repositories:

| Karpathy's layer | In your repo |
| --- | --- |
| Evidence | Your code, tests, schemas, Git history — read-only to the wiki |
| Compiled wiki | `docs/llm-wiki/` — committed to Git, owned by the whole team |
| Rules | `llm-wiki.yaml` + a validator + (soon) hooks that keep it honest |

The wiki is **committed and shared**: when your teammate's agent learns something durable, your agent knows it too after `git pull`. Local caches and bookkeeping stay gitignored.

The wiki stores what agents repeatedly need but can't cheaply re-derive:

- **components/** — what each service, module, and CLI is responsible for
- **flows/** — how behavior travels across components end to end
- **contracts/** — APIs, events, schemas, config formats and who consumes them
- **decisions/** — append-only architecture decisions, with links when one supersedes another
- **quality/** — invariants that must hold, testing strategy, known failure modes
- **operations/** — deployment, observability, recovery
- plus an index, glossary, health page (open contradictions), and maintenance log

It deliberately does **not** store code copies, dependency lists, formatting changes, or failed debugging hypotheses. Durable knowledge in; noise out.

### How maintenance works: the sticky-note trick

A "session" with an agent is many prompts, so *when* should the wiki update? Not on a timer, and not on every prompt. The design is simpler:

Every time the wiki is confirmed up to date, the helper saves a local note: *"wiki is good as of code-state ABC."* After each agent response, a millisecond-fast check compares the current code against that note:

- **You only asked questions.** Nothing changed. → silent.
- **You fixed a typo in a doc.** Changed, but not knowledge the wiki stores. → silent.
- **You added rate limiting to the API.** Real code drifted from the note. → the agent gets one quiet nudge before finishing: it updates the API contract page, saves a new note. Done.
- **You're mid-feature and typed "continue".** The agent decides "nothing durable yet," notes that decision, and won't be asked again until the code moves further.
- **You pulled 5 teammate commits overnight.** Next session start notices commits it never checked and runs the same quiet pass once.

The check itself needs no AI — it's deterministic file comparison. The *judgment* (what knowledge actually changed) is made by the coding agent already working in your repo, which is why it can be quiet: no second model, no API keys, no background daemon, no network calls.

## How to use it

From the root of any Git repository:

```sh
curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash
```

Needs only Git, a POSIX shell, `curl`, `tar`, and a SHA-256 tool. No Node, Python, Go, or Dart. Safe to rerun — it's idempotent.

Options (pass after `bash -s --`):

| Flag | Effect |
| --- | --- |
| `--version X.Y.Z` | Install a specific immutable release |
| `--no-init` | Install the helper binary without touching repo files |
| `--global` | Install to `~/.local/bin` instead of the project |
| `--dry-run` | Download and verify only |

What lands in your repo:

```
docs/llm-wiki/           # the wiki — COMMIT this, it's the team memory
llm-wiki.yaml            # config + schema version — commit
AGENTS.md / CLAUDE.md    # a short LLM Wiki section merged in (your content preserved) — commit
.llm-wiki/               # helper binary + template — gitignored, per-machine
.llm-wiki-state/         # local notes ("sticky notes"), locks — gitignored
```

Teammates don't pull binaries from Git — each machine just runs the same install command once.

Then use the helper:

| Command | What it does |
| --- | --- |
| `llm-wiki status` | Is the wiki healthy? Validation errors, open contradictions, sync state |
| `llm-wiki validate` | Full structural check: metadata, unique IDs, working links, evidence paths exist |
| `llm-wiki fingerprint --page <p>` | Show what evidence a page cites and whether it moved |
| `llm-wiki init` | (Re)install the template — never overwrites your files |
| `llm-wiki version` | Installed version |

## What works today, what to expect

**Working now:**

- One-command install with checksum verification, five platforms (macOS/Linux/Windows, arm64/amd64), idempotent reruns
- The full wiki template and schema
- Safe merging into existing `AGENTS.md`/`CLAUDE.md` (marked sections, your content untouched)
- The structural validator, status reporting, and evidence fingerprinting
- All the internal machinery for the quiet lifecycle — change classification, sync notes ("receipts"), per-worktree locks — implemented and unit-tested

**Not shipped yet:**

- The automatic hook wiring (SessionStart / Stop) and the recall/sync/audit skills, packaged as a once-per-user plugin for Claude Code and Codex

**So what should you expect right now?** After install, agents that read `AGENTS.md`/`CLAUDE.md` (both Claude Code and Codex do) are instructed to treat `docs/llm-wiki/` as project memory: read relevant pages before working, update them after durable changes, and run `llm-wiki validate`. That works today, but it relies on the agent following instructions — nothing *fires automatically* yet. The sticky-note lifecycle above describes the designed behavior that the upcoming hook + plugin release automates. There is intentionally no `.claude/` folder in your project: hooks and skills will ship in the plugin, installed once per user, so projects only carry team-owned files.

Until then, an impatient project can wire an interim check manually — a Claude Code `Stop` hook in `.claude/settings.json` that runs `./.llm-wiki/llm-wiki validate` so a broken wiki surfaces at the end of a session.

## How releases are delivered

GitHub Releases hold immutable versioned archives with checksums. A Cloudflare Worker at `llm-wiki-dev.salesshortcut.ai` serves the installer and version manifests, and redirects archive downloads to the matching GitHub release — so any released version stays installable forever via `--version`. Release procedure: `docs/maintenance.md`.

## Development

```sh
make verify                            # go test ./..., go vet, build
node --test cloudflare/worker.test.js  # delivery-layer route tests
scripts/package-release.sh --version X.Y.Z --target <os-arch> --output dist
scripts/verify-release.sh dist
```

Design history: `docs/superpowers/specs/` and `docs/superpowers/plans/`.

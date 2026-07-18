# LLM Wiki Lifecycle Hooks Design

**Date:** 2026-07-18
**Status:** Approved
**Supersedes:** the platform-adapter portion of `2026-07-16-llm-wiki-software-development-design.md` (Design 2 lifecycle remains authoritative; the dual-protocol assumption is obsolete)

## Goal

Ship the autonomous maintenance lifecycle as real hooks for Claude Code and Codex: a session-start hook that orients the agent to the wiki, and a stop hook that quietly enforces one wiki + README maintenance pass when durable knowledge drifts.

## Key research findings (verified 2026-07-17)

- Claude Code: SessionStart hooks (matchers `startup|resume|clear|compact`) inject context via plain stdout. Stop hooks return control by printing `{"decision":"block","reason":"..."}` on exit 0; `reason` becomes the agent's next instruction. Input includes `stop_hook_active` (true when a prior block in this turn already occurred); Claude Code hard-caps at 8 consecutive blocks. Hooks live in committable `.claude/settings.json`. `$CLAUDE_PROJECT_DIR` is available in commands.
- Codex CLI (stable since v0.124, enabled by default): a Claude-Code-compatible hooks system. Same `hooks.json` schema, same stdin JSON (plus `model`, `turn_id`), same stdout context injection for SessionStart, same `{"decision":"block","reason":...}` continuation for Stop. Project config at `.codex/hooks.json`, loaded after one-time approval via `/hooks`. No SessionEnd event on either platform matters to us — Stop is the last moment an agent can still act.
- Consequence: **one protocol serves both platforms.** No platform detection, no output adapters. The speculative Codex protocol in the archived M3 plan (`continue`/`stopReason` output, model-based detection) must not be implemented.

## Decisions

1. **Distribution: project-level (Approach A).** `llm-wiki init` writes hook configs into the target project; both files are committed so the team receives hooks via `git pull`. Plugin packaging remains a later milestone and will reuse the same hook subcommands unchanged.
2. **README maintenance scope:** when a session's changes affect sections the project README documents (features, usage, install steps, API), the same Stop maintenance pass updates those sections. The README stays human-authored; the agent keeps it truthful. No regeneration, no separate hook.
3. **Scope guard:** the four skills (wiki-init/recall/sync/audit) and plugin packaging are excluded. The Stop `reason` text carries the sync instructions inline.

## CLI surface

Two new subcommand groups routed in `internal/cli`:

### `llm-wiki hook session-start`

Reads hook JSON from stdin (`session_id`, `cwd`, `hook_event_name`, optional `source`). Behavior:

1. If `cwd` is not in a Git repo, `llm-wiki.yaml` is absent, or `initialized: false` → exit 0, no output.
2. Record/refresh the session baseline in `.llm-wiki-state/` (existing `state.Session`, keyed by `session_id`; reuse binds only to the same worktree).
3. Unseen-commit detection: compare HEAD against the stored `state.Observation`. Non-material unseen changes (per `materiality.ClassifyPaths`) → write an automatic `no-update` receipt and advance the observation silently. Material unseen changes or validation problems → set `StartupAudit` on the session.
4. Print an orientation packet to stdout, hard-capped at 1024 bytes:

```
LLM Wiki: this repo has team memory at <wiki_path>/.
Before exploring code, read <wiki_path>/index.md and follow links to relevant
pages (components/, flows/, contracts/, decisions/, quality/, operations/).
Shortcuts: `.llm-wiki/llm-wiki status --json` (health), `.llm-wiki/llm-wiki validate` (structure).
After durable changes the Stop hook may request one quiet maintenance pass.
[health: N issues | schema S | startup audit: yes/no]
```

Errors inside the hook never block the session: on internal failure, exit 0 with a single stderr line.

### `llm-wiki hook stop`

Reads hook JSON from stdin. State machine (deterministic, no LLM):

| Outcome | Condition | Output |
| --- | --- | --- |
| clean | `stop_hook_active` true, uninitialized repo, missing session, or no material change vs. baseline/receipt | none, exit 0 |
| synchronized | Current fingerprint matches a stored receipt | none, exit 0; baseline/observation advance |
| drift | Material change (via `materiality.ClassifyPaths` + `BuildFingerprint`) with no matching receipt | `{"decision":"block","reason":<instructions>}`, exit 0 |
| failure | `RecoveryPasses` ≥ `maintenance.max_recovery_passes`, or lease held by another owner | one concise stderr warning, exit 0 — never blocks completion |

Order of checks: honor `stop_hook_active` first (immediate silent exit). Acquire the worktree lease (existing `lock.Acquire` with `lock_wait_seconds` / `sync_lease_seconds`) before fingerprinting; on drift the lease is intentionally left held so the maintenance pass runs under it; `receipt write` releases it. On clean/synchronized the lease is released immediately.

The drift `reason` text (single string, ≤1200 chars):

> Durable project changes are not yet reflected in team memory. Before finishing: (1) review your diff and update the affected pages under `<wiki_path>/` (components/flows/contracts/decisions/quality/operations; update index.md and append one log.md entry); (2) if the changes affect anything the project README documents — features, usage, install steps, API — update those README sections too; (3) run `.llm-wiki/llm-wiki validate` and fix errors; (4) finish with `.llm-wiki/llm-wiki receipt write --kind synced` (or `--kind no-update --reason "<why>"` if nothing durable changed). Do not modify application code in this pass.

### `llm-wiki receipt write`

Flags: `--kind synced|no-update`, `--reason TEXT` (required for `no-update`, ≤500 chars), `--root PATH`.

1. Requires an active lease for this worktree (the one Stop left held); exit 5 when absent, 6 when another owner holds it.
2. Runs strict validation; refuses either kind while validation errors remain (exit 4).
3. Builds the current fingerprint (`BuildFingerprint`), writes the receipt atomically, advances session baseline + observation, resets `RecoveryPasses`, releases the lease.
4. Silent on success.

### Fingerprinting

`BuildFingerprint` (new, `internal/hook`): partitions repository state into an evidence fingerprint (worktree patch + untracked files, excluding wiki/state/config paths) and a wiki fingerprint (`llm-wiki.yaml` + tracked/untracked `.md` under `wiki_path`), plus `BaseCommit` and `Schema`. Uses existing `gitrepo.WorktreePatch`, `UntrackedPaths`, `Output`, and `fingerprint.Records`. The README is evidence: an agent's README edits during the maintenance pass are captured by the receipt fingerprint written afterward, so the next Stop is silent.

## Project wiring (written by `llm-wiki init`)

### `.claude/settings.json`

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "[ -x \"$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki\" ] && \"$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki\" hook session-start || exit 0",
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
            "command": "[ -x \"$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki\" ] && \"$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki\" hook stop || exit 0",
            "timeout": 15
          }
        ]
      }
    ]
  }
}
```

### `.codex/hooks.json`

Identical structure. Codex has no `$CLAUDE_PROJECT_DIR`; commands resolve the root from the repo: `root="$(git rev-parse --show-toplevel 2>/dev/null)" && [ -x "$root/.llm-wiki/llm-wiki" ] && "$root/.llm-wiki/llm-wiki" hook session-start || exit 0`.

The guard clause makes every hook a silent no-op when the helper binary is absent (teammate hasn't run the installer) or the directory was gitignored — zero noise, zero failures.

### Merge rules

- File absent → write whole file.
- File present with valid JSON → append only our hook entries; an entry is "ours" iff its command string contains `.llm-wiki/llm-wiki hook`; skip appending when already present (idempotent); never modify or reorder foreign hooks or other settings keys; preserve unknown fields.
- File present but malformed → leave untouched, print one warning naming the file.
- Implemented in Go inside `initrepo` (JSON round-trip with `map[string]any`), covered by unit tests including: empty file, foreign hooks present, ours already present, malformed JSON.

### Template and docs

- Template `AGENTS.md` managed block gains: "Read `<wiki_path>/index.md` before exploring code. The Stop hook may request one quiet maintenance pass; complete it and finish with `llm-wiki receipt write`."
- The hook config files are generated by `initrepo` code (merge logic), not shipped as template file copies, because merging into pre-existing user settings cannot be expressed as static template files.
- Project README (ours): new "How the hooks work" section — lifecycle, states table, per-scenario expectations. `docs/installation.md`: document the Codex one-time `/hooks` trust approval.

## Failure policy (unchanged from approved lifecycle design)

Hooks never make network calls, never invoke a second model, never modify application code or Git history, and never block completion beyond the guarded single pass. All hook-internal errors degrade to silent (session-start) or single-warning (stop) outcomes.

## Testing

- Unit (Go): session-start packet bounds and states; stop state machine incl. `stop_hook_active`, recovery-pass exhaustion, lease conflict; receipt gating on validation; fingerprint partitioning (source vs wiki vs state); settings/hooks JSON merge matrix.
- Integration (Go): fixture repo + piped hook JSON fixtures through the built binary asserting byte-exact silence on clean paths and exact block JSON on drift; the no-op guard with a missing binary.
- Live validation: dummy-project scenarios (feature / bug fix / no-op / teammate-pull) in both Claude Code and Codex.

## Delivery

v0.2.0 through the existing pipeline (archives + manifests + GitHub release + Worker deploy). Version gate: `make verify` green, all new tests green, end-to-end installer test incl. hook files present and idempotent rerun.

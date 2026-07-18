# Installing LLM Wiki

From a Git repository, run:

```sh
curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash
```

The bootstrap script downloads a versioned helper release, verifies its SHA-256 digest, installs it under `.llm-wiki/`, and initializes the current repository. Use `--no-init` to install without changing repository files, `--dry-run` to verify without installing, `--global` to install under `~/.local/bin`, or `--version VERSION` to select an immutable release.

The normal install requires Git, a POSIX shell, `curl`, `tar`, and a SHA-256 utility. It does not require Go, Dart, Mason, Homebrew, Node, or Python.

The generated `docs/llm-wiki/`, `llm-wiki.yaml`, `AGENTS.md`, and `CLAUDE.md` files are team-owned and should be committed. The `.llm-wiki/` helper directory holds a platform-specific binary and is added to `.gitignore` automatically; each machine reruns the install command above (it is idempotent) instead of committing the binary. Hooks and skills must invoke the local `.llm-wiki/llm-wiki` helper; they must not perform network requests during ordinary agent sessions.

Installation wires quiet lifecycle hooks for both Claude Code (`.claude/settings.json`) and Codex (`.codex/hooks.json`); commit both so the whole team gets them. Codex asks each developer to approve the project hooks once via the `/hooks` command. The hook commands are no-ops when `.llm-wiki/llm-wiki` is absent, so teammates who have not run the installer see no errors.

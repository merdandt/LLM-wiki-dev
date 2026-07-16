# Installing LLM Wiki

From a Git repository, run:

```sh
curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash
```

The bootstrap script downloads a versioned helper release, verifies its SHA-256 digest, installs it under `.llm-wiki/`, and initializes the current repository. Use `--no-init` to install without changing repository files, `--dry-run` to verify without installing, `--global` to install under `~/.local/bin`, or `--version VERSION` to select an immutable release.

The normal install requires Git, a POSIX shell, `curl`, `tar`, and a SHA-256 utility. It does not require Go, Dart, Mason, Homebrew, Node, or Python.

The generated `docs/llm-wiki/`, `llm-wiki.yaml`, `AGENTS.md`, and `CLAUDE.md` files are team-owned and should be committed. Hooks and skills must invoke the local `.llm-wiki/llm-wiki` helper; they must not perform network requests during ordinary agent sessions.

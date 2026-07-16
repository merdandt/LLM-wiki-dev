# LLM Wiki Dev Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Publish release v0.1.0, deploy the existing Cloudflare Worker/site, route the custom hostname, and verify installation and release behavior end to end.

**Architecture:** GitHub Releases hosts platform archives and checksums. A Cloudflare Worker serves the installer and release manifests from the checked-in `cloudflare/worker.mjs` logic and `cloudflare/site/` assets. Cloudflare DNS, SSL, custom-hostname, and routing configuration are performed through the Cloudflare MCP server only.

**Tech Stack:** Git, GitHub CLI/API, Cloudflare MCP (`search`/`execute`), Cloudflare Workers, DNS, shell installer, Node test runner, Make.

---

### Task 1: Inspect local state and credentials

**Files:**
- Read: `.gitignore`, `cloudflare/worker.mjs`, `cloudflare/site/`, release scripts, Makefile

- [ ] Run `git status --short --branch`, confirm commit/tag and generated release artifacts exist, and inspect deployment configuration without printing secret values.
- [ ] Run `gh auth status` and a read-only repository API check for `merdandt/LLM-wiki-dev`.
- [ ] Confirm Cloudflare MCP is enabled and the exported credential variables are non-empty without printing their contents.

### Task 2: Verify and publish GitHub release

**Files:**
- Read: `dist/SHA256SUMS`, five `dist/*.tar.gz` archives

- [ ] Verify each archive and checksum file is present and matches the release manifest expectations.
- [ ] Create release `v0.1.0` if absent, or inspect the existing release if already created.
- [ ] Upload exactly the five platform archives and `SHA256SUMS`, then verify asset names and checksums via GitHub API.

### Task 3: Deploy Cloudflare Worker and site

**Files:**
- Read: `cloudflare/worker.mjs`, `cloudflare/site/`

- [ ] Use Cloudflare MCP OpenAPI search to identify the account-scoped Worker script, route, zone, DNS, SSL, and custom-hostname endpoints.
- [ ] Read the current Cloudflare state through MCP, then deploy the existing Worker script and site assets using the repository's declared worker name and account.
- [ ] Configure the required zone, DNS record, SSL mode/settings, custom hostname, and route through Cloudflare MCP, preserving existing unrelated records.
- [ ] Re-read all changed resources and record exact statuses and any blocked operation.

### Task 4: Verify public release endpoints

**Files:**
- Read-only HTTP checks against `https://llm-wiki-dev.salesshortcut.ai`

- [ ] Verify `/install.sh`, latest and versioned release manifests, and every versioned archive redirect, including status, redirect target, and content type.

### Task 5: Run disposable installation verification

**Files:**
- Temporary disposable Git repository outside the project

- [ ] Create a disposable repository and run exactly `curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash`.
- [ ] Confirm initialization, selected archive download, checksum verification, and expected installed files.
- [ ] Run the same exact command again and confirm the rerun is idempotent.

### Task 6: Run final verification and push infrastructure changes

**Files:**
- Any remaining tracked infrastructure/configuration changes only

- [ ] Run `make verify`.
- [ ] Run `scripts/verify-release.sh dist`.
- [ ] Run `node --test cloudflare/worker.test.js`.
- [ ] Review `git diff` and `git status`; exclude secrets, tokens, generated binaries, archives, and checksum outputs from commits.
- [ ] Commit and push only remaining infrastructure changes, then verify the remote branch state.


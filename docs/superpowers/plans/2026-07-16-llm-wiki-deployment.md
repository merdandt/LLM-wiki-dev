# LLM Wiki Release and Cloudflare Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Validate, package, publish, and expose the LLM Wiki installer and release manifests at `https://llm-wiki-dev.salesshortcut.ai` with immutable archives hosted by GitHub Releases.

**Architecture:** Keep GitHub Releases as the immutable artifact origin. Add a small static Worker delivery layer for `install.sh` and versioned/latest JSON manifests, with the Worker redirecting archive paths to matching GitHub assets; configure the subdomain through the Cloudflare MCP server using the configured account and token. Use a disposable Git repository for end-to-end installer, initialization, checksum, and idempotency checks.

**Tech Stack:** Go, POSIX shell, GitHub CLI, Cloudflare Worker/static delivery, Cloudflare DNS/SSL/routing, curl, tar, SHA-256.

---

### Task 1: Baseline and plan validation

**Files:**
- Read: `README.md`, `Makefile`, `release/install.sh`, `release/release-manifest.example.json`, `scripts/package-release.sh`, `scripts/verify-release.sh`
- Create: `docs/superpowers/plans/2026-07-16-llm-wiki-deployment.md`

- [ ] **Step 1: Confirm the working tree and preserve the existing README draft.**

Run:

```bash
git status --short --branch
sed -n '1,240p' README.md
```

Expected: the pre-existing README content remains unchanged unless a later release-documentation change is explicitly required.

- [ ] **Step 2: Run the required baseline verification.**

Run:

```bash
make verify
```

Expected: `go test ./...`, `go vet ./...`, and `go build ./cmd/llm-wiki` exit successfully.

### Task 2: Build and verify the five release archives

**Files:**
- Modify: `release/release-manifest.json`
- Generate locally only: `dist/llm-wiki-*.tar.gz`, `dist/*.sha256`

- [ ] **Step 1: Select a release version and package each supported target.**

Run:

```bash
rm -rf dist
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64 windows-amd64; do
  scripts/package-release.sh --version "$VERSION" --target "$target" --output dist
done
```

Expected: exactly five archives exist with the target names above.

- [ ] **Step 2: Verify all archives and generate sorted SHA-256 checksums.**

Run:

```bash
scripts/verify-release.sh dist
find dist -maxdepth 1 -type f -name 'llm-wiki-*.tar.gz' -print0 | sort -z | xargs -0 shasum -a 256 > dist/SHA256SUMS
```

Expected: every archive is reported as verified and `SHA256SUMS` contains five archive digests.

### Task 3: Publish the GitHub Release

**Files:**
- Create remotely: Git tag and GitHub Release for `v$VERSION`
- Publish remotely: five archives and `SHA256SUMS`

- [ ] **Step 1: Confirm GitHub CLI authentication and repository identity.**

Run:

```bash
gh auth status
gh repo view --json nameWithOwner,defaultBranchRef
```

Expected: authenticated access to `merdandt/LLM-wiki-dev`; if unavailable, retain local artifacts and report the exact blocked action.

- [ ] **Step 2: Create the tag/release and upload only release artifacts.**

Run:

```bash
git tag -a "v$VERSION" -m "Release v$VERSION"
git push origin "v$VERSION"
gh release create "v$VERSION" dist/llm-wiki-*.tar.gz dist/SHA256SUMS --title "LLM Wiki v$VERSION" --generate-notes
```

Expected: the release page exposes all five archives and the checksum file.

### Task 4: Add and test the static Cloudflare delivery layer

**Files:**
- Create: `cloudflare/worker.js`
- Create: `cloudflare/wrangler.toml`
- Create: `cloudflare/README.md`
- Test: `cloudflare/worker.test.js` or an equivalent deterministic route test

- [ ] **Step 1: Write failing route tests.**

Tests must assert that `/install.sh`, `/releases/latest/release-manifest.json`, and `/releases/$VERSION/release-manifest.json` return the tracked static content, while `/releases/$VERSION/<archive>` redirects to the exact GitHub asset URL and unrelated paths return 404.

- [ ] **Step 2: Run the route tests and observe the expected failure.**

Run the project’s available JavaScript test command or a focused Node test command. Expected: failure because the Worker implementation is not present.

- [ ] **Step 3: Implement the minimal Worker and static assets.**

The Worker shall serve the tracked installer and manifests, use immutable release filenames, return explicit content types and cache headers, and issue HTTPS 302 redirects for archive paths. It shall not contain credentials.

- [ ] **Step 4: Run route tests, lint/check the Worker, and inspect the generated configuration.**

Expected: all route tests pass and the Worker configuration contains only public release metadata and the custom hostname binding.

### Task 5: Configure Cloudflare through the configured `cloudflare` MCP server

**Files:**
- Modify: `cloudflare/wrangler.toml` if the Cloudflare account’s zone/Worker identifiers require it

- [ ] **Step 1: Verify MCP availability.**

Run:

```bash
codex mcp list
```

Expected: `cloudflare` is enabled. Use only Cloudflare MCP tools for zone, Worker, DNS, SSL, and routing operations; do not use direct Cloudflare REST calls or a separate CLI for those operations.

- [ ] **Step 2: Create/update the Worker and route.**

Deploy the Worker with the static installer and manifest assets, bind it to `llm-wiki-dev.salesshortcut.ai/*`, and configure production HTTPS delivery.

- [ ] **Step 3: Configure DNS and SSL.**

Create or update the subdomain record required by the Worker route, enable proxied HTTPS, and set the zone SSL mode to the least permissive mode compatible with end-to-end HTTPS. Do not alter unrelated DNS records.

- [ ] **Step 4: Verify the resulting zone, DNS, SSL, route, and Worker state through MCP.**

Expected: the hostname is attached to the Worker, DNS resolves through Cloudflare, and the deployment is active.

### Task 6: End-to-end installer smoke test and idempotency check

**Files:**
- Generate outside the repository: disposable test repository and temporary install log

- [ ] **Step 1: Create a disposable Git repository outside this checkout.**

Run:

```bash
tmp_repo=$(mktemp -d)
git -C "$tmp_repo" init
git -C "$tmp_repo" config user.email test@example.com
git -C "$tmp_repo" config user.name test
```

- [ ] **Step 2: Install from the public endpoint.**

Run:

```bash
cd "$tmp_repo"
curl -fsSL https://llm-wiki-dev.salesshortcut.ai/install.sh | bash
```

Expected: the helper and template initialize `.llm-wiki/` and the wiki files; output shows a successful install.

- [ ] **Step 3: Confirm checksum verification and idempotent rerun.**

Run the installer a second time in the same disposable repository, then compare `git status --short` and the managed file contents before and after. Expected: checksum verification occurs and the second run makes no additional repository changes.

### Task 7: Commit and push repository changes without forbidden files

**Files:**
- Commit tracked infrastructure, release metadata, docs, and tests
- Exclude: secrets, tokens, generated binaries, temporary archives, and disposable repositories

- [ ] **Step 1: Audit the staged file list.**

Run:

```bash
git status --short
git diff --check
git ls-files dist
```

Expected: no generated archive is tracked and no secret/token file is staged.

- [ ] **Step 2: Run final verification.**

Run:

```bash
make verify
scripts/verify-release.sh dist
```

Expected: both commands exit 0.

- [ ] **Step 3: Commit and push the repository changes.**

Run:

```bash
git add cloudflare release scripts docs README.md Makefile
git commit -m "release: deploy llm-wiki installer"
git push origin main
```

Expected: the team branch contains the infrastructure and repository changes, with the existing README draft preserved.

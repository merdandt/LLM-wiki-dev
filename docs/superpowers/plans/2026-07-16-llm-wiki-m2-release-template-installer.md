# LLM Wiki Milestone 2 Release Template and Installer Plan

> Execute with TDD. Each task starts with a failing test, runs the named verification, and commits the completed task.

Goal: replace Mason/Dart initialization with a versioned repository template, a verified cross-platform release bundle, and a one-command installer at https://llm-wiki-dev.salesshortcut.ai/install.sh.

Architecture: ordinary tracked files under template/ are the source of truth. The Go helper owns rendering, safe extraction, initialization, updates, migrations, and validation. GitHub Releases stores immutable archives. Cloudflare serves only the bootstrap script and release metadata. Ordinary hooks use the locally installed helper and never access the network.

End-user prerequisites: Git and a supported shell. Go, Dart, Mason, Homebrew, Python, and Node are not required.

## Release contract

release/release-manifest.json contains the selected version, GitHub release base URL, installer URL, and one SHA-256 digest for each supported target: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64, and windows-amd64.

The installer downloads the manifest and matching archive over HTTPS, verifies the exact lowercase SHA-256 digest before extraction, and refuses unsupported targets, traversal entries, symlinks, hard links, or missing template content. The manifest URL is version-pinned for a release; the script never fetches mutable main-branch content.

## Task 1: Create the ordinary repository template

Files:
- template/llm-wiki.yaml
- template/.gitignore.append
- template/AGENTS.md
- template/CLAUDE.md
- template/docs/llm-wiki/index.md
- template/docs/llm-wiki/system.md
- template/docs/llm-wiki/schema.md
- template/docs/llm-wiki/glossary.md
- template/docs/llm-wiki/health.md
- template/docs/llm-wiki/log.md
- template/docs/llm-wiki/quality/invariants.md
- template/docs/llm-wiki/quality/testing.md
- template/docs/llm-wiki/quality/failure-modes.md
- template/docs/llm-wiki/decisions/_template.md
- empty marker files under components/, flows/, contracts/, and operations/
- internal/initrepo/template_test.go

The template uses the default docs/llm-wiki path and uninitialized verification fields. The helper renders project name, wiki path, context budget, and current UTC date. AGENTS.md and CLAUDE.md contain only managed blocks; existing user text is preserved.

Tests must prove every required page exists, the rendered config passes config.Load, no path escapes the repository, and no Mason metadata, Dart files, or project-local mason.yaml is present.

Run:
go test ./internal/initrepo -run TestTemplate -v

Commit:
git add template internal/initrepo/template_test.go
git commit -m "feat: add tracked software wiki template"

## Task 2: Verify release manifests and safely extract archives

Files:
- internal/installer/archive.go
- internal/installer/archive_test.go
- internal/installer/manifest.go
- internal/installer/manifest_test.go

Define Manifest and Artifact types with version, HTTPS release URL, archive filename, and lowercase 64-character SHA-256 fields. Reject unknown JSON fields, invalid versions, non-HTTPS URLs, archive names containing slashes, missing targets, checksum mismatches, traversal entries, symlinks, hard links, and archives without exactly one helper plus template/.

Extract to a temporary directory, validate all paths and regular-file permissions, then atomically install only validated files.

Tests cover valid manifests, malformed metadata, checksum mismatch, traversal archives, symlink archives, missing helper, and missing template.

Run:
go test ./internal/installer -v

Commit:
git add internal/installer
git commit -m "feat: verify llm-wiki release archives"

## Task 3: Initialize repositories from the template

Files:
- internal/initrepo/render.go
- internal/initrepo/init.go
- internal/initrepo/instructions.go
- internal/initrepo/gitignore.go
- internal/initrepo/init_test.go
- internal/cli/run.go
- internal/cli/run_test.go

Initialize must discover the Git root, validate repository-relative paths, reject .git and state overlap, reject symlinked parent components and non-regular managed files, render into a temporary directory, atomically copy only allowlisted template files, merge AGENTS.md/CLAUDE.md/.gitignore idempotently, validate with AllowUninitialized true, and leave initialized false until finalize-init.

It must never create mason.yaml, .dart_tool, or network state. A second run with identical arguments must produce no diff. Any failure rolls back files written during that invocation.

Tests cover clean repositories, custom wiki paths, repeated initialization, existing instruction text, existing @AGENTS.md, conflicting markers, symlink attacks, and user-authored .gitignore lines.

Run:
go test ./internal/initrepo ./internal/cli -run 'Test(Template|Initialize|RunInit)' -v

Commit:
git add internal/initrepo internal/cli
git commit -m "feat: initialize wiki from tracked template"

## Task 4: Add update, doctor, and uninstall

Files:
- internal/initrepo/update.go
- internal/initrepo/doctor.go
- internal/initrepo/uninstall.go
- internal/cli/run.go
- internal/cli/run_test.go

Commands:
llm-wiki update --root PATH [--version VERSION]
llm-wiki doctor --root PATH [--json]
llm-wiki uninstall --root PATH

update changes helper binaries, skills, hooks, rules, and managed blocks only; it never overwrites canonical wiki pages or application files. doctor is read-only and reports helper/template/schema versions, validation errors, hook availability, lease owner, and release reachability. uninstall removes only installer-owned clean files and managed blocks, preserves wiki/config by default, and refuses modified user files.

Tests prove update preserves wiki edits, doctor is read-only, and uninstall fails closed on modified files.

Run:
go test ./internal/initrepo ./internal/cli -run 'Test(Update|Doctor|Uninstall)' -v

Commit:
git add internal/initrepo internal/cli
git commit -m "feat: add safe wiki lifecycle commands"

## Task 5: Finalize initialization and migrate schemas

Files:
- internal/initrepo/finalize.go
- internal/migrate/migrate.go
- internal/migrate/migrate_test.go
- internal/cli/run.go

finalize-init validates the compiled wiki, rejects unresolved scaffold pages, records the baseline and observation, and atomically changes initialized to true. migrate requires the synchronization lease, backs up config and canonical wiki under .llm-wiki-state/backups/<timestamp>/, applies ordered idempotent migrations, validates, and rolls back every file on failure.

Run:
go test ./internal/migrate ./internal/initrepo -v

Commit:
git add internal/migrate internal/initrepo internal/cli
git commit -m "feat: finalize and migrate wiki state"

## Task 6: Publish the bootstrap installer

Files:
- release/install.sh
- release/release-manifest.json
- scripts/package-release.sh
- scripts/verify-release.sh
- .github/workflows/release.yml
- docs/installation.md
- internal/installer/install_script_test.go

install.sh accepts --version, --root, --no-init, --global, and --dry-run; detects platform/architecture; downloads the pinned manifest and archive; verifies SHA-256; extracts through the helper; installs under .llm-wiki/bin or ~/.local/bin/llm-wiki; runs llm-wiki init unless --no-init; and prints concise errors.

The script must be POSIX-compatible, quote variables, avoid eval, avoid executing template content, and never use mutable main URLs. Cloudflare serves https://llm-wiki-dev.salesshortcut.ai/install.sh; GitHub Releases remains the immutable artifact origin.

The release workflow builds five helper targets, archives the helper plus template/, generates sorted checksums, renders plugin assets, and publishes the manifest. Tests use a local file server and prove checksum failure, successful initialization, and idempotent re-run.

Run:
shellcheck release/install.sh scripts/package-release.sh scripts/verify-release.sh
go test ./internal/installer -run TestInstallScript -v

Commit:
git add release scripts .github/workflows/release.yml docs/installation.md internal/installer
git commit -m "feat: add verified bootstrap installer"

## Task 7: Prove the lifecycle end to end

Files:
- testdata/repos/clean/
- testdata/repos/existing-instructions/
- internal/initrepo/integration_test.go

Copy disposable repositories and prove clean initialization, custom paths, preserved instruction text, idempotent re-run, finalize-init baseline creation, update preservation of wiki edits, read-only doctor, fail-closed symlinks, and absence of Mason/Dart/project-local workspace artifacts.

Run:
go test ./internal/initrepo -run TestInitializationLifecycle -v
go test ./...

Commit:
git add testdata/repos internal/initrepo
git commit -m "test: verify release-template initialization lifecycle"

## Task 8: Milestone gate

Run:
go test ./internal/installer ./internal/initrepo ./internal/migrate
go test ./...
go vet ./...
make verify

Expected: helper builds, static template validates, checksum verification is enforced, initialization is idempotent, and the installer requires no Mason or Dart.

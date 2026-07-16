# LLM Wiki Milestone 4 Plugin Packaging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Package the shared skills, hooks, and native helper as installable Claude Code and Codex plugins from one repository.

**Architecture:** Store release metadata once, generate both platform manifests and both marketplace catalogs deterministically, and package five native helper targets into the shared plugin root. Use default `skills/` and `hooks/hooks.json` discovery to minimize adapter differences.

**Tech Stack:** Go 1.26.5, YAML, JSON, POSIX shell, Claude Code plugin schema, Codex plugin schema.

---

## Task 1: Define shared release metadata

**Files:**

- Create: `release/plugin-metadata.yaml`
- Create: `internal/pluginmeta/metadata.go`
- Test: `internal/pluginmeta/render_test.go`

- [ ] **Step 1: Create release metadata**

`release/plugin-metadata.yaml`:

```yaml
name: llm-wiki
display_name: LLM Wiki
version: 0.1.0
description: Maintain a team-shared software-development wiki with quiet in-loop agent hooks.
long_description: Compile architecture, components, flows, contracts, decisions, invariants, operations, and confirmed failure knowledge from repository evidence.
author:
  name: LLM Wiki contributors
  email: maintainers@llm-wiki.dev
homepage: https://github.com/merdandt/LLM-wiki-dev
repository: https://github.com/merdandt/LLM-wiki-dev
license: MIT
keywords:
  - software-development
  - project-memory
  - architecture
  - codex
  - claude-code
category: Productivity
template:
  name: software-wiki
  version: 0.1.0
helper:
  schema_version: 1
  targets:
    - darwin-amd64
    - darwin-arm64
    - linux-amd64
    - linux-arm64
    - windows-amd64
```

- [ ] **Step 2: Write a failing metadata-loading test**

```go
package pluginmeta

import "testing"

func TestLoadMetadata(t *testing.T) {
	got, err := Load("../../release/plugin-metadata.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "llm-wiki" || got.Version != "0.1.0" {
		t.Fatalf("unexpected metadata: %#v", got)
	}
	if len(got.Helper.Targets) != 5 {
		t.Fatalf("targets = %#v", got.Helper.Targets)
	}
}
```

Add table tests for an unknown YAML field, duplicate target, unsupported target, missing version, and invalid plugin name; each must fail before rendering.

- [ ] **Step 3: Run the test and verify it fails**

```bash
go test ./internal/pluginmeta -run TestLoadMetadata -v
```

Expected: FAIL because `Load` does not exist.

- [ ] **Step 4: Implement metadata parsing and validation**

`internal/pluginmeta/metadata.go`:

```go
package pluginmeta

import (
	"bytes"
	"errors"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Author struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

	type Template struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Helper struct {
	SchemaVersion int      `yaml:"schema_version"`
	Targets       []string `yaml:"targets"`
}

type Metadata struct {
	Name            string   `yaml:"name"`
	DisplayName     string   `yaml:"display_name"`
	Version         string   `yaml:"version"`
	Description     string   `yaml:"description"`
	LongDescription string   `yaml:"long_description"`
	Author          Author   `yaml:"author"`
	Homepage        string   `yaml:"homepage"`
	Repository      string   `yaml:"repository"`
	License         string   `yaml:"license"`
	Keywords        []string `yaml:"keywords"`
	Category        string   `yaml:"category"`
	Template        Template `yaml:"template"`
	Helper          Helper   `yaml:"helper"`
}

var pluginName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var semanticVersion = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)

func Load(path string) (Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, err
	}
	var metadata Metadata
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, err
	}
	if !pluginName.MatchString(metadata.Name) {
		return Metadata{}, errors.New("plugin name must be kebab-case")
	}
	if !semanticVersion.MatchString(metadata.Version) ||
		!semanticVersion.MatchString(metadata.Template.Version) ||
		metadata.Description == "" || metadata.Author.Name == "" {
		return Metadata{}, errors.New("valid plugin/template versions, description, and author name are required")
	}
	if len(metadata.Helper.Targets) == 0 {
		return Metadata{}, errors.New("at least one helper target is required")
	}
	allowedTargets := map[string]struct{}{
		"darwin-amd64": {}, "darwin-arm64": {}, "linux-amd64": {},
		"linux-arm64": {}, "windows-amd64": {},
	}
	seen := map[string]struct{}{}
	for _, target := range metadata.Helper.Targets {
		if _, ok := allowedTargets[target]; !ok {
			return Metadata{}, errors.New("unsupported helper target")
		}
		if _, duplicate := seen[target]; duplicate {
			return Metadata{}, errors.New("duplicate helper target")
		}
		seen[target] = struct{}{}
	}
	return metadata, nil
}
```

- [ ] **Step 5: Run tests and commit**

```bash
gofmt -w internal/pluginmeta
go test ./internal/pluginmeta -v
git add release/plugin-metadata.yaml internal/pluginmeta
git commit -m "feat: define plugin release metadata"
```

## Task 2: Generate Claude and Codex plugin manifests

**Files:**

- Create: `internal/pluginmeta/render.go`
- Modify: `internal/pluginmeta/render_test.go`
- Create: `plugins/llm-wiki/.claude-plugin/plugin.json`
- Create: `plugins/llm-wiki/.codex-plugin/plugin.json`
- Modify: `internal/cli/run.go`

- [ ] **Step 1: Write failing manifest golden tests**

```go
func TestRenderPluginManifests(t *testing.T) {
	metadata, err := Load("../../release/plugin-metadata.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render(metadata)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqualFile(t, "../../plugins/llm-wiki/.claude-plugin/plugin.json", rendered.ClaudePlugin)
	assertJSONEqualFile(t, "../../plugins/llm-wiki/.codex-plugin/plugin.json", rendered.CodexPlugin)
}
```

Implement `assertJSONEqualFile` by unmarshalling expected and actual into `any` and comparing with `reflect.DeepEqual`.

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/pluginmeta -run TestRenderPluginManifests -v
```

Expected: FAIL because `Render` and manifests do not exist.

- [ ] **Step 3: Implement render types**

Add to `internal/pluginmeta/render.go`:

```go
package pluginmeta

import "encoding/json"

type Rendered struct {
	ClaudePlugin      []byte
	CodexPlugin       []byte
	ClaudeMarketplace []byte
	CodexMarketplace  []byte
}

func pretty(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
```

- [ ] **Step 4: Render the Claude manifest**

Use this exact JSON shape:

```json
{
  "name": "llm-wiki",
  "displayName": "LLM Wiki",
  "version": "0.1.0",
  "description": "Maintain a team-shared software-development wiki with quiet in-loop agent hooks.",
  "author": {
    "name": "LLM Wiki contributors",
    "email": "maintainers@llm-wiki.dev"
  },
  "homepage": "https://github.com/merdandt/LLM-wiki-dev",
  "repository": "https://github.com/merdandt/LLM-wiki-dev",
  "license": "MIT",
  "keywords": [
    "software-development",
    "project-memory",
    "architecture",
    "codex",
    "claude-code"
  ]
}
```

Do not declare custom skill or hook paths; Claude discovers `skills/` and `hooks/hooks.json`.

- [ ] **Step 5: Render the Codex manifest**

Use this exact JSON shape:

```json
{
  "name": "llm-wiki",
  "version": "0.1.0",
  "description": "Maintain a team-shared software-development wiki with quiet in-loop agent hooks.",
  "author": {
    "name": "LLM Wiki contributors",
    "email": "maintainers@llm-wiki.dev",
    "url": "https://github.com/merdandt/LLM-wiki-dev"
  },
  "homepage": "https://github.com/merdandt/LLM-wiki-dev",
  "repository": "https://github.com/merdandt/LLM-wiki-dev",
  "license": "MIT",
  "keywords": [
    "software-development",
    "project-memory",
    "architecture",
    "codex",
    "claude-code"
  ],
  "skills": "./skills/",
  "interface": {
    "displayName": "LLM Wiki",
    "shortDescription": "Quiet, compiled project memory for coding agents",
    "longDescription": "Compile architecture, components, flows, contracts, decisions, invariants, operations, and confirmed failure knowledge from repository evidence.",
    "developerName": "LLM Wiki contributors",
    "category": "Productivity",
    "capabilities": [
      "Read",
      "Write"
    ],
    "websiteURL": "https://github.com/merdandt/LLM-wiki-dev",
    "defaultPrompt": [
      "Initialize LLM Wiki in this repository.",
      "Recall relevant project architecture before this change.",
      "Synchronize durable project knowledge after this change."
    ]
  }
}
```

Rely on the default `hooks/hooks.json` location so older Codex validators that reject a manifest `hooks` field remain compatible.

- [ ] **Step 6: Route `plugin render`**

The command:

1. Loads `release/plugin-metadata.yaml`.
2. Calls `Render`.
3. Writes all four generated JSON files with `atomicfile.Write(..., 0o644)`.
4. Produces no output when files were already current.
5. Prints only changed paths when generation updates files.

- [ ] **Step 7: Generate, test, and commit**

```bash
go run ./cmd/llm-wiki plugin render
gofmt -w internal/pluginmeta internal/cli
go test ./internal/pluginmeta ./internal/cli -v
git add internal/pluginmeta internal/cli plugins/llm-wiki/.claude-plugin plugins/llm-wiki/.codex-plugin
git commit -m "feat: generate dual plugin manifests"
```

## Task 3: Generate both marketplace catalogs

**Files:**

- Modify: `internal/pluginmeta/render.go`
- Modify: `internal/pluginmeta/render_test.go`
- Create: `.claude-plugin/marketplace.json`
- Create: `.agents/plugins/marketplace.json`

- [ ] **Step 1: Add failing marketplace golden tests**

```go
func TestRenderMarketplaces(t *testing.T) {
	metadata, err := Load("../../release/plugin-metadata.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render(metadata)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqualFile(t, "../../.claude-plugin/marketplace.json", rendered.ClaudeMarketplace)
	assertJSONEqualFile(t, "../../.agents/plugins/marketplace.json", rendered.CodexMarketplace)
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/pluginmeta -run TestRenderMarketplaces -v
```

Expected: FAIL because marketplace output is empty.

- [ ] **Step 3: Render the Claude marketplace**

Exact shape:

```json
{
  "name": "llm-wiki",
  "owner": {
    "name": "LLM Wiki contributors",
    "email": "maintainers@llm-wiki.dev"
  },
  "description": "Software-development project memory plugins.",
  "plugins": [
    {
      "name": "llm-wiki",
      "source": "./plugins/llm-wiki",
      "displayName": "LLM Wiki",
      "description": "Maintain a team-shared software-development wiki with quiet in-loop agent hooks.",
      "version": "0.1.0",
      "category": "Productivity",
      "tags": [
        "software-development",
        "project-memory",
        "architecture"
      ]
    }
  ]
}
```

- [ ] **Step 4: Render the Codex marketplace**

Exact shape:

```json
{
  "name": "llm-wiki",
  "interface": {
    "displayName": "LLM Wiki"
  },
  "plugins": [
    {
      "name": "llm-wiki",
      "source": {
        "source": "local",
        "path": "./plugins/llm-wiki"
      },
      "policy": {
        "installation": "AVAILABLE",
        "authentication": "ON_INSTALL"
      },
      "category": "Productivity"
    }
  ]
}
```

- [ ] **Step 5: Generate and test**

```bash
go run ./cmd/llm-wiki plugin render
go test ./internal/pluginmeta -v
```

Expected: PASS and both catalogs are formatted with two-space JSON indentation.

- [ ] **Step 6: Commit**

```bash
git add internal/pluginmeta .claude-plugin .agents/plugins
git commit -m "feat: add Claude and Codex marketplaces"
```

## Task 4: Build and checksum all native helper targets

**Files:**

- Create: `.gitattributes`
- Create: `scripts/package-release.sh`
- Create: `plugins/llm-wiki/assets/template/`
- Create: `plugins/llm-wiki/assets/release-checksums.json`
- Create: `internal/pluginmeta/checksums.go`
- Create: `internal/pluginmeta/checksums_test.go`
- Modify: `.gitignore`
- Modify: `Makefile`

- [ ] **Step 1: Create binary attributes**

`.gitattributes`:

```gitattributes
plugins/llm-wiki/bin/** binary
*.sh text eol=lf
*.ps1 text eol=lf
*.json text eol=lf
*.yaml text eol=lf
*.yml text eol=lf
```

- [ ] **Step 2: Create the packaging script**

`scripts/package-release.sh`:

```sh
#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$repo_root"

go run ./cmd/llm-wiki plugin render >/dev/null
version="$(awk '/^version:/ { print $2; exit }' release/plugin-metadata.yaml)"
ldflags="-s -w -X github.com/merdandt/LLM-wiki-dev/internal/cli.Version=$version"

rm -rf plugins/llm-wiki/assets/template
mkdir -p plugins/llm-wiki/assets/template
git ls-files template |
  while IFS= read -r source; do
    relative="${source#template/}"
    destination="plugins/llm-wiki/assets/template/$relative"
    mkdir -p "$(dirname "$destination")"
    cp "$source" "$destination"
  done

find plugins/llm-wiki/bin -type f \
  \( -name llm-wiki -o -name llm-wiki.exe \) -delete

build() {
  goos="$1"
  goarch="$2"
  target="$3"
  suffix="$4"
  output="plugins/llm-wiki/bin/$target/llm-wiki$suffix"
  mkdir -p "$(dirname "$output")"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$ldflags" -o "$output" ./cmd/llm-wiki
}

build darwin amd64 darwin-amd64 ""
build darwin arm64 darwin-arm64 ""
build linux amd64 linux-amd64 ""
build linux arm64 linux-arm64 ""
build windows amd64 windows-amd64 ".exe"

go run ./cmd/llm-wiki plugin checksums \
  --root plugins/llm-wiki \
  --output plugins/llm-wiki/assets/release-checksums.json
```

- [ ] **Step 3: Implement `plugin checksums`**

Implement `pluginmeta.BuildChecksums(root, version string) (ChecksumManifest, error)` in `internal/pluginmeta/checksums.go`. It walks `root/bin`, skips `.gitkeep`, rejects symlinks and unexpected filenames, hashes only `llm-wiki` or `llm-wiki.exe` regular files with SHA-256, stores slash-normalized paths relative to `root`, and returns:

```json
{
  "version": "0.1.0",
  "algorithm": "sha256",
  "files": {
    "bin/darwin-amd64/llm-wiki": "hex-digest",
    "bin/darwin-arm64/llm-wiki": "hex-digest",
    "bin/linux-amd64/llm-wiki": "hex-digest",
    "bin/linux-arm64/llm-wiki": "hex-digest",
    "bin/windows-amd64/llm-wiki.exe": "hex-digest"
  }
}
```

`internal/pluginmeta/checksums_test.go`:

```go
package pluginmeta

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildChecksumsHashesFilesAndMarshalsSortedPaths(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"bin/z-target/llm-wiki": "z",
		"bin/a-target/llm-wiki": "a",
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := BuildChecksums(root, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	for relative, content := range files {
		sum := sha256.Sum256([]byte(content))
		if got.Files[relative] != hex.EncodeToString(sum[:]) {
			t.Fatalf("%s digest = %q", relative, got.Files[relative])
		}
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	aIndex := bytes.Index(data, []byte("bin/a-target"))
	zIndex := bytes.Index(data, []byte("bin/z-target"))
	if aIndex < 0 || zIndex < 0 || aIndex > zIndex {
		t.Fatalf("paths are not sorted:\n%s", data)
	}
}
```

Route `llm-wiki plugin checksums --root PATH --output PATH` through this function, load the version from shared metadata, append one trailing newline, and write the output with `atomicfile.Write(..., 0o644)`.

- [ ] **Step 4: Update ignore and build targets**

Keep development binaries ignored:

```gitignore
plugins/llm-wiki/bin/*/llm-wiki
plugins/llm-wiki/bin/*/llm-wiki.exe
```

Add:

```make
.PHONY: package

package:
	./scripts/package-release.sh
```

- [ ] **Step 5: Build all targets**

```bash
chmod +x scripts/package-release.sh
./scripts/package-release.sh
```

Expected: the source template is copied into plugin assets, and five binaries plus one checksum manifest are generated.

- [ ] **Step 6: Verify embedded versions**

Run the native binary for the current platform:

```bash
plugins/llm-wiki/bin/darwin-arm64/llm-wiki version
```

Expected:

```text
llm-wiki 0.1.0
```

Use the matching local target when not on macOS ARM64.

- [ ] **Step 7: Commit packaging source and release artifacts**

```bash
git add .gitattributes .gitignore Makefile scripts/package-release.sh internal/pluginmeta internal/cli plugins/llm-wiki/assets
git add -f plugins/llm-wiki/bin/*/llm-wiki plugins/llm-wiki/bin/*/llm-wiki.exe
git commit -m "build: package cross-platform plugin binaries"
```

## Task 5: Verify manifests, skills, hooks, and checksums as one package

**Files:**

- Create: `scripts/verify-release.sh`
- Create: `internal/pluginmeta/package_test.go`
- Modify: `Makefile`

- [ ] **Step 1: Write a failing package-integrity test**

The Go test must assert:

- Both manifest names and versions equal shared metadata.
- Both marketplace entries point to `./plugins/llm-wiki`.
- Exactly four skill directories exist.
- `hooks/hooks.json` exists.
- `assets/template/llm-wiki.yaml` exists and its version equals shared template metadata.
- Every metadata target has one binary.
- Every binary matches `release-checksums.json`.
- Mach-O, ELF, and PE magic bytes match each declared target, and Unix binaries have an executable bit.
- No path in either manifest escapes the plugin root.

- [ ] **Step 2: Run the test and verify any missing checks fail**

```bash
go test ./internal/pluginmeta -run TestPackageIntegrity -v
```

Expected: FAIL until all package checks are implemented.

- [ ] **Step 3: Create the release verifier**

`scripts/verify-release.sh`:

```sh
#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$repo_root"

go run ./cmd/llm-wiki plugin render
git diff --exit-code -- \
  .claude-plugin/marketplace.json \
  .agents/plugins/marketplace.json \
  plugins/llm-wiki/.claude-plugin/plugin.json \
  plugins/llm-wiki/.codex-plugin/plugin.json \
  plugins/llm-wiki/assets/template \
  plugins/llm-wiki/assets/release-checksums.json \
  plugins/llm-wiki/bin

go test ./internal/pluginmeta -run TestPackageIntegrity -v
```

- [ ] **Step 4: Add optional official CLI validation**

Append this non-destructive validation to `scripts/verify-release.sh`:

```sh
if command -v claude >/dev/null 2>&1; then
  claude plugin validate plugins/llm-wiki --strict
  claude plugin validate .
fi

if command -v codex >/dev/null 2>&1; then
  codex_home="$(mktemp -d)"
  trap 'rm -rf "$codex_home"' EXIT HUP INT TERM
  CODEX_HOME="$codex_home" codex plugin marketplace add "$repo_root"
  CODEX_HOME="$codex_home" codex plugin marketplace list
fi
```

The temporary `CODEX_HOME` prevents validation from changing the maintainer's real plugin configuration. The verifier remains optional when either executable is absent and fails when an installed client rejects the package.

- [ ] **Step 5: Update the gate**

```make
.PHONY: release-verify

release-verify:
	./scripts/verify-release.sh

verify: fmt test vet build package hook-test release-verify
```

- [ ] **Step 6: Run and commit**

```bash
chmod +x scripts/verify-release.sh
make verify
git add scripts/verify-release.sh internal/pluginmeta Makefile
git commit -m "test: verify dual-platform plugin package"
```

## Task 6: Smoke-test local marketplace installation

**Files:**

- Create: `testdata/plugins/local-install.md`
- Create: `internal/pluginmeta/install_smoke_test.go`

- [ ] **Step 1: Document exact local smoke commands**

`testdata/plugins/local-install.md`:

````markdown
# Local plugin smoke test

## Claude Code

```bash
claude plugin marketplace add .
claude plugin install llm-wiki@llm-wiki --scope local
claude plugin list
```

Expected: `llm-wiki@llm-wiki` is installed and enabled.

## Codex

```bash
codex plugin marketplace add .
codex plugin marketplace list
```

Then install `llm-wiki` from the local `llm-wiki` marketplace in the Codex plugin interface.

Expected: the plugin exposes four skills and the SessionStart/Stop hooks.
````

- [ ] **Step 2: Add a non-destructive smoke test**

The test creates a temporary checkout root, copies `.claude-plugin`, `.agents`, and `plugins`, parses both copied marketplace files, resolves every local source against that copied checkout root, and asserts the resolved directory contains both plugin manifests, `hooks/hooks.json`, and four `SKILL.md` files. It sets temporary `CLAUDE_CONFIG_DIR` and `CODEX_HOME` values for any subprocess and asserts the real values are never used.

- [ ] **Step 3: Run the milestone gate**

```bash
make verify
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add testdata/plugins internal/pluginmeta
git commit -m "test: add local plugin installation smoke test"
```

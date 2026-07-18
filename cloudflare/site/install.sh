#!/bin/sh
set -eu

manifest_root=${LLM_WIKI_MANIFEST_ROOT:-https://llm-wiki-dev.salesshortcut.ai/releases}
version=
root=.
no_init=0
global=0
dry_run=0

usage() {
  echo "Usage: install.sh [--version VERSION] [--root PATH] [--no-init] [--global] [--dry-run]"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; version=$2; shift 2 ;;
    --root) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; root=$2; shift 2 ;;
    --no-init) no_init=1; shift ;;
    --global) global=1; shift ;;
    --dry-run) dry_run=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "llm-wiki: unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

have() { command -v "$1" >/dev/null 2>&1; }
for command in curl tar awk sed mktemp find install; do
  have "$command" || { echo "llm-wiki: required command not found: $command" >&2; exit 3; }
done

sha256() {
  if have sha256sum; then sha256sum "$1" | awk '{print $1}'; return; fi
  if have shasum; then shasum -a 256 "$1" | awk '{print $1}'; return; fi
  echo "llm-wiki: sha256sum or shasum is required" >&2
  exit 3
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in darwin|linux) ;; *) echo "llm-wiki: unsupported operating system: $os" >&2; exit 3 ;; esac
arch=$(uname -m)
case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "llm-wiki: unsupported architecture: $arch" >&2; exit 3 ;; esac
target=$os-$arch

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t llm-wiki)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
manifest_url=$manifest_root/latest/release-manifest.json
[ -n "$version" ] && manifest_url=$manifest_root/$version/release-manifest.json
manifest=$tmp/release-manifest.json
curl -fsSL "$manifest_url" -o "$manifest" || { echo "llm-wiki: unable to download release manifest: $manifest_url" >&2; exit 4; }

release_base=$(sed -n 's/.*"release_base_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$manifest" | sed -n '1p')
archive=$(sed -n "s/.*\"$target.archive\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$manifest" | sed -n '1p')
expected=$(sed -n "s/.*\"$target.sha256\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$manifest" | sed -n '1p')
[ -n "$release_base" ] && [ -n "$archive" ] && [ -n "$expected" ] || { echo "llm-wiki: no supported artifact for $target" >&2; exit 4; }
case "$release_base/$archive" in https://*) ;; *) echo "llm-wiki: manifest release URL must use HTTPS" >&2; exit 4 ;; esac
if ! awk -v value="$expected" 'BEGIN { exit !(length(value) == 64 && value !~ /[^0-9a-f]/) }'; then
  echo "llm-wiki: manifest checksum is invalid" >&2
  exit 4
fi

archive_path=$tmp/$archive
curl -fsSL "$release_base/$archive" -o "$archive_path" || { echo "llm-wiki: unable to download release archive" >&2; exit 4; }
actual=$(sha256 "$archive_path")
[ "$actual" = "$expected" ] || { echo "llm-wiki: checksum verification failed" >&2; exit 4; }
if tar -tzf "$archive_path" | awk 'BEGIN { bad=0 } /^\// || /(^|\/)\.\.(\/|$)/ { bad=1 } END { exit bad }'; then :; else
  echo "llm-wiki: release archive contains an unsafe path" >&2
  exit 4
fi
if tar -tvzf "$archive_path" | awk 'BEGIN { bad=0 } substr($1, 1, 1) ~ /[lbh]/ { bad=1 } END { exit bad }'; then :; else
  echo "llm-wiki: release archive contains a link or device entry" >&2
  exit 4
fi

if [ "$dry_run" -eq 1 ]; then
  echo "llm-wiki: verified $target release ($archive)"
  exit 0
fi

destination=$root/.llm-wiki
[ "$global" -eq 1 ] && destination=${LLM_WIKI_BIN_DIR:-$HOME/.local/bin}
mkdir -p "$destination"
extract=$tmp/extract
mkdir "$extract"
tar -xzf "$archive_path" -C "$extract"
helper=$(find "$extract" -type f \( -name llm-wiki -o -name llm-wiki.exe \) -perm -u+x | sed -n '1p')
[ -n "$helper" ] || { echo "llm-wiki: release archive does not contain llm-wiki" >&2; exit 4; }
helper_name=llm-wiki
case "$helper" in *.exe) helper_name=llm-wiki.exe ;; esac
install -m 0755 "$helper" "$destination/$helper_name"
if [ -d "$extract/llm-wiki/template" ]; then
  rm -rf "$destination/template"
  cp -R "$extract/llm-wiki/template" "$destination/template"
fi

if [ "$global" -eq 0 ] && [ "$no_init" -eq 0 ]; then
  LLM_WIKI_TEMPLATE_DIR="$destination/template" "$destination/$helper_name" init --root "$root"
fi

banner=$("$destination/$helper_name" version 2>/dev/null || echo "llm-wiki")
docs_url="https://github.com/merdandt/LLM-wiki-dev#readme"

if [ "$global" -eq 1 ]; then
  cat <<EOF

$banner installed at $destination/$helper_name.

  Run "llm-wiki init" inside a Git repository to set one up.

  Docs: $docs_url
EOF
elif [ "$no_init" -eq 1 ]; then
  cat <<EOF

$banner installed at $destination/$helper_name.

  Helper installed without touching repository files.
  Run ".llm-wiki/llm-wiki init" when ready.

  Docs: $docs_url
EOF
else
  initialized=$(sed -n 's/^initialized:[[:space:]]*//p' "$root/llm-wiki.yaml" 2>/dev/null | sed -n '1p')
  if [ "$initialized" = "true" ]; then
    cat <<EOF

$banner installed. Wiki already compiled - lifecycle hooks are active.

  Codex only: run /hooks once in your next session to approve the project hooks.

  Useful commands:
    .llm-wiki/llm-wiki status      wiki health and sync state
    .llm-wiki/llm-wiki validate    structural check

  Docs: $docs_url
EOF
  else
    cat <<EOF

$banner installed.

  Created for you:
    docs/llm-wiki/          team wiki (scaffold - an agent compiles it)
    llm-wiki.yaml           configuration
    AGENTS.md / CLAUDE.md   agent instructions (your content preserved)
    .claude/settings.json   Claude Code lifecycle hooks
    .codex/hooks.json       Codex lifecycle hooks

  Next steps:
    1. Review and commit the files above - the wiki is team memory, shared via git.
    2. Codex only: run /hooks once in your next session to approve the project hooks.
    3. Open a coding-agent session (Claude Code or Codex). The session-start hook
       asks the agent to compile the wiki from your codebase and finish with:
         .llm-wiki/llm-wiki finalize-init
       After that, maintenance runs quietly at the end of every session.

  Useful commands:
    .llm-wiki/llm-wiki status      wiki health and sync state
    .llm-wiki/llm-wiki validate    structural check

  Docs: $docs_url
EOF
  fi
fi

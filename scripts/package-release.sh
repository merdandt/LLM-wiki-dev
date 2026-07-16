#!/bin/sh
set -eu

version=${LLM_WIKI_VERSION:-dev}
output=dist
target=

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) version=$2; shift 2 ;;
    --output) output=$2; shift 2 ;;
    --target) target=$2; shift 2 ;;
    -h|--help) echo "Usage: package-release.sh [--version VERSION] [--output DIR] [--target OS-ARCH]"; exit 0 ;;
    *) echo "package-release.sh: unknown argument: $1" >&2; exit 2 ;;
  esac
done

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
[ -n "$target" ] || target=$(go env GOOS)-$(go env GOARCH)
os=${target%-*}
arch=${target#*-}
case "$os/$arch" in
  darwin/amd64|darwin/arm64|linux/amd64|linux/arm64|windows/amd64) ;;
  *) echo "package-release.sh: unsupported target: $target" >&2; exit 2 ;;
esac

mkdir -p "$output"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
binary=$work/helper
ext=
[ "$os" = windows ] && ext=.exe
GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -ldflags "-X github.com/merdandt/LLM-wiki-dev/internal/cli.Version=$version" -o "$binary$ext" ./cmd/llm-wiki
tar_root=$work/llm-wiki
mkdir "$tar_root"
cp "$binary$ext" "$tar_root/llm-wiki$ext"
cp -R template "$tar_root/template"
archive="$output/llm-wiki-$target.tar.gz"
tar -czf "$archive" -C "$work" llm-wiki
echo "$archive"

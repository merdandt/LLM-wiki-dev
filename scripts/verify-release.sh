#!/bin/sh
set -eu

dist=${1:-dist}
found=0
for archive in "$dist"/llm-wiki-*.tar.gz; do
  [ -f "$archive" ] || continue
  found=1
  tar -tzf "$archive" | awk '
    BEGIN { helper=0; template=0; bad=0 }
    /^\// || /(^|\/)\.\.(\/|$)/ { bad=1 }
    /(^|\/)llm-wiki(\.exe)?$/ { helper=1 }
    /(^|\/)template\/llm-wiki\.yaml$/ { template=1 }
    END { exit !(bad == 0 && helper == 1 && template == 1) }'
  echo "verified $archive"
done
[ "$found" -eq 1 ] || { echo "verify-release.sh: no release archives in $dist" >&2; exit 1; }

#!/usr/bin/env bash
# Moves a module's pin in every manifest to its latest version, or to the given
# tag, branch, or commit. Untagged modules resolve to a pseudo-version.
#
#   update-module.sh <module path> [<version>]
set -euo pipefail
cd "$(git rev-parse --show-toplevel)/otel-industrial-collector/manifests"

if [ $# -lt 1 ]; then
  echo "usage: $0 <module path> [<version>]" >&2
  exit 1
fi
mod=$1
query=${2:-latest}

new=$(GOFLAGS=-mod=mod go list -m -f '{{.Version}}' "$mod@$query")
files=$(grep -l "^  - gomod: $mod " ./*.yaml)
old=$(sed -n "s#^  - gomod: $mod \([^ ]*\).*#\1#p" $files | sort -u | paste -sd, -)
# shellcheck disable=SC2086
sed -i "s#^\(  - gomod: $mod \)[^ ]*#\1$new#" $files
echo "$mod: $old -> $new"

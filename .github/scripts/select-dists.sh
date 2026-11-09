#!/bin/sh
# Prints a JSON array of the manifests (without .yaml) to build for the changes
# between BASE and HEAD: every manifest when the build tooling changed, a
# manifest whose yaml changed, and every manifest that uses a changed in-repo
# module, directly or through another in-repo module.
#
#   select-dists.sh <base-ref>     # changed files from <base-ref>...HEAD
#   select-dists.sh --all          # every manifest
set -eu
if [ $# -ne 1 ]; then
  echo "usage: $0 <base-ref>|--all" >&2
  exit 1
fi
cd "$(git rev-parse --show-toplevel)/otel-industrial-collector/manifests"

repo=github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector
all=$(for f in *.yaml; do basename "$f" .yaml; done)

emit() { jq -R . | jq -cs 'map(select(. != ""))'; }

if [ "$1" = "--all" ]; then
  printf '%s\n' "$all" | emit
  exit 0
fi

changed=$(git diff --name-only --diff-filter=d "$1...HEAD")
if printf '%s\n' "$changed" | grep -qE '^otel-industrial-collector/manifests/Makefile$|^\.github/workflows/build\.yml$|^\.github/scripts/(retry|select-dists)\.sh$'; then
  printf '%s\n' "$all" | emit
  exit 0
fi

# In-repo module directories (relative to otel-industrial-collector) that a
# manifest builds: the ones it lists, plus the in-repo modules they require.
# Always called through $(...), so m does not leak into the caller.
modules() {
  for m in $(sed -n "s#^  - gomod: $repo/\([^ ]*\) .*#\1#p" "$1.yaml"); do
    echo "$m"
    sed -n "s#^[[:space:]]$repo/\([^ ]*\) .*#\1#p" "../$m/go.mod"
  done | sort -u
}

for d in $all; do
  if printf '%s\n' "$changed" | grep -qx "otel-industrial-collector/manifests/$d.yaml"; then
    echo "$d"
    continue
  fi
  for m in $(modules "$d"); do
    if printf '%s\n' "$changed" | grep -q "^otel-industrial-collector/$m/"; then
      echo "$d"
      break
    fi
  done
done | emit

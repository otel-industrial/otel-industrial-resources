#!/bin/sh
# Bumps the OTel collector core and contrib modules to one upstream release:
# the in-repo go.mod files, the manifests and builder configs, the OCB version,
# and the docs that name the release. Modules from outside OTel are left alone;
# bump those by hand.
#
# The go.mod files are moved with `multimod sync`, one core module set at a
# time with a commit after each, as contrib's update-otel does. Contrib syncs
# against a sibling checkout of opentelemetry-collector; this runs locally, so
# it clones that repo at the release tag into a temporary directory instead.
#
#   update-otel.sh <core beta> <core stable> [<contrib>]
#   update-otel.sh 0.164.0 1.70.0 0.164.1
#
# contrib defaults to the core beta version.
set -eu
cd "$(git rev-parse --show-toplevel)"

if [ $# -lt 2 ]; then
  echo "usage: $0 <core beta> <core stable> [<contrib>]" >&2
  exit 1
fi
beta=$1 stable=$2 contrib=${3:-$1}
col=otel-industrial-collector
contrib_path=github.com/open-telemetry/opentelemetry-collector-contrib

# A failed or interrupted run resets the branch to where it started, so start
# from a clean tree.
test -z "$(git status --porcelain)" || { echo "local repository not clean" >&2; exit 1; }
start=$(git rev-parse HEAD)
core=$(mktemp -d)
finished=
trap 'rm -rf "$core"; test -n "$finished" || { git reset -q --hard "$start"; find . -name "*.bak" -type f -delete; }' EXIT
trap 'exit 1' HUP INT TERM

# Current versions, read from the manifests. check-otel-versions.sh keeps each
# of them to a single value.
single() {
  v=$(sort -u)
  if [ -z "$v" ] || [ "$(printf '%s\n' "$v" | wc -l)" -ne 1 ]; then
    echo "manifests disagree on the current $1 version; run check-otel-versions.sh" >&2
    exit 1
  fi
  echo "$v"
}
old_beta=$(sed -n 's/^  otelcol_version: //p' "$col"/manifests/*.yaml | single "core beta")
old_stable=$(grep -ohE 'go\.opentelemetry\.io/collector/[^ ]* v1\.[0-9.]+' "$col"/manifests/*.yaml | sed 's/.* v//' | single "core stable")
old_contrib=$(grep -ohE "$contrib_path/[^ ]* v[0-9.]+" "$col"/manifests/*.yaml | sed 's/.* v//' | single contrib)

# go.mod files: sync each core module set, committing between them, since
# multimod sync needs a clean tree. multimod treats every go.mod on disk as a
# module, so remove any OCB build output first.
make -s clean-dists
git clone -q --depth 1 --branch "v$beta" https://github.com/open-telemetry/opentelemetry-collector "$core"
multimod="go -C internal/tools tool multimod"
$multimod sync -s=true -o "$core" -m stable --commit-hash "v$stable"
git diff --quiet || git commit -qam "multimod update stable modules to v$stable"
$multimod sync -s=true -o "$core" -m beta --commit-hash "v$beta"
git diff --quiet || git commit -qam "multimod update beta modules to v$beta"
make -s gotidy

# Everything else that names the release. Contrib lines first, since contrib
# and core can share a version number.
esc() { printf '%s\n' "$1" | sed 's/\./\\./g'; }
files=$(git grep -lE "$(esc "$old_beta")|$(esc "$old_stable")|$(esc "$old_contrib")" -- ':!*go.mod' ':!*go.sum' ':!.github/scripts/update-otel.sh')
# shellcheck disable=SC2086
sed -i.bak -E \
  -e "/opentelemetry-collector-contrib/ s/(^|[^0-9.])$(esc "$old_contrib")([^0-9]|$)/\1$contrib\2/g" \
  -e "/opentelemetry-collector-contrib/! s/(^|[^0-9.])$(esc "$old_beta")([^0-9]|$)/\1$beta\2/g" \
  -e "/opentelemetry-collector-contrib/! s/(^|[^0-9.])$(esc "$old_stable")([^0-9]|$)/\1$stable\2/g" \
  $files
find . -name "*.bak" -type f -delete

sh .github/scripts/check-otel-versions.sh
git commit -qam "Update the OTel collector to v$beta in the manifests and docs"
finished=1

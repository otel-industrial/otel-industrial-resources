#!/usr/bin/env bash
# Bumps the OTel collector core and contrib modules to one upstream release:
# the in-repo go.mod files, the manifests and builder configs, the OCB version,
# and the docs that name the release. Modules from outside OTel are left alone;
# bump those by hand.
#
#   update-otel.sh <core beta> <core stable> [<contrib>]
#   update-otel.sh 0.164.0 1.70.0 0.164.1
#
# contrib defaults to the core beta version.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

if [ $# -lt 2 ]; then
  echo "usage: $0 <core beta> <core stable> [<contrib>]" >&2
  exit 1
fi
beta=$1 stable=$2 contrib=${3:-$1}
col=otel-industrial-collector
contrib_path=github.com/open-telemetry/opentelemetry-collector-contrib

# Current versions, read from the manifests. check-otel-versions.sh keeps each
# of them to a single value.
single() {
  local v
  v=$(sort -u)
  if [ "$(wc -l <<<"$v")" -ne 1 ] || [ -z "$v" ]; then
    echo "manifests disagree on the current $1 version; run check-otel-versions.sh" >&2
    exit 1
  fi
  echo "$v"
}
old_beta=$(sed -n 's/^  otelcol_version: //p' "$col"/manifests/*.yaml | single "core beta")
old_stable=$(grep -ohE 'go\.opentelemetry\.io/collector/[^ ]* v1\.[0-9.]+' "$col"/manifests/*.yaml | sed 's/.* v//' | single "core stable")
old_contrib=$(grep -ohE "$contrib_path/[^ ]* v[0-9.]+" "$col"/manifests/*.yaml | sed 's/.* v//' | single contrib)

# go.mod files: move every OTel collector requirement to the new release.
while read -r dir; do
  targets=$(go mod edit -json "$dir/go.mod" | jq -r --arg beta "$beta" --arg stable "$stable" --arg contrib "$contrib" --arg cp "$contrib_path/" '
    .Require[]? | .Path as $p |
    if ($p | startswith($cp)) then "\($p)@v\($contrib)"
    elif ($p | startswith("go.opentelemetry.io/collector")) then
      (if (.Version | startswith("v0.")) then "\($p)@v\($beta)" else "\($p)@v\($stable)" end)
    else empty end')
  [ -z "$targets" ] && continue
  echo "updating $dir"
  # shellcheck disable=SC2086
  (cd "$dir" && go get $targets && go mod tidy)
done < <(find "$col" -name go.mod -not -path "$col/manifests/*" -exec dirname {} \; | sort)

# Everything else that names the release. Contrib lines first, since contrib
# and core can share a version number.
esc() { sed 's/\./\\./g' <<<"$1"; }
files=$(git grep -lE "$(esc "$old_beta")|$(esc "$old_stable")|$(esc "$old_contrib")" -- ':!*go.mod' ':!*go.sum' ':!.github/scripts/update-otel.sh')
# shellcheck disable=SC2086
sed -i -E \
  -e "/opentelemetry-collector-contrib/ s/(^|[^0-9.])$(esc "$old_contrib")([^0-9]|$)/\1$contrib\2/g" \
  -e "/opentelemetry-collector-contrib/! s/(^|[^0-9.])$(esc "$old_beta")([^0-9]|$)/\1$beta\2/g" \
  -e "/opentelemetry-collector-contrib/! s/(^|[^0-9.])$(esc "$old_stable")([^0-9]|$)/\1$stable\2/g" \
  $files

bash .github/scripts/check-otel-versions.sh

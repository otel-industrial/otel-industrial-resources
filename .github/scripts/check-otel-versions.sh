#!/bin/sh
# Fails when the in-repo modules, manifests, and builder disagree on the OTel
# collector release. Core beta (v0.x), core stable (v1.x), and contrib modules
# must each be at a single version everywhere, so MVS cannot lift a receiver to
# a collector release it was never built against.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 1

col=otel-industrial-collector
gomods=$(find "$col" -name go.mod -not -path "$col/manifests/*")
yamls=$(ls "$col"/manifests/*.yaml "$col"/receiver/*/builder-config.yaml)

# Prints "<category> <version> <file>" for every OTel collector module found.
versions() {
  # shellcheck disable=SC2086
  grep -HoE '(go\.opentelemetry\.io/collector[^ @]*|github\.com/open-telemetry/opentelemetry-collector-contrib/[^ @]*)[ @]v[0-9]+\.[0-9]+\.[0-9]+[^ ]*' $gomods $yamls |
    sed -E 's#^([^:]*):(.*)[ @](v[^ ]*)$#\2 \3 \1#' |
    awk '{
      cat = ($1 ~ /opentelemetry-collector-contrib/) ? "contrib" : ($2 ~ /^v0\./ ? "core-beta" : "core-stable")
      print cat, $2, $3
    }'
  # otelcol_version and the builder version are core beta releases.
  # shellcheck disable=SC2086
  grep -HoE 'otelcol_version: [0-9.]+' $yamls | sed -E 's#^([^:]*):otelcol_version: (.*)#core-beta v\2 \1#'
  grep -HoE 'OCB_VERSION \?= [0-9.]+' "$col/manifests/Makefile" | sed -E 's#^([^:]*):OCB_VERSION \?= (.*)#core-beta v\2 \1#'
  grep -HoE 'cmd/builder@v[0-9.]+' .github/workflows/*.yml | sed -E 's#^([^:]*):cmd/builder@(.*)#core-beta \2 \1#'
}

fail=0
all=$(versions | sort -u)
for cat in core-beta core-stable contrib; do
  n=$(printf '%s\n' "$all" | awk -v c="$cat" '$1 == c { print $2 }' | sort -u | wc -l)
  n=$((n)) # BSD wc pads the count with spaces
  [ "$n" -le 1 ] && continue
  echo "OTel $cat modules are at $n different versions:"
  printf '%s\n' "$all" | awk -v c="$cat" '$1 == c { print "  " $2 "  " $3 }' | sort -u
  fail=1
done
exit $fail

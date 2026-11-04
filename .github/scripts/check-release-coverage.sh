#!/bin/sh
# Fails when a Go module is added without wiring it into the release.
# `multimod verify` already fails for a go.mod missing from versions.yaml; this
# covers the ways around it.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 1

repo=github.com/otel-industrial/otel-industrial-resources
manifests=otel-industrial-collector/manifests
fail=0

# Only modules that are never published may be excluded from the module set.
allowed_excluded="$repo/internal/tools $repo/otel-industrial-collector/internal/tools"
while read -r m; do
  [ -n "$m" ] || continue
  case " $allowed_excluded " in *" $m "*) continue ;; esac
  echo "$m is in excluded-modules; add it to the module set in versions.yaml"
  fail=1
done <<EOF
$(sed -n '/^excluded-modules:/,$ s/^  - //p' versions.yaml)
EOF

# Every collector component in the module set is built by at least one manifest.
while read -r m; do
  case "$m" in
    */receiver/* | */processor/* | */exporter/* | */extension/* | */connector/*) ;;
    *) continue ;;
  esac
  grep -q "gomod: $m " "$manifests"/*.yaml && continue
  echo "$m is not in any manifest under $manifests"
  fail=1
done <<EOF
$(sed -n '/^module-sets:/,/^excluded-modules:/ s/^      - //p' versions.yaml)
EOF

# Every manifest carries the set version, as dist.version and on each in-repo
# module, so prepare-release moves them all together.
version=$(sed -n 's/^    version: v//p' versions.yaml)
for f in "$manifests"/*.yaml; do
  grep -q "^  version: $version$" "$f" || {
    echo "$f: set dist.version to $version"
    fail=1
  }
  while read -r m v; do
    [ -n "$m" ] || continue
    [ "$v" = "v$version" ] && continue
    echo "$f: $m is at $v; use v$version"
    fail=1
  done <<EOF
$(sed -n "s#^  - gomod: \($repo/[^ ]*\) \([^ ]*\).*#\1 \2#p" "$f")
EOF
done

exit $fail

#!/usr/bin/env bash
# Fails when a Go module is added without wiring it into the release.
# `multimod verify` already fails for a go.mod missing from versions.yaml; this
# covers the ways around it.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 1

repo=github.com/otel-industrial/otel-industrial-resources
manifests=(otel-industrial-collector/manifests/*.yaml)
fail=0

# Only modules that are never published may be excluded from the module set.
allowed_excluded=("$repo/internal/tools")
while read -r m; do
  [[ " ${allowed_excluded[*]} " == *" $m "* ]] && continue
  echo "$m is in excluded-modules; add it to the module set in versions.yaml"
  fail=1
done < <(sed -n '/^excluded-modules:/,$ s/^  - //p' versions.yaml)

# Every collector component in the module set is built by at least one manifest.
while read -r m; do
  case "$m" in
    */receiver/* | */processor/* | */exporter/* | */extension/* | */connector/*) ;;
    *) continue ;;
  esac
  grep -q "gomod: $m " "${manifests[@]}" && continue
  echo "$m is not in any manifest under otel-industrial-collector/manifests"
  fail=1
done < <(sed -n '/^module-sets:/,/^excluded-modules:/ s/^      - //p' versions.yaml)

# A manifest replaces each in-repo module it uses, and each in-repo module those
# require, so a release builds them from the tagged commit.
for f in "${manifests[@]}"; do
  for m in $(sed -n "s#^  - gomod: \($repo/[^ ]*\) .*#\1#p" "$f"); do
    for dep in "$m" $(sed -n "s#^\t\($repo/[^ ]*\) .*#\1#p" "${m#"$repo/"}/go.mod"); do
      grep -q "^  - $dep => " "$f" && continue
      echo "$f: add a replaces entry for $dep"
      fail=1
    done
  done
done

exit $fail

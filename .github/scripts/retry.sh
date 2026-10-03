#!/usr/bin/env bash
# retry.sh — run a command, retrying only on transient network failures.
#
# Wraps build/test steps so a Go module-proxy or checksum-DB blip
# (proxy.golang.org / sum.golang.org HTTP/2 stream resets, i/o timeouts)
# does not fail the job. Bounded: a persistent outage still fails after
# RETRY_MAX attempts. By default it retries ONLY on a known transient
# signature, so a real compile or test failure is never masked. Set
# RETRY_ANY=1 to retry on any non-zero exit.
#
# Usage: bash .github/scripts/retry.sh <command> [args...]
# Env:   RETRY_MAX (default 3), RETRY_DELAY base backoff seconds (default 10),
#        RETRY_ANY (default 0).
set -uo pipefail

max="${RETRY_MAX:-3}"
base_delay="${RETRY_DELAY:-10}"
retry_any="${RETRY_ANY:-0}"

# Signatures of transient network failures that clear on a retry.
transient_re='proxy\.golang\.org|sum\.golang\.org|i/o timeout|TLS handshake timeout|connection reset|connection refused|unexpected EOF|stream error|INTERNAL_ERROR|net/http: request canceled|Client\.Timeout exceeded|Service Unavailable|Bad Gateway|Gateway Time-?out|Too Many Requests|temporary failure|no such host'

attempt=1
while true; do
  log="$(mktemp)"
  "$@" 2>&1 | tee "$log"
  status="${PIPESTATUS[0]}"

  if [ "$status" -eq 0 ]; then
    rm -f "$log"
    exit 0
  fi

  if [ "$attempt" -ge "$max" ]; then
    echo "retry: '$*' failed after ${max} attempt(s) (exit ${status}); giving up." >&2
    rm -f "$log"
    exit "$status"
  fi

  if [ "$retry_any" != "1" ] && ! grep -qiE "$transient_re" "$log"; then
    echo "retry: '$*' failed (exit ${status}) with no transient-network signature; not retrying." >&2
    rm -f "$log"
    exit "$status"
  fi
  rm -f "$log"

  delay=$(( base_delay * attempt ))
  echo "retry: transient failure on attempt ${attempt}/${max} (exit ${status}); retrying '$*' in ${delay}s..." >&2
  sleep "$delay"
  attempt=$(( attempt + 1 ))
done

#!/usr/bin/env bash
# Validates the industrial semantic conventions registry in model/ with
# OpenTelemetry Weaver, run from its container image.
#
#   bash check.sh                     # Docker
#   CONTAINER=podman bash check.sh    # Podman

set -euo pipefail
cd "$(dirname "$0")"

WEAVER_VERSION=v0.24.2
ENGINE="${CONTAINER:-docker}"

"$ENGINE" run --rm \
  -v "$PWD/model:/model:ro" \
  "docker.io/otel/weaver:$WEAVER_VERSION" \
  registry check --registry /model

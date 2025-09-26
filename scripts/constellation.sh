#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
export GOCACHE="$ROOT/.gocache"
export GOMODCACHE="$ROOT/.gomodcache"
if ! [ -f "$ROOT/static/constellation.json" ]; then
  mkdir -p "$ROOT/static"
fi
GOFLAGS=${GOFLAGS:-}
GOFLAGS="${GOFLAGS} -buildvcs=false"
GOFLAGS="$GOFLAGS" go run "$ROOT/scripts/constellation.go" "$@"

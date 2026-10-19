#!/usr/bin/env bash
# Minimal Relay CI helper (full Agent port of release/version still TODO — see docs/phases/P09-CI_CD.md).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

cmd="${1:-}"
case "$cmd" in
  check)
    unformatted="$(gofmt -l .)"
    if [[ -n "$unformatted" ]]; then
      echo "gofmt needed:" >&2
      echo "$unformatted" >&2
      exit 1
    fi
    go mod tidy
    git diff --exit-code -- go.mod go.sum
    go vet ./...
    go test -race -count=1 ./...
    echo "check ok"
    ;;
  last)
    tr -d '[:space:]' < VERSION.md
    echo
    ;;
  version)
    ver="${2:-}"
    if [[ ! "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "usage: $0 version X.Y.Z" >&2
      exit 2
    fi
    echo "$ver" > VERSION.md
    # sync version.go const
    perl -0pi -e "s/const Version = \"[^\"]+\"/const Version = \"$ver\"/" version.go
    echo "VERSION.md and version.go -> $ver"
    ;;
  *)
    echo "usage: $0 {check|last|version X.Y.Z}" >&2
    exit 2
    ;;
esac

#!/usr/bin/env bash
# Release helper. Daily work stays normal git add / commit / push.
# GitHub Actions run only on tag vX.Y.Z (Relay Release) — not on ordinary main pushes.
#
#   bash scripts/ci-cd.sh last              # show current + latest released version
#   bash scripts/ci-cd.sh version X.Y.Z     # bump, local check, commit, push
#   bash scripts/ci-cd.sh release X.Y.Z     # tag, push, watch Relay Release
#   bash scripts/ci-cd.sh watch             # gh run watch (release runs)
#   bash scripts/ci-cd.sh web               # gh run view --web
#   bash scripts/ci-cd.sh check             # local gate (gofmt, tidy, vet, race tests)
#   bash scripts/ci-cd.sh help
#
# Semver single source of truth: VERSION.md (one line, X.Y.Z).
# See docs/phases/P09-CI_CD.md
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

CMD="${1:-help}"
VERSION_FILE="VERSION.md"

die() { echo "error: $*" >&2; exit 1; }
info() { echo "==> $*"; }
ok() { echo "OK: $*"; }

need_gh() {
  command -v gh >/dev/null 2>&1 || die "gh CLI required (brew install gh && gh auth login)"
}

# relay_version reads the sole human/release SSOT: VERSION.md
relay_version() {
  [[ -f "$VERSION_FILE" ]] || die "missing $VERSION_FILE"
  local ver
  ver="$(tr -d ' \t\r\n' <"$VERSION_FILE")"
  [[ -n "$ver" ]] || die "$VERSION_FILE is empty"
  echo "$ver"
}

# go_relay_version reads the compiled constant (must match VERSION.md).
go_relay_version() {
  sed -n 's/^const relayVersion = "\([^"]*\)"/\1/p' version.go
}

latest_tag() {
  git fetch --tags --quiet origin 2>/dev/null || true
  git tag -l 'v[0-9]*' --sort=-v:refname | head -1
}

confirm() {
  local prompt="$1"
  local reply
  read -r -p "$prompt [y/N] " reply
  [[ "$reply" == "y" || "$reply" == "Y" ]]
}

require_semver() {
  local ver="$1"
  [[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "version must be semver X.Y.Z (got: $ver)"
}

guardrails() {
  if [[ ! -f .gitignore ]] || ! grep -q '^/dist/' .gitignore; then
    die ".gitignore must ignore /dist/"
  fi
  if git ls-files 'dist/*' 2>/dev/null | grep -q .; then
    die "dist/ is tracked by git — run: git rm -r --cached dist/"
  fi
  if git diff --cached --name-only | grep -E '(^|/)dist/|\.deb$|\.rpm$|\.sha256$' >/dev/null; then
    die "staged dist/ or package files — unstage them (CI builds artifacts)"
  fi
}

# require_release_pipeline refuses to tag unless Relay Release can actually run.
require_release_pipeline() {
  local missing=()
  [[ -f .github/workflows/release.yml ]] || missing+=(".github/workflows/release.yml")
  [[ -f scripts/package-linux.sh ]] || missing+=("scripts/package-linux.sh")
  [[ -f packaging/systemd/infrasible-relay.service ]] || missing+=("packaging/systemd/infrasible-relay.service")
  if ((${#missing[@]})); then
    die "cannot release — missing: ${missing[*]} (see docs/phases/P09-CI_CD.md)"
  fi
  # Tag triggers workflows from the tagged commit; files must already be on origin/main.
  git show "origin/main:.github/workflows/release.yml" >/dev/null 2>&1 || \
    die "origin/main has no .github/workflows/release.yml — commit and push it, then release"
  git show "origin/main:scripts/package-linux.sh" >/dev/null 2>&1 || \
    die "origin/main has no scripts/package-linux.sh — commit and push it, then release"
}

# watch_release_run waits for the Relay Release run for tag v$ver; removes orphan tag if none starts.
watch_release_run() {
  local ver="$1"
  local run_id=""
  local i
  need_gh
  for i in $(seq 1 20); do
    run_id="$(
      gh run list --workflow=release.yml --limit 10 \
        --json databaseId,headBranch \
        --jq ".[] | select(.headBranch == \"v${ver}\") | .databaseId" \
        | head -1
    )"
    [[ -n "$run_id" ]] && break
    sleep 2
  done
  if [[ -z "$run_id" ]]; then
    info "Relay Release did not start — removing orphan tag v$ver"
    git push origin ":refs/tags/v$ver" >/dev/null 2>&1 || true
    git tag -d "v$ver" >/dev/null 2>&1 || true
    die "no Relay Release run for v$ver (is release.yml on the tagged commit?)"
  fi
  info "watching Relay Release run $run_id"
  gh run watch "$run_id" --exit-status
}

fmt_tidy_vet_test() {
  info "gofmt -w ."
  gofmt -w .
  local unformatted
  unformatted="$(gofmt -l .)"
  [[ -z "$unformatted" ]] || { echo "$unformatted"; die "gofmt left files dirty"; }

  info "go mod tidy"
  go mod tidy
  git diff --exit-code -- go.mod go.sum >/dev/null || {
    git diff -- go.mod go.sum || true
    die "go.mod/go.sum changed — commit them before continuing"
  }

  info "go vet ./..."
  go vet ./...

  info "go test -race -count=1 ./..."
  go test -race -count=1 ./...
}

# set_version_files writes VERSION.md (SSOT) then syncs version.go for the binary.
set_version_files() {
  local ver="$1"
  local tmp
  require_semver "$ver"

  printf '%s\n' "$ver" >"$VERSION_FILE"
  [[ "$(relay_version)" == "$ver" ]] || die "failed to write $VERSION_FILE"

  tmp="$(mktemp)"
  sed "s/^const relayVersion = \".*\"/const relayVersion = \"$ver\"/" version.go >"$tmp"
  mv "$tmp" version.go
  [[ "$(go_relay_version)" == "$ver" ]] || die "failed to sync version.go to $ver"
}

assert_version_in_sync() {
  local file_ver go_ver
  file_ver="$(relay_version)"
  go_ver="$(go_relay_version)"
  [[ "$file_ver" == "$go_ver" ]] || die "$VERSION_FILE ($file_ver) != version.go ($go_ver) — run: bash scripts/ci-cd.sh version $file_ver"
}

cmd_last() {
  local source_ver tag gh_ver
  source_ver="$(relay_version)"
  assert_version_in_sync
  tag="$(latest_tag)"
  gh_ver=""
  if command -v gh >/dev/null 2>&1; then
    gh_ver="$(gh release view --json tagName -q .tagName 2>/dev/null || true)"
  fi

  echo "VERSION.md:            $source_ver"
  echo "version.go (synced):   $(go_relay_version)"
  echo "latest git tag:        ${tag:-"(none)"}"
  echo "latest GitHub Release: ${gh_ver:-"(none or gh unavailable)"}"
  echo
  echo "Next for a new release:"
  echo "  bash scripts/ci-cd.sh version X.Y.Z"
  echo "  bash scripts/ci-cd.sh release X.Y.Z"
}

cmd_check() {
  guardrails
  assert_version_in_sync
  fmt_tidy_vet_test
  ok "local verify passed"
}

cmd_version() {
  local ver="${2:-}"
  [[ -n "$ver" ]] || die "usage: bash scripts/ci-cd.sh version X.Y.Z"
  require_semver "$ver"

  local current
  current="$(relay_version)"
  [[ "$ver" != "$current" ]] || die "$VERSION_FILE is already $ver"

  local branch
  branch="$(git rev-parse --abbrev-ref HEAD)"
  [[ "$branch" == "main" ]] || die "bump version on main (currently: $branch)"

  if ! git diff --quiet || ! git diff --cached --quiet; then
    die "working tree dirty — commit or stash your daily work first"
  fi

  if git ls-remote --exit-code --refs origin "refs/tags/v$ver" >/dev/null 2>&1; then
    die "origin already has tag v$ver — pick a new version"
  fi

  echo "Bump version: $current → $ver (VERSION.md + sync version.go; commit + push to origin/main)"
  if ! confirm "Update $VERSION_FILE (and sync version.go), commit, and git push?"; then
    die "cancelled"
  fi

  set_version_files "$ver"
  guardrails
  fmt_tidy_vet_test

  git add "$VERSION_FILE" version.go
  git commit -m "chore(release): prepare Infrasible Relay v$ver"
  ok "committed version bump to $ver"

  info "git push origin main"
  git push origin HEAD
  ok "pushed v$ver prepare to main (no GitHub Actions on ordinary pushes)"
  echo
  echo "When ready to publish (GitHub verifies + packages once on the tag):"
  echo "  bash scripts/ci-cd.sh release $ver"
}

cmd_watch() {
  need_gh
  info "gh run watch"
  # Interactive: gh picks an in-progress run. Non-interactive / none active: watch latest.
  if [[ -t 0 ]] && [[ -t 1 ]]; then
    if gh run watch --exit-status; then
      return 0
    fi
  fi
  local run_id
  run_id="$(gh run list --limit 1 --json databaseId -q '.[0].databaseId' 2>/dev/null || true)"
  [[ -n "$run_id" && "$run_id" != "null" ]] || die "no workflow runs found — push to origin first"
  info "watching run $run_id"
  gh run watch "$run_id" --exit-status
}

cmd_web() {
  need_gh
  info "gh run view --web"
  gh run view --web
}

cmd_release() {
  local ver="${2:-}"
  [[ -n "$ver" ]] || die "usage: bash scripts/ci-cd.sh release X.Y.Z"
  require_semver "$ver"

  # Local suite already ran in `version` / `check`. GitHub Release is the remote gate.
  guardrails
  assert_version_in_sync

  local source_ver
  source_ver="$(relay_version)"
  [[ "$ver" == "$source_ver" ]] || die "$VERSION_FILE is $source_ver but you passed $ver — run: bash scripts/ci-cd.sh version $ver"

  local branch
  branch="$(git rev-parse --abbrev-ref HEAD)"
  [[ "$branch" == "main" ]] || die "release only from main (currently: $branch)"

  if ! git diff --quiet || ! git diff --cached --quiet; then
    die "working tree dirty — commit everything on main first"
  fi

  git fetch origin main
  [[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || \
    die "main != origin/main — git push first"

  require_release_pipeline

  if git ls-remote --exit-code --refs origin "refs/tags/v$ver" >/dev/null 2>&1; then
    die "origin already has tag v$ver"
  fi

  echo
  echo "Ship GitHub Release v$ver from $(git rev-parse --short HEAD) on main."
  if ! confirm "Create and push tag v$ver now?"; then
    die "release cancelled"
  fi

  git tag "v$ver"
  git push origin "v$ver"
  ok "tag v$ver pushed — watching Relay Release"
  echo
  watch_release_run "$ver"
  ok "Relay Release finished for v$ver"
  echo
  echo "View: gh release view v$ver"
  echo "Or:   bash scripts/ci-cd.sh web"
}

cmd_help() {
  cat <<'EOF'
scripts/ci-cd.sh

Semver SSOT: VERSION.md (one line, X.Y.Z). version.go is synced from it.
GitHub Actions: only Relay Release on tag vX.Y.Z (commit must be on main).

Daily (you):
  git add … && git commit -m "…" && git push
  bash scripts/ci-cd.sh check            # optional local gate

New release:
  bash scripts/ci-cd.sh last
  bash scripts/ci-cd.sh version X.Y.Z    # bump + local check + push (no Actions)
  bash scripts/ci-cd.sh release X.Y.Z    # tag + watch Relay Release (verify/package/publish)

Optional:
  bash scripts/ci-cd.sh watch
  bash scripts/ci-cd.sh web
  bash scripts/ci-cd.sh help
EOF
}

case "$CMD" in
  last) cmd_last ;;
  version) cmd_version "$@" ;;
  watch) cmd_watch ;;
  web) cmd_web ;;
  check) cmd_check ;;
  release) cmd_release "$@" ;;
  help|-h|--help) cmd_help ;;
  *) die "unknown: $CMD (try: bash scripts/ci-cd.sh help)" ;;
esac

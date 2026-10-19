# P09 — CI / CD for Infrasible Relay

**Parent brief:** [PROMPT.md](../PROMPT.md)  
**Copy from this Agent repo exactly** (rename Agent → Relay only where noted):

| Agent source | Relay destination |
|---|---|
| [`docs/CI_CD.md`](../../docs/CI_CD.md) | Relay `docs/CI_CD.md` (this phase is the brief) |
| [`scripts/ci-cd.sh`](../../scripts/ci-cd.sh) | Relay `scripts/ci-cd.sh` |
| [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) | Relay `.github/workflows/ci.yml` |
| [`.github/workflows/release.yml`](../../.github/workflows/release.yml) | Relay `.github/workflows/release.yml` |
| [`scripts/package-linux.sh`](../../scripts/package-linux.sh) | Relay `scripts/package-linux.sh` |
| [`version.go`](../../version.go) + [`version_test.go`](../../version_test.go) | Relay equivalents (`relayVersion`) |
| [`docs/TEST.md`](../../docs/TEST.md) | Relay `docs/TEST.md` |
| [`.gitignore`](../../.gitignore) `/dist/` | Same |

Do **not** invent a different release model. Port the Agent tooling; change product names and workflow titles only.

---

## 1. Operator-facing doc (Relay `docs/CI_CD.md`)

Ship this text in the Relay repo (same structure as Agent `docs/CI_CD.md`):

```markdown
# CI / CD — Infrasible Relay

| Where | When | What |
|---|---|---|
| `scripts/ci-cd.sh` | daily / ship | check, version, release |
| **Relay CI** | every push / PR | `gofmt`, tidy, vet, race tests |
| **Relay Release** | tag `vX.Y.Z` only | Linux packages + GitHub Release |

Push to `main` never publishes a Release.

**Single version source:** `VERSION.md` (one line `X.Y.Z`).
`bash scripts/ci-cd.sh version X.Y.Z` writes that file and syncs `version.go` for the binary.
Do not put the number in other markdown.

## Everyday

git add … && git commit -m "…" && git push
bash scripts/ci-cd.sh watch   # optional
bash scripts/ci-cd.sh check   # optional local gate (gofmt, tidy, vet, race tests)

Always run `gofmt -w .` (or `bash scripts/ci-cd.sh check`) before push — Relay CI fails closed on formatting.

## Ship a version

bash scripts/ci-cd.sh watch   # optional
bash scripts/ci-cd.sh check   # optional local gate
bash scripts/ci-cd.sh last
bash scripts/ci-cd.sh version X.Y.Z   # write VERSION.md + sync version.go, commit, push, watch Relay CI
bash scripts/ci-cd.sh release X.Y.Z   # tag vX.Y.Z, watch Relay Release
gh release view vX.Y.Z

Do not reuse an existing `vX.Y.Z` tag. Do not commit `dist/` (gitignored).
Local packages without tagging: `bash scripts/package-linux.sh`.
```

---

## 2. Rules (identical to Agent)

| Rule | Detail |
|---|---|
| Semver SSOT | `VERSION.md` — **one line** `X.Y.Z` only |
| Binary sync | `version.go` const kept in sync by `ci-cd.sh version` |
| Drift test | Unit test fails if `VERSION.md` ≠ `version.go` |
| Push ≠ release | `main` runs **Relay CI** only; **tag** runs **Relay Release** |
| No `dist/` in git | `.gitignore` must contain `/dist/`; CI builds artifacts |
| Format fail-closed | Non-empty `gofmt -l .` → CI red |
| Branch | `version` and `release` only from **`main`** |
| Dirty tree | Refuse bump/release if uncommitted changes |
| Tag reuse | Refuse if `origin` already has `vX.Y.Z` |
| Confirm prompts | Interactive `y/N` before bump commit/push and before tag push |

---

## 3. `scripts/ci-cd.sh` — port Agent script

Copy [`scripts/ci-cd.sh`](../../scripts/ci-cd.sh) into the Relay repo and apply **only** these renames:

| Agent | Relay |
|---|---|
| `agent_version` / `go_agent_version` | `relay_version` / `go_relay_version` |
| `const agentVersion =` | `const relayVersion =` |
| Commit message `prepare Infrasible Agent v$ver` | `prepare Infrasible Relay v$ver` |
| Log text `watching Agent CI` / `Agent Release` | `watching Relay CI` / `Relay Release` |
| Help text mentioning Agent | Relay |

### Commands (behavior must match Agent)

| Command | Behavior |
|---|---|
| `last` | Assert VERSION.md ↔ version.go sync; print VERSION.md, version.go, latest git tag, latest GitHub Release; print next steps |
| `check` | `guardrails` → assert sync → `gofmt -w .` → tidy + diff exit → `go vet` → `go test -race -count=1 ./...` |
| `version X.Y.Z` | On `main`, clean tree, tag not on origin; confirm; write VERSION.md; sed-sync version.go; check; commit; `git push origin HEAD`; `sleep 2`; `gh run watch --exit-status` |
| `release X.Y.Z` | On `main`, clean, HEAD == origin/main, VERSION.md == arg; confirm; `git tag v$ver`; push tag; watch Release |
| `watch` | `gh run watch --exit-status` |
| `web` | `gh run view --web` |
| `help` | Usage (same shape as Agent help) |

### `guardrails` (same as Agent)

1. `.gitignore` must contain `/dist/` (line match).  
2. `dist/*` must not be tracked.  
3. Staged files must not include `dist/`, `*.deb`, `*.rpm`, `*.sha256`.

### `fmt_tidy_vet_test` (same as Agent)

```sh
gofmt -w .
# fail if gofmt -l . non-empty
go mod tidy
git diff --exit-code -- go.mod go.sum   # or die "commit go.mod/go.sum"
go vet ./...
go test -race -count=1 ./...
```

### `set_version_files` (same as Agent)

```sh
printf '%s\n' "$ver" > VERSION.md
# sed: s/^const relayVersion = ".*"/const relayVersion = "$ver"/
```

Require `gh` for watch/web/version/release watch steps (`brew install gh && gh auth login`).

---

## 4. `version.go` + drift test

Mirror Agent [`version.go`](../../version.go) / [`version_test.go`](../../version_test.go):

```go
package main

// relayVersion is the semver stamped into packages and release tags.
// Single source of truth for humans and release tooling is VERSION.md;
// scripts/ci-cd.sh version keeps this constant in sync.
const relayVersion = "0.1.0"

// protocolMajor is the Live wire protocol major (Agent Live is 2).
const protocolMajor = 2

// protocolMinor is the Live wire protocol minor.
const protocolMinor = 0
```

```go
func TestRelayVersionMatchesVERSIONMD(t *testing.T) {
    raw, err := os.ReadFile("VERSION.md")
    // trim space; fail if empty or != relayVersion
}

func TestProtocolVersion(t *testing.T) {
    if protocolMajor != 2 || protocolMinor != 0 {
        t.Fatalf(...)
    }
}
```

---

## 5. GitHub Actions — copy Agent workflows

### `.github/workflows/ci.yml` (from Agent CI)

```yaml
# Fast verify on every push/PR — same checks as: bash scripts/ci-cd.sh check
# Does NOT build packages or publish releases. See docs/CI_CD.md
name: Relay CI
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:
permissions:
  contents: read
jobs:
  verify:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: false
      - name: gofmt
        run: |
          set -euo pipefail
          unformatted="$(gofmt -l .)"
          if [[ -n "$unformatted" ]]; then
            echo "::error::Run: gofmt -w ."
            echo "$unformatted"
            exit 1
          fi
      - name: tidy
        run: |
          go mod tidy
          git diff --exit-code -- go.mod go.sum
      - name: vet and test
        run: |
          go vet ./...
          go test -race -count=1 ./...
```

Changes from Agent: workflow `name: Relay CI` only. Steps identical.

### `.github/workflows/release.yml` (from Agent Release)

```yaml
# Package + publish when ci-cd.sh release pushes tag vX.Y.Z
# Does NOT run on ordinary pushes to main. See docs/CI_CD.md
name: Relay Release
on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'
permissions:
  contents: write
concurrency:
  group: relay-release
  cancel-in-progress: false
jobs:
  release:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: false
      - name: Match tag to VERSION.md
        run: |
          set -euo pipefail
          VERSION="${GITHUB_REF_NAME#v}"
          SOURCE="$(tr -d ' \t\r\n' <VERSION.md)"
          GO_VER="$(sed -n 's/^const relayVersion = "\([^"]*\)"/\1/p' version.go)"
          [[ "$VERSION" == "$SOURCE" ]] || {
            echo "Tag v$VERSION must match VERSION.md ($SOURCE)"
            exit 1
          }
          [[ "$SOURCE" == "$GO_VER" ]] || {
            echo "VERSION.md ($SOURCE) != version.go ($GO_VER)"
            exit 1
          }
          if gh release view "v$VERSION" >/dev/null 2>&1; then
            echo "Release v$VERSION already exists"
            exit 1
          fi
          echo "VERSION=$VERSION" >> "$GITHUB_ENV"
        env:
          GH_TOKEN: ${{ github.token }}
      - name: Verify
        run: |
          test -z "$(gofmt -l .)"
          go mod tidy
          git diff --exit-code -- go.mod go.sum
          go vet ./...
          go test -race -count=1 ./...
      - name: Package
        run: |
          set -euo pipefail
          sudo apt-get update -qq
          sudo apt-get install -y rpm
          bash scripts/package-linux.sh
          (cd dist && sha256sum -c SHA256SUMS)
      - name: Publish GitHub Release
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh release create "v$VERSION" dist/* \
            --generate-notes \
            --title "Infrasible Relay v$VERSION" \
            --target "$GITHUB_SHA"
```

Differences from Agent: `name`, `concurrency.group: relay-release`, sed for `relayVersion`, release title `Infrasible Relay`.

---

## 6. `scripts/package-linux.sh`

Port Agent [`scripts/package-linux.sh`](../../scripts/package-linux.sh):

- Read version from `VERSION.md`; require match with `relayVersion` in `version.go`.  
- If `GITHUB_REF_TYPE=tag`, require tag `v$VERSION`.  
- Build Linux binaries (same arch matrix as Agent unless you deliberately shrink — prefer **same** amd64/arm64/armv7/armv6).  
- Emit `dist/infrasible-relay-linux-$target`, tar.gz, `.deb` / `.rpm` where tools exist.  
- Write `dist/SHA256SUMS`.  
- Package names / systemd unit / paths use **`infrasible-relay`** (see [P08](P08-PACKAGING_LINUX.md)).  
- Never commit `dist/`.

---

## 7. Everyday and ship (exact command sequence)

### Everyday

```sh
git add … && git commit -m "…" && git push
bash scripts/ci-cd.sh watch   # optional
bash scripts/ci-cd.sh check   # optional local gate (gofmt, tidy, vet, race tests)
```

### Ship

```sh
bash scripts/ci-cd.sh watch   # optional
bash scripts/ci-cd.sh check   # optional local gate
bash scripts/ci-cd.sh last
bash scripts/ci-cd.sh version X.Y.Z   # write VERSION.md + sync version.go, commit, push, watch Relay CI
bash scripts/ci-cd.sh release X.Y.Z   # tag vX.Y.Z, watch Relay Release
gh release view vX.Y.Z
```

Do not reuse an existing `vX.Y.Z` tag. Do not commit `dist/`.  
Local packages without tagging: `bash scripts/package-linux.sh`.

---

## 8. Tests doc (Relay `docs/TEST.md`)

Mirror Agent [`docs/TEST.md`](../../docs/TEST.md):

```sh
go test ./...                           # full suite local
go test -race -count=1 ./...            # CI-equivalent
bash scripts/ci-cd.sh check             # full local gate
```

If Relay CI fails on `gofmt`, run `gofmt -w .` and confirm `gofmt -l .` prints nothing before pushing.

---

## 9. `.gitignore` (required lines)

```gitignore
# Build outputs — produced by CI / scripts/package-linux.sh, never commit.
/dist/
*.deb
*.rpm
```

---

## 10. Doc-set vs product version (Agent-repo note)

| Artifact | Location | Meaning |
|---|---|---|
| This brief | Agent repo [`relay/VERSION.md`](../VERSION.md) | Documentation set for juniors — **not** Relay product semver |
| Relay product | Relay repo `VERSION.md` | Binary / packages / tags |

Junior implements the product VERSION.md in the **sibling** repo. Do not bump Agent `VERSION.md` when editing this brief.

---

## 11. Implementer checklist

- [ ] Copy and rename `scripts/ci-cd.sh` (Agent → Relay strings).  
- [ ] Copy `ci.yml` / `release.yml` with Relay names + `relayVersion` sed.  
- [ ] Port `package-linux.sh` for `infrasible-relay` packaging layout.  
- [ ] `VERSION.md` + `version.go` + drift test.  
- [ ] `docs/CI_CD.md` + `docs/TEST.md` as above.  
- [ ] `.gitignore` `/dist/`.  
- [ ] Prove: `check` green → `version` → green Relay CI → `release` → green Relay Release → `gh release view vX.Y.Z`.

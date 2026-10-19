# CI / CD — Infrasible Relay

Use **`scripts/ci-cd.sh`** for version bumps and releases. Do not create tags by hand.

| Where | When | What |
|---|---|---|
| `bash scripts/ci-cd.sh check` | anytime locally | `gofmt`, tidy, vet, race tests |
| `bash scripts/ci-cd.sh version` | prepare a release | write `VERSION.md`, sync `version.go`, commit, push |
| `bash scripts/ci-cd.sh release` | publish | tag `vX.Y.Z`, GitHub verifies + packages + Release |
| **Relay Release** (GitHub) | tag on a **main** commit only | remote verify, Linux packages, GitHub Release |

Ordinary pushes to `main` do **not** run GitHub Actions.  
**Version SSOT:** one line in `VERSION.md` (`X.Y.Z`). Do not put the number elsewhere in docs.

Requires: clean tree on `main`, `gh` logged in for release watch / viewing.

---

## Everyday

```sh
git add … && git commit -m "…" && git push
bash scripts/ci-cd.sh check   # optional local gate before you push
```

---

## Ship a version

```sh
bash scripts/ci-cd.sh last              # see current vs last tag / Release
bash scripts/ci-cd.sh version X.Y.Z     # answer y — local check, commit, push
bash scripts/ci-cd.sh release X.Y.Z     # answer y — tag, watch Relay Release
gh release view vX.Y.Z                  # or: bash scripts/ci-cd.sh web
```

Do not reuse an existing `vX.Y.Z` tag. Do not commit `dist/` (gitignored).

Local packages without tagging:

```sh
bash scripts/package-linux.sh
```

---

## Commands

| Command | What it does |
|---|---|
| `last` | Print `VERSION.md`, `version.go`, latest tag, latest GitHub Release |
| `check` | Local verify only |
| `version X.Y.Z` | Bump files, local check, commit, push (no Actions) |
| `release X.Y.Z` | Tag + push + watch **Relay Release** |
| `watch` | `gh run watch` |
| `web` | Open the run in the browser |
| `help` | Short help |

```sh
bash scripts/ci-cd.sh help
```

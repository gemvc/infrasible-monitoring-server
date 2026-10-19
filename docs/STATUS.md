# Monitoring Relay — documentation & implementation status

**Doc-set version:** [`VERSION.md`](VERSION.md) (one line `X.Y.Z` — version of this *brief*)  
**Relay product version:** [`../VERSION.md`](../VERSION.md)  
**Updated:** 2026-10-20

This file is the only place that states whether the brief and implementation are ready. Do not duplicate pass/fail tables elsewhere.

## Verdict

| Track | Status |
|---|---|
| Implementer brief (`PROMPT.md`) | **READY** — completed phase briefs removed; open gate is P10 |
| Relay Go implementation | **DONE** — `cmd/infrasible-relay`, `internal/*` |
| Unit / fake-conn acceptance | **PARTIAL** — R1, R2/R3, R4, R7, R8 in `internal/livesession`; R5/R6/R9 partial |
| Lab mTLS + real Agent | **PASS** — `./scripts/lab-up.sh` |
| Packaging + ship path | **DONE** — `packaging/`, `scripts/package-linux.sh`, tag-only **Relay Release**, [CI_CD.md](CI_CD.md) |
| Desktop Live subscriber protocol | **STUB ONLY** — LogHub / NoopHub |
| Persistence API | **OUT OF SCOPE** |

**Bottom line:** v1 core and ship tooling are in place. Close [phases/P10-ACCEPTANCE.md](phases/P10-ACCEPTANCE.md) R1–R9 with evidence, then `bash scripts/ci-cd.sh version` / `release`.

## How to use

1. Read [`../README.md`](../README.md).  
2. Operator ship: [`CI_CD.md`](CI_CD.md) via `scripts/ci-cd.sh`.  
3. Product brief: [`PROMPT.md`](PROMPT.md).  
4. Exit when [`phases/P10-ACCEPTANCE.md`](phases/P10-ACCEPTANCE.md) R1–R9 pass with evidence.

## Phases folder

Completed briefs (P00–P09, P11) were removed after implementation. Only **P10** remains under `phases/`.

# Monitoring Relay — documentation & implementation status

**Doc-set version:** [`VERSION.md`](VERSION.md) (one line `X.Y.Z` — version of this *brief*)  
**Relay product version:** [`../VERSION.md`](../VERSION.md)  
**Updated:** 2026-10-19

This file is the only place that states whether the brief and implementation are ready. Do not duplicate pass/fail tables elsewhere.

## Verdict

| Track | Status |
|---|---|
| Implementer brief (`PROMPT.md` + `phases/`) | **READY** |
| Relay Go implementation | **IN PROGRESS** — core packages in this repo (`cmd/infrasible-relay`, `internal/*`) |
| Unit / fake-conn acceptance | **PARTIAL** — R1, R2/R3, R4, R7, R8 covered in `internal/livesession`; R5 via MarkStale on exit; R6 queue unit test; R9 method check in HTTP handler |
| Lab mTLS + real Agent | **PASS** — Docker Desktop lab (`./scripts/lab-up.sh`): Hello → `session.accepted` → `hub.publish_live` against published Agent from GitHub |
| Packaging / CI port (P08–P09) | **NOT STARTED** — systemd unit stub only; Relay CI workflow present |
| Desktop Live subscriber protocol | **STUB ONLY** — LogHub / NoopHub |
| Persistence API | **OUT OF SCOPE** |

**Bottom line:** Relay v1 core (protocol, registrar, livesession, tlsfront, config, cmd) is scaffolded and unit-tested. Finish lab Agent proof, packaging, and Agent-style `ci-cd.sh` before claiming R1–R9 complete.

## How to use

1. Read [`../README.md`](../README.md).  
2. Read [`PROMPT.md`](PROMPT.md) end-to-end.  
3. Follow [`phases/P11-BUILD_ORDER.md`](phases/P11-BUILD_ORDER.md).  
4. Exit when [`phases/P10-ACCEPTANCE.md`](phases/P10-ACCEPTANCE.md) R1–R9 pass with evidence.

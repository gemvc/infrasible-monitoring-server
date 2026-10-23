# Monitoring Relay — documentation & implementation status

**Relay product version:** [`../VERSION.md`](../VERSION.md)  
**Updated:** 2026-10-20

This file is the only place that states whether the brief and implementation are ready. Do not duplicate pass/fail tables elsewhere.

## Verdict

| Track | Status |
|---|---|
| Implementer brief (`PROMPT.md`) | **READY** — open gate P10 closed with evidence |
| Relay Go implementation | **DONE** — `cmd/infrasible-relay`, `internal/*` |
| Unit / fake-conn acceptance | **PASS** — R1–R9, D1–D3, P1–P3 in-tree (`go test ./…`); P4 ops-documented |
| Lab mTLS + real Agent | **PASS** — `./scripts/lab-up.sh` |
| Packaging + ship path | **DONE** — deb/rpm (systemd + user/dirs) + tar.gz via `scripts/package-linux.sh`; tag-only **Relay Release**, [CI_CD.md](CI_CD.md) |
| Desktop Live subscriber protocol | **SHIPPED (v0.2.x add-on)** — `DesktopHub` + `/v2/desktop/live` (mTLS Desktop CA + Desktop ACL); Agent Live path unchanged |
| Persistence API | **OUT OF SCOPE** |

**Bottom line:** v1 Agent Live exit criteria met; Desktop subscriber dial path is available behind `hub: "desktop"`. On 2026-10-23 an operator reported a working Agent → Relay → Desktop Live flow. That operator observation complements the lab tests; it is not a substitute for production PKI, reconnect/failure, security or release-candidate qualification. Durable history belongs to a separate Persistence service, not Relay. Ship with `bash scripts/ci-cd.sh version` / `release` when ready to publish.

## How to use

1. Read [`../README.md`](../README.md).  
2. Operator ship: [`CI_CD.md`](CI_CD.md) via `scripts/ci-cd.sh`.  
3. Product brief: [`PROMPT.md`](PROMPT.md).  
4. Evidence matrix: [`phases/P10-ACCEPTANCE.md`](phases/P10-ACCEPTANCE.md) §4.

## Phases folder

Completed briefs (P00–P09, P11) were removed after implementation. Only **P10** remains under `phases/` (acceptance + evidence).

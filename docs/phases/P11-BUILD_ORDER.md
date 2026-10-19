# P11 — Build order checklist

**Parent brief:** [PROMPT.md](../PROMPT.md)  
Follow in order. Do not skip acceptance hooks.

---

## Day 0 — Repo bootstrap

- [ ] Create sibling repo `infrasible-relay` (Go module).  
- [ ] Add `VERSION.md` (`0.1.0`), `version.go`, `.gitignore` (`/dist/`).  
- [ ] Add `README.md` linking back to Agent `relay/PROMPT.md`.  
- [ ] `go mod init` + CI stub workflow (gofmt/vet/test).

---

## Day 1 — Protocol

- [ ] `internal/protocol` types + `DecodeAgentMessage` / encode helpers.  
- [ ] Golden JSON tests from [P03](P03-WIRE_AGENT_LIVE.md).  
- [ ] `ValidateAgentHello` table tests.

---

## Day 2 — Registrar

- [ ] ACL JSON schema + `NewFileRegistry`.  
- [ ] Lookup deny/allow tests ([P05](P05-REGISTRAR_FILE.md)).  
- [ ] Fingerprint helper (DER SHA-256).

---

## Day 3 — Fan-out stub + observe

- [ ] `SubscriberHub`, `NoopHub`, `LogHub`.  
- [ ] `DropOldestQueue` unit test (R6 building block).  
- [ ] `observe.Metrics` counters.

---

## Day 4 — Live session (fake conn)

- [ ] `SessionMap` Bind/Unbind replace.  
- [ ] `ServeAgent` with fake `Conn`: R2, R4, R5, R7, R8.  
- [ ] Heartbeat writer + silence timer.

---

## Day 5 — TLS front + main

- [ ] `tlsfront.ListenAndServe` TLS 1.3 + RequireAndVerifyClientCert.  
- [ ] WS upgrade on `live_path`.  
- [ ] `config.Load` / `Validate` + `cmd` wiring.  
- [ ] Lab mTLS fixtures (makefile or `scripts/lab-certs.sh`).

---

## Day 6 — Integration with Agent

- [ ] Run published Infrasible Agent against local Relay.  
- [ ] Confirm Hello → live → heartbeats in logs.  
- [ ] R1 with wrong cert; R9 smoke.  
- [ ] Record evidence in Relay `STATUS.md` / test output.

---

## Day 7 — Packaging

- [ ] systemd unit + postinst user/dirs.  
- [ ] `.deb` and `.rpm` scripts.  
- [ ] Example config/ACL under `/usr/share/…/examples`.  
- [ ] Install on a clean VM; end-to-end with Agent.

---

## Day 8 — CI/CD ship path (exact Agent port — [P09](P09-CI_CD.md))

- [ ] Copy Agent `scripts/ci-cd.sh` → Relay; rename Agent/`agentVersion` → Relay/`relayVersion`.  
- [ ] Copy `.github/workflows/ci.yml` as **Relay CI**; `release.yml` as **Relay Release** (`relayVersion` sed, title Infrasible Relay).  
- [ ] Port `scripts/package-linux.sh` for `infrasible-relay` + `SHA256SUMS`.  
- [ ] `VERSION.md` + `version.go` + drift test; `.gitignore` `/dist/`.  
- [ ] `docs/CI_CD.md` + `docs/TEST.md` matching Agent shape.  
- [ ] Prove: `bash scripts/ci-cd.sh check` → `version X.Y.Z` → green Relay CI → `release X.Y.Z` → `gh release view vX.Y.Z`.  
- [ ] Tag only when R1–R9 + D1–D3 green.

---

## Definition of done (v1)

| Gate | Source |
|---|---|
| R1–R9 | [P10](P10-ACCEPTANCE.md) |
| D1–D3 | [P06](P06-DESKTOP_STUB.md) / P10 |
| Packages installable | [P08](P08-PACKAGING_LINUX.md) |
| Tag release works | [P09](P09-CI_CD.md) |
| No Persistence/Desktop invent | [P01](P01-REQUIREMENTS.md) must-not |

---

## After v1 (backlog reminders)

1. Real Desktop hub behind `SubscriberHub`.  
2. HTTP registrar client behind `Registry`.  
3. CRL/OCSP.  
4. Multi-instance session lease.  
5. Dual-read `live_sample` with Agent team.  

Do not start these until v1 acceptance is green.

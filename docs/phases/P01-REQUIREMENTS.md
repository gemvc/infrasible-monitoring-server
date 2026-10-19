# P01 — Product requirements (Relay v1)

**Parent brief:** [PROMPT.md](../PROMPT.md)

---

## 1. Must

| ID | Requirement |
|---|---|
| M01 | Listen TLS 1.3 WebSocket with **client certificate required and verified** against configured Agent CA. |
| M02 | Authorize every session via `Registry.Lookup` (file ACL in v1); deny unregistered or wrong-channel Agents. |
| M03 | Negotiate `agent_hello` → `hello_accepted` before any `type: live` processing. |
| M04 | Accept executable Agent frames: `type: live` and `type: heartbeat` (protocol major 2). |
| M05 | Send application heartbeats (or equivalent traffic) so Agents do not hit ~3× silence. |
| M06 | Enforce **one active Live session per `agent_id`** (document replace-on-reconnect). |
| M07 | Bound message size (`max_message_bytes`, default 262144); reject/close on overflow. |
| M08 | Call `SubscriberHub.PublishLive` / `MarkStale` (stub OK); never block Agent read loop unboundedly. |
| M09 | Emit observe metrics/logs without secrets. |
| M10 | Versioned `config.json` validated at process start; invalid config → exit non-zero. |
| M11 | Linux packages (`.deb` / `.rpm`) + systemd non-root service. |
| M12 | CI: gofmt, tidy, vet, race tests; tag-only release. |
| M13 | Pass acceptance R1–R9 ([P10](P10-ACCEPTANCE.md)). |

---

## 2. Must not

| ID | Forbidden |
|---|---|
| N01 | Persist monitoring samples / act as time-series DB. |
| N02 | Accept Persistence 900s POST bodies on the Live listener. |
| N03 | Treat “valid client cert” alone as permission to ingest. |
| N04 | Require Agents to open inbound ports. |
| N05 | Replay Live history to Desktop/subscribers after reconnect. |
| N06 | Bundle or optionally require PostgreSQL inside Relay. |
| N07 | Merge Relay + Agent into one dual-mode binary. |
| N08 | Invent production Desktop subscriber wire in v1 (interface/stub only). |
| N09 | Log private keys, full client PEMs, or ACL secrets. |
| N10 | Silently ignore first-message-not-Hello (must close). |

---

## 3. Non-goals (explicit out of scope)

- Persistence API implementation and ACK semantics.  
- Historical chart API.  
- Full PKI enrollment/rotation automation (lab: operator-staged certs + file ACL).  
- Multi-region Relay mesh (document sticky/session map later; v1 = single instance OK).  
- Proposed Agent `live_sample` migration (accept `type: live` only for now).  
- Hot config reload (restart is OK for v1).

---

## 4. Quality attributes

| Attribute | Target |
|---|---|
| Isolation | One bad Agent must not stall others (timeouts, size caps). |
| Backpressure | Per-agent bounded drop of oldest live toward hub. |
| Clarity | Structured reject reasons: `unknown_cert`, `channel_denied`, `hello_invalid`, `oversized`, `silence`. |
| Operability | `config validate` CLI or equivalent; journald-friendly logs. |
| Security | Fail closed; TLS 1.3; key file mode `0600`. |

---

## 5. Stakeholders

| Role | Care about |
|---|---|
| Agent team | Wire compatibility with shipped Agent |
| Desktop team | Stable `SubscriberHub` / future subscribe ACL |
| Persistence team | No channel confusion; they own HTTPS finals |
| Operators | Packages, paths, runbook, sidecar Agent on Relay host |

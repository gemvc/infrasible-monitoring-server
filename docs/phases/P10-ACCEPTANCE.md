# P10 — Acceptance matrix (Relay exit criteria)

**Parent brief:** [PROMPT.md](../PROMPT.md)  
**Upstream cases:** Agent [`docs/design/MONITORING_RELAY.md`](../../docs/design/MONITORING_RELAY.md) §8  
v1 is **done** when R1–R9 pass and D1–D3 pass with stub hub.

---

## 1. Agent Live cases

| # | Case | Setup | Expected |
|---|---|---|---|
| **R1** | Unregistered client cert | Valid TLS client cert **not** in ACL | Connection denied or closed at/before Hello; **no** frame ingest; `auth_rejects` increments |
| **R2** | Registered happy path | ACL row + matching Hello | `agent_hello` → `hello_accepted` → `type: live` + heartbeats both ways |
| **R3** | Wrong `authorized_agent_id` | N/A (Relay bug if emitted) | Relay tests must assert accept echoes Hello id **exactly**; never ship wrong echo |
| **R4** | First message not Hello | Send `type: live` first | Close; no frames processed |
| **R5** | Agent disconnect | Close WS after live flowing | `MarkStale(agent_id)`; no history replay from Relay |
| **R6** | Slow subscribers | Hub queue capacity 2; stall consumer | Drop oldest; **other** Agents unaffected; `drops` increments |
| **R7** | Second connect same `agent_id` | Two clients with same ACL id | Single active session; replace policy closes old |
| **R8** | Oversized frame | Frame > `max_message_bytes` | Reject/close; no huge allocation / OOM |
| **R9** | Persistence-shaped traffic on Live | HTTP POST final JSON to Live path / wrong upgrade | Not accepted as Live session |

---

## 2. Desktop stub cases

| # | Case | Expected |
|---|---|---|
| **D1** | LogHub publish | Live frames logged; process stable |
| **D2** | MarkStale on disconnect | Logged/called once per policy |
| **D3** | hub=noop | Silent; still passes R2 |

---

## 3. Packaging / ops cases

| # | Case | Expected |
|---|---|---|
| **P1** | Invalid config | Process exits non-zero; no listen |
| **P2** | Key mode not 0600 | Fail validate or refuse start (document) |
| **P3** | `config validate` CLI | Exit 0 on good file |
| **P4** | systemd restart after ACL edit | New Agents authorized; old denials persist until restart |

---

## 4. Evidence

Recorded **2026-10-20**. Fake-conn / unit suite: `go test ./...`. Lab mTLS path: `./scripts/lab-up.sh` (Agent image from `gemvc/infrasible-agent`). Relay product `VERSION.md` = `0.2.2`.

| Case | Evidence |
|---|---|
| **R1** | `TestAcceptanceR1_UnknownIdentity` |
| **R2** | `TestAcceptanceR2_HappyPath` (live + peer HB + Relay→Agent HB) |
| **R3** | Asserted inside `TestAcceptanceR2_HappyPath` (`authorized_agent_id` echo) |
| **R4** | `TestAcceptanceR4_FirstNotHello` |
| **R5** | `TestAcceptanceR5_DisconnectMarkStale` |
| **R6** | `TestAcceptanceR6_BufferedHubIsolation` + `TestAcceptanceR6_SlowHubOtherAgentsOK` |
| **R7** | `TestAcceptanceR7_ReplaceSession` |
| **R8** | `TestAcceptanceR8_Oversized` |
| **R9** | `TestAcceptanceR9_PersistencePOST` |
| **D1/D2** | `TestLogHubNoPanic` |
| **D3** | `TestAcceptanceD3_NoopHubHappyPath` |
| **P1** | `TestAcceptanceP1_InvalidConfig` (`Validate` fails → `run` exits before listen) |
| **P2** | `TestAcceptanceP2_KeyModeNot0600` |
| **P3** | `TestAcceptanceP3_ConfigValidateCLI` (`validateConfigFile` / `config validate`) |
| **P4** | Documented in README ops notes + PROMPT §8: ACL load-once; `systemctl restart` after edit |

Do not claim production certification until PKI/Desktop owners sign off — staging “lab pass” is enough for Relay v1 implementer handoff.

---

## 5. Non-acceptance (explicit)

These passing does **not** mean:

- Desktop Live UI works.  
- Persistence ACK works.  
- Multi-region Relay HA works.  
- CRL/OCSP revocation is complete.

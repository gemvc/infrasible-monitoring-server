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

For each R-case record:

- Test name (`TestAcceptanceR2_HappyPath`) or manual checklist with date.  
- Agent version used (from Agent `VERSION.md` / release tag).  
- Relay commit SHA.

Do not claim production certification until PKI/Desktop owners sign off — staging “lab pass” is enough for Relay v1 implementer handoff.

---

## 5. Non-acceptance (explicit)

These passing does **not** mean:

- Desktop Live UI works.  
- Persistence ACK works.  
- Multi-region Relay HA works.  
- CRL/OCSP revocation is complete.

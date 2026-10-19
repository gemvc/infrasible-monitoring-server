# P03 — Agent Live wire contract

**Parent brief:** [PROMPT.md](../PROMPT.md)  
**Upstream SSOT:** [`docs/design/MONITORING_RELAY.md`](../../docs/design/MONITORING_RELAY.md)  
If this file and the Agent doc disagree on JSON fields, **prefer the Agent doc** and update this brief.

---

## 1. Transport

| Item | Value |
|---|---|
| Scheme | `wss://` only |
| Path (conventional) | `/v2/agents/live` |
| TLS | 1.3 |
| Client auth | Required + verified against Agent CA (`client_ca_file`) |
| Server identity | Agent verifies `server_name` / SNI against Relay cert |
| Message framing | WebSocket **text** frames carrying UTF-8 JSON |

---

## 2. Trust layers (recap)

1. **A — TLS:** handshake must succeed with client cert.  
2. **B — ACL:** fingerprint → `agent_id` + `live` + this `endpoint_id`.  
3. **C — Hello:** first application message must be valid `agent_hello`.  
4. **D — Session:** heartbeats, silence, size limits.

---

## 3. Message catalog

### 3.1 `agent_hello` (Agent → Relay) — first message only

```json
{
  "type": "agent_hello",
  "protocol_major": 2,
  "protocol_minor": 0,
  "agent_id": "<stable-agent-id>",
  "agent_version": "<semver>",
  "channel": "live",
  "schema_versions": ["2.0"],
  "capabilities": ["live_1hz", "sample_quality", "metric_cadence_v1", "metric_freshness_v1"],
  "sent_at_utc": "<RFC3339 UTC>"
}
```

| Field | Relay validation |
|---|---|
| `type` | Must be `agent_hello` |
| `protocol_major` | Must be `2` |
| `channel` | Must be `live` |
| `agent_id` | Non-empty; must equal ACL-bound id for this cert |
| `agent_version` | Present; log only |
| `schema_versions` | Must include a version Relay accepts (`2.0`) |
| `capabilities` | Informational for v1 |
| `sent_at_utc` | Parseable RFC3339 preferred; soft-fail policy: log if bad, still allow if other fields OK (document choice; prefer strict parse) |

First-message timeout: ~**10 seconds** after WS open.

### 3.2 `hello_accepted` (Relay → Agent)

```json
{
  "type": "hello_accepted",
  "protocol_major": 2,
  "protocol_minor": 0,
  "schema_version": "2.0",
  "authorized_agent_id": "<same agent_id>",
  "authorized_channel": "live",
  "endpoint_id": "<this-relay-endpoint-id>",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144
}
```

| Field | Rule |
|---|---|
| `authorized_agent_id` | **Exact echo** of Hello `agent_id` |
| `authorized_channel` | Always `live` |
| `endpoint_id` | From Relay config |
| `heartbeat_seconds` | Prefer `15`; must be in `(0, 300]` |
| `max_message_bytes` | Prefer `262144` |

Agent fail-closed checks these fields; wrong echo → Agent disconnects (acceptance R3: Relay must never emit wrong id).

### 3.3 `type: live` (Agent → Relay)

```json
{
  "type": "live",
  "agent_id": "<stable-agent-id>",
  "frame": {
    "observed_at_ms": 1792324801000,
    "elapsed_ms": 1000,
    "metrics": {
      "cpu": {
        "status": "valid",
        "measured_at_ms": 1792324801000,
        "unit": "percent",
        "value": {"total_pct": 12.5}
      }
    }
  }
}
```

Relay v1 validation:

- `type` == `live`  
- `agent_id` matches session  
- raw size ≤ `max_message_bytes`  
- JSON parseable  

Relay **must not** require a fixed metric set (Agent `enabled_families` varies).  
Relay **must not** persist. Forward opaque JSON bytes (or re-encode) to hub.

Do **not** require proposed `type: live_sample` yet.

### 3.4 `type: heartbeat` (both directions)

```json
{ "type": "heartbeat", "sent_at_utc": "2026-10-18T12:00:00Z" }
```

- Agent sends on interval from `hello_accepted`.  
- Relay should also send on the same interval (or faster).  
- Peer silence ≈ **3 × heartbeat_seconds** → Relay closes and `MarkStale`.

---

## 4. Unknown message types

**Fail closed:** close the session (do not silently ignore after Hello). Document in code comments and ops runbook.

---

## 5. What Agents send next (you do not implement)

Agents may also POST to Persistence HTTPS — different URL, different service. If someone POSTs a final-shaped body to the Live path: reject (R9).

---

## 6. Test fixtures

Align with Agent `internal/transport/contract_test.go` expectations:

- Happy path: Hello → accept → live + heartbeat.  
- Negative: bad Hello, missing cert (tlsfront), ACL miss.

Lab: generate CA, server cert (CN/SAN = Relay hostname), client cert; put fingerprint in ACL.

# Full engineered prompt — build Infrasible Monitoring Relay

**You are implementing a new Go service:** `infrasible-relay` (sibling repository; **not** this Agent repo).  
**Primary job (v1):** accept many Agent Live WebSocket sessions (mTLS + Hello + `type: live` + heartbeats), authorize via file ACL, keep one session per `agent_id`, expose observe metrics, and call a **Desktop stub hub** (no real Desktop protocol yet).  
**Success:** acceptance cases R1–R9 in [phases/P10-ACCEPTANCE.md](phases/P10-ACCEPTANCE.md) pass against a real Agent or the Agent’s contract expectations.

Read this document completely before writing code. Depth lives in [phases/](phases/). Doc readiness: [STATUS.md](STATUS.md).

---

## 1. Mission and ecosystem

```text
┌─────────────┐  outbound WSS/mTLS   ┌──────────────────┐  stub hub (v1)  ┌────────────────────┐
│ Linux Agent │ ──────────────────► │ Monitoring Relay │ ···············► │ Desktop (exists;   │
│ (many hosts)│   live samples only │ (YOU BUILD THIS) │  no real wire    │ Live not ready yet)│
└─────────────┘                     └──────────────────┘                   └────────────────────┘
        │
        │ outbound HTTPS/mTLS (NOT Relay)
        ▼
┌─────────────────┐
│ Persistence API │  ← 900s finals only; separate product; PostgreSQL
└─────────────────┘
```

| Service | Role | Your job |
|---|---|---|
| **Infrasible Agent** | Collects host metrics; dials out Live + Persistence | Already shipping — you consume its Live wire |
| **Monitoring Relay** | Terminate Live WSS; AuthZ; ephemeral fan-out | **Build this** |
| **Persistence API** | Durable 900s finals + ACK | Separate team/repo — do not implement here |
| **Desktop app** | Operator UI (already developed) | Not ready for realtime/historic yet — **stub only** |

**Hard product rule:** Do **not** merge Relay and Persistence into one binary with “optional PostgreSQL.” Same physical host with two systemd units is fine. Relay never stores monitoring history and never needs PostgreSQL.

**Monitor the Relay host:** install a normal Infrasible Agent on the same machine (sidecar). Prefer pointing that Agent at a peer Relay when possible so a dead primary does not blind you.

Depth: [P00-CONTEXT.md](phases/P00-CONTEXT.md).

---

## 2. Hard rules (fail closed)

1. **No history database** in Relay — no SQLite/Postgres/time-series for samples.  
2. **No Persistence traffic** on the Live listener — wrong channel.  
3. **mTLS success ≠ authorization** — always `registrar.Lookup` after TLS.  
4. **Agents dial out only** — Relay listens; Agents never open inbound management ports.  
5. **One active Live session per `agent_id`** — new connect replaces old; mark stale on disconnect.  
6. **Never block Agent readers** on slow Desktop — drop oldest live frames; count drops.  
7. **No secrets in logs** — no private keys, full cert PEMs, or tokens.  
8. **Protocol major 2** — reject other majors. Prefer `heartbeat_seconds: 15`.  
9. **v1 Desktop = stub** — implement `SubscriberHub` interface; do not invent final Desktop wire.  
10. **Clean module boundaries** — see §4 and [P02](phases/P02-ARCHITECTURE.md).

Requirements list: [P01-REQUIREMENTS.md](phases/P01-REQUIREMENTS.md).

---

## 3. Technology and repo layout

- **Language:** Go 1.23+  
- **Module path (suggested):** `github.com/<org>/infrasible-relay`  
- **WebSocket:** prefer `github.com/coder/websocket` (Agent uses it) or document an equivalent  
- **TLS:** Go `crypto/tls` — TLS 1.3, `ClientAuth: RequireAndVerifyClientCert`

```text
infrasible-relay/
  VERSION.md                 # product semver SSOT (one line X.Y.Z)
  version.go                 # synced const for binary
  cmd/infrasible-relay/      # main only — wire dependencies
  internal/
    config/                  # load + validate config.json
    protocol/                # message types + encode/decode
    tlsfront/                # TLS listener + WS upgrade
    registrar/               # Registry interface + FileRegistry
    livesession/             # Hello, loop, session map, silence
    fanout/                  # SubscriberHub + NoopHub/LogHub
    observe/                 # counters + structured logs
  packaging/                 # systemd unit, deb/rpm scripts
  scripts/ci-cd.sh
  .github/workflows/
  go.mod
```

---

## 4. Modules, clean code, contracts

### Principles

| Principle | How you apply it |
|---|---|
| **Single responsibility** | One package = one job (TLS ≠ Hello ≠ ACL ≠ fan-out) |
| **DRY** | All wire JSON types live in `internal/protocol` only |
| **Dependency rule** | `cmd` → `livesession`/`tlsfront` → `registrar`/`fanout`/`protocol`; never reverse |
| **Contracts** | Interfaces at boundaries: `Registry`, `SubscriberHub`, `Conn` abstraction if needed |
| **Fail closed** | Unknown cert, bad Hello, oversized frame → close; never “best effort ingest” |
| **Testability** | Table tests for decode/ACL; integration test with mTLS fixtures |

### Forbidden couplings

- `livesession` must not import packaging, CI, or Persistence client code.  
- `fanout` must not parse Agent Hello (that is `livesession` + `protocol`).  
- `registrar` must not open WebSockets.  
- `observe` must not decide AuthZ.  
- No global mutable “god” struct that every package writes.

Architecture depth: [P02-ARCHITECTURE.md](phases/P02-ARCHITECTURE.md).  
API signatures: [P04-MODULE_API.md](phases/P04-MODULE_API.md).

---

## 5. Trust layers (all must pass)

| Layer | What | Fail → |
|---|---|---|
| **A** Transport | TLS 1.3 + verify client cert against Agent CA | No WS session |
| **B** Registrar ACL | Cert identity → `agent_id` + `live` channel + this endpoint | Close |
| **C** Hello | First message `agent_hello`; echo `authorized_agent_id` | Close |
| **D** Session | Heartbeats both ways; silence ≈ 3× interval; size cap | Close + MarkStale |

Wire detail: [P03-WIRE_AGENT_LIVE.md](phases/P03-WIRE_AGENT_LIVE.md).  
File ACL: [P05-REGISTRAR_FILE.md](phases/P05-REGISTRAR_FILE.md).

---

## 6. Executable Agent Live wire (must implement)

Path convention: `wss://<relay-host>/v2/agents/live`

### 6.1 `agent_hello` (Agent → Relay)

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

Reject if: wrong `type`, `protocol_major` ≠ 2, `channel` ≠ `live`, empty/`agent_id` ≠ ACL binding, Live not allowed.

### 6.2 `hello_accepted` (Relay → Agent)

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

Echo the Agent’s ID **exactly**. Prefer `heartbeat_seconds: 15`.

### 6.3 `type: live` (Agent → Relay)

```json
{
  "type": "live",
  "agent_id": "<stable-agent-id>",
  "frame": {
    "observed_at_ms": 1792324801000,
    "elapsed_ms": 1000,
    "metrics": { }
  }
}
```

Do **not** require proposed `live_sample` yet. Do **not** persist frames.

### 6.4 Heartbeat (both directions)

```json
{ "type": "heartbeat", "sent_at_utc": "2026-10-18T12:00:00Z" }
```

### 6.5 Session loop (pseudocode)

```text
on_tls_websocket_accept:
  identity = peer client cert fingerprint / SPIFFE-style key
  acl = registrar.Lookup(identity)          # fail → close
  msg = read_first_json(timeout ~10s)
  if msg.type != "agent_hello": close
  if not authorize_live(acl, msg.agent_id): close
  write hello_accepted(...)
  Bind(agent_id, conn)                      # replace older if any
  loop:
    msg = read_json(max_message_bytes)
    switch msg.type:
      live:      hub.PublishLive(agent_id, raw); touch last_frame
      heartbeat: touch last_peer
      other:     close (document: fail closed)
    if peer_silent > 3*heartbeat: close
  on_close: hub.MarkStale(agent_id); Unbind if still this conn
```

Upstream SSOT: Agent `docs/design/MONITORING_RELAY.md`.

---

## 7. Named APIs (implement exactly these responsibilities)

| Package | Symbol | Input → Output |
|---|---|---|
| `protocol` | `DecodeAgentMessage(b []byte)` | → `(Message, error)` |
| `protocol` | `EncodeHelloAccepted(h HelloAccepted)` | → `([]byte, error)` |
| `protocol` | `EncodeHeartbeat(sentAt time.Time)` | → `([]byte, error)` |
| `tlsfront` | `ListenAndServe(ctx, cfg TLSConfig, h http.Handler)` | → `error` |
| `registrar` | `Registry.Lookup(ctx, id ClientIdentity)` | → `(ACLEntry, error)` |
| `registrar` | `NewFileRegistry(path string)` | → `(*FileRegistry, error)` |
| `livesession` | `ServeAgent(ctx, conn, id, reg, hub, obs, cfg)` | → (blocks until done) |
| `livesession` | `SessionMap.Bind(agentID, s)` / `Unbind(agentID, s)` | replace / clear |
| `fanout` | `SubscriberHub.PublishLive(agentID, frame []byte)` | drop-oldest if slow |
| `fanout` | `SubscriberHub.MarkStale(agentID)` | notify stub/log |
| `fanout` | `NewLogHub(logger)` / `NewNoopHub()` | stub implementations |
| `observe` | `Metrics` counters + `IncAuthReject(reason)` | no secrets |
| `config` | `Load(path) (Config, error)` + `Validate(Config) error` | fail at start |

Full signatures: [P04-MODULE_API.md](phases/P04-MODULE_API.md).

---

## 8. File registrar (v1)

JSON ACL file (path from config). Example:

```json
{
  "endpoint_id": "relay-primary",
  "agents": [
    {
      "agent_id": "agt_01HXYZ",
      "cert_fingerprint_sha256": "ab:cd:…",
      "channels": ["live"],
      "live_endpoints": ["relay-primary"]
    }
  ]
}
```

Lookup: fingerprint from client cert → entry → require `live` in `channels` and this `endpoint_id` in `live_endpoints`. Unknown → deny.

Reload: document process restart for v1 (no hot reload required). Later: HTTP registrar client behind same `Registry` interface.

Detail: [P05-REGISTRAR_FILE.md](phases/P05-REGISTRAR_FILE.md).

---

## 9. Desktop stub (v1)

Transible **Desktop already exists** but is **not ready** to consume realtime Live or historic APIs.  
Your v1 job:

```go
type SubscriberHub interface {
    PublishLive(agentID string, frame []byte)
    MarkStale(agentID string)
}
```

Ship `LogHub` (debug) and `NoopHub`. When Desktop is ready, add `DesktopHub` behind the **same** interface without changing `livesession`.

Detail: [P06-DESKTOP_STUB.md](phases/P06-DESKTOP_STUB.md).

---

## 10. Config, observe, ops

Example `/etc/infrasible-relay/config.json`:

```json
{
  "config_version": 1,
  "listen_addr": ":8443",
  "live_path": "/v2/agents/live",
  "endpoint_id": "relay-primary",
  "server_cert_file": "/var/lib/infrasible-relay/server.crt",
  "server_key_file": "/var/lib/infrasible-relay/server.key",
  "client_ca_file": "/etc/infrasible-relay/agents-ca.pem",
  "acl_file": "/etc/infrasible-relay/acl.json",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "log"
}
```

Observe (minimum): connected agents, auth rejects by reason, frames forwarded, drops, last frame age per agent. Expose via structured logs + optional Prometheus text endpoint **localhost-only** or Unix socket (document choice).

Detail: [P07-CONFIG_AND_OPS.md](phases/P07-CONFIG_AND_OPS.md).

---

## 11. Versioning

| Kind | Where | Notes |
|---|---|---|
| Relay **product** semver | Relay repo `VERSION.md` (one line) | Packages, tags `vX.Y.Z`, Hello-facing `endpoint` metadata |
| **Protocol** major/minor | Wire JSON `protocol_major` / `protocol_minor` | Major **2** today; not the same as product semver |
| This **brief** semver | [`relay/VERSION.md`](VERSION.md) in Agent repo | Documentation set only |

Sync `VERSION.md` → `version.go` in CI (mirror Agent pattern). Never stamp product versions into random markdown.

---

## 12. Linux packaging

- User/group: `infrasible-relay` (non-root).  
- Config: `/etc/infrasible-relay/config.json`  
- Data/TLS: `/var/lib/infrasible-relay/` — server key mode `0600`, owner service user.  
- systemd unit: `infrasible-relay.service` — listens inbound WSS.  
- Distros: Ubuntu/Debian (`.deb`) and RHEL/Fedora (`.rpm`).  
- Do **not** bundle PostgreSQL.

Detail: [P08-PACKAGING_LINUX.md](phases/P08-PACKAGING_LINUX.md).

---

## 13. CI / CD (exactly this Agent project)

Port Agent [`docs/CI_CD.md`](../docs/CI_CD.md), [`scripts/ci-cd.sh`](../scripts/ci-cd.sh), [`.github/workflows/ci.yml`](../.github/workflows/ci.yml), [`.github/workflows/release.yml`](../.github/workflows/release.yml), and [`scripts/package-linux.sh`](../scripts/package-linux.sh). Rename Agent → Relay / `agentVersion` → `relayVersion` only. Do not invent a different ship model.

| Where | When | What |
|---|---|---|
| `scripts/ci-cd.sh` | daily / ship | check, version, release |
| **Relay CI** | every push / PR | `gofmt`, tidy, vet, race tests |
| **Relay Release** | tag `vX.Y.Z` only | Linux packages + GitHub Release |

Push to `main` never publishes a Release. Semver SSOT: Relay `VERSION.md` (one line); `version` syncs `version.go`.

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

Do not reuse an existing `vX.Y.Z` tag. Do not commit `dist/`. Local packages without tagging: `bash scripts/package-linux.sh`.

Full port checklist + workflow YAML: [P09-CI_CD.md](phases/P09-CI_CD.md).

---

## 14. Acceptance exit criteria (summary)

| # | Case | Expected |
|---|---|---|
| R1 | Unregistered client cert | Denied / closed; no ingest |
| R2 | Registered Agent happy path | Hello → accept → live + heartbeats |
| R3 | Wrong `authorized_agent_id` | Must not happen; Agent would reconnect |
| R4 | First message not Hello | Close |
| R5 | Agent disconnect | `MarkStale`; no history replay |
| R6 | Slow hub | Drop oldest; other Agents OK |
| R7 | Second connect same `agent_id` | Replace; single active |
| R8 | Oversized frame | Reject/close |
| R9 | Persistence-shaped HTTP on Live | Not accepted |

Full matrix: [P10-ACCEPTANCE.md](phases/P10-ACCEPTANCE.md).

---

## 15. Build order

Follow [P11-BUILD_ORDER.md](phases/P11-BUILD_ORDER.md): protocol → registrar → tlsfront → livesession → observe → config/main → packaging → CI → lab with real Agent.

---

## 16. Out of scope (do not build in Relay v1)

- Persistence API / PostgreSQL / 900s ACK  
- Historical charts API  
- Real Desktop subscriber protocol  
- PKI issuance/rotation automation (operators stage certs; file ACL is enough for lab)  
- Embedding Agent collectors inside Relay  
- Dual-mode “Agent or Relay” binary  

---

## 17. Lab smoke (definition of done for first staging)

1. Generate lab CA + Relay server cert + Agent client cert.  
2. Write `acl.json` binding fingerprint → `agent_id`.  
3. Run Relay; configure Agent `live_url` / mTLS paths; restart Agent.  
4. Confirm logs: Hello accepted, `type: live` arriving, heartbeats both ways.  
5. Disconnect Agent → `MarkStale` logged.  
6. R1–R9 automated or manual checklist signed.

When Desktop becomes ready, implement a real hub behind `SubscriberHub` without rewriting the Agent Live path.

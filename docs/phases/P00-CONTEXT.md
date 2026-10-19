# P00 — System context

**Parent brief:** [PROMPT.md](../PROMPT.md)  
**Purpose:** Orient the implementer in the Infrasible monitoring ecosystem before writing Relay code.

---

## 1. Actors

| Actor | What it is today | Relation to Relay |
|---|---|---|
| **Infrasible Agent** | Go daemon on each Linux host; collects metrics; SQLite; outbound Live + Persistence | Dials `wss://…/v2/agents/live` with mTLS |
| **Monitoring Relay** | **You build this** — central Live Bridge | Terminates Agent Live; AuthZ; ephemeral forward |
| **Persistence API** | Separate central service (PostgreSQL-backed durable store) | Agents POST 900s finals here — **not** via Relay |
| **Desktop (Transible)** | Existing operator application | **Already developed**, but **not ready** yet for realtime Live or historic charts from central APIs |
| **PKI / registrar** | Control plane for certs ↔ agent_id ACL | v1 Relay uses a **file ACL**; later may call a real registrar behind the same interface |
| **Operator** | Installs packages, stages certs, edits config via SSH/Ansible | Never uses Live as a management channel |

---

## 2. Data flows

```text
  [N × Linux hosts]
         │
         │  WSS/mTLS  type: live (~1 Hz) + heartbeats
         ▼
  [Monitoring Relay] ----stub----> [Desktop Live UI]   (future; LogHub/NoopHub in v1)
         │
         │  (no sample storage)

  [N × Linux hosts]
         │
         │  HTTPS/mTLS  900s immutable finals
         ▼
  [Persistence API + PostgreSQL] ----future----> [Desktop Historic UI]
```

**Rule of channels**

- **Live** = fresh samples only; lose-on-disconnect is OK; no replay from Relay.  
- **Persistence** = durable quarters; ACK required; never mixed into Live WebSocket.  
- **Desktop** will eventually subscribe to Live via Relay and read history via Persistence/Historical API — two different contracts.

---

## 3. Why Relay is separate from Persistence

| | Relay | Persistence |
|---|---|---|
| State | Ephemeral sessions | Durable rows |
| Storage | None for metrics | PostgreSQL required for production |
| Protocol | WSS | HTTPS POST |
| Failure mode | Drop oldest live; mark stale | Must not lose unACKed finals |
| Scale knob | Concurrent sockets + fan-out | Write throughput + retention |

**Do not** ship one installer with “with or without PostgreSQL.” That conflates two products.  
**Do** allow co-location: same VM runs `infrasible-relay.service` and `infrasible-persistence.service` (or vendor Persistence) as **two units**.

---

## 4. Why Desktop is a stub in Relay v1

- Desktop product already exists for ops (SSH, inventory, etc.).  
- Realtime multi-server Live and central historic views are **not wired** yet.  
- Inventing a final Desktop subscriber protocol in Relay v1 would create throwaway work and block Agent Live delivery.  
- Relay still defines `SubscriberHub` so Desktop can plug in later without rewriting session code. See [P06-DESKTOP_STUB.md](P06-DESKTOP_STUB.md).

---

## 5. Monitoring the Relay host itself

The Relay process is infrastructure; it must be monitored like any other server:

1. Install **Infrasible Agent** on the Relay host (sidecar).  
2. Prefer Live target = **peer Relay** or secondary endpoint so primary outage is still visible.  
3. Relay **observe** metrics (sessions, rejects, drops) are *service* health — complementary to Agent host metrics (CPU, disk, etc.).

---

## 6. Trust boundary sketch

```text
Internet / VPC
    │
    ▼
Relay public (or VPC) WSS :8443
    │  TLS terminate + client cert required
    ▼
AuthZ (file ACL / registrar)
    │
    ▼
livesession map (agent_id → conn)
    │
    ▼
SubscriberHub (stub → later Desktop ACL)
```

Agents never accept inbound Live. Desktop never reads Agent SQLite over the network for Live.

---

## 7. References

- Agent wire contract: [`docs/design/MONITORING_RELAY.md`](../../docs/design/MONITORING_RELAY.md)  
- Agent architecture context: [`docs/design/AGENT_ARCHITECTURE.md`](../../docs/design/AGENT_ARCHITECTURE.md)  
- Agent ship status: [`docs/STATUS.md`](../../docs/STATUS.md)

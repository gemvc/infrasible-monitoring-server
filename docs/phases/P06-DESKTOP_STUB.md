# P06 — Desktop subscriber stub

**Parent brief:** [PROMPT.md](../PROMPT.md)

---

## 1. Desktop readiness (fact for implementers)

The **Transible Desktop** application is **already fully developed** for existing operator workflows.  
It is **not yet ready** to consume:

- realtime multi-server Live streams from the Monitoring Relay, or  
- historic charts from Persistence / Historical APIs.

Therefore Relay **v1 must not** invent a production Desktop WebSocket/HTTP subscribe protocol. Doing so would create throwaway coupling and delay Agent Live delivery.

Relay **v1 must** still define a clean extension point so Desktop can connect later without rewriting Agent session code.

---

## 2. Contract: `SubscriberHub`

```go
package fanout

// SubscriberHub is the only livesession dependency for outbound fan-out.
// v1 implementations: NoopHub, LogHub.
// Future: DesktopHub (authenticated subscribers, per-agent ACL).
type SubscriberHub interface {
    // PublishLive delivers one fresh Live envelope (JSON bytes) for agentID.
    // Must not block unbounded; drop oldest on pressure and count drops.
    PublishLive(agentID string, frame []byte)

    // MarkStale signals that agentID has no active Live session.
    // Desktop UI must show disconnected/stale when a real hub exists.
    MarkStale(agentID string)
}
```

### Rules

| Rule | Detail |
|---|---|
| Fresh only | Never queue historical backlog across Agent reconnects |
| Backpressure | Per-agent queue capacity small (e.g. 2); drop **oldest** |
| Isolation | Slow Desktop must not stall Agent read loops |
| AuthZ later | Which Desktop users see which `agent_id`s is **not** v1 |
| No Persistence | Hub never reads PostgreSQL or Agent SQLite |

---

## 3. v1 implementations

### `NoopHub`

- `PublishLive` / `MarkStale` are no-ops.  
- Use for load tests focused on Agent ingest.

### `LogHub`

- Logs at debug/info: `publish_live agent_id=… bytes=…` and `mark_stale agent_id=…`.  
- Truncate payload in logs (e.g. first 200 bytes) — never dump secrets (frames should not contain keys anyway).  
- Default for staging so operators see activity in journald.

Config: `"hub": "log"` | `"noop"`.

---

## 4. Future Desktop hub (do not implement in v1)

When Desktop team is ready, add roughly:

```text
Desktop ── auth ──► Relay subscriber endpoint ──► DesktopHub ──► per-user agent ACL
                         ▲
                         │ PublishLive / MarkStale from livesession
```

Requirements for that future work (informational only):

- Separate authenticated channel from Agent Live.  
- Forward fresh frames only; on Agent reconnect start from new samples.  
- UI shows stale on `MarkStale`.  
- Same `SubscriberHub` interface — **zero** changes to Hello/live loop.

---

## 5. Acceptance for stub

| ID | Case | Expected |
|---|---|---|
| D1 | Happy path with LogHub | Each live frame logs once; no panic |
| D2 | Agent disconnect | `MarkStale` invoked once per disconnect (or once per replace policy) |
| D3 | Swap Noop ↔ Log via config | Restart picks hub; livesession untouched |

Agent-facing R5/R6 still apply with stub hub ([P10](P10-ACCEPTANCE.md)).

# P02 — Architecture, modularity, clean code

**Parent brief:** [PROMPT.md](../PROMPT.md)  
**APIs:** [P04-MODULE_API.md](P04-MODULE_API.md)

---

## 1. Logical modules

```text
cmd/infrasible-relay
        │
        ├── config.Load / Validate
        ├── observe.New
        ├── registrar.NewFileRegistry
        ├── fanout.NewLogHub | NewNoopHub
        └── tlsfront.ListenAndServe ──► livesession.ServeAgent
                                              │
                                              ├── protocol.Decode / Encode
                                              ├── registrar.Lookup
                                              ├── SessionMap.Bind / Unbind
                                              └── hub.PublishLive / MarkStale
```

| Package | Responsibility | Owns | Must not |
|---|---|---|---|
| `protocol` | Wire JSON types + encode/decode | Message structs | TLS, ACL, fan-out |
| `tlsfront` | TLS 1.3 listener, client-cert verify, WS upgrade | Cert files load | Hello logic |
| `registrar` | Identity → ACL | File parse / later HTTP client | WebSockets |
| `livesession` | Hello, frame loop, silence, session map | Per-agent conn lifetime | Persistence, packaging |
| `fanout` | Subscriber hub abstraction | Backpressure drop policy toward Desktop | Parse Hello |
| `observe` | Counters, structured logs | Metric names | Auth decisions |
| `config` | Versioned JSON config | Validation rules | Runtime sessions |

---

## 2. Dependency rules

1. Lower packages never import higher ones (`protocol` is a leaf).  
2. `cmd` is the only place that constructs the full graph.  
3. Cross-cutting concerns (logging, metrics) injected as interfaces/structs — not package-level globals that hide tests.  
4. Prefer small interfaces (`Registry`, `SubscriberHub`) defined next to the consumer or in a tiny `internal/contracts` if shared — avoid “interface soup.”

---

## 3. Clean-code checklist

| Practice | Relay application |
|---|---|
| **SRP** | One reason to change per package |
| **DRY** | One decoder for Agent messages; no copy-paste JSON tags across packages |
| **Fail closed** | Default deny; explicit allow paths |
| **Explicit errors** | Typed or sentinel errors for AuthZ vs protocol vs I/O |
| **No magic** | Heartbeat 15, max 262144, silence 3× — named constants in config/defaults |
| **Concurrency** | Per-session goroutine; session map guarded by mutex or `sync.Map` with clear replace rules |
| **Resource limits** | Read deadlines; max frame size; bounded hub queues (e.g. capacity 2, drop oldest) |
| **Test first at contracts** | Table tests for `DecodeAgentMessage`, ACL Lookup, Hello validation |

---

## 4. Session map contract

- Key: `agent_id` (string from Hello, must match ACL).  
- Value: active connection + generation token (monotonic) so late Unbind of an old conn does not clear a newer Bind.  
- On Bind of same `agent_id`: close previous conn; notify hub stale then fresh (or single stale then new live — document one policy).  
- Recommended policy: **replace** — close old, `MarkStale` once, then serve new session.

---

## 5. Error taxonomy (log / observe labels)

| Code | Meaning |
|---|---|
| `tls_handshake` | Client cert missing/untrusted |
| `unknown_identity` | Fingerprint not in ACL |
| `channel_denied` | No `live` or wrong endpoint |
| `hello_timeout` | No first message in time |
| `hello_invalid` | Bad type/fields/id mismatch |
| `oversized` | Frame > max |
| `peer_silence` | 3× heartbeat elapsed |
| `replaced` | Superseded by new session |

Never attach key material to these events.

---

## 6. Versioning inside the architecture

- Product semver (`VERSION.md`) identifies the **binary/package**.  
- Wire `protocol_major` identifies the **message contract** (2).  
- Changing wire incompatible → bump protocol major + coordinated Agent release.  
- Changing packaging paths / observe names → product semver only if operators must react.

---

## 7. Future extension points (design now, implement later)

| Extension | Hook |
|---|---|
| HTTP registrar | New type implementing `Registry` |
| Real Desktop protocol | New type implementing `SubscriberHub` |
| Multi-instance Relay | External session lease; still one Live per agent globally |
| `live_sample` dual-read | Branch in `protocol` + livesession; keep `type: live` until Agents migrate |

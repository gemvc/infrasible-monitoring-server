# P07 — Configuration, observe, runbook

**Parent brief:** [PROMPT.md](../PROMPT.md)

---

## 1. Config file

Path: `/etc/infrasible-relay/config.json`  
Loaded at process start only (restart to apply).

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

| Field | Validation |
|---|---|
| `config_version` | Must be `1` |
| `listen_addr` | Non-empty host:port |
| `live_path` | Must start with `/` |
| `endpoint_id` | Non-empty; must match ACL file `endpoint_id` |
| `*_file` paths | Must exist and be readable by service user at start |
| `server_key_file` | Regular file; mode `0600`; owned by service user (fail or hard warn) |
| `heartbeat_seconds` | `(0, 300]` |
| `max_message_bytes` | `> 0` and ≤ reasonable cap (e.g. 1 MiB) |
| `hub` | `log` or `noop` |
| Unknown fields | Fail validation |

```sh
infrasible-relay config validate --config=/etc/infrasible-relay/config.json
sudo systemctl restart infrasible-relay
```

---

## 2. Observe metrics (minimum)

| Metric | Type | Labels / notes |
|---|---|---|
| `relay_agents_connected` | gauge | Current Live sessions |
| `relay_auth_rejects_total` | counter | `reason=` unknown_identity, channel_denied, … |
| `relay_frames_total` | counter | `agent_id=` (bounded cardinality — or omit high-card label in Prometheus; use logs) |
| `relay_drops_total` | counter | Hub backpressure drops |
| `relay_last_frame_unixtime` | gauge | Per agent in snapshot API / logs |
| `relay_hello_accepts_total` | counter | Successful Hellos |

**Cardinality caution:** prefer aggregated counters + sampled per-agent logs over unbounded Prometheus label sets.

### Exposure options (pick one, document)

1. Structured JSON logs to journald (always).  
2. Optional `GET 127.0.0.1:<admin>/metrics` Prometheus text — **localhost bind only**.  

Never expose admin metrics on the public Live listener without auth.

---

## 3. Logging rules

- Levels: info for session open/close; debug for per-frame if needed.  
- Include: `agent_id`, reject `reason`, `endpoint_id`, durations.  
- Exclude: private keys, full PEMs, raw ACL file dumps, Authorization headers (future).

---

## 4. Operator runbook

### Healthy

```sh
systemctl status infrasible-relay
journalctl -u infrasible-relay -f
# expect: hello_accepted, frames, heartbeats
```

### Agents fail to connect

| Symptom | Check |
|---|---|
| TLS errors on Agent | Relay server cert SAN vs Agent `server_name`; Agent trusts `ca_file` |
| Handshake OK, then close | ACL fingerprint; `agent_id` match; `channels` / `live_endpoints` |
| Hello then silence timeout | Relay not sending heartbeats; firewall dropping |
| Reconnect loops | Wrong `authorized_agent_id` echo; protocol major |

### After ACL change

1. Edit `acl.json`.  
2. Validate JSON.  
3. `systemctl restart infrasible-relay`.  
4. Confirm Agent reconnects.

### Relay host monitoring

Install Infrasible Agent sidecar; point Live at peer Relay when possible. Relay observe metrics ≠ host CPU metrics.

### Disk / memory

Relay should stay light (no sample DB). If RSS grows, check session leaks / unbounded hub buffers — both are bugs.

---

## 5. Security ops

- Rotate Relay server cert: stage files, restart.  
- Revoke Agent: remove ACL row (+ CRL later); restart.  
- Backup: config + ACL + certs (keys encrypted at rest in ops vault — not Relay’s job).

---

## 6. Relation to Agent config

Agents use (shape):

```json
"live_url": "wss://relay.example.com/v2/agents/live",
"server_name": "relay.example.com",
"ca_file": "...",
"client_cert_file": "...",
"client_key_file": "..."
```

Relay does not push config to Agents. Operators stage Agent config via existing Desktop SSH/Ansible paths.

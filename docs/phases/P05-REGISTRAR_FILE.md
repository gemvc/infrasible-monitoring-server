# P05 — File registrar ACL (v1)

**Parent brief:** [PROMPT.md](../PROMPT.md)  
**Interface:** [P04-MODULE_API.md](P04-MODULE_API.md) §3

v1 uses a **local JSON ACL file**. A future HTTP registrar implements the same `Registry` interface without changing `livesession`.

---

## 1. File location

Default: `/etc/infrasible-relay/acl.json` (path from `config.acl_file`).  
Owner: `root:infrasible-relay` or `infrasible-relay:infrasible-relay`.  
Mode: `0640` recommended (readable by service user, not world).

---

## 2. Schema

```json
{
  "acl_version": 1,
  "endpoint_id": "relay-primary",
  "agents": [
    {
      "agent_id": "agt_01HEXAMPLE",
      "cert_fingerprint_sha256": "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
      "channels": ["live"],
      "live_endpoints": ["relay-primary"],
      "comment": "optional operator note"
    }
  ]
}
```

| Field | Required | Rules |
|---|---|---|
| `acl_version` | yes | Must be `1` for this schema |
| `endpoint_id` | yes | Must equal Relay `config.endpoint_id` (validate at load) |
| `agents` | yes | Array; may be empty (deny all) |
| `agent_id` | yes | Non-empty; unique in file |
| `cert_fingerprint_sha256` | yes | 64 hex chars, lowercase preferred; unique |
| `channels` | yes | Must include `"live"` for Live access |
| `live_endpoints` | yes | Must include this Relay’s `endpoint_id` |
| `comment` | no | Ignored by code |

**Fingerprint algorithm:** SHA-256 over the DER bytes of the **leaf** client certificate. Document encoding: **lowercase hex without colons** (normalize colons away on Lookup if operators paste OpenSSL-style output).

---

## 3. Lookup algorithm

```text
fp = SHA256(leaf_cert.Raw) as lowercase hex
entry = index_by_fingerprint[fp]
if missing → ErrUnknownIdentity
if hello.agent_id != entry.agent_id → ErrAgentMismatch
if "live" not in entry.channels → ErrChannelDenied
if config.endpoint_id not in entry.live_endpoints → ErrChannelDenied
return entry
```

Order: TLS already verified the cert chain; ACL only authorizes registration binding.

---

## 4. Load and reload

| Behavior | v1 |
|---|---|
| Load | Once at process start in `NewFileRegistry` |
| Invalid JSON / duplicate fingerprint / duplicate agent_id | Fail start |
| Hot reload | **Not required** — restart systemd after ACL edit |
| Caching | In-memory map after parse |

Optional later: `SIGHUP` reload behind the same struct — not a v1 acceptance gate.

---

## 5. Operator workflow (lab)

1. Issue Agent client cert from lab CA.  
2. Compute fingerprint:  
   `openssl x509 -in client.crt -outform DER | sha256sum`  
3. Add ACL row; set Agent `agent_id` in Hello to the same string Agents already have in SQLite (`agent_id` is created on Agent first boot — copy from Agent health/status).  
4. `systemctl restart infrasible-relay`  
5. Restart Agent with `live_url` pointing at Relay.

---

## 6. What file ACL is not

- Not a substitute for certificate revocation lists long-term (add CRL/OCSP later at tlsfront).  
- Not Persistence channel authorization (Persistence has its own ACL).  
- Not Desktop subscriber authorization (future Desktop hub).

---

## 7. Tests

| Case | Expect |
|---|---|
| Known fp + matching hello id | OK |
| Unknown fp | `ErrUnknownIdentity` |
| Wrong hello id | `ErrAgentMismatch` |
| channels without live | `ErrChannelDenied` |
| live_endpoints mismatch | `ErrChannelDenied` |
| Duplicate fp in file | Load error |

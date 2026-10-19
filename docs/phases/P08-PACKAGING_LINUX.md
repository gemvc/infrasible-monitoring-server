# P08 — Linux packaging and install layout

**Parent brief:** [PROMPT.md](../PROMPT.md)  
Mirror the spirit of Agent packaging (`packaging/`, non-root systemd) but **inbound listener** instead of outbound-only.

---

## 1. Targets

| Distro family | Package |
|---|---|
| Ubuntu / Debian | `.deb` |
| RHEL / Fedora / Rocky / Alma | `.rpm` |

Architectures: start with `amd64`/`x86_64`; add `arm64` when CI allows.

**Do not** bundle PostgreSQL or Agent binaries inside the Relay package.

---

## 2. Filesystem layout

| Path | Purpose |
|---|---|
| `/usr/bin/infrasible-relay` | Binary |
| `/etc/infrasible-relay/config.json` | Main config (example installed if missing) |
| `/etc/infrasible-relay/acl.json` | File ACL (example empty/deny-all if missing) |
| `/etc/infrasible-relay/agents-ca.pem` | Operator-staged Agent CA (not auto-generated) |
| `/var/lib/infrasible-relay/server.crt` | Relay server certificate |
| `/var/lib/infrasible-relay/server.key` | Relay private key — mode **`0600`**, owner service user |
| `/lib/systemd/system/infrasible-relay.service` | Unit file |

---

## 3. Service user

- Create system user/group: `infrasible-relay` (nologin, home `/var/lib/infrasible-relay`).  
- Binary runs as that user.  
- Capabilities: bind high ports by default (`:8443`); if binding `:443`, document `CAP_NET_BIND_SERVICE` or reverse-proxy TLS termination (prefer app-level TLS for mTLS client certs — proxy must forward client certs correctly or terminate mTLS at Relay).

**Recommendation:** Relay terminates mTLS itself on `:8443` (or internal VIP). Avoid naive TLS-offload that strips client certificates.

---

## 4. systemd unit (sketch)

```ini
[Unit]
Description=Infrasible Monitoring Relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=infrasible-relay
Group=infrasible-relay
ExecStart=/usr/bin/infrasible-relay run --config=/etc/infrasible-relay/config.json
Restart=on-failure
RestartSec=3
# Hardening (tune as needed)
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/infrasible-relay
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

`postinst`: create user/dirs; install example config/ACL **only if absent**; **do not** auto-start until certs exist (or start and fail closed with clear logs).

---

## 5. Example config on first install

Ship commented examples under `/usr/share/infrasible-relay/examples/` and copy to `/etc` if missing. Empty ACL = deny all Agents (safe default).

---

## 6. Co-location with Persistence

Same host may install Persistence package separately. Shared nothing except possibly ops CA procedures. Two units:

```text
infrasible-relay.service
infrasible-persistence.service   # other product
infrasible-agent.service         # sidecar to monitor this host
```

---

## 7. Version in packages

Package version = Relay product `VERSION.md` (`X.Y.Z`).  
Tag Release assets: `infrasible-relay_X.Y.Z_amd64.deb`, etc.  
Do not embed Agent version numbers in Relay package names.

---

## 8. Lab install checklist

1. Install `.deb`/`.rpm`.  
2. Stage `agents-ca.pem`, `server.crt`, `server.key` (`0600`).  
3. Write `acl.json` + `config.json`; validate.  
4. `systemctl enable --now infrasible-relay`.  
5. Open firewall for Live port from Agent networks only.  
6. Point an Agent at `wss://…/v2/agents/live` and verify Hello.

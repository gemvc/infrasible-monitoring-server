# Infrasible Monitoring Relay

Central **Live Bridge** for [Infrasible Agents](https://github.com/gemvc/infrasible-agent): terminate many outbound Agent Live WebSocket sessions (TLS 1.3 mTLS + Hello + `type: live` + heartbeats), authorize via file ACL, keep one session per `agent_id`, and fan out to Desktop subscribers through `DesktopHub` (Desktop dials Relay; never the reverse).

**Not** the Persistence API. Relay never stores monitoring history and never needs PostgreSQL.

**Product version:** [`VERSION.md`](VERSION.md) (one line `X.Y.Z`)  
**Implementer brief:** [`docs/PROMPT.md`](docs/PROMPT.md) · **Status:** [`docs/STATUS.md`](docs/STATUS.md)  
**Agent wire SSOT:** [MONITORING_RELAY.md](https://github.com/gemvc/infrasible-agent/blob/main/docs/design/MONITORING_RELAY.md)

## Quick start — Docker Desktop lab (recommended)

Full Relay ↔ Agent Live path (mTLS, Hello, `type: live`) via Docker Compose:

```sh
./scripts/lab-up.sh
```

Details: [`lab/README.md`](lab/README.md). Tear down: `docker compose -f lab/docker-compose.yml down -v`.

## Quick start — local Go only

Requires Go 1.23+.

```sh
bash scripts/lab-certs.sh ./testdata/certs
go run ./cmd/infrasible-relay run --config=./testdata/certs/config.json
```

Point an Agent at `wss://127.0.0.1:8443/v2/agents/live` with the generated client certs and matching `agent_id` from `testdata/certs/acl.json`.

With `hub: "desktop"` (lab-certs default), Desktop dials `wss://127.0.0.1:8443/v2/desktop/live?agent_id=agt_lab_01` using the **Desktop** client cert (separate CA + `desktop-acl.json`). First message is `desktop_subscribe` (not Agent Hello). Relay forwards authorized Agent `type: live` bytes and emits `type: stale` on disconnect.

```sh
go test ./...
go run ./cmd/infrasible-relay version
```

## Layout

```text
cmd/infrasible-relay/     main — wire dependencies
internal/
  config/                 config.json load + validate
  protocol/               wire JSON encode/decode (Agent)
  tlsfront/               TLS 1.3 + mTLS listener (Agent CA + optional Desktop CA)
  registrar/              Agent file ACL Registry
  livesession/            Agent Hello, loop, session map, silence
  desktophub/             DesktopHub + Desktop ACL + /v2/desktop/live
  fanout/                 SubscriberHub (LogHub / NoopHub / BufferedHub / MultiHub)
  observe/                counters (no secrets)
docs/                     PROMPT, STATUS, CI_CD; phases/P10 acceptance
```

## Ops notes (v1)

- **ACL reload:** file ACL is loaded once at process start. After editing `acl.json` or `desktop-acl.json`, restart the unit (`systemctl restart infrasible-relay`) so new identities are authorized; denials for removed rows persist until restart.
- **Config check:** `infrasible-relay config validate --config=/etc/infrasible-relay/config.json` (exit 0 on good file; also checks ACL `endpoint_id` match).
- **Observe:** JSON snapshot at `http://127.0.0.1:9090/metrics` (override bind with `RELAY_ADMIN_ADDR`; default localhost-only). Not Prometheus text format.
- **Ship:** [`docs/CI_CD.md`](docs/CI_CD.md) via `bash scripts/ci-cd.sh`. Packages: `.deb` / `.rpm` (systemd + examples) and `.tar.gz`.

## Hard rules (v1)

- mTLS success ≠ authorization — always ACL lookup after TLS  
- One active Live session per `agent_id` (replace on reconnect)  
- Never block Agent readers on slow Desktop — drop oldest  
- Desktop dials Relay on `/v2/desktop/live` (Desktop CA + Desktop ACL); Agent Live path untouched; no Persistence traffic on Live  

## License

See [LICENSE](LICENSE).

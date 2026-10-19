# Infrasible Monitoring Relay

Central **Live Bridge** for [Infrasible Agents](https://github.com/gemvc/infrasible-agent): terminate many outbound Agent Live WebSocket sessions (TLS 1.3 mTLS + Hello + `type: live` + heartbeats), authorize via file ACL, keep one session per `agent_id`, and fan out through a Desktop **stub** hub.

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

```sh
go test ./...
go run ./cmd/infrasible-relay version
```

## Layout

```text
cmd/infrasible-relay/     main — wire dependencies
internal/
  config/                 config.json load + validate
  protocol/               wire JSON encode/decode
  tlsfront/               TLS 1.3 + mTLS listener
  registrar/              file ACL Registry
  livesession/            Hello, loop, session map, silence
  fanout/                 SubscriberHub stub (LogHub / NoopHub)
  observe/                counters (no secrets)
docs/                     PROMPT, STATUS, CI_CD; phases/P10 acceptance
```

## Hard rules (v1)

- mTLS success ≠ authorization — always ACL lookup after TLS  
- One active Live session per `agent_id` (replace on reconnect)  
- Never block Agent readers on slow Desktop — drop oldest  
- Desktop = `SubscriberHub` stub only; no Persistence traffic on Live  

## License

See [LICENSE](LICENSE).

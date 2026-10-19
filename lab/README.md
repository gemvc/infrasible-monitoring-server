# Docker Desktop lab — Relay ↔ Agent Live

Self-contained stack:

1. **bootstrap** — lab CA, Relay server cert (`DNS:relay`), Agent client cert, seed Agent SQLite, write ACL + configs  
2. **relay** — mTLS WSS Live Bridge on `:8443`  
3. **agent** — Linux Infrasible Agent dialing `wss://relay:8443/v2/agents/live`

Agent image is built from the public repo [gemvc/infrasible-agent](https://github.com/gemvc/infrasible-agent).

## Prerequisites

- Docker Desktop running  
- Network access to pull base images and clone the Agent repo (build time)

## Run

From the **monitoring-server** repo root:

```sh
./scripts/lab-up.sh
```

Or manually:

```sh
docker compose -f lab/docker-compose.yml up --build
```

Success looks like Relay logs containing `session.accepted` and `hub.publish_live`.

```sh
curl -s http://127.0.0.1:9090/metrics | jq .
docker compose -f lab/docker-compose.yml logs -f relay agent
```

## Tear down

```sh
docker compose -f lab/docker-compose.yml down -v
```

`down -v` removes the lab volume (certs, ACL, seeded `agent_id`).

## Overrides

| Env | Default | Purpose |
|---|---|---|
| `AGENT_REPO` | `https://github.com/gemvc/infrasible-agent.git` | Agent git URL for image build |
| `AGENT_REF` | `main` | Branch/tag to clone |

```sh
AGENT_REF=v0.4.1 ./scripts/lab-up.sh
```

## Notes

- Lab containers run as root so key files mode `0600` stay readable (prod packages use `infrasible-relay` / `infrasible-agent` users).  
- Persistence is intentionally **not** in this lab.  
- Host port **8443** (Live) and **9090** (localhost metrics JSON) are published.

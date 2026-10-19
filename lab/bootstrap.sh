#!/usr/bin/env bash
# Prepare shared /lab volume: mTLS material, seeded Agent DB, ACL, Relay+Agent configs.
set -euo pipefail

LAB="${LAB_DIR:-/lab}"
ENDPOINT_ID="${ENDPOINT_ID:-relay-primary}"
RELAY_HOST="${RELAY_HOST:-relay}"
RELAY_PORT="${RELAY_PORT:-8443}"

mkdir -p "$LAB"/{certs,relay,agent,agent-data,state}
cd "$LAB/certs"

if [[ ! -f ca.pem ]]; then
  echo "generating lab PKI..."
  openssl req -x509 -newkey rsa:2048 -nodes -keyout ca.key -out ca.pem -days 3650 \
    -subj "/CN=Infrasible Lab Agents CA" >/dev/null 2>&1

  openssl req -newkey rsa:2048 -nodes -keyout server.key -out server.csr \
    -subj "/CN=${RELAY_HOST}" >/dev/null 2>&1
  openssl x509 -req -in server.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
    -out server.crt -days 825 -sha256 \
    -extfile <(printf "subjectAltName=DNS:%s,DNS:localhost,IP:127.0.0.1" "$RELAY_HOST") >/dev/null 2>&1
  chmod 600 server.key

  openssl req -newkey rsa:2048 -nodes -keyout agent.key -out agent.csr \
    -subj "/CN=agt_lab_client" >/dev/null 2>&1
  openssl x509 -req -in agent.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
    -out agent.crt -days 825 -sha256 >/dev/null 2>&1
  chmod 600 agent.key
  rm -f server.csr agent.csr
fi

FP=$(openssl x509 -in agent.crt -outform DER | openssl dgst -sha256 | awk '{print $2}')

# Seed Agent SQLite so we learn the stable agent_id before Relay starts.
OFFLINE_CFG="$LAB/agent/offline.json"
cat >"$OFFLINE_CFG" <<EOF
{
  "config_version": 1,
  "data_path": "$LAB/agent-data/agent.db",
  "enabled_families": ["cpu", "memory", "load"],
  "monitoring_servers": []
}
EOF

if [[ ! -f "$LAB/agent-data/agent.db" ]]; then
  echo "seeding agent database..."
  # Agent is Linux-only; create identity then exit.
  timeout 4s infrasible-agent run --config="$OFFLINE_CFG" >/tmp/agent-seed.log 2>&1 || true
fi

if [[ ! -f "$LAB/agent-data/agent.db" ]]; then
  echo "failed to seed agent.db; log:" >&2
  cat /tmp/agent-seed.log >&2 || true
  exit 1
fi

AGENT_ID=$(infrasible-agent health status --config="$OFFLINE_CFG" | jq -r .agent_id)
if [[ -z "$AGENT_ID" || "$AGENT_ID" == "null" ]]; then
  echo "could not read agent_id" >&2
  exit 1
fi
echo "$AGENT_ID" >"$LAB/state/agent_id"
echo "$FP" >"$LAB/state/agent_fingerprint"
echo "agent_id=$AGENT_ID fingerprint=$FP"

# ACL binds cert fingerprint → agent_id for this Relay endpoint.
cat >"$LAB/relay/acl.json" <<EOF
{
  "acl_version": 1,
  "endpoint_id": "${ENDPOINT_ID}",
  "agents": [
    {
      "agent_id": "${AGENT_ID}",
      "cert_fingerprint_sha256": "${FP}",
      "channels": ["live"],
      "live_endpoints": ["${ENDPOINT_ID}"],
      "comment": "docker lab agent"
    }
  ]
}
EOF

cat >"$LAB/relay/config.json" <<EOF
{
  "config_version": 1,
  "listen_addr": ":${RELAY_PORT}",
  "live_path": "/v2/agents/live",
  "endpoint_id": "${ENDPOINT_ID}",
  "server_cert_file": "${LAB}/certs/server.crt",
  "server_key_file": "${LAB}/certs/server.key",
  "client_ca_file": "${LAB}/certs/ca.pem",
  "acl_file": "${LAB}/relay/acl.json",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "log"
}
EOF

# Live Agent config (reuses seeded DB + lab client certs).
cat >"$LAB/agent/config.json" <<EOF
{
  "config_version": 1,
  "data_path": "$LAB/agent-data/agent.db",
  "enabled_families": ["cpu", "memory", "load"],
  "monitoring_servers": [
    {
      "id": "lab-relay",
      "live_url": "wss://${RELAY_HOST}:${RELAY_PORT}/v2/agents/live",
      "server_name": "${RELAY_HOST}",
      "ca_file": "${LAB}/certs/ca.pem",
      "client_cert_file": "${LAB}/certs/agent.crt",
      "client_key_file": "${LAB}/certs/agent.key"
    }
  ]
}
EOF

# Public material readable; private keys must stay 0600 (Relay + Agent enforce this).
chmod -R a+rX "$LAB/relay" "$LAB/agent" "$LAB/state"
chmod -R a+rwX "$LAB/agent-data"
chmod 755 "$LAB" "$LAB/certs" "$LAB/relay" "$LAB/agent" "$LAB/agent-data"
chmod 644 "$LAB/certs/ca.pem" "$LAB/certs/server.crt" "$LAB/certs/agent.crt"
chmod 600 "$LAB/certs/server.key" "$LAB/certs/agent.key"
# Do not chmod -R the certs dir after this — it would clear 0600 on keys.

# Write a marker for depends_on condition
date -u +%Y-%m-%dT%H:%M:%SZ >"$LAB/state/ready"
echo "lab bootstrap complete → $LAB"

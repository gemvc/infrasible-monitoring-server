#!/usr/bin/env bash
# Generate lab CA + Relay server cert + Agent client cert for local mTLS.
set -euo pipefail

OUT="${1:-./testdata/certs}"
mkdir -p "$OUT"
cd "$OUT"

openssl req -x509 -newkey rsa:2048 -nodes -keyout ca.key -out ca.pem -days 3650 \
  -subj "/CN=Infrasible Lab Agents CA"

openssl req -newkey rsa:2048 -nodes -keyout server.key -out server.csr \
  -subj "/CN=localhost"
openssl x509 -req -in server.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out server.crt -days 825 -sha256 \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1")
chmod 600 server.key

openssl req -newkey rsa:2048 -nodes -keyout agent.key -out agent.csr \
  -subj "/CN=agt_lab_01"
openssl x509 -req -in agent.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out agent.crt -days 825 -sha256
chmod 600 agent.key

FP=$(openssl x509 -in agent.crt -outform DER | openssl dgst -sha256 | awk '{print $2}')
cat > acl.json <<EOF
{
  "acl_version": 1,
  "endpoint_id": "relay-primary",
  "agents": [
    {
      "agent_id": "agt_lab_01",
      "cert_fingerprint_sha256": "${FP}",
      "channels": ["live"],
      "live_endpoints": ["relay-primary"],
      "comment": "lab agent"
    }
  ]
}
EOF

cat > config.json <<EOF
{
  "config_version": 1,
  "listen_addr": "127.0.0.1:8443",
  "live_path": "/v2/agents/live",
  "endpoint_id": "relay-primary",
  "server_cert_file": "$(pwd)/server.crt",
  "server_key_file": "$(pwd)/server.key",
  "client_ca_file": "$(pwd)/ca.pem",
  "acl_file": "$(pwd)/acl.json",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "log"
}
EOF

echo "Lab material written to $OUT"
echo "Agent fingerprint: $FP"
echo "Run: go run ./cmd/infrasible-relay run --config=$OUT/config.json"

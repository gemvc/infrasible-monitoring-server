#!/usr/bin/env bash
# Build and run the full Docker Desktop lab (Relay + Agent + mTLS).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMPOSE=(docker compose -f lab/docker-compose.yml)

echo "==> building & starting lab"
"${COMPOSE[@]}" up --build -d

echo "==> waiting for Relay accept + live frames"
deadline=$((SECONDS + 120))
ok=0
while (( SECONDS < deadline )); do
  if "${COMPOSE[@]}" logs relay 2>/dev/null | grep -q 'session.accepted'; then
    if "${COMPOSE[@]}" logs relay 2>/dev/null | grep -q 'hub.publish_live'; then
      ok=1
      break
    fi
  fi
  sleep 2
done

echo
echo "==> recent relay logs"
"${COMPOSE[@]}" logs --tail=40 relay || true
echo
echo "==> recent agent logs"
"${COMPOSE[@]}" logs --tail=40 agent || true
echo
echo "==> lab state"
"${COMPOSE[@]}" run --rm --no-deps -v infrasible-lab_labdata:/lab debian:bookworm-slim \
  bash -lc 'echo agent_id=$(cat /lab/state/agent_id 2>/dev/null); echo fp=$(cat /lab/state/agent_fingerprint 2>/dev/null); ls -la /lab/state /lab/certs 2>/dev/null | head' \
  2>/dev/null || docker run --rm -v infrasible-lab_labdata:/lab debian:bookworm-slim \
  bash -lc 'echo agent_id=$(cat /lab/state/agent_id); echo fp=$(cat /lab/state/agent_fingerprint)'

if [[ "$ok" -eq 1 ]]; then
  echo
  echo "LAB PASS: Hello accepted and live frames observed on Relay"
  echo "Metrics snapshot: curl -s http://127.0.0.1:9090/metrics | jq ."
  echo "Stop: docker compose -f lab/docker-compose.yml down -v"
  exit 0
fi

echo
echo "LAB FAIL: did not observe session.accepted + hub.publish_live within timeout" >&2
echo "Debug: docker compose -f lab/docker-compose.yml logs -f" >&2
exit 1

#!/usr/bin/env bash
#
# quick-start.sh — full end-to-end demo of the IaaS Platform: bring up
# PostgreSQL, boot the server, sign up, create an organization and a compute
# instance, watch it reach "running", check billing, then tear down.
#
# Prerequisites: docker, curl, jq, Go 1.26+. Runs from the repo root.
#
# Usage:  bash examples/quick-start.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BASE_URL="${BASE_URL:-http://localhost:8080}"
EMAIL="${EMAIL:-demo_$(date +%s)@example.com}"
PASSWORD="${PASSWORD:-changeme123}"

for cmd in docker curl jq go; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "missing required command: $cmd" >&2; exit 1; }
done

say() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }

say "Starting PostgreSQL (docker compose up -d)"
docker compose up -d

say "Starting the server (go run ./cmd/server)"
: >/tmp/iaas-quickstart.log
go run ./cmd/server >/tmp/iaas-quickstart.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true; docker compose down -v >/dev/null 2>&1 || true' EXIT

say "Waiting for readiness at $BASE_URL/readyz"
for _ in $(seq 1 30); do
  if curl -sf "$BASE_URL/readyz" >/dev/null 2>&1; then echo "  ready"; break; fi
  sleep 1
done

req() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json')
  [[ -n "$JWT" ]] && args+=(-H "Authorization: Bearer $JWT")
  [[ -n "$body" ]] && args+=(-d "$body")
  curl "${args[@]}" "$BASE_URL$path"
}

say "Signing up $EMAIL"
SIGNUP=$(req POST /api/v1/auth/signup "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Demo User\"}")
JWT=$(echo "$SIGNUP" | jq -r '.token')
echo "$SIGNUP" | jq -c '{user:.user.email, token:.token[0:24] + "…", api_key:(.api_key[0:12] + "…" if .api_key else null)}'

say "Creating organization 'demo-org'"
ORG=$(req POST /api/v1/orgs '{"name":"Demo Org","slug":"demo-org"}')
ORG_ID=$(echo "$ORG" | jq -r '.id')
echo "$ORG" | jq -c .

say "Creating instance web-01 (pending → running in ~5s)"
INST=$(req POST "/api/v1/orgs/$ORG_ID/instances" '{"name":"web-01","instance_type":"vm","region":"us-east-1","image":"ubuntu-24.04","cpu_cores":2,"memory_mb":4096,"disk_gb":50}')
INSTANCE_ID=$(echo "$INST" | jq -r '.id')
echo "$INST" | jq -c '{id,name,status,region}'

say "Watching the async lifecycle"
for _ in $(seq 1 12); do
  S=$(req GET "/api/v1/orgs/$ORG_ID/instances/$INSTANCE_ID" | jq -r '.status')
  printf '  %s\r' "$S"
  [[ "$S" == "running" ]] && { printf '  %s\n' "$S"; break; }
  sleep 1
done

say "Billing summary"
req GET "/api/v1/orgs/$ORG_ID/billing/usage" | jq -c .

say "Demo complete. Dashboard: $BASE_URL  (log: /tmp/iaas-quickstart.log)"
say "Tearing down: stopping server and removing the Postgres volume"

#!/usr/bin/env bash
#
# api-usage.sh — end-to-end walkthrough of the IaaS Platform HTTP API using
# curl + jq. Every authenticated call works with either a JWT (Bearer) or the
# API key (X-API-Key) returned at signup.
#
# Prerequisites: curl, jq, and a running server (see ../README.md or
# quick-start.sh). Override via env:
#   BASE_URL  (default http://localhost:8080)
#   EMAIL     (default dev_<ts>@example.com — a fresh user each run)
#   PASSWORD  (default changeme123)
#
# Usage:  bash examples/api-usage.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
EMAIL="${EMAIL:-dev_$(date +%s)@example.com}"
PASSWORD="${PASSWORD:-changeme123}"

say()  { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
req()  { # req METHOD PATH [JSON_BODY]  — uses the JWT when present
  local method="$1" path="$2" body="${3:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json')
  [[ -n "$JWT" ]] && args+=(-H "Authorization: Bearer $JWT")
  [[ -n "$body" ]] && args+=(-d "$body")
  curl "${args[@]}" "$BASE_URL$path"
}
json() { jq -c "$@"; }

say "Signing up ${EMAIL}"
SIGNUP=$(req POST /api/v1/auth/signup "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Demo User\"}")
echo "$SIGNUP" | json .
JWT=$(echo "$SIGNUP" | json -r '.token')
API_KEY=$(echo "$SIGNUP" | json -r '.api_key // empty')
echo "API key (shown once): ${API_KEY:0:12}…"

say "GET /api/v1/me"
req GET /api/v1/me | json .

say "Creating an organization"
ORG=$(req POST /api/v1/orgs '{"name":"Demo Org","slug":"demo-org"}')
echo "$ORG" | json .
ORG_ID=$(echo "$ORG" | json -r '.id')

say "Listing organizations"
req GET /api/v1/orgs | json .

say "Creating an instance (provisioning is asynchronous — expect 202 pending)"
INST=$(req POST "/api/v1/orgs/$ORG_ID/instances" '{"name":"web-01","instance_type":"vm","region":"us-east-1","image":"ubuntu-24.04","cpu_cores":2,"memory_mb":4096,"disk_gb":50}')
echo "$INST" | json .
INSTANCE_ID=$(echo "$INST" | json -r '.id')

say "GET /api/v1/orgs/$ORG_ID/instances/$INSTANCE_ID (poll until running)"
for _ in $(seq 1 12); do
  STATUS=$(req GET "/api/v1/orgs/$ORG_ID/instances/$INSTANCE_ID" | json -r '.status')
  echo "  status: $STATUS"
  [[ "$STATUS" == "running" ]] && break
  sleep 2
done

say "Stopping the instance (async, returns 202)"
req POST "/api/v1/orgs/$ORG_ID/instances/$INSTANCE_ID/stop" | json .

say "Listing instances"
req GET "/api/v1/orgs/$ORG_ID/instances" | json .

say "Billing — record usage then read the 30-day summary"
req POST "/api/v1/orgs/$ORG_ID/billing/usage" '{"resource_type":"cpu_hours","quantity":2}' >/dev/null
req GET "/api/v1/orgs/$ORG_ID/billing/usage" | json .

say "Generating an invoice for the current month"
req POST "/api/v1/orgs/$ORG_ID/billing/invoices/generate" | json .
req GET "/api/v1/orgs/$ORG_ID/billing/invoices" | json .

say "Terminating the instance"
req POST "/api/v1/orgs/$ORG_ID/instances/$INSTANCE_ID/terminate" | json .

say "Done. Reuse the account with EMAIL=$EMAIL PASSWORD=$PASSWORD"
[[ -n "$API_KEY" ]] && echo "Or use the API key: curl -H 'X-API-Key: $API_KEY' $BASE_URL/api/v1/me"

#!/usr/bin/env sh
set -eu

base_url=${BALANCE_URL:-http://localhost:8080}
body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT

health_status=000
attempt=0
while [ "$attempt" -lt 30 ]; do
  health_status=$(curl -sS -o "$body_file" -w '%{http_code}' "$base_url/api/balance/healthz" || printf '000')
  if [ "$health_status" = "200" ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$health_status" != "200" ]; then
  echo "balance health failed: HTTP $health_status" >&2
  cat "$body_file" >&2
  exit 1
fi

analyze_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/analyze" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"0445-000"}]}')
if [ "$analyze_status" != "501" ]; then
  echo "balance analyze connectivity failed: HTTP $analyze_status" >&2
  cat "$body_file" >&2
  exit 1
fi

if ! grep -q '"code":"tb1_not_implemented"' "$body_file"; then
  echo "balance analyze returned an unexpected body" >&2
  cat "$body_file" >&2
  exit 1
fi

echo "balance smoke: health=200 analyze=501"

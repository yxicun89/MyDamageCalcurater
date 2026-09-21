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

# TB1: the local overlay mounts testdata/pokemon-types.example.json (fictional IDs from 9001-000)
# and sets BALANCE_POKEMON_TYPES_PATH, so a known ID must return 200 with the analysis.
# Right after a rollout the Ingress can briefly route to a terminating Pod (502/503), so retry only those.
attempt=0
while :; do
  analyze_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
    -X POST "$base_url/api/balance/v1/team-balance/analyze" \
    -H 'Content-Type: application/json' \
    -H 'X-Device-Id: smoke-device' \
    -H 'X-Session-Id: smoke-session' \
    --data '{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9002-000"}]}' || printf '000')
  case "$analyze_status" in
    000|502|503) ;;
    *) break ;;
  esac
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    break
  fi
  sleep 1
done
if [ "$analyze_status" != "200" ]; then
  echo "balance analyze failed: HTTP $analyze_status (is the example read model mounted?)" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"members"' '"teamSummary"' '"pokemonId":"9001-000"' '"quadWeak"' '"source":"type"'; do
  if ! grep -q "$key" "$body_file"; then
    echo "balance analyze body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done

unknown_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/analyze" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9999-999"}]}')
if [ "$unknown_status" != "422" ] || ! grep -q '"code":"unknown_pokemon"' "$body_file"; then
  echo "balance analyze unknown pokemon: HTTP $unknown_status, want 422 unknown_pokemon" >&2
  cat "$body_file" >&2
  exit 1
fi

echo "balance smoke: health=200 analyze=200 unknown=422"

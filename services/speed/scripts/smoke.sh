#!/usr/bin/env sh
set -eu

base_url=${SPEED_URL:-http://localhost:8080}
body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT

health_status=000
attempt=0
while [ "$attempt" -lt 30 ]; do
  health_status=$(curl -sS -o "$body_file" -w '%{http_code}' "$base_url/api/speed/healthz" || printf '000')
  if [ "$health_status" = "200" ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$health_status" != "200" ]; then
  echo "speed health failed: HTTP $health_status" >&2
  cat "$body_file" >&2
  exit 1
fi

# SP0 (ADR-0600 §4/§5): the local overlay mounts testdata/pokemon.example.json (fictional IDs
# from 9001-000) and sets SPEED_POKEMON_PATH, so the pokemon list must answer 200 with the
# 8 example pokemon. Right after a rollout the Ingress can briefly route to a terminating Pod
# (502/503), so retry only those.
attempt=0
while :; do
  pokemon_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
    "$base_url/api/speed/v1/pokemon" \
    -H 'X-Device-Id: smoke-device' \
    -H 'X-Session-Id: smoke-session' || printf '000')
  case "$pokemon_status" in
    000|502|503) ;;
    *) break ;;
  esac
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    break
  fi
  sleep 1
done
if [ "$pokemon_status" != "200" ]; then
  echo "speed pokemon list failed: HTTP $pokemon_status (is the example read model mounted?)" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"regulationId":"example"' '"pokemonId":"9001-000"' '"nameJa":"カソウドリ"' '"baseSpeed":100'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "speed pokemon list body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done
count=$(grep -o '"pokemonId"' "$body_file" | wc -l | tr -d ' ')
if [ "$count" != "8" ]; then
  echo "speed pokemon list count = $count, want 8" >&2
  cat "$body_file" >&2
  exit 1
fi

missing_headers_status=$(curl -sS -o "$body_file" -w '%{http_code}' "$base_url/api/speed/v1/pokemon")
if [ "$missing_headers_status" != "400" ] || ! grep -qF '"code":"invalid_request"' "$body_file"; then
  echo "speed pokemon list without headers: HTTP $missing_headers_status, want 400 invalid_request" >&2
  cat "$body_file" >&2
  exit 1
fi

echo "speed smoke: health=200 pokemon=200 (count=8) missing_headers=400"

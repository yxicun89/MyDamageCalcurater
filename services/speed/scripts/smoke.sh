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
for key in '"regulationId":"example"' '"pokemonId":"9001-000"' '"nameJa":"テストカソウドリ"' '"baseSpeed":100'; do
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

# SP1 (ADR-0601 §3-§5): the table of the example read model. 9002-000 and 9005-000 share base
# speed 81, so their max-scarf rows (146 x 1.5 = 219) form one tie tier, in pokemonId order.
table_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  "$base_url/api/speed/v1/table?presets=max-scarf" \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' || printf '000')
if [ "$table_status" != "200" ]; then
  echo "speed table failed: HTTP $table_status" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"regulationId":"example"' '"presets":["max-scarf"]' '"speed":219'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "speed table body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done
# The generated types encode fields in alphabetical order: a tier is {"entries":[...],"speed":N}.
tie_pattern='"entries":\[\{[^}]*"pokemonId":"9002-000"[^}]*\},\{[^}]*"pokemonId":"9005-000"[^}]*\}\],"speed":219'
if ! grep -qE "$tie_pattern" "$body_file"; then
  echo "speed table has no tie tier 219 with 9002-000 and 9005-000" >&2
  cat "$body_file" >&2
  exit 1
fi
tier_count=$(grep -o '"speed":' "$body_file" | wc -l | tr -d ' ')
if [ "$tier_count" != "7" ]; then
  echo "speed table tier count = $tier_count, want 7 (8 pokemon, one tie)" >&2
  cat "$body_file" >&2
  exit 1
fi

invalid_presets_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  "$base_url/api/speed/v1/table?presets=unknown" \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session')
if [ "$invalid_presets_status" != "400" ] || ! grep -qF '"code":"invalid_request"' "$body_file"; then
  echo "speed table with unknown presets: HTTP $invalid_presets_status, want 400 invalid_request" >&2
  cat "$body_file" >&2
  exit 1
fi

# SP2 (ADR-0602 §3/§4): one's own position in the table. 9002-000 at max is 146, which 9005-000 also
# reaches (same base speed 81), so the tie holds both rows and faster/slower split the remaining 46 of
# the 48 rows (8 pokemon x 6 presets).
position_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  "$base_url/api/speed/v1/position" \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  -H 'Content-Type: application/json' \
  -d '{"mode":"preset","pokemonId":"9002-000","preset":"max","scarf":false}' || printf '000')
if [ "$position_status" != "200" ]; then
  echo "speed position failed: HTTP $position_status" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"speed":146' '"faster":29' '"slower":17' '"pokemonId":"9005-000"'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "speed position body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done

invalid_position_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  "$base_url/api/speed/v1/position" \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  -H 'Content-Type: application/json' \
  -d '{"mode":"preset","pokemonId":"9002-000","preset":"max","scarf":false,"sp":32}')
if [ "$invalid_position_status" != "400" ] || ! grep -qF '"code":"invalid_request"' "$body_file"; then
  echo "speed position with a field the mode does not need: HTTP $invalid_position_status, want 400 invalid_request" >&2
  cat "$body_file" >&2
  exit 1
fi

unknown_position_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  "$base_url/api/speed/v1/position" \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  -H 'Content-Type: application/json' \
  -d '{"mode":"preset","pokemonId":"9999-000","preset":"max","scarf":false}')
if [ "$unknown_position_status" != "422" ] || ! grep -qF '"code":"unknown_pokemon"' "$body_file"; then
  echo "speed position with an unknown pokemonId: HTTP $unknown_position_status, want 422 unknown_pokemon" >&2
  cat "$body_file" >&2
  exit 1
fi

echo "speed smoke: health=200 pokemon=200 (count=8) missing_headers=400 table=200 (tiers=7, tie 219) invalid_presets=400 position=200 (146, 29/2/17) invalid_position=400 unknown_pokemon=422"

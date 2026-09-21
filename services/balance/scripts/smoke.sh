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
  if ! grep -qF "$key" "$body_file"; then
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
if [ "$unknown_status" != "422" ] || ! grep -qF '"code":"unknown_pokemon"' "$body_file"; then
  echo "balance analyze unknown pokemon: HTTP $unknown_status, want 422 unknown_pokemon" >&2
  cat "$body_file" >&2
  exit 1
fi

# TB2 (ADR-0016): the local overlay also mounts testdata/moves.example.json (fictional IDs from move-9001)
# and sets BALANCE_MOVES_PATH. move-9001/move-9002 are fire (special/physical), move-9006 is a status move,
# move-9005 is normal. analyze already answered 200 above, so the Pod is routable; no retry here.
coverage_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/coverage" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9001-000","moveIds":["move-9001","move-9002","move-9006"]},{"pokemonId":"9002-000","moveIds":["move-9005"]}]}' || printf '000')
if [ "$coverage_status" != "200" ]; then
  echo "balance coverage failed: HTTP $coverage_status (is the example move read model mounted?)" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"members"' '"teamCoverage"' '"attackTypes":["fire"]' '"bestMultiplier"' '"effectiveMembers"' '"superEffectiveMembers"'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "balance coverage body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done

unknown_move_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/coverage" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9001-000","moveIds":["move-9999"]}]}')
if [ "$unknown_move_status" != "422" ] || ! grep -qF '"code":"unknown_move"' "$body_file"; then
  echo "balance coverage unknown move: HTTP $unknown_move_status, want 422 unknown_move" >&2
  cat "$body_file" >&2
  exit 1
fi

echo "balance smoke: health=200 analyze=200 unknown=422 coverage=200 unknown_move=422"

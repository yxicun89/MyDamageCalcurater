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

# TB3 (ADR-0017): the local overlay also mounts testdata/abilities.example.json (fictional IDs from ability-9001)
# and sets BALANCE_ABILITIES_PATH. ability-9002 absorbs water (9002-000 grass: water x0), ability-9004 multiplies
# super effective hits by 3/4 (9003-000 water/ground: grass x4 -> x3).
ability_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/analyze" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9002-000","abilityId":"ability-9002"},{"pokemonId":"9003-000","abilityId":"ability-9004"}]}' || printf '000')
if [ "$ability_status" != "200" ]; then
  echo "balance analyze with abilityId failed: HTTP $ability_status (is the example ability read model mounted?)" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"abilityId":"ability-9002"' '"abilityId":"ability-9004"' '"effect":"absorb"' '"effect":"multiplier"' '"source":"ability"' '"multiplier":"3"'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "balance analyze with abilityId body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done

unknown_ability_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/analyze" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9001-000","abilityId":"ability-9999"}]}')
if [ "$unknown_ability_status" != "422" ] || ! grep -qF '"code":"unknown_ability"' "$body_file"; then
  echo "balance analyze unknown ability: HTTP $unknown_ability_status, want 422 unknown_ability" >&2
  cat "$body_file" >&2
  exit 1
fi

# TB4 (ADR-0400): threats reuses the three example read models above. 9002-000 grass with move-9001 (fire) and
# 9003-000 water/ground with ability-9004 and no moves, against 9005-000 ice with move-9008 (ice):
# incoming x2 and x1 (ice vs water/ground is x1, so the super effective x3/4 does not apply), outgoing x2 and null.
threats_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/threats" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9002-000","moveIds":["move-9001"]},{"pokemonId":"9003-000","moveIds":[],"abilityId":"ability-9004"}],"threats":[{"pokemonId":"9005-000","moveIds":["move-9008"]}]}' || printf '000')
if [ "$threats_status" != "200" ]; then
  echo "balance threats failed: HTTP $threats_status (are the example read models mounted?)" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"threats"' '"matchups"' '"attackTypes":["ice"]' '"incoming":"2",' '"incoming":"1",' '"outgoing":"2",' '"outgoing":null' '"safeMembers":0' '"superEffectiveMembers":1'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "balance threats body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done

unknown_threat_move_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/threats" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9002-000","moveIds":[]}],"threats":[{"pokemonId":"9005-000","moveIds":["move-9999"]}]}')
if [ "$unknown_threat_move_status" != "422" ] || ! grep -qF '"code":"unknown_move"' "$body_file" || ! grep -qF '"message":"unknown moveId: move-9999"' "$body_file"; then
  echo "balance threats unknown move: HTTP $unknown_threat_move_status, want 422 unknown_move" >&2
  cat "$body_file" >&2
  exit 1
fi

# TB5 (ADR-0401): recommendations reuses the three example read models (the pokemon read model also carries
# nameJa and abilityIds). 9005-000 (ice) alone resists only ice, so 17 attack types are defense holes; steel/fairy
# (9006-000 テストメタル) is among the first 10 candidates, and ability-9002 (absorb water, 9001-000) and
# ability-9001 (immune to ground, 9006-000) fill holes as ability options. No move: offenseHoles is [].
recommendations_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/team-balance/recommendations" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"members":[{"pokemonId":"9005-000","moveIds":[]}]}' || printf '000')
if [ "$recommendations_status" != "200" ]; then
  echo "balance recommendations failed: HTTP $recommendations_status (is the example pokemon read model mounted?)" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"defenseHoles"' '"offenseHoles":[]' '"candidates"' '"abilityOptions"' '"types":["steel","fairy"]' '"nameJa":"テストメタル"' '"abilityId":"ability-9002"' '"abilityId":"ability-9001"' '"multiplier":"0"'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "balance recommendations body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done

# TB6 (ADR-0404): move-range/analyze takes moveIds only. move-9003 is water: with the bundled type chart,
# 9002-000 (テストリーフ grass) and 9006-001 (テストドラゴン dragon/flying) take it at x1/2, and 9001-000
# (テストバード fire/flying) only reaches x0 through ability-9002 (absorb water), so it is in walledByAbility.
move_range_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/move-range/analyze" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"moveIds":["move-9003"]}' || printf '000')
if [ "$move_range_status" != "200" ]; then
  echo "balance move-range failed: HTTP $move_range_status (are the example read models mounted?)" >&2
  cat "$body_file" >&2
  exit 1
fi
for key in '"attackTypes":["water"]' '"typeChart"' '"walledBy"' '"walledByAbility"' '"nameJa":"テストリーフ"' '"nameJa":"テストドラゴン"' '"abilityId":"ability-9002","bestMultiplier":"0"'; do
  if ! grep -qF "$key" "$body_file"; then
    echo "balance move-range body is missing $key" >&2
    cat "$body_file" >&2
    exit 1
  fi
done

unknown_range_move_status=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/move-range/analyze" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"moveIds":["move-9999"]}')
if [ "$unknown_range_move_status" != "422" ] || ! grep -qF '"code":"unknown_move"' "$body_file"; then
  echo "balance move-range unknown move: HTTP $unknown_range_move_status, want 422 unknown_move" >&2
  cat "$body_file" >&2
  exit 1
fi

# move-9006 and move-9012 are status moves: a move set without any attack move is 400 (ADR-0404 §2).
status_only_range=$(curl -sS -o "$body_file" -w '%{http_code}' \
  -X POST "$base_url/api/balance/v1/move-range/analyze" \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: smoke-device' \
  -H 'X-Session-Id: smoke-session' \
  --data '{"moveIds":["move-9006","move-9012"]}')
if [ "$status_only_range" != "400" ] || ! grep -qF '"code":"invalid_request"' "$body_file"; then
  echo "balance move-range status-only move set: HTTP $status_only_range, want 400 invalid_request" >&2
  cat "$body_file" >&2
  exit 1
fi

echo "balance smoke: health=200 analyze=200 unknown=422 coverage=200 unknown_move=422 ability=200 unknown_ability=422 threats=200 threats_unknown_move=422 recommendations=200 move_range=200 move_range_unknown_move=422 move_range_status_only=400"

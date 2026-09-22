#!/usr/bin/env bash
# read model(pokedex export)で動く balance の疎通確認(ADR-0403)。先頭のポケモンで analyze と recommendations が 200 になること。
set -euo pipefail

base_url=${BALANCE_URL:-http://localhost:8080}
readmodel_dir=${BALANCE_READMODEL_DIR:-data/generated/readmodel}
pokemon_id=$(jq -r '.pokemon[0].pokemonId' "$readmodel_dir/pokemon-types.json")
body="{\"members\":[{\"pokemonId\":\"${pokemon_id}\",\"moveIds\":[]}]}"
headers=(-H 'Content-Type: application/json' -H 'X-Device-Id: smoke-device' -H 'X-Session-Id: smoke-session')

code=000
for _ in $(seq 1 30); do
  code=$(curl -sS -o /dev/null -w '%{http_code}' "$base_url/api/balance/healthz" || printf '000')
  [ "$code" = 200 ] && break
  sleep 1
done
[ "$code" = 200 ] || { echo "balance health failed: HTTP $code" >&2; exit 1; }

post() {
  local path=$1 data=$2 status
  for _ in $(seq 1 30); do
    status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$base_url/api/balance/v1/team-balance/$path" "${headers[@]}" --data "$data" || printf '000')
    case "$status" in 000|502|503) sleep 1 ;; *) break ;; esac
  done
  echo "$status"
}
analyze=$(post analyze "{\"members\":[{\"pokemonId\":\"${pokemon_id}\"}]}")
recommend=$(post recommendations "$body")
[ "$analyze" = 200 ] && [ "$recommend" = 200 ] || { echo "balance readmodel smoke failed: analyze=$analyze recommendations=$recommend" >&2; exit 1; }
echo "balance readmodel smoke: pokemon=${pokemon_id} analyze=200 recommendations=200"

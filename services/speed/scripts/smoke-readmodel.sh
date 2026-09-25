#!/usr/bin/env bash
# read model(pokedex export)で動く speed の疎通確認(ADR-0603 §4)。
# read model の先頭のポケモンで /api/speed/v1/pokemon(そのポケモンを含む)と /api/speed/v1/table が 200 になること。
set -euo pipefail

base_url=${SPEED_URL:-http://localhost:8080}
readmodel_dir=${SPEED_READMODEL_DIR:-data/generated/readmodel}
pokemon_id=$(jq -r '.pokemon[0].pokemonId' "$readmodel_dir/speed-pokemon.json")
[ -n "$pokemon_id" ] && [ "$pokemon_id" != null ] \
  || { echo "could not read the first pokemonId from $readmodel_dir/speed-pokemon.json" >&2; exit 1; }

body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT

code=000
for _ in $(seq 1 30); do
  code=$(curl -sS -o /dev/null -w '%{http_code}' "$base_url/api/speed/healthz" || printf '000')
  [ "$code" = 200 ] && break
  sleep 1
done
[ "$code" = 200 ] || { echo "speed health failed: HTTP $code" >&2; exit 1; }

# ロールアウト直後は Ingress が終了中の Pod に振ることがある(502/503)ので、そのときだけ待って繰り返す。
get() {
  local path=$1 status
  for _ in $(seq 1 30); do
    status=$(curl -sS -o "$body_file" -w '%{http_code}' "$base_url$path" \
      -H 'X-Device-Id: 11111111-1111-1111-1111-111111111111' -H 'X-Session-Id: 22222222-2222-2222-2222-222222222222' || printf '000')
    case "$status" in 000|502|503) sleep 1 ;; *) break ;; esac
  done
  echo "$status"
}

list=$(get /api/speed/v1/pokemon)
[ "$list" = 200 ] || { echo "speed pokemon list failed: HTTP $list" >&2; cat "$body_file" >&2; exit 1; }
grep -qF "\"pokemonId\":\"${pokemon_id}\"" "$body_file" \
  || { echo "speed pokemon list does not contain ${pokemon_id}" >&2; exit 1; }

table=$(get /api/speed/v1/table)
[ "$table" = 200 ] || { echo "speed table failed: HTTP $table" >&2; cat "$body_file" >&2; exit 1; }

echo "speed readmodel smoke: pokemon=${pokemon_id} list=200 table=200"

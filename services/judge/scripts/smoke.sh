#!/usr/bin/env sh
set -eu

# JD0: judge にはまだ healthz 以外の endpoint が無い(ADR-0700 §5。JD1 で判定 API を足す)。
base_url=${JUDGE_URL:-http://localhost:8080}
body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT

status=000
attempt=0
while [ "$attempt" -lt 30 ]; do
  status=$(curl -sS -o "$body_file" -w '%{http_code}' "$base_url/api/judge/healthz" || printf '000')
  if [ "$status" = "200" ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$status" != "200" ]; then
  echo "judge health failed: HTTP $status" >&2
  cat "$body_file" >&2
  exit 1
fi
if ! grep -qF '"status":"ok"' "$body_file"; then
  echo "judge health body is missing status:ok" >&2
  cat "$body_file" >&2
  exit 1
fi

echo "judge smoke: OK"

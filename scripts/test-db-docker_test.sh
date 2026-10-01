#!/usr/bin/env bash
# scripts/test-db-docker.sh の自動テスト(issue #223)。make test-scripts から流す。
# docker と make は偽物に差し替える(実コンテナは起動しない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SCRIPT="$ROOT/scripts/test-db-docker.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/nodocker"

# 偽の docker: 呼び出しを記録する。FAKE_DOCKER_INFO_RC で docker info の結果を変えられる。
cat > "$work/bin/docker" <<'FAKE'
#!/usr/bin/env bash
echo "$*" >> "${FAKE_LOG:?}"
case "$1" in
  info) exit "${FAKE_DOCKER_INFO_RC:-0}" ;;
  port) echo "127.0.0.1:65000" ;;
  exec)
    [ -n "${FAKE_EXEC_SLEEP:-}" ] && sleep "$FAKE_EXEC_SLEEP"
    exit "${FAKE_EXEC_RC:-0}"
    ;;
esac
exit 0
FAKE
# 偽の make: 渡された DSN の DB 名を記録し、FAKE_MAKE_RC で終わる。
cat > "$work/bin/make" <<'FAKE'
#!/usr/bin/env bash
echo "make $* POKEDEX=${POKEDEX_TEST_DSN##*/} RECORD=${RECORD_TEST_DSN##*/} TEAM=${TEAM_TEST_DSN##*/}" >> "${FAKE_LOG:?}"
exit "${FAKE_MAKE_RC:-0}"
FAKE
# 偽の openssl: 固定のパスワードを返す(記録全体にこの値が出ないことを確かめるため)。
cat > "$work/bin/openssl" <<'FAKE'
#!/usr/bin/env bash
echo FAKESECRET0123456789
FAKE
chmod +x "$work/bin/docker" "$work/bin/make" "$work/bin/openssl"
for tool in bash env git date openssl sed head mktemp cat; do ln -s "$(command -v "$tool")" "$work/nodocker/$tool"; done

# run_case 環境変数... — スクリプトを流し、終了コードを rc に、記録を log に残す。
run_case() {
  : > "$work/log"
  rc=0
  out=$(cd "$ROOT" && env FAKE_LOG="$work/log" MAKE="$work/bin/make" PATH="$work/bin:$PATH" "$@" "$SCRIPT" 2>&1) || rc=$?
  log=$(cat "$work/log")
}

run_case
if [ "$rc" -eq 0 ] && echo "$log" | grep -q "make --no-print-directory test-db POKEDEX=pokedex_test?parseTime=true RECORD=record_test?parseTime=true TEAM=team_test?parseTime=true"; then
  ok "成功時は _test の3 DB を渡して make test-db を流し、0 で終わる"
else
  ng "成功時(終了コード ${rc}): ${out} / ${log}"
fi
if echo "$log" | grep -q "^rm -f pokecalc-testdb-mysql-" && echo "$log" | grep -q "^network rm pokecalc-testdb-"; then
  ok "成功時もコンテナとネットワークを消す"
else
  ng "成功時に後片付けしていない: ${log}"
fi
if echo "$log" | grep -q "SET GLOBAL tidb_skip_isolation_level_check=1"; then
  ok "TiDB に tidb_skip_isolation_level_check を設定する"
else
  ng "TiDB の設定が無い: ${log}"
fi
if echo "$log" | grep -E "^run " | grep -vq "@sha256:"; then
  ng "digest 固定でないイメージを起動している: $(echo "$log" | grep -E '^run ')"
else
  ok "起動するイメージはすべて digest 固定"
fi
if echo "$log" | grep -v "^make " | grep -q "FAKESECRET"; then
  ng "パスワードの値を docker のコマンドライン引数に出している"
else
  ok "パスワードの値を docker のコマンドライン引数に出さない"
fi

run_case FAKE_MAKE_RC=3
if [ "$rc" -ne 0 ] && echo "$log" | grep -q "^rm -f pokecalc-testdb-mysql-"; then
  ok "テストが失敗したら非0で終わり、それでも後片付けする"
else
  ng "テスト失敗時(終了コード ${rc}): ${log}"
fi

run_case TEST_DB_WAIT_SECONDS=2 FAKE_EXEC_RC=1
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "起動しない" && ! echo "$log" | grep -q "^make " && echo "$log" | grep -q "^rm -f pokecalc-testdb-mysql-"; then
  ok "DB が起動しなければ、テストを流さずに失敗し、後片付けする"
else
  ng "起動待ちのタイムアウト(終了コード ${rc}): ${out}"
fi

# TERM で止めたら、待ちを続けずにすぐ終わり、後片付けする。
: > "$work/log"
(cd "$ROOT" && exec env FAKE_LOG="$work/log" MAKE="$work/bin/make" PATH="$work/bin:$PATH" FAKE_EXEC_SLEEP=1 FAKE_EXEC_RC=1 "$SCRIPT" >/dev/null 2>&1) &
pid=$!
sleep 1
kill -TERM "$pid"
start=$SECONDS
wait "$pid"
rc=$?
if [ "$rc" -ne 0 ] && [ $((SECONDS - start)) -le 5 ] && grep -q "^rm -f pokecalc-testdb-mysql-" "$work/log"; then
  ok "TERM で止めると、すぐ非0で終わり後片付けする"
else
  ng "TERM のとき(終了コード ${rc}、$((SECONDS - start)) 秒): $(cat "$work/log")"
fi

run_case FAKE_DOCKER_INFO_RC=1
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "Docker が起動していない" && ! echo "$log" | grep -q "^make "; then
  ok "Docker が起動していなければ、テストを流さずに失敗する"
else
  ng "Docker 停止時(終了コード ${rc}): ${out}"
fi

rc=0
out=$(cd "$ROOT" && PATH="$work/nodocker" "$work/nodocker/bash" "$SCRIPT" 2>&1) || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "docker が無い"; then
  ok "docker が無ければスキップせず失敗する"
else
  ng "docker が無いとき(終了コード ${rc}): ${out}"
fi

if [ "$failures" -ne 0 ]; then
  echo "test-db-docker_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "test-db-docker_test: すべて成功"

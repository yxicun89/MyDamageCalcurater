#!/usr/bin/env bash
# scripts/master-release.sh の自動テスト(issue #108・#403 D21、ADR-0135)。make test-scripts から流す。
# kubectl・make・go は PATH の先頭に置いた偽物に差し替える(実クラスタ・実 DB には触らない)。
# 偽物は呼び出しを $LOG に1行ずつ残し、版は $STATE の下のファイルで持つ(rollout/入れ替えで新版になる)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SCRIPT="$ROOT/scripts/master-release.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/rm" "$work/state"
readonly LOG="$work/log"
readonly STATE="$work/state"
readonly OLD="pokeapi=v1@aaaaaaaa"
readonly NEW="pokeapi=v2@bbbbbbbb"

cat > "$work/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
echo "kubectl $*" >> "$LOG"
args="$*"
case "$args" in
  "config current-context") echo "k3d-pokecalc" ;;
  *"get jobs -o json"*) cat "$FAKE_JOBS_FILE" ;;
  *"wait --for=condition=complete"*) exit "${FAKE_WAIT_RC:-0}" ;;
  *"applications.argoproj.io pokecalc-balance"*) [ "${FAKE_ARGO_BALANCE:-}" = 1 ] && echo application/pokecalc-balance || exit 1 ;;
  *"applications.argoproj.io pokecalc-speed"*) exit 1 ;;
  *"rollout restart deployment/calc"*) cp "$STATE/db" "$STATE/calc" ;;
  *"rollout status deployment/calc"*) exit "${FAKE_ROLLOUT_RC:-0}" ;;
  *"services/speed:http/proxy/healthz"*) [ -s "$STATE/speed" ] && printf '{"status":"ok","dataVersion":"%s"}\n' "$(cat "$STATE/speed")" || exit 1 ;;
  *"get --raw"*) [ -s "$STATE/calc" ] && printf '{"status":"ok","dataVersion":"%s"}\n' "$(cat "$STATE/calc")" || exit 1 ;;
  *"deployment/balance"*) printf '{"metadata":{"annotations":{"pokecalc.example/data-version":"%s"}}}\n' "$(cat "$STATE/balance")" ;;
  *) echo "unexpected kubectl: $args" >&2; exit 99 ;;
esac
FAKE
cat > "$work/bin/make" <<'FAKE'
#!/usr/bin/env bash
echo "make $* [API_SMOKE_STRICT=${API_SMOKE_STRICT:-}]" >> "$LOG"
case "$*" in
  *pokedex-export*) [ "${FAKE_EXPORT_RC:-0}" = 0 ] || exit 1; printf '{"schemaVersion":1,"dataVersion":"%s"}\n' "$(cat "$STATE/db")" > "$READMODEL_DIR/metadata.json" ;;
  *balance-k3d-deploy-readmodel*) cp "$STATE/db" "$STATE/balance" ;;
  *speed-k3d-deploy-readmodel*) cp "$STATE/db" "$STATE/speed" ;;
  *api-smoke*) exit "${FAKE_SMOKE_RC:-0}" ;;
  *-smoke-readmodel*) exit 0 ;;
  *) echo "unexpected make: $*" >&2; exit 99 ;;
esac
FAKE
cat > "$work/bin/go" <<'FAKE'
#!/usr/bin/env bash
echo "go $*" >> "$LOG"
exit "${FAKE_CHECK_RC:-0}"
FAKE
chmod +x "$work/bin/kubectl" "$work/bin/make" "$work/bin/go"

job_json() { # <名前> <status の JSON> <作成時刻>
  printf '{"metadata":{"name":"%s","creationTimestamp":"%s","ownerReferences":[{"kind":"CronJob","name":"pokedex-import"}]},"status":%s}' "$1" "$3" "$2"
}
echo "{\"items\":[$(job_json pokedex-import-old '{"succeeded":1}' 2026-01-01T00:00:00Z),$(job_json pokedex-import-new '{"succeeded":1}' 2026-02-01T00:00:00Z)]}" > "$work/jobs-done.json"
echo "{\"items\":[$(job_json pokedex-import-old '{"succeeded":1}' 2026-01-01T00:00:00Z),$(job_json pokedex-import-new '{"conditions":[{"type":"Failed","status":"True"}]}' 2026-02-01T00:00:00Z)]}" > "$work/jobs-failed.json"
echo "{\"items\":[$(job_json pokedex-import-new '{"active":1}' 2026-02-01T00:00:00Z)]}" > "$work/jobs-running.json"
echo '{"items":[]}' > "$work/jobs-none.json"

# run 期待終了コード 説明 出力に含む語 DBの版 calc balance speed [環境変数...]
run() {
  local want="$1" desc="$2" needle="$3" db="$4" calc="$5" balance="$6" speed="$7"
  shift 7
  printf '%s' "$db" > "$STATE/db"; printf '%s' "$calc" > "$STATE/calc"
  printf '%s' "$balance" > "$STATE/balance"; printf '%s' "$speed" > "$STATE/speed"
  rm -f "$work/rm/metadata.json"; : > "$LOG"
  local rc=0 out
  out=$(cd "$ROOT" && env PATH="$work/bin:$PATH" LOG="$LOG" STATE="$STATE" READMODEL_DIR="$work/rm" \
    POKEDEX_DATABASE_DSN="user:SECRETPW@tcp(127.0.0.1:1)/db" FAKE_JOBS_FILE="$work/jobs-done.json" \
    READYZ_TRIES=2 READYZ_SLEEP=0 "$@" "$SCRIPT" 2>&1) || rc=$?
  if [ "$rc" -ne "$want" ]; then
    ng "${desc}: 終了コード ${rc}(期待 ${want}) 出力: ${out}"
  elif [ -n "$needle" ] && ! echo "$out" | grep -qF -- "$needle"; then
    ng "${desc}: 出力に '${needle}' が無い: ${out}"
  elif echo "$out" | grep -qF SECRETPW; then
    ng "${desc}: DSN が出力に出ている"
  else
    ok "$desc"
  fi
}
logged() { grep -qF -- "$1" "$LOG"; }
line_of() { grep -nF -- "$1" "$LOG" | head -1 | cut -d: -f1; }

# 版が変わっていれば、順序どおりに反映して成功する
run 0 "版が変わっていれば反映し、最後に完了と表示する" "master-release: 完了" "$NEW" "$OLD" "$OLD" "$OLD"
if logged "pokedex-export" && logged "balance-k3d-deploy-readmodel" && logged "speed-k3d-deploy-readmodel" \
  && logged "rollout restart deployment/calc" && logged "api-smoke [API_SMOKE_STRICT=1]" \
  && logged "balance-smoke-readmodel" && logged "speed-smoke-readmodel" && logged "go run ./cmd/checkreadmodel"; then
  ok "export・検証・balance/speed 入れ替え・calc 再起動・smoke(strict)がすべて呼ばれた"
else
  ng "反映の各段が呼ばれていない: $(cat "$LOG")"
fi
a=$(line_of "pokedex-export"); b=$(line_of "balance-k3d-deploy-readmodel"); c=$(line_of "rollout restart deployment/calc"); d=$(line_of "api-smoke")
if [ "$a" -lt "$b" ] && [ "$b" -lt "$c" ] && [ "$c" -lt "$d" ]; then ok "段の順序: export → read model 入れ替え → calc 再起動 → smoke"; else ng "段の順序が違う: $a $b $c $d"; fi
if logged "create job"; then ng "Job を作っている"; else ok "Job を作らない"; fi

# 変化なし
run 0 "版が同じなら変化なしで 0 終了する" "変化なし" "$OLD" "$OLD" "$OLD" "$OLD"
if logged "pokedex-export" && ! logged "k3d-deploy-readmodel" && ! logged "rollout restart" && ! logged "smoke"; then
  ok "変化なしのときは入れ替え・rollout・smoke をしない"
else
  ng "変化なしなのに反映している: $(cat "$LOG")"
fi

# import Job
run 1 "import Job が無ければ import を先に流すよう案内して非0" "先に" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_JOBS_FILE="$work/jobs-none.json"
if logged "pokedex-export"; then ng "Job が無いのに export している"; else ok "Job が無ければ export まで進まない"; fi
run 1 "最新の Job が失敗していれば非0" "失敗している" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_JOBS_FILE="$work/jobs-failed.json"
run 1 "実行中の Job が時間内に完了しなければ非0" "完了しなかった" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_JOBS_FILE="$work/jobs-running.json" FAKE_WAIT_RC=1
run 0 "実行中の Job は完了を待って進む" "master-release: 完了" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_JOBS_FILE="$work/jobs-running.json"
if logged "wait --for=condition=complete job/pokedex-import-new"; then ok "最新の Job だけを待つ"; else ng "最新の Job を待っていない"; fi

# 途中失敗
run 1 "export 失敗で非0" "read model の export に失敗" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_EXPORT_RC=1
run 1 "read model の検証失敗で非0" "read model 検証に失敗" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_CHECK_RC=1
if logged "k3d-deploy-readmodel"; then ng "検証失敗後に入れ替えている"; else ok "検証に失敗したら入れ替えない"; fi
run 1 "calc の rollout 失敗で非0、旧版の calc を表示する" "STALE calc" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_ROLLOUT_RC=1
run 1 "smoke 失敗で非0" "api-smoke に失敗" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_SMOKE_RC=1
run 1 "Argo CD 管理の balance は飛ばし、旧版の balance を表示して非0" "STALE balance" "$NEW" "$OLD" "$OLD" "$OLD" FAKE_ARGO_BALANCE=1
if logged "speed-k3d-deploy-readmodel" && ! logged "balance-k3d-deploy-readmodel"; then ok "Argo CD 管理の balance だけ入れ替えない"; else ng "Argo CD 管理の扱いが違う"; fi

if [ "$failures" -ne 0 ]; then
  echo "master-release_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "master-release_test: すべて成功"

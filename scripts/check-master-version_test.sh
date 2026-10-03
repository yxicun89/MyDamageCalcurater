#!/usr/bin/env bash
# scripts/check-master-version.sh の自動テスト(issue #281・#108)。make test-scripts から流す。
# kubectl は PATH の先頭に置いた偽物に差し替える(実クラスタには触らない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SCRIPT="$ROOT/scripts/check-master-version.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/rm"
printf '{"schemaVersion":1,"dataVersion":"pokeapi=v1@aaaaaaaa"}\n' > "$work/rm/metadata.json"

cat > "$work/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
# 偽の kubectl: context と、calc の /readyz・balance/speed の Deployment 注釈(環境変数で指定)だけに答える。
args="$*"
case "$args" in
  "config current-context") echo "k3d-pokecalc" ;;
  *"get --raw"*) [ -n "${FAKE_CALC:-}" ] && printf '{"status":"ok","dataVersion":"%s"}\n' "$FAKE_CALC" || exit 1 ;;
  *"deployment/balance"*) printf '{"metadata":{"annotations":{"pokecalc.example/data-version":"%s"}}}\n' "${FAKE_BALANCE:-}" ;;
  *"deployment/speed"*) printf '{"metadata":{"annotations":{"pokecalc.example/data-version":"%s"}}}\n' "${FAKE_SPEED:-}" ;;
  *) echo "unexpected kubectl: $args" >&2; exit 99 ;;
esac
FAKE
chmod +x "$work/bin/kubectl"

readonly V="pokeapi=v1@aaaaaaaa"

# run 期待終了コード 説明 出力に含む語 [環境変数...]
run() {
  local want="$1" desc="$2" needle="$3"
  shift 3
  local rc=0 out
  out=$(cd "$ROOT" && env PATH="$work/bin:$PATH" READMODEL_DIR="$work/rm" "$@" "$SCRIPT" 2>&1) || rc=$?
  if [ "$rc" -ne "$want" ]; then
    ng "${desc}: 終了コード ${rc}(期待 ${want}) 出力: ${out}"
  elif [ -n "$needle" ] && ! echo "$out" | grep -qF -- "$needle"; then
    ng "${desc}: 出力に '${needle}' が無い: ${out}"
  else
    ok "$desc"
  fi
}

run 0 "3つとも期待と同じ版なら成功" "版が一致" FAKE_CALC="$V" FAKE_BALANCE="$V" FAKE_SPEED="$V"
run 1 "calc だけ旧版なら失敗し、calc を名指しする" "STALE calc" FAKE_CALC="pokeapi=v0@bbbbbbbb" FAKE_BALANCE="$V" FAKE_SPEED="$V"
run 1 "balance が注釈無し(未配備)なら失敗し、balance を名指しする" "STALE balance" FAKE_CALC="$V" FAKE_BALANCE="" FAKE_SPEED="$V"
run 1 "speed が旧版なら失敗し、speed を名指しする" "STALE speed" FAKE_CALC="$V" FAKE_BALANCE="$V" FAKE_SPEED="old"
run 1 "calc に届かない(dataVersion が取れない)なら失敗する" "STALE calc" FAKE_CALC="" FAKE_BALANCE="$V" FAKE_SPEED="$V"
run 1 "metadata.json が無ければ失敗する" "metadata.json" READMODEL_DIR="$work/none" FAKE_CALC="$V" FAKE_BALANCE="$V" FAKE_SPEED="$V"

if [ "$failures" -ne 0 ]; then
  echo "check-master-version_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "check-master-version_test: すべて成功"

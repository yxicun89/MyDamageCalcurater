#!/usr/bin/env bash
# scripts/require-k3d-context.sh の自動テスト(issue #295)。make test-scripts から流す。
# kubectl は PATH の先頭に置いた偽物に差し替える(context を返すだけ。実クラスタには触らない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly GUARD="$ROOT/scripts/require-k3d-context.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/nokube"
cat > "$work/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
# 偽の kubectl: `config current-context` だけに答える。それ以外が呼ばれたら記録して失敗する。
if [ "$*" = "config current-context" ]; then
  if [ -z "${FAKE_KUBE_CONTEXT:-}" ]; then
    echo 'error: current-context is not set' >&2
    exit 1
  fi
  echo "$FAKE_KUBE_CONTEXT"
  exit 0
fi
echo "$*" >> "${FAKE_KUBE_LOG:?}"
exit 99
FAKE
chmod +x "$work/bin/kubectl"
for tool in bash env; do ln -s "$(command -v "$tool")" "$work/nokube/$tool"; done

# run 期待終了コード 説明 [環境変数...] — ガードを流し、終了コードと出力を確かめる。
run() {
  local want="$1" desc="$2"
  shift 2
  local rc=0 out
  : > "$work/kubectl.log"
  out=$(env PATH="$work/bin:$PATH" FAKE_KUBE_LOG="$work/kubectl.log" "$@" "$GUARD" test-caller 2>&1) || rc=$?
  if [ "$rc" -ne "$want" ]; then
    ng "${desc}: 終了コード ${rc}(期待 ${want}) 出力: ${out}"
  elif [ -s "$work/kubectl.log" ]; then
    ng "${desc}: current-context 以外の kubectl を呼んだ: $(cat "$work/kubectl.log")"
  elif [ "$want" -ne 0 ] && ! echo "$out" | grep -q "test-caller"; then
    ng "${desc}: 失敗の理由に呼び出し元の名前が無い: ${out}"
  else
    ok "$desc"
  fi
}

run 0 "既定(CLUSTER 無し)で context が k3d-pokecalc なら通す" env -u CLUSTER FAKE_KUBE_CONTEXT=k3d-pokecalc
run 1 "context が別クラスタなら止める" env -u CLUSTER FAKE_KUBE_CONTEXT=gke_prod
run 1 "context が未設定なら止める" env -u CLUSTER FAKE_KUBE_CONTEXT=
run 0 "CLUSTER=foo なら k3d-foo を要求する(一致)" env CLUSTER=foo FAKE_KUBE_CONTEXT=k3d-foo
run 1 "CLUSTER=foo なら k3d-pokecalc は通さない" env CLUSTER=foo FAKE_KUBE_CONTEXT=k3d-pokecalc
run 1 "接頭辞だけ一致する context(k3d-pokecalc2)は通さない" env -u CLUSTER FAKE_KUBE_CONTEXT=k3d-pokecalc2

rc=0
out=$(PATH="$work/nokube" "$work/nokube/bash" "$GUARD" test-caller 2>&1) || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "kubectl が無い"; then
  ok "kubectl が無ければ理由を出して止める"
else
  ng "kubectl が無いとき(終了コード ${rc}): ${out}"
fi

if [ "$failures" -ne 0 ]; then
  echo "require-k3d-context_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "require-k3d-context_test: すべて成功"

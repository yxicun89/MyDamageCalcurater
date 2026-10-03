#!/usr/bin/env bash
# scripts/k3d-deploy-tagged.sh の自動テスト(issue #291)。make test-scripts から流す。
# 使い捨ての git リポジトリにスクリプトをコピーし、docker・k3d・kubectl は偽物に差し替える(実クラスタには触らない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
repo="$work/repo"
mkdir -p "$repo/scripts" "$repo/svc" "$work/bin"
cp "$ROOT/scripts/k3d-deploy-tagged.sh" "$ROOT/scripts/image-tag.sh" "$ROOT/scripts/require-k3d-context.sh" "$repo/scripts/"
echo a > "$repo/svc/a.txt"
git -C "$repo" init -q
git -C "$repo" add -A
git -C "$repo" -c user.name=test -c user.email=deploy-tagged-test commit -qm init
head=$(git -C "$repo" rev-parse --short=12 HEAD)

log="$work/calls.log"
cat > "$work/bin/docker" <<'FAKE'
#!/usr/bin/env bash
echo "docker $*" >> "${FAKE_LOG:?}"
FAKE
cat > "$work/bin/k3d" <<'FAKE'
#!/usr/bin/env bash
echo "k3d $*" >> "${FAKE_LOG:?}"
FAKE
cat > "$work/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
case "$*" in
  "config current-context") echo "${FAKE_KUBE_CONTEXT:-k3d-pokecalc}" ;;
  "kustomize "*) echo "image: pokecalc/gateway:local"; echo "image: pokecalc/calc:local" ;;
  "apply -f -") echo "kubectl apply:" >> "${FAKE_LOG:?}"; cat >> "${FAKE_LOG:?}" ;;
  *) echo "kubectl $*" >> "${FAKE_LOG:?}" ;;
esac
FAKE
chmod +x "$work/bin/"*

# run 説明 期待終了コード [環境変数...] — デプロイを流す(引数は caller overlay 画像...)。
run() {
  local desc="$1" want="$2"
  shift 2
  : > "$log"
  local rc=0
  (cd "$repo" && env PATH="$work/bin:$PATH" FAKE_LOG="$log" CLUSTER=pokecalc TAG_PATHS=svc "$@" \
    ./scripts/k3d-deploy-tagged.sh test-deploy deploy/overlay pokecalc/gateway pokecalc/calc) >"$work/out" 2>&1 || rc=$?
  if [ "$rc" -eq "$want" ]; then ok "$desc(終了コード ${want})"; else ng "${desc}: 終了コード ${rc}(期待 ${want}): $(cat "$work/out")"; fi
}

has() { if grep -qF -- "$1" "$log"; then ok "$2"; else ng "$2(ログに '$1' が無い): $(cat "$log")"; fi; }
hasnot() { if grep -qF -- "$1" "$log"; then ng "$2(ログに '$1' がある): $(cat "$log")"; else ok "$2"; fi; }

run "クリーンな作業ツリー" 0
has "docker tag pokecalc/gateway:local pokecalc/gateway:${head}" "gateway の local をコミットのタグへ付け替える"
has "docker tag pokecalc/calc:local pokecalc/calc:${head}" "calc の local をコミットのタグへ付け替える"
has "k3d image import pokecalc/gateway:${head} pokecalc/calc:${head} --cluster pokecalc" "コミットのタグのイメージを k3d へ入れる"
has "image: pokecalc/gateway:${head}" "apply するマニフェストの image がコミットのタグ"
hasnot "image: pokecalc/gateway:local" "apply するマニフェストに :local を残さない"
has "kubectl -n pokecalc rollout status deployment/gateway" "gateway の rollout を待つ"
hasnot "rollout restart" "クリーンなら rollout restart しない(履歴を汚さず undo を効かせる)"
first=$(cat "$log")
run "同じコミットでもう一度(冪等)" 0
if [ "$(cat "$log")" = "$first" ]; then ok "2回目の呼び出しも同じ操作列(冪等)"; else ng "2回目の操作列が違う"; fi

echo x >> "$repo/svc/a.txt"
run "未コミットの変更あり" 0
has "pokecalc/gateway:${head}-dirty" "未コミットの変更があれば -dirty のタグ"
has "kubectl -n pokecalc rollout restart deployment/gateway deployment/calc" "-dirty は同名タグで中身が変わるので rollout restart する"
git -C "$repo" checkout -q -- svc/a.txt

run "context が別クラスタ" 1 FAKE_KUBE_CONTEXT=gke_prod
hasnot "docker tag" "別クラスタなら docker に触らない"
hasnot "kubectl apply" "別クラスタなら apply しない"

if [ "$failures" -ne 0 ]; then
  echo "k3d-deploy-tagged_test: ${failures} 件失敗" >&2
  exit 1
fi

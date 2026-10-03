#!/usr/bin/env bash
# scripts/k3d-m2-deploy.sh の自動テスト(ADR-0226)。make test-scripts から流す。
# 使い捨ての git リポジトリへスクリプトをコピーし、kubectl・docker・k3d・TiDB Operator の導入・タグ付きデプロイは
# 偽物に差し替える(実クラスタには触らない)。確かめること:
#   - 順序(TidbInitializer の完了待ち → migrate Job の適用 → record・team のデプロイ。ADR-0211 AC-T8)
#   - 非致命(ある段が失敗しても NATS は入る。後続の依存する段はスキップし、最後に非ゼロで終わる)
#   - Secret は kubectl create で作り(apply しない)、値を出力に出さない
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
repo="$work/repo"
state="$work/state"
mkdir -p "$repo/scripts" "$repo/deploy/k8s/base" "$work/bin" "$state"
cp "$ROOT/scripts/k3d-m2-deploy.sh" "$ROOT/scripts/require-k3d-context.sh" "$repo/scripts/"
echo "kind: Namespace" > "$repo/deploy/k8s/base/namespace.yaml"
# TiDB Operator の導入・タグ付きデプロイは別スクリプトなので、呼ばれたことだけ記録する偽物に差し替える。
cat > "$repo/scripts/tidb-operator-bootstrap.sh" <<'FAKE'
#!/usr/bin/env bash
echo "bootstrap" >> "${FAKE_LOG:?}"
[ "${FAKE_FAIL:-}" != "bootstrap" ]
FAKE
cat > "$repo/scripts/k3d-deploy-tagged.sh" <<'FAKE'
#!/usr/bin/env bash
echo "deploy-tagged $*" >> "${FAKE_LOG:?}"
[ "${FAKE_FAIL:-}" != "tagged" ]
FAKE
chmod +x "$repo/scripts/"*.sh
git -C "$repo" init -q
git -C "$repo" add -A
git -C "$repo" -c user.name=test -c user.email=m2-deploy-test commit -qm init

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
args="$*"
case "$args" in
  "config current-context") echo "${FAKE_KUBE_CONTEXT:-k3d-pokecalc}"; exit 0 ;;
  "-n pokecalc get secret tidb-root-auth -o jsonpath={.data.root}") printf 'rootpw-SENTINEL' | base64; exit 0 ;;
  "-n pokecalc get secret "*"-o jsonpath="*) exit 0 ;; # 既存 Secret にキーが無い(空)
  "-n pokecalc get secret "*)
    name="${args#-n pokecalc get secret }"; name="${name%% *}"
    [ -e "${FAKE_STATE:?}/secret-$name" ]; exit $? ;;
  "-n pokecalc get statefulset/"*) exit 0 ;;
  "-n pokecalc rollout status statefulset/pokecalc-tidb-tikv "*) echo "kubectl $args" >> "${FAKE_LOG:?}"; [ "${FAKE_FAIL:-}" != "tikv" ]; exit $? ;;
  "-n pokecalc wait --for=jsonpath={.status.phase}=Completed tidbinitializer/pokecalc "*) echo "kubectl $args" >> "${FAKE_LOG:?}"; [ "${FAKE_FAIL:-}" != "init" ]; exit $? ;;
  "create -f "*)
    f="${args#create -f }"
    n=$(ls "${FAKE_STATE:?}" | wc -l | tr -d ' ')
    cp "$f" "${FAKE_STATE:?}/manifest-$n"
    name=$(awk '/^  name:/ {print $2; exit}' "$f")
    : > "${FAKE_STATE:?}/secret-$name"
    echo "kubectl create -f <manifest:$name>" >> "${FAKE_LOG:?}"
    echo "secret/$name created"
    exit 0 ;;
esac
echo "kubectl $args" >> "${FAKE_LOG:?}"
FAKE
chmod +x "$work/bin/"*

log="$work/calls.log"
# run 説明 期待終了コード [環境変数...] — M2 のデプロイを流す。
run() {
  local desc="$1" want="$2"
  shift 2
  : > "$log"
  rm -f "$state"/*
  local rc=0
  (cd "$repo" && env PATH="$work/bin:$PATH" FAKE_LOG="$log" FAKE_STATE="$state" CLUSTER=pokecalc M2_POLL_SECONDS=0 "$@" \
    ./scripts/k3d-m2-deploy.sh) >"$work/out" 2>&1 || rc=$?
  if [ "$rc" -eq "$want" ]; then ok "$desc(終了コード ${want})"; else ng "${desc}: 終了コード ${rc}(期待 ${want}): $(cat "$work/out")"; fi
}

has() { if grep -qF -- "$1" "$log"; then ok "$2"; else ng "$2(ログに '$1' が無い): $(cat "$log")"; fi; }
hasnot() { if grep -qF -- "$1" "$log"; then ng "$2(ログに '$1' がある): $(cat "$log")"; else ok "$2"; fi; }
outhas() { if grep -qF -- "$1" "$work/out"; then ok "$2"; else ng "$2(出力に '$1' が無い): $(cat "$work/out")"; fi; }
# lineno <文字列>: ログでその文字列が最初に現れる行番号(無ければ 0)。
lineno() { grep -nF -- "$1" "$log" | head -n 1 | cut -d: -f1; }

# --- 全部成功 ---
run "すべて成功" 0
has "bootstrap" "TiDB Operator を導入する"
has "kubectl apply -k deploy/k8s/overlays/local/tidb" "TidbCluster・TidbInitializer を適用する"
has "kubectl create -f <manifest:tidb-root-auth>" "tidb-root-auth を create で作る"
has "kubectl create -f <manifest:record-db-auth>" "record-db-auth を create で作る"
has "kubectl create -f <manifest:team-db-auth>" "team-db-auth を create で作る"
hasnot "apply -f <manifest" "Secret を kubectl apply で作らない(平文が注釈に残る。issue #327)"
has "deploy-tagged k3d-m2-deploy deploy/k8s/overlays/local-m2 pokecalc/record pokecalc/team" "record・team を local-m2 overlay でタグ付きデプロイする"
has "docker build -q -f services/record/Dockerfile --target server -t pokecalc/record:local ." "record の server イメージを local タグで build する"
has "docker build -q -f services/team/Dockerfile --target migrate -t pokecalc/team-migrate:0.1.0 ." "team の migrate イメージを build する"
has "kubectl -n pokecalc delete job record-migrate team-migrate --ignore-not-found" "古い migrate Job を消す"
has "kubectl -n pokecalc rollout status statefulset/nats" "NATS の Ready を待つ"
init_line=$(lineno "tidbinitializer/pokecalc")
job_line=$(lineno "apply -f deploy/k8s/base/record/job-migrate.yaml")
tagged_line=$(lineno "deploy-tagged")
if [ "$init_line" -gt 0 ] && [ "$init_line" -lt "$job_line" ] && [ "$job_line" -lt "$tagged_line" ]; then
  ok "順序: TidbInitializer の完了待ち → migrate Job の適用 → record・team のデプロイ(ADR-0211 AC-T8)"
else
  ng "順序が違う(init=${init_line} job=${job_line} tagged=${tagged_line}): $(cat "$log")"
fi
# Secret の値(乱数・root のパスワード)が出力に出ていないこと。
leak=0
for m in "$state"/manifest-*; do
  while IFS= read -r secret_value; do
    [ -n "$secret_value" ] && grep -qF -- "$secret_value" "$work/out" "$log" && leak=1
  done < <(sed -nE 's/^  [a-z-]+: "(.*)"$/\1/p' "$m" | sed -E 's/^[a-z_]+:([^@]*)@.*$/\1/')
done
grep -qF "rootpw-SENTINEL" "$work/out" "$log" && leak=1
if [ "$leak" = 0 ]; then ok "Secret の値を出力・ログに出さない"; else ng "Secret の値が出力かログに出ている"; fi

# --- 既存の tidb-root-auth は作り直さない ---
: > "$log"; rm -f "$state"/*
: > "$state/secret-tidb-root-auth"
rc=0
(cd "$repo" && env PATH="$work/bin:$PATH" FAKE_LOG="$log" FAKE_STATE="$state" M2_POLL_SECONDS=0 ./scripts/k3d-m2-deploy.sh) >"$work/out" 2>&1 || rc=$?
[ "$rc" -eq 0 ] && ok "既存の tidb-root-auth があっても成功する" || ng "既存の tidb-root-auth で失敗: $(cat "$work/out")"
hasnot "<manifest:tidb-root-auth>" "既存の tidb-root-auth を作り直さない"

# --- 失敗の扱い(非致命だが、最後に非ゼロ。後続の依存する段はスキップ) ---
run "TiDB Operator の導入に失敗" 1 FAKE_FAIL=bootstrap
hasnot "overlays/local/tidb" "Operator が無ければ TiDB を適用しない"
hasnot "deploy-tagged" "Operator が無ければ record・team を入れない"
has "kubectl apply -k deploy/k8s/overlays/local/nats -n pokecalc" "TiDB が使えなくても NATS は入れる"
outhas "TiDB Operator の導入に失敗" "失敗の理由を表示する"
outhas "M2 は k3d で動いていない" "最後に失敗をまとめて表示する(黙って成功扱いにしない)"

run "TiKV が Ready にならない" 1 FAKE_FAIL=tikv
hasnot "tidbinitializer/pokecalc" "TiKV が Ready でなければ TidbInitializer を待たない"
hasnot "deploy-tagged" "TiKV が Ready でなければ record・team を入れない"
outhas "tikv が Ready にならなかった" "失敗の理由(tikv)を表示する"

run "TidbInitializer が完了しない" 1 FAKE_FAIL=init
hasnot "job-migrate.yaml" "TidbInitializer が完了しなければ migrate Job を適用しない(ADR-0211 AC-T8)"
hasnot "deploy-tagged" "TidbInitializer が完了しなければ record・team を入れない"
hasnot "<manifest:record-db-auth>" "TidbInitializer が完了しなければ record-db-auth を作らない"
has "kubectl apply -k deploy/k8s/overlays/local/nats -n pokecalc" "TidbInitializer が完了しなくても NATS は入れる"
outhas "TidbInitializer が Completed にならなかった" "失敗の理由(TidbInitializer)を表示する"

run "record・team のデプロイに失敗" 1 FAKE_FAIL=tagged
outhas "record・team のデプロイ(apply・rollout)に失敗した" "デプロイ失敗を表示して非ゼロで終わる"

# --- context ---
run "別クラスタの context" 1 FAKE_KUBE_CONTEXT=k3d-other
hasnot "kubectl apply" "別クラスタには何も適用しない"
hasnot "bootstrap" "別クラスタには Operator を導入しない"

# --- 呼び出し側(静的な検査。deploy-latest は実クラスタ・docker・make を使うので文面で確かめる) ---
latest="$ROOT/scripts/k3d-deploy-latest.sh"
if grep -qE '^CLUSTER="\$CLUSTER" \./scripts/k3d-m2-deploy\.sh \|\| m2_failed=1' "$latest"; then
  ok "deploy-latest が M2 のデプロイを呼び、失敗を記録して続行する"
else
  ng "deploy-latest が k3d-m2-deploy.sh を非致命で呼んでいない"
fi
if grep -qE 'if \[ "\$missing" = 1 \] \|\| \[ "\$m2_failed" = 1 \]' "$latest"; then
  ok "deploy-latest は M2 の失敗を最後に非ゼロで終わる(黙って成功扱いにしない)"
else
  ng "deploy-latest が M2 の失敗で非ゼロにならない"
fi
if grep -qF './scripts/k3d-m2-deploy.sh' "$ROOT/scripts/up.sh"; then
  ok "up.sh も同じスクリプトで M2 を入れる"
else
  ng "up.sh が k3d-m2-deploy.sh を呼んでいない"
fi

if [ "$failures" -ne 0 ]; then
  echo "k3d-m2-deploy_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "k3d-m2-deploy_test: すべて成功"

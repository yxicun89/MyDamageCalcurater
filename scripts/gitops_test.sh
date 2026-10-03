#!/usr/bin/env bash
# balance / speed の GitOps(Argo CD)まわりの自動テスト(ADR-0408。issue #263・#292)。
# `make test-scripts`(make test に含む)から流す。
#
# 対象:
#   - deploy/argocd/appproject.yaml(AppProject pokecalc。ADR-0408 §1)
#   - services/{balance,speed}/deploy/argocd/application.yaml(spec.project。ADR-0408 §1)
#   - scripts/gitops/{argocd-local-app,check-gitops,publish-image,local-registry-push,k3d-deploy-readmodel}.sh
#     (SERVICE= で動く共通スクリプト。ADR-0408 §2)と、services/{balance,speed}/ 側の呼び出し(Makefile・薄いラッパー)
#   - services/balance/deploy/local-registry/(レジストリの永続化。ADR-0408 §3)
#   - docs/runbooks/{balance,speed}.md(GitOps に戻す標準手順。ADR-0408 §4)
#   - argocd-initial-admin-secret の削除案内(ADR-0408 §5)
#
# 方式は scripts/observability-slo_test.sh と同じ: 手書きの ok/ng ヘルパー付きの bash。
# YAML は本物の helm でローカルの最小 chart を描画して JSON にし(chart repo へは行かない)、jq で読む。
# kustomize は `kubectl kustomize` の描画だけを使う(apply しない)。
#
# 実クラスタ・実ネットワークには触らない:
#   - スクリプトを実際に流すケースは、PATH の先頭に偽の kubectl・docker・crane・k3d・curl・go を置く
#     (kubectl は kustomize だけ本物に渡し、apply は内容を記録するだけ。他は呼び出しを記録するだけ)。
#   - 念のため KUBECONFIG を存在しないファイルに、HTTP(S) のプロキシを閉じたポートに向ける。
#   - 状態を書き換えて検査するケースは、一時ディレクトリの使い捨て git リポジトリにリポジトリと同じ配置で
#     必要なファイルだけをコピーして流す(本物の作業ツリーは書き換えない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SERVICES="balance speed"
readonly GITOPS_SCRIPTS="argocd-local-app check-gitops publish-image local-registry-push k3d-deploy-readmodel"
readonly APPPROJECT_REL="deploy/argocd/appproject.yaml"
readonly REGISTRY_DIR_REL="services/balance/deploy/local-registry"
readonly PROJECT_NAME="pokecalc"
readonly PLACEHOLDER_REPO="https://git.example.invalid/pokecalc.git"
readonly IN_CLUSTER="https://kubernetes.default.svc"
readonly DEST_NAMESPACE="pokecalc"
# argocd-local-app.sh に `git remote get-url origin` として返す架空の URL(実在しないアカウント名)。
readonly FAKE_ORIGIN="https://github.com/example-owner/pokecalc.git"
readonly ADMIN_SECRET_CMD="kubectl -n argocd delete secret argocd-initial-admin-secret"

REAL_HELM=$(command -v helm || true)
readonly REAL_HELM
REAL_KUBECTL=$(command -v kubectl || true)
readonly REAL_KUBECTL
REAL_GIT=$(command -v git)
readonly REAL_GIT
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

FAILURES=0
PASSES=0
CURRENT=""

ok() { PASSES=$((PASSES + 1)); }
ng() {
  printf '  NG [%s]: %s\n' "$CURRENT" "$1" >&2
  FAILURES=$((FAILURES + 1))
}
begin() {
  CURRENT=$1
  printf '%s\n' "- $1"
}

# ---------------------------------------------------------------------------
# YAML → JSON

# 作業ディレクトリは呼び出しごとに mktemp で分ける($(...) のサブシェルでは連番のカウンタが親に戻らず、
# 前の呼び出しの分割ファイルを読んでしまうため)。
# yaml_doc_json YAMLファイル — 1文書の YAML を JSON(1行)にする。
yaml_doc_json() {
  local file=$1 chart
  if [ -z "$REAL_HELM" ]; then
    echo "helm が PATH に無い(doctor.sh の前提ツール)" >&2
    return 1
  fi
  chart=$(mktemp -d "$WORK/yq.XXXXXX")
  mkdir -p "$chart/templates" "$WORK/yq-home"
  printf 'apiVersion: v2\nname: q\nversion: 0.0.0\n' >"$chart/Chart.yaml"
  printf '# R {{ toYaml .Values | fromYaml | toJson }}\n' >"$chart/templates/q.yaml"
  HELM_CONFIG_HOME="$WORK/yq-home" HELM_CACHE_HOME="$WORK/yq-home" HELM_DATA_HOME="$WORK/yq-home" \
    "$REAL_HELM" template q "$chart" -f "$file" 2>"$chart/err" >"$chart/out" || {
    cat "$chart/err" >&2
    return 1
  }
  sed -n 's/^# R //p' "$chart/out"
}

# yaml_stream_json YAMLファイル — `---` 区切りの複数文書を JSON 配列(1行)にする。空の文書は捨てる。
yaml_stream_json() {
  local file=$1 dir n last part out
  dir=$(mktemp -d "$WORK/split.XXXXXX")
  # 文書 i を $dir/i.yaml に書き、最後の番号を返す(先頭が `---` のときや空の文書はファイルができない)。
  last=$(awk -v dir="$dir" '
    BEGIN { n = 0; f = dir "/0.yaml" }
    /^---[[:space:]]*$/ { n++; f = dir "/" n ".yaml"; next }
    { print > f }
    END { print n }
  ' "$file")
  out="$dir/all.jsonl"
  : >"$out"
  for n in $(seq 0 "$last"); do
    part="$dir/$n.yaml"
    [ -f "$part" ] || continue
    if grep -Eq '^[^#[:space:]]' "$part"; then
      yaml_doc_json "$part" >>"$out" || return 1
    fi
  done
  jq -cs '.' "$out"
}

# kustomize_json ディレクトリ — `kubectl kustomize` の描画結果を JSON 配列にする(描画に失敗したら非0)。
kustomize_json() {
  local rendered
  rendered=$(mktemp "$WORK/kustomize.XXXXXX")
  "$REAL_KUBECTL" kustomize "$1" >"$rendered" 2>"$rendered.err" || {
    cat "$rendered.err" >&2
    return 1
  }
  yaml_stream_json "$rendered"
}

# gitops_kinds — balance / speed の gitops overlay が実際に描画する種別を「group/kind」で1行ずつ(整列・重複なし)。
# group は apiVersion の `/` の前(core の `v1` は空文字)。AppProject の namespaceResourceWhitelist と突き合わせる。
gitops_kinds() {
  local svc
  for svc in $SERVICES; do
    kustomize_json "$ROOT/services/$svc/deploy/k8s/overlays/gitops" || return 1
  done | jq -r '.[] | ((.apiVersion | if test("/") then split("/")[0] else "" end) + "/" + .kind)' | sort -u
}

# ---------------------------------------------------------------------------
# 使い捨てリポジトリと偽コマンド

# new_sandbox 名前 — リポジトリと同じ配置で、GitOps の検査に必要なファイルだけをコピーした使い捨て git リポジトリ。
new_sandbox() {
  local dir="$WORK/sandbox/$1" svc
  mkdir -p "$dir/scripts" "$dir/deploy"
  "$REAL_GIT" -C "$dir" init -q
  cp "$ROOT/scripts/require-k3d-context.sh" "$dir/scripts/require-k3d-context.sh"
  [ -d "$ROOT/scripts/gitops" ] && cp -R "$ROOT/scripts/gitops" "$dir/scripts/gitops"
  [ -d "$ROOT/deploy/argocd" ] && cp -R "$ROOT/deploy/argocd" "$dir/deploy/argocd"
  for svc in $SERVICES; do
    mkdir -p "$dir/services/$svc"
    cp -R "$ROOT/services/$svc/deploy" "$dir/services/$svc/deploy"
    [ -d "$ROOT/services/$svc/scripts" ] && cp -R "$ROOT/services/$svc/scripts" "$dir/services/$svc/scripts"
  done
  printf '%s' "$dir"
}

# make_fakes — 偽コマンドを $WORK/bin に作る。呼び出しは $WORK/log/calls、kubectl apply の内容は $WORK/log/applied.yaml。
make_fakes() {
  local name
  mkdir -p "$WORK/bin" "$WORK/log"
  for name in docker crane k3d curl go; do
    cat >"$WORK/bin/$name" <<EOF
#!/usr/bin/env bash
printf '%s %s\n' "$name" "\$*" >>"$WORK/log/calls"
exit 0
EOF
    chmod +x "$WORK/bin/$name"
  done
  cat >"$WORK/bin/kubectl" <<EOF
#!/usr/bin/env bash
printf 'kubectl %s\n' "\$*" >>"$WORK/log/calls"
sub=""
# --context X / -n X の値を subcommand と取り違えないよう、既知の subcommand だけを拾う。
for a in "\$@"; do
  case "\$a" in
    kustomize|apply|version|get|create|delete|rollout|annotate|port-forward|patch|replace) sub=\$a; break ;;
  esac
done
case "\$*" in
  "config current-context") printf 'k3d-pokecalc\n'; exit 0 ;;
esac
case "\$sub" in
  kustomize) exec "$REAL_KUBECTL" "\$@" ;;
  version) exit 0 ;;
  get)
    # 既定は何も出さず成功。FAKE_APP_EXISTS があれば Application が在る、FAKE_LIVE_DEPLOY_ANNOTATIONS があれば
    # 生きている Deployment の annotations(JSON)として返す(issue #237 の手動 overlay との取り合いの検査用)。
    case "\$*" in
      *application*) [ -n "\${FAKE_APP_EXISTS:-}" ] && printf 'application.argoproj.io/pokecalc-x\n' ;;
      *deployment*) [ -n "\${FAKE_LIVE_DEPLOY_ANNOTATIONS:-}" ] && printf '%s\n' "\$FAKE_LIVE_DEPLOY_ANNOTATIONS" ;;
    esac
    exit 0 ;;
  apply)
    prev=""
    for a in "\$@"; do
      case "\$prev" in
        -f|--filename)
          printf '%s\n' '---' >>"$WORK/log/applied.yaml"
          if [ "\$a" = "-" ]; then cat >>"$WORK/log/applied.yaml"; else cat "\$a" >>"$WORK/log/applied.yaml"; fi ;;
        -k|--kustomize)
          printf '%s\n' '---' >>"$WORK/log/applied.yaml"
          "$REAL_KUBECTL" kustomize "\$a" >>"$WORK/log/applied.yaml" ;;
      esac
      case "\$a" in
        --filename=*)
          printf '%s\n' '---' >>"$WORK/log/applied.yaml"
          cat "\${a#--filename=}" >>"$WORK/log/applied.yaml" ;;
      esac
      prev=\$a
    done
    exit 0 ;;
  *) exit 0 ;;
esac
EOF
  chmod +x "$WORK/bin/kubectl"
  cat >"$WORK/bin/git" <<EOF
#!/usr/bin/env bash
if [ "\$*" = "remote get-url origin" ]; then
  printf '%s\n' "$FAKE_ORIGIN"
  exit 0
fi
exec "$REAL_GIT" "\$@"
EOF
  chmod +x "$WORK/bin/git"
}

reset_log() {
  rm -rf "$WORK/log"
  mkdir -p "$WORK/log"
  : >"$WORK/log/calls"
  : >"$WORK/log/applied.yaml"
}

# run_isolated 作業ディレクトリ コマンド... — 偽コマンドを先頭にした PATH で流す。stdout+stderr は $WORK/log/out。
run_isolated() {
  local dir=$1
  shift
  (
    cd "$dir" &&
      PATH="$WORK/bin:$PATH" KUBECONFIG="$WORK/no-such-kubeconfig" \
        HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 \
        http_proxy=http://127.0.0.1:9 https_proxy=http://127.0.0.1:9 NO_PROXY="" no_proxy="" \
        "$@"
  ) >"$WORK/log/out" 2>&1
}

# side_effects — 偽コマンドへの呼び出しのうち、クラスタ・レジストリ・ビルドに触るもの(kustomize・version 以外)。
side_effects() {
  grep -E '^(docker|crane|k3d|curl|go) ' "$WORK/log/calls"
  grep -E '^kubectl ' "$WORK/log/calls" | grep -Ev '^kubectl( .*)? (kustomize|version)( |$)'
}

# ---------------------------------------------------------------------------
# ADR-0408 §1: AppProject

test_appproject() {
  begin "AppProject: deploy/argocd/appproject.yaml が pokecalc 用に repo・宛先・種別を限定する(#263)"
  local file="$ROOT/$APPPROJECT_REL" docs doc expected actual
  if [ -f "$file" ]; then ok; else ng "$APPPROJECT_REL が無い"; return; fi
  docs=$(yaml_stream_json "$file") || { ng "$APPPROJECT_REL を YAML として読めない"; return; }
  if [ "$(jq 'length' <<<"$docs")" = 1 ]; then ok; else ng "$APPPROJECT_REL は AppProject 1 文書だけにする"; fi
  doc=$(jq -c '.[0]' <<<"$docs")

  [ "$(jq -r '.apiVersion' <<<"$doc")" = "argoproj.io/v1alpha1" ] && ok || ng "apiVersion が argoproj.io/v1alpha1 でない"
  [ "$(jq -r '.kind' <<<"$doc")" = "AppProject" ] && ok || ng "kind が AppProject でない"
  [ "$(jq -r '.metadata.name' <<<"$doc")" = "$PROJECT_NAME" ] && ok || ng "metadata.name が $PROJECT_NAME でない"
  # Argo CD は自分の namespace(argocd)にある AppProject しか読まない。
  [ "$(jq -r '.metadata.namespace' <<<"$doc")" = "argocd" ] && ok || ng "metadata.namespace が argocd でない"

  # sourceRepos: application.yaml と同じ placeholder だけ(実 URL は Git に書かず適用時に埋める。ADR-0018 §4)。
  actual=$(jq -c '.spec.sourceRepos' <<<"$doc")
  expected=$(jq -cn --arg r "$PLACEHOLDER_REPO" '[$r]')
  [ "$actual" = "$expected" ] && ok || ng "spec.sourceRepos が $expected でない(実際: $actual)"
  if grep -Eq 'github\.com|gitlab\.com|git@' "$file"; then ng "$APPPROJECT_REL に実在のホストの repo URL が書かれている"; else ok; fi

  # destinations: pokecalc namespace × in-cluster の組だけ。
  actual=$(jq -cS '.spec.destinations' <<<"$doc")
  expected=$(jq -cSn --arg ns "$DEST_NAMESPACE" --arg s "$IN_CLUSTER" '[{namespace: $ns, server: $s}]')
  [ "$actual" = "$expected" ] && ok || ng "spec.destinations が $expected でない(実際: $actual)"

  # clusterResourceWhitelist: 明示的な空配列(cluster スコープ資源を同期させない)。
  actual=$(jq -c '.spec.clusterResourceWhitelist' <<<"$doc")
  [ "$actual" = "[]" ] && ok || ng "spec.clusterResourceWhitelist が [] でない(実際: $actual)"

  # namespaceResourceWhitelist: ワイルドカードなし・gitops overlay が実際に描画する種別とちょうど一致。
  if jq -e '[.spec.namespaceResourceWhitelist // [] | .[] | (.group, .kind) | tostring | test("\\*")] | any' <<<"$doc" >/dev/null; then
    ng "spec.namespaceResourceWhitelist にワイルドカード * がある"
  else ok; fi
  if jq -e '(.spec.namespaceResourceWhitelist | type) == "array" and all(.spec.namespaceResourceWhitelist[]; has("group") and has("kind"))' <<<"$doc" >/dev/null; then
    ok
  else
    ng "spec.namespaceResourceWhitelist の各要素は group と kind を両方持つ(core は group: \"\")"
  fi
  expected=$(gitops_kinds) || { ng "gitops overlay を描画できない"; return; }
  actual=$(jq -r '.spec.namespaceResourceWhitelist // [] | .[] | "\(.group)/\(.kind)"' <<<"$doc" | sort -u)
  if [ "$actual" = "$expected" ]; then ok; else
    ng "spec.namespaceResourceWhitelist が gitops overlay の種別と一致しない。期待: $(tr '\n' ' ' <<<"$expected")/ 実際: $(tr '\n' ' ' <<<"$actual")"
  fi
  if [ "$(jq -r '.spec.namespaceResourceWhitelist // [] | length' <<<"$doc")" = "$(printf '%s\n' "$expected" | grep -c .)" ]; then ok; else
    ng "spec.namespaceResourceWhitelist の要素数が gitops overlay の種別の数と合わない(重複か余分な種別)"
  fi

  # kustomize で描画できる(構文の確認。apply はしない)。
  mkdir -p "$WORK/appproject-render"
  cp "$file" "$WORK/appproject-render/appproject.yaml"
  printf 'resources:\n  - appproject.yaml\n' >"$WORK/appproject-render/kustomization.yaml"
  if "$REAL_KUBECTL" kustomize "$WORK/appproject-render" >/dev/null 2>&1; then ok; else ng "$APPPROJECT_REL が kubectl kustomize で描画できない"; fi
}

test_application_project() {
  local svc rel docs doc
  for svc in $SERVICES; do
    begin "Application: $svc の spec.project が $PROJECT_NAME で、AppProject の許可範囲に収まる(#263)"
    rel="services/$svc/deploy/argocd/application.yaml"
    docs=$(yaml_stream_json "$ROOT/$rel") || { ng "$rel を読めない"; continue; }
    doc=$(jq -c '.[0]' <<<"$docs")
    [ "$(jq -r '.spec.project' <<<"$doc")" = "$PROJECT_NAME" ] && ok ||
      ng "$rel の spec.project が $PROJECT_NAME でない(実際: $(jq -r '.spec.project' <<<"$doc"))"
    [ "$(jq -r '.spec.destination.namespace' <<<"$doc")" = "$DEST_NAMESPACE" ] && ok || ng "$rel の destination.namespace が $DEST_NAMESPACE でない"
    [ "$(jq -r '.spec.destination.server' <<<"$doc")" = "$IN_CLUSTER" ] && ok || ng "$rel の destination.server が $IN_CLUSTER でない"
    [ "$(jq -r '.spec.source.repoURL' <<<"$doc")" = "$PLACEHOLDER_REPO" ] && ok || ng "$rel の repoURL が placeholder でない"
    if jq -e '.spec.syncPolicy.automated' <<<"$doc" >/dev/null 2>&1; then ng "$rel で自動 sync が有効になっている(ADR-0408 §4 で却下)"; else ok; fi
  done
}

# ---------------------------------------------------------------------------
# ADR-0408 §2: scripts/gitops/ の共通スクリプト

test_common_scripts_exist() {
  begin "共通スクリプト: scripts/gitops/ に機能名の5本があり、構文が正しい(#263)"
  local name file
  for name in $GITOPS_SCRIPTS; do
    file="$ROOT/scripts/gitops/$name.sh"
    if [ -f "$file" ]; then ok; else ng "scripts/gitops/$name.sh が無い"; continue; fi
    [ -x "$file" ] && ok || ng "scripts/gitops/$name.sh に実行権限が無い"
    if bash -n "$file" 2>/dev/null; then ok; else ng "scripts/gitops/$name.sh が bash -n を通らない"; fi
    if grep -q 'SERVICE' "$file"; then ok; else ng "scripts/gitops/$name.sh が SERVICE を参照していない"; fi
  done
}

test_service_required() {
  begin "共通スクリプト: SERVICE が未設定・空・未知・パス混入なら、外部コマンドを呼ぶ前に非0で終了する(#263)"
  local name label rc effects
  make_fakes
  for name in $GITOPS_SCRIPTS; do
    [ -f "$ROOT/scripts/gitops/$name.sh" ] || { ng "scripts/gitops/$name.sh が無い"; continue; }
    for label in unset empty unknown traversal; do
      reset_log
      case "$label" in
        unset) run_isolated "$ROOT" env -u SERVICE "./scripts/gitops/$name.sh" template ;;
        empty) run_isolated "$ROOT" env SERVICE= "./scripts/gitops/$name.sh" template ;;
        unknown) run_isolated "$ROOT" env SERVICE=calc "./scripts/gitops/$name.sh" template ;;
        traversal) run_isolated "$ROOT" env SERVICE=../balance "./scripts/gitops/$name.sh" template ;;
      esac
      rc=$?
      if [ "$rc" -ne 0 ]; then ok; else ng "$name.sh が SERVICE=$label で成功した(非0で止まるべき)"; fi
      if grep -q 'SERVICE' "$WORK/log/out"; then ok; else ng "$name.sh の SERVICE=$label のエラーに SERVICE の語が無い(原因が分からない)"; fi
      effects=$(side_effects)
      if [ -z "$effects" ]; then ok; else ng "$name.sh が SERVICE=$label のまま外部コマンドを呼んだ: $(tr '\n' ';' <<<"$effects")"; fi
    done
  done
}

# is_thin_wrapper ファイル サービス 機能名 — 共通スクリプトを SERVICE=<サービス> で呼ぶだけの数行か。
is_thin_wrapper() {
  local file=$1 svc=$2 name=$3 body
  body=$(grep -Ev '^[[:space:]]*(#|$)' "$file" | grep -Ev '^[[:space:]]*set[[:space:]]+-')
  [ "$(printf '%s\n' "$body" | grep -c .)" -le 3 ] || return 1
  printf '%s\n' "$body" | grep -q "scripts/gitops/$name.sh" || return 1
  printf '%s\n' "$body" | grep -Eq "SERVICE=\"?$svc\"?" || return 1
}

# make_recipe Makefile ターゲット — ターゲットのレシピ行(タブ始まり)を返す。
make_recipe() {
  awk -v t="$2" '
    index($0, t ":") == 1 { in_t = 1; next }
    in_t && /^\t/ { print; next }
    in_t { exit }
  ' "$1"
}

test_no_duplicated_logic() {
  begin "共通化: services/{balance,speed}/scripts に5本のロジックの複製が残らず、Makefile が SERVICE= で共通スクリプトに届く(#263)"
  local svc name file mk target recipe pair
  for svc in $SERVICES; do
    for name in $GITOPS_SCRIPTS; do
      file="$ROOT/services/$svc/scripts/$name.sh"
      [ -e "$file" ] || { ok; continue; }
      if is_thin_wrapper "$file" "$svc" "$name"; then ok; else
        ng "services/$svc/scripts/$name.sh が残っているが、scripts/gitops/$name.sh を SERVICE=$svc で呼ぶだけの薄いラッパー(3行以下)になっていない"
      fi
    done
    mk="$ROOT/services/$svc/Makefile"
    for pair in gitops-template-check:check-gitops gitops-check:check-gitops docker-push:publish-image \
      registry-push:local-registry-push argocd-app:argocd-local-app k3d-deploy-readmodel:k3d-deploy-readmodel; do
      target="$svc-${pair%%:*}"
      name=${pair#*:}
      recipe=$(make_recipe "$mk" "$target")
      if [ -z "$recipe" ]; then ng "services/$svc/Makefile に $target のレシピが無い(ターゲット名は変えない。ADR-0408 §2)"; continue; fi
      if printf '%s\n' "$recipe" | grep -q "scripts/gitops/$name.sh" && printf '%s\n' "$recipe" | grep -Eq "SERVICE=\"?$svc\"?"; then
        ok
      elif printf '%s\n' "$recipe" | grep -q "scripts/$name.sh" && [ -f "$ROOT/services/$svc/scripts/$name.sh" ] &&
        is_thin_wrapper "$ROOT/services/$svc/scripts/$name.sh" "$svc" "$name"; then
        ok
      else
        ng "$target が SERVICE=$svc で scripts/gitops/$name.sh(または薄いラッパー)を呼んでいない"
      fi
    done
  done
  # 回帰: speed の template 検査が balance の設定を読んでいない(SERVICE の取り違え)ことは下の make で確かめる。
}

# ---------------------------------------------------------------------------
# ADR-0408 §1: check-gitops.sh の project 検査(#263 の境界値)と、共通化で既存の検査が落ちていないこと

# set_project ファイル 行 — application.yaml の `  project:` 行を差し替える(行が空なら削除)。
set_project() {
  local file=$1 line=$2
  awk -v line="$line" '
    /^[[:space:]]+project:/ { if (line != "") print line; next }
    { print }
  ' "$file" >"$file.new" && mv "$file.new" "$file"
}

check_gitops_in() {
  local dir=$1 svc=$2 mode=$3
  reset_log
  run_isolated "$dir" env SERVICE="$svc" ./scripts/gitops/check-gitops.sh "$mode"
}

test_check_gitops_project() {
  local svc dir app rc case_line label
  make_fakes
  for svc in $SERVICES; do
    begin "check-gitops.sh: $svc の Application が project: $PROJECT_NAME 以外なら失敗する(#263 境界値)"
    dir=$(new_sandbox "project-$svc-ok")
    [ -f "$dir/scripts/gitops/check-gitops.sh" ] || { ng "scripts/gitops/check-gitops.sh が無い"; continue; }
    check_gitops_in "$dir" "$svc" template
    rc=$?
    if [ "$rc" -eq 0 ]; then ok; else ng "project: $PROJECT_NAME のままの $svc で template 検査が失敗した: $(tail -1 "$WORK/log/out")"; fi

    for label in default quoted-default missing other; do
      dir=$(new_sandbox "project-$svc-$label")
      app="$dir/services/$svc/deploy/argocd/application.yaml"
      case "$label" in
        default) case_line="  project: default" ;;
        quoted-default) case_line='  project: "default"' ;;
        missing) case_line="" ;;
        other) case_line="  project: pokecalc-other" ;;
      esac
      set_project "$app" "$case_line"
      check_gitops_in "$dir" "$svc" template
      rc=$?
      if [ "$rc" -ne 0 ]; then ok; else ng "$svc の project を $label にしても template 検査が通った"; fi
      if grep -qi 'project' "$WORK/log/out"; then ok; else ng "$svc の project=$label の失敗メッセージに project の語が無い"; fi
    done
  done
}

test_check_gitops_regressions() {
  local dir rc
  make_fakes
  begin "check-gitops.sh: 共通化の前からあった検査(自動 sync・実 URL・newTag・placeholder digest)が残っている"
  [ -f "$ROOT/scripts/gitops/check-gitops.sh" ] || { ng "scripts/gitops/check-gitops.sh が無い"; return; }

  dir=$(new_sandbox regress-automated)
  printf '  syncPolicy:\n    automated: {}\n' >>"$dir/services/balance/deploy/argocd/application.yaml"
  check_gitops_in "$dir" balance template
  rc=$?
  [ "$rc" -ne 0 ] && ok || ng "automated sync を入れても balance の template 検査が通った"

  dir=$(new_sandbox regress-real-url)
  sed -i.bak "s#$PLACEHOLDER_REPO#$FAKE_ORIGIN#" "$dir/services/speed/deploy/argocd/application.yaml"
  check_gitops_in "$dir" speed template
  rc=$?
  [ "$rc" -ne 0 ] && ok || ng "application.yaml の repoURL を実 URL にしても speed の template 検査が通った"

  dir=$(new_sandbox regress-newtag)
  printf '    newTag: v1\n' >>"$dir/services/balance/deploy/k8s/overlays/gitops/kustomization.yaml"
  check_gitops_in "$dir" balance template
  rc=$?
  [ "$rc" -ne 0 ] && ok || ng "gitops overlay に newTag を足しても balance の template 検査が通った"

  dir=$(new_sandbox regress-zero-digest)
  sed -i.bak -E 's/digest: sha256:[0-9a-f]{64}/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' \
    "$dir/services/balance/deploy/k8s/overlays/gitops/kustomization.yaml"
  check_gitops_in "$dir" balance ready
  rc=$?
  [ "$rc" -ne 0 ] && ok || ng "digest が全0の placeholder でも balance の ready 検査が通った"

  # SERVICE の取り違え検出: speed の overlay だけ壊したとき、balance の検査は通り speed の検査は落ちる。
  dir=$(new_sandbox regress-crossed)
  printf '    newTag: v1\n' >>"$dir/services/speed/deploy/k8s/overlays/gitops/kustomization.yaml"
  check_gitops_in "$dir" balance template
  rc=$?
  [ "$rc" -eq 0 ] && ok || ng "speed の overlay だけ壊したのに SERVICE=balance の template 検査が落ちた(サービスの取り違え)"
  check_gitops_in "$dir" speed template
  rc=$?
  [ "$rc" -ne 0 ] && ok || ng "speed の overlay を壊したのに SERVICE=speed の template 検査が通った(サービスの取り違え)"
}

test_make_template_check() {
  begin "回帰: make balance-gitops-template-check speed-gitops-template-check が通る(#263 の検証コマンド)"
  local out
  if out=$(cd "$ROOT" && KUBECONFIG="$WORK/no-such-kubeconfig" make -s balance-gitops-template-check speed-gitops-template-check 2>&1); then
    ok
  else
    ng "make の template 検査が失敗: $(tail -3 <<<"$out" | tr '\n' ' ')"
  fi
}

# ---------------------------------------------------------------------------
# ADR-0408 §1・§2: argocd-local-app.sh が AppProject も Application と一緒に(先に)適用する

test_argocd_local_app_applies_project() {
  begin "argocd-local-app.sh: AppProject を Application より先に、repo URL を埋めて適用する(placeholder を残さない)"
  local dir rc docs project_idx app_idx
  make_fakes
  [ -f "$ROOT/scripts/gitops/argocd-local-app.sh" ] || { ng "scripts/gitops/argocd-local-app.sh が無い"; return; }
  dir=$(new_sandbox local-app)
  # ready 検査は pokedex の digest が placeholder(全0)だと落ちる。リポジトリ本体は placeholder のまま、sandbox の中だけ実 digest 風の値にして通す。
  sed -i.bak -E '/name: pokecalc\/pokedex/,/digest:/ s/digest: .*/digest: sha256:1111111111111111111111111111111111111111111111111111111111111111/' \
    "$dir/services/balance/deploy/k8s/overlays/gitops/kustomization.yaml"
  reset_log
  run_isolated "$dir" env SERVICE=balance ./scripts/gitops/argocd-local-app.sh
  rc=$?
  if [ "$rc" -eq 0 ]; then ok; else ng "SERVICE=balance で失敗した: $(tail -2 "$WORK/log/out" | tr '\n' ' ')"; return; fi

  if grep -q 'example.invalid' "$WORK/log/applied.yaml"; then ng "適用した内容に placeholder(example.invalid)が残っている"; else ok; fi
  docs=$(yaml_stream_json "$WORK/log/applied.yaml") || { ng "適用した内容を YAML として読めない"; return; }
  project_idx=$(jq --arg n "$PROJECT_NAME" '[.[] | .kind == "AppProject" and .metadata.name == $n] | index(true)' <<<"$docs")
  app_idx=$(jq '[.[] | .kind == "Application" and .metadata.name == "pokecalc-balance"] | index(true)' <<<"$docs")
  if [ "$project_idx" != "null" ]; then ok; else ng "AppProject $PROJECT_NAME を適用していない(Application の project が解決できない)"; fi
  if [ "$app_idx" != "null" ]; then ok; else ng "Application pokecalc-balance を適用していない"; fi
  if [ "$project_idx" != "null" ] && [ "$app_idx" != "null" ] && [ "$project_idx" -lt "$app_idx" ]; then ok; else
    ng "AppProject を Application より先に適用していない"
  fi
  if jq -e --arg u "$FAKE_ORIGIN" '[.[] | select(.kind == "AppProject")][0].spec.sourceRepos == [$u]' <<<"$docs" >/dev/null 2>&1; then ok; else
    ng "適用した AppProject の sourceRepos が origin の URL だけになっていない(Application の repo が許可されない)"
  fi
  if jq -e --arg u "$FAKE_ORIGIN" --arg p "$PROJECT_NAME" \
    '[.[] | select(.kind == "Application")][0] | .spec.source.repoURL == $u and .spec.project == $p' <<<"$docs" >/dev/null 2>&1; then
    ok
  else
    ng "適用した Application の repoURL / project が期待どおりでない"
  fi
  if jq -e 'all(.[]; .metadata.namespace == "argocd")' <<<"$docs" >/dev/null 2>&1; then ok; else
    ng "argocd namespace 以外に何かを適用している"
  fi
}

# ---------------------------------------------------------------------------
# ADR-0408 §3: レジストリの永続化(#292)

test_registry_pvc() {
  begin "レジストリ: emptyDir をやめ、local-path の 2Gi PVC に /var/lib/registry を載せる(#292)"
  local docs workload mount_vol claim pvc
  docs=$(kustomize_json "$ROOT/$REGISTRY_DIR_REL") || { ng "kubectl kustomize $REGISTRY_DIR_REL が描画できない"; return; }
  ok
  if jq -e '[.. | objects | has("emptyDir")] | any' <<<"$docs" >/dev/null; then ng "emptyDir が残っている(Pod を作り直すと push 済みイメージが消える)"; else ok; fi

  workload=$(jq -c '[.[] | select((.kind == "Deployment" or .kind == "StatefulSet") and .metadata.name == "registry")][0]' <<<"$docs")
  if [ "$workload" != "null" ]; then ok; else ng "registry の Deployment / StatefulSet が無い"; return; fi
  mount_vol=$(jq -r '.spec.template.spec.containers[] | select(.name == "registry") | .volumeMounts[]? | select(.mountPath == "/var/lib/registry") | .name' <<<"$workload")
  if [ -n "$mount_vol" ]; then ok; else ng "/var/lib/registry のマウントが無い"; return; fi

  if [ "$(jq -r '.kind' <<<"$workload")" = "StatefulSet" ]; then
    pvc=$(jq -c --arg v "$mount_vol" '[.spec.volumeClaimTemplates[]? | select(.metadata.name == $v)][0]' <<<"$workload")
    if [ "$pvc" != "null" ]; then ok; else ng "StatefulSet の volumeClaimTemplates に $mount_vol が無い"; return; fi
  else
    claim=$(jq -r --arg v "$mount_vol" '.spec.template.spec.volumes[]? | select(.name == $v) | .persistentVolumeClaim.claimName // empty' <<<"$workload")
    if [ -n "$claim" ]; then ok; else ng "/var/lib/registry のボリューム $mount_vol が persistentVolumeClaim でない"; return; fi
    pvc=$(jq -c --arg c "$claim" '[.[] | select(.kind == "PersistentVolumeClaim" and .metadata.name == $c)][0]' <<<"$docs")
    if [ "$pvc" != "null" ]; then ok; else ng "claimName $claim の PersistentVolumeClaim が描画結果に無い"; return; fi
    [ "$(jq -r '.metadata.namespace' <<<"$pvc")" = "balance-registry" ] && ok || ng "PVC の namespace が balance-registry でない"
    # hostPort 5000 と RWO ボリュームは同時に1つの Pod しか持てないので Recreate を保つ。
    [ "$(jq -r '.spec.strategy.type' <<<"$workload")" = "Recreate" ] && ok || ng "Deployment の strategy が Recreate でない"
  fi
  [ "$(jq -r '.spec.resources.requests.storage' <<<"$pvc")" = "2Gi" ] && ok || ng "PVC の容量が 2Gi でない(実際: $(jq -r '.spec.resources.requests.storage' <<<"$pvc"))"
  [ "$(jq -r '.spec.storageClassName' <<<"$pvc")" = "local-path" ] && ok || ng "PVC の storageClassName が local-path でない"
  if jq -e '.spec.accessModes | index("ReadWriteOnce")' <<<"$pvc" >/dev/null 2>&1; then ok; else ng "PVC の accessModes に ReadWriteOnce が無い"; fi
}

# ---------------------------------------------------------------------------
# ADR-0408 §4・§5: runbook

# line_of ファイル 固定文字列 first|last — 一致する行番号(無ければ 0)。
line_of() {
  local n
  if [ "$3" = first ]; then
    n=$(grep -nF -- "$2" "$1" | head -1 | cut -d: -f1)
  else
    n=$(grep -nF -- "$2" "$1" | tail -1 | cut -d: -f1)
  fi
  printf '%s' "${n:-0}"
}

test_runbooks_return_to_gitops() {
  local svc rel file push sync last_sync last_out tail_text
  for svc in $SERVICES; do
    begin "runbook: $svc の標準手順(registry-push → digest を PR → main マージ後 sync)と、local 上書き後に GitOps へ戻す手順(#292)"
    rel="docs/runbooks/$svc.md"
    file="$ROOT/$rel"
    push=$(line_of "$file" "make -s $svc-registry-push" first)
    [ "$push" = 0 ] && push=$(line_of "$file" "make $svc-registry-push" first)
    sync=$(line_of "$file" "app sync pokecalc-$svc" first)
    last_sync=$(line_of "$file" "app sync pokecalc-$svc" last)
    last_out=$(line_of "$file" "OutOfSync" last)
    [ "$push" != 0 ] && ok || ng "$rel に make $svc-registry-push が無い"
    if grep -qF "services/$svc/deploy/k8s/overlays/gitops/kustomization.yaml" "$file"; then ok; else ng "$rel に digest を書く先(gitops overlay の kustomization.yaml)が無い"; fi
    [ "$sync" != 0 ] && ok || ng "$rel に app sync pokecalc-$svc が無い"
    if [ "$push" != 0 ] && [ "$sync" != 0 ] && [ "$push" -lt "$sync" ]; then ok; else ng "$rel で registry-push が sync より前に書かれていない"; fi
    if sed -n "${push:-1},${sync:-1}p" "$file" | grep -q 'PR' && sed -n "${push:-1},${sync:-1}p" "$file" | grep -q 'main'; then ok; else
      ng "$rel の registry-push と sync の間に「PR で main に入れる」旨が無い"
    fi
    # local overlay で上書きすると OutOfSync になる、と書いた後に、sync で戻す手順と Synced の確認がある。
    if [ "$last_out" != 0 ]; then ok; else ng "$rel に local 上書きで OutOfSync になる旨が無い"; fi
    if [ "$last_out" != 0 ] && [ "$last_sync" -gt "$last_out" ]; then ok; else
      ng "$rel で OutOfSync になる旨の後に、app sync pokecalc-$svc で GitOps の状態に戻す手順が無い(runbook の終状態が OutOfSync のまま)"
    fi
    tail_text=$(sed -n "${last_out:-1},\$p" "$file")
    if printf '%s\n' "$tail_text" | grep -q 'Synced'; then ok; else ng "$rel の戻す手順に Synced になったことの確認が無い"; fi
    if grep -Eq 'syncPolicy|automated' "$file" && ! grep -Eq '(自動|automated)[^。]*(しない|使わない|無効)' "$file"; then
      ng "$rel が自動 sync を案内している(ADR-0408 §4 で却下)"
    else ok; fi
  done
}

test_initial_admin_secret() {
  begin "argocd-initial-admin-secret: 初回ログイン後に削除する案内が bootstrap の完了メッセージと runbook にある(実行はしない。#263)"
  local script="$ROOT/scripts/argocd-bootstrap.sh" runbook="$ROOT/docs/runbooks/balance.md"
  if grep -qF "$ADMIN_SECRET_CMD" "$runbook"; then ok; else ng "docs/runbooks/balance.md に \`$ADMIN_SECRET_CMD\` の案内が無い"; fi
  if grep -qF "$ADMIN_SECRET_CMD" "$script"; then ok; else ng "scripts/argocd-bootstrap.sh の完了メッセージに \`$ADMIN_SECRET_CMD\` の案内が無い"; fi
  # 削除は人が行う(認証情報の操作。ADR-0408 §5)。スクリプト自身が実行する行があってはならない。
  if grep -F 'argocd-initial-admin-secret' "$script" | grep -Ev '^[[:space:]]*(#|echo|printf)' | grep -Eq 'delete'; then
    ng "scripts/argocd-bootstrap.sh が argocd-initial-admin-secret を自分で削除している(案内だけにする)"
  else ok; fi
  # 案内は適用が終わった後(rollout status の後)に出す。
  local rollout msg
  rollout=$(line_of "$script" "rollout status deployment/argocd-server" last)
  msg=$(line_of "$script" "$ADMIN_SECRET_CMD" last)
  if [ "$msg" != 0 ] && [ "$rollout" != 0 ] && [ "$msg" -gt "$rollout" ]; then ok; else
    ng "削除の案内が argocd-server の rollout status より後(完了時)に出ていない"
  fi
}

# ---------------------------------------------------------------------------
# issue #237・ADR-0412: gitops overlay が read model を起動時に自分で用意する
# (initContainer で pokedex export → emptyDir → 本体は読むだけ。Git に実データも ConfigMap も置かない。ADR-0002)

# サービスごとの read model(環境変数名=ファイル名)。pokedex export のファイル名は ADR-0105 §5 で固定。
readmodel_env_files() {
  case "$1" in
    balance) printf '%s\n' BALANCE_POKEMON_TYPES_PATH=pokemon-types.json BALANCE_MOVES_PATH=moves.json BALANCE_ABILITIES_PATH=abilities.json ;;
    speed) printf '%s\n' SPEED_POKEMON_PATH=speed-pokemon.json ;;
  esac
}

# overlay_deploy_json サービス — gitops overlay の描画から Deployment <svc> だけを JSON 配列で取り出す。
overlay_deploy_json() {
  kustomize_json "$ROOT/services/$1/deploy/k8s/overlays/gitops" | jq -c --arg n "$1" '[.[] | select(.kind == "Deployment" and .metadata.name == $n)]'
}

test_gitops_overlay_readmodel() {
  local svc deploy rendered init main pair var file mount mmount
  for svc in $SERVICES; do
    begin "gitops overlay($svc): initContainer(pokedex export)→emptyDir→本体が読む。ConfigMap・Secret・実データは Git に無い(#237)"
    rendered=$(kustomize_json "$ROOT/services/$svc/deploy/k8s/overlays/gitops") || { ng "描画に失敗した"; continue; }
    deploy=$(overlay_deploy_json "$svc")
    if [ "$(jq 'length' <<<"$deploy")" = 1 ]; then ok; else ng "Deployment $svc が1つでない"; continue; fi
    if [ "$(jq '[.[] | select(.kind == "ConfigMap" or .kind == "Secret")] | length' <<<"$rendered")" = 0 ]; then ok; else ng "gitops overlay が ConfigMap / Secret を描画している(read model は ConfigMap にしない。ADR-0002・1 MiB 上限)"; fi

    init=$(jq -c '.[0].spec.template.spec.initContainers // [] | map(select(.name == "readmodel-export")) | .[0] // empty' <<<"$deploy")
    if [ -n "$init" ]; then ok; else ng "initContainer readmodel-export が無い"; continue; fi
    # image は pokedex(server イメージ)を digest 固定で。タグ・latest は不可(ADR-0405)。
    if jq -e '.image | test("^[A-Za-z0-9._:/-]+/pokecalc/pokedex@sha256:[0-9a-f]{64}$")' <<<"$init" >/dev/null; then ok; else ng "initContainer の image が <registry>/pokecalc/pokedex@sha256:<digest> でない: $(jq -r .image <<<"$init")"; fi
    # 出力先は emptyDir のマウント先。pokedex は ENTRYPOINT=/pokedex なので args だけで export を呼ぶ。
    mount=$(jq -r '.volumeMounts // [] | map(select(.name == "readmodel")) | .[0].mountPath // empty' <<<"$init")
    if [ -n "$mount" ] && jq -e --arg m "$mount" '.args == ["export", "-out", $m]' <<<"$init" >/dev/null; then ok; else ng "initContainer の args が [export, -out, <readmodel のマウント先>] でない"; fi
    if jq -e '(.volumeMounts // []) | map(select(.name == "readmodel" and .readOnly != true)) | length == 1' <<<"$init" >/dev/null; then ok; else ng "initContainer の readmodel マウントが書き込み可でない"; fi
    # DSN は reader ロール(SELECT のみ)を Secret から。root の pokedex-dsn は使わない(ADR-0110)。
    if jq -e '(.env // []) | map(select(.name == "POKEDEX_DATABASE_DSN" and .valueFrom.secretKeyRef.name == "mysql-auth" and .valueFrom.secretKeyRef.key == "pokedex-reader-dsn")) | length == 1' <<<"$init" >/dev/null; then ok; else ng "POKEDEX_DATABASE_DSN が Secret mysql-auth の pokedex-reader-dsn 由来でない"; fi
    if jq -e '[(.env // [])[] | select(.value != null and (.name | test("DSN")))] | length == 0' <<<"$init" >/dev/null; then ok; else ng "DSN を平文の value で渡している"; fi
    # 本体と同じ水準のハードニング(readOnlyRootFilesystem でも emptyDir には書ける)。
    if jq -e '.securityContext | .allowPrivilegeEscalation == false and .readOnlyRootFilesystem == true and .runAsNonRoot == true and (.capabilities.drop | index("ALL") != null)' <<<"$init" >/dev/null; then ok; else ng "initContainer の securityContext が不十分(allowPrivilegeEscalation=false・readOnlyRootFilesystem・runAsNonRoot・drop ALL)"; fi
    if jq -e '.resources.limits.memory != null and .resources.limits.cpu != null' <<<"$init" >/dev/null; then ok; else ng "initContainer に resources.limits が無い"; fi

    # volume は emptyDir(ConfigMap ではない)。
    if jq -e '.[0].spec.template.spec.volumes // [] | map(select(.name == "readmodel" and .emptyDir != null and .configMap == null)) | length == 1' <<<"$deploy" >/dev/null; then ok; else ng "volume readmodel が emptyDir でない"; fi
    # 本体: 読み取り専用でマウントし、各 *_PATH がそのマウント先のファイルを指す。DSN は渡さない(DB に届かない)。
    main=$(jq -c --arg n "$svc" '.[0].spec.template.spec.containers | map(select(.name == $n)) | .[0]' <<<"$deploy")
    mmount=$(jq -r '(.volumeMounts // []) | map(select(.name == "readmodel" and .readOnly == true)) | .[0].mountPath // empty' <<<"$main")
    if [ -n "$mmount" ]; then ok; else ng "本体が readmodel を readOnly でマウントしていない"; continue; fi
    for pair in $(readmodel_env_files "$svc"); do
      var=${pair%%=*}
      file=${pair#*=}
      if jq -e --arg v "$var" --arg p "$mmount/$file" '(.env // []) | map(select(.name == $v and .value == $p)) | length == 1' <<<"$main" >/dev/null; then ok; else ng "本体の $var が $mmount/$file でない"; fi
    done
    if jq -e '[(.env // [])[] | select(.name | test("DSN|DATABASE"))] | length == 0' <<<"$main" >/dev/null; then ok; else ng "本体(業務 API)に DB の DSN が渡っている(read model を読むだけにする。ADR-0012 §6)"; fi
    # 本体の image は従来どおり digest 固定。
    if jq -e --arg s "/pokecalc/$svc@sha256:" '.image | contains($s)' <<<"$main" >/dev/null; then ok; else ng "本体の image が digest 固定の pokecalc/$svc でない"; fi
  done
}

# gitops overlay の pokedex image は digest 固定で、check-gitops.sh が見る(issue #237・ADR-0405)。
test_check_gitops_readmodel() {
  local svc dir rc ov
  make_fakes
  for svc in $SERVICES; do
    begin "check-gitops.sh: $svc の gitops overlay の read model 供給経路(pokedex の digest・initContainer・ConfigMap 不使用)を検査する(#237)"
    ov="services/$svc/deploy/k8s/overlays/gitops"
    dir=$(new_sandbox "rm-ok-$svc")
    check_gitops_in "$dir" "$svc" template
    rc=$?
    if [ "$rc" -eq 0 ]; then ok; else ng "正しい overlay の template 検査が失敗: $(tail -1 "$WORK/log/out")"; fi

    if grep -q 'name: pokecalc/pokedex' "$ROOT/$ov/kustomization.yaml"; then ok; else ng "$ov/kustomization.yaml の images に pokecalc/pokedex が無い"; fi

    # pokedex の image が tag 指定(digest でない)なら失敗。
    dir=$(new_sandbox "rm-pokedex-tag-$svc")
    sed -i.bak -E '/name: pokecalc\/pokedex/,/digest:/ s/digest: .*/newTag: v1/' "$dir/$ov/kustomization.yaml"
    check_gitops_in "$dir" "$svc" template
    rc=$?
    if [ "$rc" -ne 0 ]; then ok; else ng "pokedex image を tag 指定にしても $svc の template 検査が通った"; fi

    # ready では pokedex の digest が全0の placeholder なら失敗(本体の digest が実値でも)。
    dir=$(new_sandbox "rm-pokedex-zero-$svc")
    sed -i.bak -E '/name: pokecalc\/pokedex/,/digest:/ s/digest: .*/digest: sha256:0000000000000000000000000000000000000000000000000000000000000000/' "$dir/$ov/kustomization.yaml"
    check_gitops_in "$dir" "$svc" ready
    rc=$?
    if [ "$rc" -ne 0 ] && grep -qi 'pokedex' "$WORK/log/out"; then ok; else ng "pokedex の digest が placeholder でも $svc の ready 検査が通った(メッセージに pokedex の語も無い)"; fi

    # 描画に initContainer が無い(read model の供給経路が消えた)なら template でも失敗。
    dir=$(new_sandbox "rm-no-init-$svc")
    awk '/^patches:/ {skip=1; next} skip && /^[[:space:]]+-/ {next} skip && /^[[:space:]]+[a-z]/ {next} {skip=0; print}' "$dir/$ov/kustomization.yaml" >"$dir/$ov/k.new" && mv "$dir/$ov/k.new" "$dir/$ov/kustomization.yaml"
    check_gitops_in "$dir" "$svc" template
    rc=$?
    if [ "$rc" -ne 0 ] && grep -qiE 'initContainer|read model' "$WORK/log/out"; then ok; else ng "read model の供給経路(patch)を消しても $svc の template 検査が通った"; fi

    # ConfigMap を描画に混ぜたら失敗(ConfigMap は使わない)。
    dir=$(new_sandbox "rm-configmap-$svc")
    printf 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s-readmodel\ndata:\n  x: y\n' "$svc" >"$dir/$ov/cm.yaml"
    sed -i.bak 's#^resources:#resources:\n  - cm.yaml#' "$dir/$ov/kustomization.yaml"
    check_gitops_in "$dir" "$svc" template
    rc=$?
    if [ "$rc" -ne 0 ] && grep -qi 'ConfigMap' "$WORK/log/out"; then ok; else ng "gitops overlay に ConfigMap を足しても $svc の template 検査が通った"; fi
  done
}

# 手動 overlay(local-readmodel)と Argo CD が同じ Deployment を取り合わない(issue #237)。
test_manual_overlay_guard() {
  local svc dir rc f upper
  make_fakes
  for svc in $SERVICES; do
    upper=$(printf '%s' "$svc" | tr '[:lower:]' '[:upper:]')
    begin "手動デプロイと Argo CD の取り合い($svc): Application が在れば k3d-deploy-readmodel.sh は止まり、check-gitops.sh ready は local 上書きを検出する(#237)"
    # (1) Application pokecalc-$svc が在るクラスタでは、手動 apply を既定で拒否する(ALLOW_MANUAL_OVERLAY=1 で明示したときだけ進む)。
    dir=$(new_sandbox "guard-apply-$svc")
    mkdir -p "$dir/data/generated/readmodel"
    for f in pokemon-types.json moves.json abilities.json speed-pokemon.json; do printf '{}\n' >"$dir/data/generated/readmodel/$f"; done
    reset_log
    run_isolated "$dir" env FAKE_APP_EXISTS=1 SERVICE="$svc" ./scripts/gitops/k3d-deploy-readmodel.sh
    rc=$?
    if [ "$rc" -ne 0 ] && grep -qiE 'argo|ALLOW_MANUAL_OVERLAY' "$WORK/log/out"; then ok; else ng "Application が在るのに k3d-deploy-readmodel.sh が止まらない(rc=$rc)"; fi
    if ! grep -E '^kubectl .*apply' "$WORK/log/calls" | grep -q .; then ok; else ng "拒否したのに kubectl apply を呼んだ"; fi
    # (2) Application が無ければガードは邪魔をしない。
    reset_log
    run_isolated "$dir" env SERVICE="$svc" ./scripts/gitops/k3d-deploy-readmodel.sh
    if ! grep -qE 'ALLOW_MANUAL_OVERLAY' "$WORK/log/out"; then ok; else ng "Application が無いのにガードが働いた"; fi

    # (3) check-gitops.sh ready: 生きている Deployment に local-readmodel の annotation が残っていたら失敗(sync で戻す案内付き)。
    dir=$(new_sandbox "guard-check-$svc")
    reset_log
    run_isolated "$dir" env FAKE_LIVE_DEPLOY_ANNOTATIONS='{"pokecalc.example/readmodel-hash":"abc"}' SERVICE="$svc" "${upper}_GITOPS_REPO_URL=$FAKE_ORIGIN" ./scripts/gitops/check-gitops.sh ready
    rc=$?
    if [ "$rc" -ne 0 ] && grep -q 'readmodel-hash' "$WORK/log/out" && grep -qE 'sync' "$WORK/log/out"; then ok; else ng "local-readmodel の annotation が残っていても ready 検査が通った(rc=$rc)"; fi
  done
}

test_runbooks_no_manual_apply_with_argocd() {
  local svc file
  for svc in $SERVICES; do
    begin "runbook($svc): 「Argo CD 有効時は手動 apply しない」と、gitops overlay の initContainer が read model を作る旨を明記(#237)"
    file="$ROOT/docs/runbooks/$svc.md"
    if grep -qE 'Argo CD 有効時は手動 apply しない' "$file"; then ok; else ng "docs/runbooks/$svc.md に「Argo CD 有効時は手動 apply しない」の明記が無い"; fi
    if grep -q 'initContainer' "$file" && grep -q 'pokedex' "$file" && grep -q 'emptyDir' "$file"; then ok; else ng "docs/runbooks/$svc.md に gitops overlay の initContainer(pokedex export)→ emptyDir の説明が無い"; fi
    if grep -q 'pokedex-registry-push' "$file"; then ok; else ng "docs/runbooks/$svc.md に pokedex image の digest を gitops overlay へ書く手順(make pokedex-registry-push)が無い"; fi
  done
}

# 文書: ADR-0412(方式 a・speed の digest の扱い・取り合い対策)と、plan.md・speed の未配備表記(#237)。
test_issue237_docs() {
  local adr f
  begin "ドキュメント: ADR-0412 と ADR-0018/0403/0605 の追記、plan.md の行、speed の digest の扱い(#237)"
  adr=$(ls "$ROOT"/docs/adr/0412-*.md 2>/dev/null | head -1)
  if [ -n "$adr" ]; then ok; else ng "docs/adr/0412-*.md が無い"; return; fi
  for f in 0018-balance-local-gitops-verification 0403-balance-readmodel-wiring 0605-speed-sp5-gitops; do
    if grep -q '0412' "$ROOT/docs/adr/$f.md"; then ok; else ng "docs/adr/$f.md に ADR-0412 への追記が無い"; fi
  done
  for f in initContainer emptyDir pokedex-reader-dsn NetworkPolicy ConfigMap '#237'; do
    if grep -q -- "$f" "$adr"; then ok; else ng "ADR-0412 に「$f」の記述が無い"; fi
  done
  # plan は区画ごとのファイルに分かれている(ADR-0170)。索引と全区画を検査する。
  if grep -q '#237' "$ROOT/docs/plan.md" "$ROOT"/docs/plan/*.md; then ok; else ng "docs/plan.md・docs/plan/*.md に issue #237 の行が無い"; fi
  # speed の digest: 実 digest か、placeholder のままなら「未配備」と speed-design・runbook・ADR に明示。
  if grep -q 'digest: sha256:0\{64\}' "$ROOT/services/speed/deploy/k8s/overlays/gitops/kustomization.yaml"; then
    for f in docs/speed-design.md docs/runbooks/speed.md "docs/adr/$(basename "$adr")"; do
      if grep -q '未配備' "$ROOT/$f"; then ok; else ng "speed の gitops digest が placeholder なのに $f に「未配備」の明示が無い"; fi
    done
  else ok; fi
}

# ---------------------------------------------------------------------------

if [ -z "$REAL_KUBECTL" ] || [ -z "$REAL_HELM" ] || ! command -v jq >/dev/null 2>&1; then
  echo "gitops test: kubectl・helm・jq が要る(scripts/doctor.sh の前提ツール)" >&2
  exit 1
fi

test_appproject
test_application_project
test_common_scripts_exist
test_service_required
test_no_duplicated_logic
test_check_gitops_project
test_check_gitops_regressions
test_make_template_check
test_argocd_local_app_applies_project
test_registry_pvc
test_runbooks_return_to_gitops
test_initial_admin_secret
test_gitops_overlay_readmodel
test_check_gitops_readmodel
test_manual_overlay_guard
test_runbooks_no_manual_apply_with_argocd
test_issue237_docs

if [ "$FAILURES" -gt 0 ]; then
  printf 'gitops test: %d failed, %d passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
printf 'gitops test: all %d checks passed\n' "$PASSES"

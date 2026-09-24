#!/usr/bin/env bash
# 計算API SLO(p99・可用性)の記録ルールとダッシュボード(ADR-0407)の自動テスト。
# `make test-scripts`(make test に含む)から流す。
#
# 対象:
#   - deploy/k8s/base/observability/prometheusrules/calc-slo.yaml(PrometheusRule。ADR-0407 §1〜3)
#   - deploy/k8s/base/observability/dashboards/calc-slo.json(Grafana ダッシュボード。ADR-0407 §4)
#   - deploy/k8s/base/observability/dashboards/kustomization.yaml(configMapGenerator。ADR-0407 §4)
#   - deploy/k8s/base/observability/kustomization.yaml(親。上の2つを resources に含む)
#
# 方式は scripts/observability-bootstrap_test.sh(ADR-0406)と同じ: 手書きの ok/ng ヘルパー付きの bash。
# YAML は本物の helm でローカルの最小 chart を描画して構造として読み(chart repo へは行かない)、JSON は jq で読む。
# kustomize は `kubectl kustomize` の描画だけを使う(apply しない。実クラスタ・実ネットワークに触らない)。
# PromQL はパーサーを使わず文字列として検査する(ADR-0407 の式の要素が含まれていることの確認)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly OBS_REL="deploy/k8s/base/observability"
readonly RULE_REL="$OBS_REL/prometheusrules/calc-slo.yaml"
readonly DASH_DIR_REL="$OBS_REL/dashboards"
readonly DASH_REL="$DASH_DIR_REL/calc-slo.json"
readonly DASH_KUST_REL="$DASH_DIR_REL/kustomization.yaml"
readonly PARENT_KUST_REL="$OBS_REL/kustomization.yaml"
readonly KPS_VALUES_REL="$OBS_REL/values/kube-prometheus-stack.yaml"
readonly NAMESPACE="observability"

# ADR-0407 §3 の記録ルール名。
readonly RULE_P99="calc_job:calc_request_duration_seconds:p99_5m"
readonly RULE_AVAIL="calc_job:calc_availability_ratio:5m"
# ADR-0407 §1 の対象エンドポイント(calc-svc の計算3つ。/healthz・/readyz・/api/pokedex/* は含めない)。
readonly CALC_PATHS="/api/calc /api/calc/bulk /api/calc/reverse"

REAL_HELM=$(command -v helm || true)
readonly REAL_HELM
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
# YAML を構造として読む(scripts/observability-bootstrap_test.sh と同じ方式)
# ---------------------------------------------------------------------------

QCOUNT=0
# yaml_query YAMLファイル テンプレート本体 — 本体の中では $v が YAML 全体(map)。
# 結果として出したい行は `# R ` で始める。標準出力には `# R ` を外した行だけを返す。
yaml_query() {
  local file=$1 body=$2 chart
  if [ -z "$REAL_HELM" ]; then
    echo "helm が PATH に無い(doctor.sh の前提ツール)" >&2
    return 1
  fi
  QCOUNT=$((QCOUNT + 1))
  chart="$WORK/yq/$QCOUNT"
  mkdir -p "$chart/templates" "$WORK/yq-home"
  printf 'apiVersion: v2\nname: q\nversion: 0.0.0\n' >"$chart/Chart.yaml"
  printf '{{- $v := toYaml .Values | fromYaml -}}\n%s\n' "$body" >"$chart/templates/q.yaml"
  HELM_CONFIG_HOME="$WORK/yq-home" HELM_CACHE_HOME="$WORK/yq-home" HELM_DATA_HOME="$WORK/yq-home" \
    "$REAL_HELM" template q "$chart" -f "$file" 2>"$chart/err" >"$chart/out" || {
    cat "$chart/err" >&2
    return 1
  }
  sed -n 's/^# R //p' "$chart/out"
}

# yaml_get YAMLファイル キー... — dig でたどった値を JSON で返す(無ければ null)。
yaml_get() {
  local file=$1 keys="" k
  shift
  for k in "$@"; do keys="$keys \"$k\""; done
  yaml_query "$file" "# R {{ dig$keys nil \$v | toJson }}"
}

expect_json() {
  if [ "$2" = "$3" ]; then ok; else ng "$1 が $3 であるべきところ ${2:-(読めない)}"; fi
}

# rule_expr 記録ルール名 — PrometheusRule の中でその record を持つルールの expr。
# 複数行で書かれた式も1行の文字列として検査できるよう、改行を空白に置き換えて返す。
rule_expr() {
  yaml_query "$ROOT/$RULE_REL" '{{ range (dig "spec" "groups" list $v) }}{{ range (dig "rules" list .) }}{{ if eq (toString (dig "record" "" .)) "'"$1"'" }}# R {{ dig "expr" "" . | toJson }}
{{ end }}{{ end }}{{ end }}' | jq -r '.' | tr '\n' ' ' | sed -E 's/[[:space:]]+$//'
}

# path_sets 式 — 式中の path=~"..." をすべて取り出し、1つずつ「選択肢を整列して空白区切り」にした行を返す。
path_sets() {
  printf '%s\n' "$1" | grep -oE 'path=~"[^"]*"' | sed -E 's/^path=~"(.*)"$/\1/' |
    while IFS= read -r alt; do printf '%s\n' "$alt" | tr '|' '\n' | sort | tr '\n' ' ' | sed 's/ $//'; echo; done
}

expected_path_set() { printf '%s\n' $CALC_PATHS | sort | tr '\n' ' ' | sed 's/ $//'; }

# expect_calc_paths ラベル 式 — path=~ がちょうど計算3エンドポイントだけを対象にしている(1つ以上あり、すべてが同じ集合)。
expect_calc_paths() {
  local label=$1 expr=$2 sets want bad
  sets=$(path_sets "$expr")
  want=$(expected_path_set)
  if [ -z "$sets" ]; then
    ng "$label に path=~\"...\" の絞り込みが無い(対象: $CALC_PATHS)"
    return
  fi
  bad=$(printf '%s\n' "$sets" | grep -vxF "$want" || true)
  if [ -z "$bad" ]; then ok; else
    ng "$label の path=~ が計算3エンドポイント($want)と一致しない: $(printf '%s' "$bad" | tr '\n' ';')"
  fi
}

# ---------------------------------------------------------------------------
# 1. PrometheusRule(ADR-0407 §1〜3)
# ---------------------------------------------------------------------------

test_rule_shape() {
  begin "PrometheusRule: $RULE_REL が monitoring.coreos.com/v1 の PrometheusRule で namespace $NAMESPACE にある"
  local f="$ROOT/$RULE_REL" name
  [ -f "$f" ] || { ng "$RULE_REL が無い"; return; }
  expect_json "apiVersion" "$(yaml_get "$f" apiVersion)" '"monitoring.coreos.com/v1"'
  expect_json "kind" "$(yaml_get "$f" kind)" '"PrometheusRule"'
  expect_json "metadata.namespace" "$(yaml_get "$f" metadata namespace)" "\"$NAMESPACE\""
  name=$(yaml_get "$f" metadata name)
  if printf '%s' "$name" | grep -Eq '^"[^"]+"$'; then ok; else ng "metadata.name が空: ${name:-(読めない)}"; fi
}

test_rule_records() {
  begin "PrometheusRule: 記録ルール $RULE_P99 と $RULE_AVAIL がちょうど1つずつあり、アラートルールが無い(ADR-0407 §2〜3)"
  local f="$ROOT/$RULE_REL" records r n alerts
  [ -f "$f" ] || { ng "$RULE_REL が無い"; return; }
  records=$(yaml_query "$f" '{{ range (dig "spec" "groups" list $v) }}{{ range (dig "rules" list .) }}# R {{ dig "record" "" . }}
{{ end }}{{ end }}') || { ng "$RULE_REL が YAML として読めない"; return; }
  for r in "$RULE_P99" "$RULE_AVAIL"; do
    n=$(printf '%s\n' "$records" | grep -cxF "$r" || true)
    if [ "$n" = 1 ]; then ok; else ng "記録ルール $r が $n 個(ちょうど1つであるべき)"; fi
  done
  alerts=$(yaml_query "$f" '{{ range (dig "spec" "groups" list $v) }}{{ range (dig "rules" list .) }}{{ if hasKey . "alert" }}# R {{ .alert }}
{{ end }}{{ end }}{{ end }}')
  if [ -z "$alerts" ]; then ok; else ng "アラートルール(alert:)がある(ADR-0407 §2 で作らないと決めた): $(printf '%s' "$alerts" | tr '\n' ';')"; fi
}

test_rule_p99_expr() {
  begin "PrometheusRule: $RULE_P99 の式が job=\"calc\"・計算3エンドポイントのヒストグラムから histogram_quantile(0.99, ...) を5分窓で求める(ADR-0407 §1)"
  [ -f "$ROOT/$RULE_REL" ] || { ng "$RULE_REL が無い"; return; }
  local e
  e=$(rule_expr "$RULE_P99")
  [ -n "$e" ] || { ng "$RULE_P99 の expr が無い"; return; }
  if printf '%s' "$e" | grep -Eq 'histogram_quantile\([[:space:]]*0\.99[[:space:]]*,'; then ok; else ng "histogram_quantile(0.99, ...) を使っていない: $e"; fi
  if printf '%s' "$e" | grep -q 'http_request_duration_seconds_bucket'; then ok; else ng "http_request_duration_seconds_bucket を使っていない: $e"; fi
  if printf '%s' "$e" | grep -q 'job="calc"'; then ok; else ng "job=\"calc\" で絞っていない: $e"; fi
  expect_calc_paths "$RULE_P99 の式" "$e"
  if printf '%s' "$e" | grep -q '\[5m\]'; then ok; else ng "5分窓([5m])でない: $e"; fi
  if printf '%s' "$e" | grep -Eq 'by[[:space:]]*\([[:space:]]*le[[:space:]]*\)'; then ok; else ng "le で集約(by (le))していない: $e"; fi
}

test_rule_availability_expr() {
  begin "PrometheusRule: $RULE_AVAIL の式が計算3エンドポイントの 5xx 以外 / 全体 の比を5分窓で求める(ADR-0407 §1)"
  [ -f "$ROOT/$RULE_REL" ] || { ng "$RULE_REL が無い"; return; }
  local e n
  e=$(rule_expr "$RULE_AVAIL")
  [ -n "$e" ] || { ng "$RULE_AVAIL の expr が無い"; return; }
  n=$(printf '%s' "$e" | grep -o 'http_requests_total' | wc -l | tr -d ' ')
  if [ "$n" -ge 2 ]; then ok; else ng "http_requests_total を分子・分母の2か所で使っていない($n か所): $e"; fi
  if printf '%s' "$e" | grep -q 'job="calc"'; then ok; else ng "job=\"calc\" で絞っていない: $e"; fi
  expect_calc_paths "$RULE_AVAIL の式" "$e"
  # 5xx 除外は分子だけ(分母にもあると常に 1 になる)。
  n=$(printf '%s' "$e" | grep -oE 'status!~"5\.\."' | wc -l | tr -d ' ')
  if [ "$n" = 1 ]; then ok; else ng "5xx 除外(status!~\"5..\")が分子の1か所だけでない($n か所): $e"; fi
  if printf '%s' "$e" | grep -q '\[5m\]'; then ok; else ng "5分窓([5m])でない: $e"; fi
  if printf '%s' "$e" | grep -q ')[[:space:]]*/[[:space:]]*'; then ok; else ng "比(分子 / 分母)になっていない: $e"; fi
  # 5xx 除外が「/」より前(分子側)に出てくることまで確かめる(分母だけに入っていて全体としては
  # 1回・比の形になっている、という誤りを見逃さないため)。
  local numerator
  numerator=$(printf '%s' "$e" | sed -E 's#^(.*)\)[[:space:]]*/[[:space:]]*.*$#\1#')
  if printf '%s' "$numerator" | grep -q 'status!~"5\.\."'; then
    ok
  else
    ng "5xx 除外(status!~\"5..\")が「/」より前(分子側)に見つからない: $e"
  fi
}

test_rule_discovery() {
  begin "PrometheusRule: kube-prometheus-stack の Prometheus がこの記録ルールを拾う(release ラベル必須の既定を外すか、ルールに合うセレクタ)"
  local v="$ROOT/$KPS_VALUES_REL" r="$ROOT/$RULE_REL" nil sel labels kv missing=""
  [ -f "$v" ] || { ng "$KPS_VALUES_REL が無い"; return; }
  [ -f "$r" ] || { ng "$RULE_REL が無い"; return; }
  nil=$(yaml_get "$v" prometheus prometheusSpec ruleSelectorNilUsesHelmValues)
  sel=$(yaml_query "$v" '{{ range $k, $x := (dig "prometheus" "prometheusSpec" "ruleSelector" "matchLabels" dict $v) }}# R {{ $k }}={{ $x }}
{{ end }}')
  if [ -n "$sel" ]; then
    labels=$(yaml_query "$r" '{{ range $k, $x := (dig "metadata" "labels" dict $v) }}# R {{ $k }}={{ $x }}
{{ end }}' || true)
    while IFS= read -r kv; do
      [ -n "$kv" ] || continue
      printf '%s\n' "$labels" | grep -qxF "$kv" || missing="$missing $kv"
    done <<<"$sel"
    if [ -z "$missing" ]; then ok; else ng "PrometheusRule が ruleSelector のラベルを持たない:$missing"; fi
  elif [ "$nil" = false ]; then
    ok
  else
    ng "prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues が false でなく、ruleSelector も無い(既定では release ラベルの無い PrometheusRule を拾わない)"
  fi
}

# ---------------------------------------------------------------------------
# 2. ダッシュボード JSON(ADR-0407 §4)
# ---------------------------------------------------------------------------

# jq でダッシュボードの全パネル(row の中の panels も含む。row 自体は除く)を返すフィルタ。
readonly JQ_PANELS='[.panels[]? | ., (.panels[]?)] | map(select(.type != "row"))'

dash_jq() { jq -r "$1" "$ROOT/$DASH_REL"; }

test_dashboard_json() {
  begin "ダッシュボード: $DASH_REL が JSON として読め、title・uid と panels 配列を持つ"
  [ -f "$ROOT/$DASH_REL" ] || { ng "$DASH_REL が無い"; return; }
  if jq -e . "$ROOT/$DASH_REL" >/dev/null 2>&1; then ok; else ng "$DASH_REL が JSON として読めない"; return; fi
  if [ "$(dash_jq '(.title | type) == "string" and (.title | length) > 0')" = true ]; then ok; else ng "title が無い"; fi
  # sidecar で読み込むダッシュボードの URL を固定するため uid を持たせる。
  if [ "$(dash_jq '(.uid | type) == "string" and (.uid | length) > 0')" = true ]; then ok; else ng "uid が無い"; fi
  if [ "$(dash_jq '(.panels | type) == "array"')" = true ]; then ok; else ng "panels が配列でない"; fi
}

test_dashboard_panels() {
  begin "ダッシュボード: p99 の時系列(100ms しきい値)・可用性の時系列(%)・直近値の数値パネル(p99・可用性)がある(ADR-0407 §4)"
  [ -f "$ROOT/$DASH_REL" ] || { ng "$DASH_REL が無い"; return; }
  jq -e . "$ROOT/$DASH_REL" >/dev/null 2>&1 || { ng "$DASH_REL が JSON として読めない"; return; }
  local n
  n=$(dash_jq "$JQ_PANELS | length")
  if [ "$n" -ge 3 ]; then ok; else ng "パネルが3つ未満($n)"; fi

  # 1. p99 の時系列: timeseries で p99 の記録ルールを参照する。
  local p99ts
  p99ts=$(dash_jq "$JQ_PANELS | map(select(.type == \"timeseries\" and ([.targets[]?.expr // \"\"] | any(contains(\"$RULE_P99\"))))) | length")
  if [ "$p99ts" -ge 1 ]; then ok; else ng "p99($RULE_P99)を参照する timeseries パネルが無い"; return; fi
  # 単位は秒(s。Grafana が ms 表示にする)か、式で1000倍して ms。しきい値 100ms のラインを引く。
  local okline
  okline=$(dash_jq "$JQ_PANELS | map(select(.type == \"timeseries\" and ([.targets[]?.expr // \"\"] | any(contains(\"$RULE_P99\")))))
    | any(
        (.fieldConfig.defaults.unit as \$u
          | [.fieldConfig.defaults.thresholds.steps[]?.value] as \$vals
          | ((\$u == \"s\" and (\$vals | index(0.1))) or (\$u == \"ms\" and (\$vals | index(100)))))
        and ((.fieldConfig.defaults.custom.thresholdsStyle.mode // \"off\") != \"off\"))")
  if [ "$okline" = true ]; then ok; else
    ng "p99 の timeseries に 100ms のしきい値ライン(unit s なら 0.1・ms なら 100 の threshold step と、custom.thresholdsStyle.mode が off 以外)が無い"
  fi

  # 2. 可用性の時系列: timeseries で可用性の記録ルールを参照し、パーセント表示。
  local avts
  avts=$(dash_jq "$JQ_PANELS | map(select(.type == \"timeseries\" and ([.targets[]?.expr // \"\"] | any(contains(\"$RULE_AVAIL\")))))
    | map(select(.fieldConfig.defaults.unit == \"percentunit\" or .fieldConfig.defaults.unit == \"percent\")) | length")
  if [ "$avts" -ge 1 ]; then ok; else ng "可用性($RULE_AVAIL)を参照しパーセント表示(unit percentunit/percent)の timeseries パネルが無い"; fi

  # 3. 直近の値: 数値パネル(stat)で p99・可用性それぞれを出す。
  local r c
  for r in "$RULE_P99" "$RULE_AVAIL"; do
    c=$(dash_jq "$JQ_PANELS | map(select(.type == \"stat\" and ([.targets[]?.expr // \"\"] | any(contains(\"$r\"))))) | length")
    if [ "$c" -ge 1 ]; then ok; else ng "$r を参照する数値パネル(type stat)が無い"; fi
  done
}

test_dashboard_uses_recording_rules() {
  begin "ダッシュボード: すべてのクエリが記録ルールを参照し、生の histogram_quantile・rate を評価しない(ADR-0407 §3)"
  [ -f "$ROOT/$DASH_REL" ] || { ng "$DASH_REL が無い"; return; }
  jq -e . "$ROOT/$DASH_REL" >/dev/null 2>&1 || { ng "$DASH_REL が JSON として読めない"; return; }
  local exprs e bad=""
  exprs=$(dash_jq "$JQ_PANELS | [.[].targets[]? | .expr // \"\"] | .[]")
  if [ -n "$exprs" ]; then ok; else ng "targets[].expr が1つも無い"; return; fi
  while IFS= read -r e; do
    if ! printf '%s' "$e" | grep -qF -e "$RULE_P99" -e "$RULE_AVAIL"; then bad="$bad [$e]"; fi
    if printf '%s' "$e" | grep -Eq 'histogram_quantile|http_request_duration_seconds|http_requests_total|rate\('; then bad="$bad [$e]"; fi
  done <<<"$exprs"
  if [ -z "$bad" ]; then ok; else ng "記録ルールを使わないクエリ(または生のメトリクス・rate/histogram_quantile を含むクエリ)がある:$bad"; fi
}

test_dashboard_no_alert() {
  begin "ダッシュボード: Grafana のアラート定義(alert)を含まない(ADR-0407 §2)"
  [ -f "$ROOT/$DASH_REL" ] || { ng "$DASH_REL が無い"; return; }
  jq -e . "$ROOT/$DASH_REL" >/dev/null 2>&1 || { ng "$DASH_REL が JSON として読めない"; return; }
  if [ "$(dash_jq '[.. | objects | has("alert")] | any')" = false ]; then ok; else ng "alert キーがある(アラートは作らない)"; fi
}

# ---------------------------------------------------------------------------
# 3. kustomize(dashboards/ の configMapGenerator・親の resources・描画)
# ---------------------------------------------------------------------------

test_dashboard_kustomization() {
  begin "kustomize: $DASH_DIR_REL が calc-slo.json を持つ ConfigMap を grafana_dashboard: \"1\" ラベル付きで生成する(ADR-0407 §4)"
  command -v kubectl >/dev/null 2>&1 || { ng "kubectl が PATH に無い"; return; }
  [ -f "$ROOT/$DASH_KUST_REL" ] || { ng "$DASH_KUST_REL が無い"; return; }
  local gen out f n
  gen=$(yaml_query "$ROOT/$DASH_KUST_REL" '{{ range (dig "configMapGenerator" list $v) }}# R gen
{{ end }}')
  if [ -n "$gen" ]; then ok; else ng "$DASH_KUST_REL に configMapGenerator が無い"; fi
  if out=$(kubectl kustomize "$ROOT/$DASH_DIR_REL" 2>&1); then ok; else ng "kubectl kustomize $DASH_DIR_REL が失敗: $out"; return; fi
  n=$(printf '%s\n' "$out" | grep -cE '^kind: ConfigMap$' || true)
  if [ "$n" = 1 ]; then ok; else ng "描画結果の ConfigMap が1つでない: $n"; return; fi
  f="$WORK/dash-cm.yaml"
  printf '%s\n' "$out" >"$f"
  # sidecar は文字列の "1" と照合する(数値 1 ではない)。
  expect_json "ConfigMap の metadata.labels.grafana_dashboard" "$(yaml_get "$f" metadata labels grafana_dashboard)" '"1"'
  if yaml_query "$f" '{{ range $k, $x := (dig "data" dict $v) }}# R {{ $k }}
{{ end }}' | grep -qxF "calc-slo.json"; then ok; else ng "ConfigMap の data に calc-slo.json が無い"; fi
}

test_parent_kustomization() {
  begin "kustomize: 親 $PARENT_KUST_REL の resources に prometheusrules/ と dashboards/ がある"
  local f="$ROOT/$PARENT_KUST_REL" res
  [ -f "$f" ] || { ng "$PARENT_KUST_REL が無い"; return; }
  res=$(yaml_query "$f" '{{ range (dig "resources" list $v) }}# R {{ . }}
{{ end }}') || { ng "$PARENT_KUST_REL が YAML として読めない"; return; }
  if printf '%s\n' "$res" | grep -Eq '^prometheusrules/calc-slo\.yaml$|^prometheusrules/?$'; then ok; else
    ng "resources に prometheusrules/calc-slo.yaml(または prometheusrules/)が無い"
  fi
  if printf '%s\n' "$res" | grep -Eq '^dashboards/?$'; then ok; else ng "resources に dashboards/ が無い"; fi
}

test_render() {
  begin "kustomize: $OBS_REL の描画結果に PrometheusRule 1つと grafana_dashboard ラベルの ConfigMap 1つが namespace $NAMESPACE で含まれる"
  command -v kubectl >/dev/null 2>&1 || { ng "kubectl が PATH に無い"; return; }
  local out n
  if out=$(kubectl kustomize "$ROOT/$OBS_REL" 2>&1); then ok; else ng "kubectl kustomize $OBS_REL が失敗: $out"; return; fi
  printf '%s\n' "$out" >"$WORK/rendered.yaml"
  n=$(grep -cE '^kind: PrometheusRule$' "$WORK/rendered.yaml" || true)
  if [ "$n" = 1 ]; then ok; else ng "描画結果の PrometheusRule が1つでない: $n"; fi
  # 文書ごとに kind・namespace・grafana_dashboard ラベルを拾う(kubectl kustomize の出力は metadata 直下が2字下げ)。
  local docs
  docs=$(awk '
    function flush() { if (kind != "") print kind "|" ns "|" dash; kind = ""; ns = ""; dash = "" }
    /^---$/ { flush(); next }
    /^kind: / { kind = $2 }
    /^  namespace: / { ns = $2 }
    /^    grafana_dashboard: / { dash = $2 }
    END { flush() }
  ' "$WORK/rendered.yaml")
  n=$(printf '%s\n' "$docs" | grep -cE '^ConfigMap\|[^|]*\|"1"$' || true)
  if [ "$n" = 1 ]; then ok; else ng "grafana_dashboard: \"1\" の ConfigMap が1つでない: $n"; fi
  if printf '%s\n' "$docs" | grep -E '^PrometheusRule\|' | grep -qxF "PrometheusRule|$NAMESPACE|"; then ok; else
    ng "PrometheusRule の namespace が $NAMESPACE でない"
  fi
  if printf '%s\n' "$docs" | grep -E '^ConfigMap\|[^|]*\|"1"$' | grep -qE "^ConfigMap\|$NAMESPACE\|"; then ok; else
    ng "ダッシュボードの ConfigMap の namespace が $NAMESPACE でない(Grafana sidecar は既定で自 namespace だけを探す)"
  fi
}

test_cloud_overlay_untouched() {
  begin "kustomize: cloud overlay に PrometheusRule・ダッシュボードの ConfigMap が混ざらない(ADR-0406 §5)"
  command -v kubectl >/dev/null 2>&1 || { ng "kubectl が PATH に無い"; return; }
  local out
  if out=$(kubectl kustomize "$ROOT/deploy/k8s/overlays/cloud" 2>&1); then ok; else ng "cloud overlay が描画できない: $out"; return; fi
  if printf '%s\n' "$out" | grep -qE '^kind: PrometheusRule$'; then ng "cloud overlay の描画結果に PrometheusRule がある"; else ok; fi
  if printf '%s\n' "$out" | grep -qE '^[[:space:]]+grafana_dashboard:'; then ng "cloud overlay の描画結果にダッシュボードの ConfigMap がある"; else ok; fi
}

# ---------------------------------------------------------------------------

test_rule_shape
test_rule_records
test_rule_p99_expr
test_rule_availability_expr
test_rule_discovery
test_dashboard_json
test_dashboard_panels
test_dashboard_uses_recording_rules
test_dashboard_no_alert
test_dashboard_kustomization
test_parent_kustomization
test_render
test_cloud_overlay_untouched

if [ "$FAILURES" -gt 0 ]; then
  printf 'observability-slo test: %d failed, %d passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
printf 'observability-slo test: all %d checks passed\n' "$PASSES"

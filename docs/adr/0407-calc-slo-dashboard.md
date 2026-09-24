# ADR-0407: 計算APIのSLO(p99 < 100ms・可用性)とダッシュボード(P7-2)

- 状態: 採用(2026-09-25。docs/plan.md M4「運用」P7-2。P7-1〈ADR-0406〉の上に乗る)
- 日付: 2026-09-25
- 関連: docs/plan.md M4、ADR-0406(P7-1。メトリクス計測・kube-prometheus-stack/Loki/Alloy導入)

## 背景
P7-1 で calc-svc を含む6サービスに `/metrics`(`http_requests_total`・`http_request_duration_seconds`)が入り、
Prometheus が実際に scrape できる状態になった。plan.md の M4 は「計算API p99 < 100ms、可用性」を SLO として求めている。
対象の「計算API」は calc-svc の実際の計算エンドポイント3つ(`POST /api/calc`・`POST /api/calc/bulk`・
`POST /api/calc/reverse`)。`/healthz`・`/readyz`・pokedex プロキシ経由の `/api/pokedex/*` は対象外(計算そのものではない)。

## 決定

### 1. SLI(測る対象)は2つ、PromQLの式で表す。新しい計測コードは足さない
P7-1 の `http_requests_total`・`http_request_duration_seconds`(job="calc"、path が計算3エンドポイントのいずれか)
だけで両方とも計算できるため、calc-svc 側にコードは追加しない。

- **レイテンシ**: `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="calc",
  path=~"/api/calc|/api/calc/bulk|/api/calc/reverse"}[5m])) by (le))`(直近5分window のp99。単位は秒のまま持ち、
  ダッシュボードパネルの unit は `s` にする。Grafana が1秒未満の値を自動で ms 表示に切り替える)。
- **可用性**: `sum(rate(http_requests_total{job="calc",path=~"/api/calc|/api/calc/bulk|/api/calc/reverse",
  status!~"5.."}[5m])) / sum(rate(http_requests_total{job="calc",path=~"/api/calc|/api/calc/bulk|/api/calc/reverse"}[5m]))`
  (5xx以外を「利用可能」とみなす。4xx〈不正な入力〉はクライアント起因なのでSLOの不可用には数えない)。

### 2. 正式なエラーバジェット・多窓バーンレートアラートは今回作らない(先回りしない)
個人利用規模で、オンコール・アラート通知先が無い状態での多段バーンレートアラート(SRE本流のSLO運用)は過剰と判断する。
**ダッシュボードで目視できること**を今回のゴールにする。しきい値ライン(100ms)をパネルに引くだけにとどめ、
`PrometheusRule` によるアラート発報は作らない。必要になったら別ADRで追加する。

### 3. 記録ルール(PrometheusRule)は作る。ダッシュボードのクエリを軽くするため
毎回 `rate()`/`histogram_quantile()` を生クエリで評価するのではなく、`PrometheusRule`(kube-prometheus-stack の
CRD)で以下の記録ルールを追加する(評価済みの時系列を事前計算しておく。個人利用規模では必須ではないが、
ADR-0406(P7-1)で ServiceMonitor を CRD ベースの宣言的リソースとしてコード管理したのと同じ流儀に揃える)。
- `calc_job:calc_request_duration_seconds:p99_5m`
- `calc_job:calc_availability_ratio:5m`
`deploy/k8s/base/observability/prometheusrules/calc-slo.yaml`(`metadata.namespace: observability`)に置く
(ServiceMonitor と同じ場所の並び)。ServiceMonitor(ADR-0406 §4)と同じ理由で、`PrometheusRule` も
kube-prometheus-stack の release ラベルを持たないため、`values/kube-prometheus-stack.yaml` の
`prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues: false` を追加して Prometheus に読ませる。

### 4. ダッシュボードは JSON をコードとして repo にコミットし、ConfigMap 経由で自動発見させる
kube-prometheus-stack の Grafana sidecar は `grafana_dashboard: "1"` ラベルを持つ ConfigMap を自動で読み込む
(chart 既定機能。追加の設定不要)。`deploy/k8s/base/observability/dashboards/calc-slo.json` を Grafana ダッシュボード
JSON(手書きまたは `helm template`/Grafana UI からエクスポートしたものを整形)として作り、
`deploy/k8s/base/observability/dashboards/kustomization.yaml`(`configMapGenerator`)でラベル付き ConfigMap を生成する。

パネル構成(最小限。増やさない):
1. p99 レイテンシの時系列(§3の記録ルール。100msのしきい値ラインを重ねる)。
2. 可用性の時系列(§3の記録ルール。パーセント表示)。
3. 直近の値(単一の数値パネル。p99と可用性それぞれ)。

### 5. Prometheus の保持期間
kube-prometheus-stack の既定保持期間(chart 既定値。バージョンによって異なるが概ね10日)のままにする。個人利用でSLOを長期
(30日等)で見る必要が出たら、`prometheus.prometheusSpec.retention` を values で上書きする(今回は変更しない。
先回りしない)。

## 影響と制約
- calc-svc 以外のサービス(balance・speed・judge・gateway・pokedex)のSLOはP7-2の対象外。将来必要になったら
  同じ記録ルール・ダッシュボードのパターンを複製する。
- ダッシュボードJSONはGrafanaのUIでバージョンやパネルIDが変わると差分が読みにくくなる。手で整形して
  読みやすさを保つ(自動エクスポートした巨大JSONをそのまま貼らない)。
- アラート通知(Slack/メール等)は範囲外。ダッシュボードを人が見て判断する運用。

## 却下した案
- 多窓バーンレートアラート(SRE本流のSLO運用)を今回作る: 個人利用・通知先が無い状態では過剰。ダッシュボード目視で十分。
- 6サービス全部にSLOダッシュボードを広げる: plan.mdが明示するのは「計算API」のみ。他サービスは要望が出たら追加する。
- Prometheusの保持期間を今回延ばす: 個人利用の既定運用で困っていない。必要になったら変更する。

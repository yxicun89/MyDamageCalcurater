# ADR-0420: balance-svc の SLO(p99 < 500ms・可用性)とダッシュボード(P7-2 の延長)

- 状態: 採用(2026-10-02)
- 日付: 2026-10-02
- 関連: ADR-0407(calc の SLO。この ADR はその方式の踏襲)、ADR-0406(メトリクス)、ADR-0409(recommendations の同時実行上限)、
  docs/type-balance-design.md §11

## 背景
type-balance-design.md §11 に「balance の SLO・ダッシュボード: なし(calc のみ。ADR-0407)」とあった。balance-svc も
ADR-0406 で `http_requests_total`・`http_request_duration_seconds`(`job="balance"`)を出しているので、
ADR-0407 と同じ方式で SLO を目視できるようにする。

## 決定

### 1. SLI(新しい計測コードは足さない)
対象は balance の計算 API 5つ。`services/balance/api/openapi.yaml` のパスと、メトリクスの `path` ラベル(route pattern。
`httpmetrics` が `c.Path()` を使う)が一致する。`/healthz`・`/api/balance/healthz`・`/metrics` は対象外。

`/api/balance/v1/team-balance/analyze`・`/api/balance/v1/team-balance/coverage`・`/api/balance/v1/team-balance/threats`・
`/api/balance/v1/team-balance/recommendations`・`/api/balance/v1/move-range/analyze`

- レイテンシ: `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="balance",path=~"<5つ>"}[5m])) by (le))`
- 可用性: 同じ絞り込みで `status!~"5.."` の rate / 全体の rate(ADR-0407 §1 と同じ形。4xx は不可用に数えない)。

### 2. 目標値: p99 < 500ms。recommendations は別系列にしない
- 実測(`BenchmarkRecommendations`、1,500件・6メンバー・limit 20、Apple M5 Pro、2026-10-02): 約 **3.5 ms/op**・約 0.53 MB/op
  (ADR-0409)。最も重い recommendations でも計算本体は数 ms で、他のエンドポイントと桁が違わない。
  よって別系列・別しきい値にする根拠が無く、1系列にまとめる。
- 500ms の理由: 計算本体は数 ms なので、p99 を押し上げるのは CPU limit 100m でのスロットリング・同時実行・GC。
  それらを許容しつつ「明らかに遅い」を捉える値として 500ms にする。Prometheus 既定バケット(`.005 … .25 .5 1`)の境界に
  500ms があり、`histogram_quantile` の補間誤差が出ない。calc の 100ms とはそろえない(サービスの CPU limit・計算の性質が違う)。
- 本番相当の負荷での実測ではない(ベンチは単発・開発機)。実クラスタで p99 が常に 500ms 付近なら見直す(別 ADR)。

### 3. 可用性: 503 overloaded(ADR-0409)は不可用に数える
`status!~"5.."` の定義をそのまま使うので、同時実行上限による `503`(`overloaded`)は不可用に数える。理由:
- 利用者から見れば「リクエストが処理されなかった」ことに変わりなく、上限を超えるほど混む状況は SLO で気づきたい信号である。
- 4xx のように呼び出し側の誤りではなく、サービス側の容量(64Mi・100m の設計)に起因する。
- balance は 429 を返さない(待たせず 503)。
- `overloaded` だけを除外するには `code` ラベルが要るが `http_requests_total` に無く、新しい計測が必要になる(方針に反する)。
  `master_unavailable`(503)も同様に不可用に数える(マスタ未投入は実際に使えない状態)。

### 4. 記録ルール・ダッシュボード・アラート
ADR-0407 §2〜4 と同じ。
- 記録ルール: `balance_job:balance_request_duration_seconds:p99_5m`・`balance_job:balance_availability_ratio:5m`
  (`prometheusrules/balance-slo.yaml`。ruleSelectorNilUsesHelmValues は ADR-0407 で設定済み)。
- ダッシュボード: `dashboards/balance-slo.json`(uid `balance-slo`。パネル構成は calc と同じ4つ、しきい値ラインは 500ms)を
  `balance-slo-dashboard` ConfigMap(`grafana_dashboard: "1"`)として取り込む。
- **アラートは作らない**(ADR-0407 §2 と同じ理由: 個人利用・通知先なし。必要なら別 ADR)。

### 5. 検査
`scripts/observability-slo_test.sh`(`make test-scripts`)を calc・balance の2対象で流す。式・job・path 集合(5つ)・5分窓・
5xx 除外が分子のみ・しきい値ライン・記録ルール参照のみ・alert 無し・ConfigMap・親 kustomization を静的に検査する。

## 検証状況
実クラスタでの確認(Prometheus がルールを読む・Grafana に表示される)は**未実施**(共有クラスタに触れていない)。
静的検査と `kubectl kustomize` の描画のみ。

## 却下した案
- recommendations を別系列・別しきい値にする: 実測で他と桁が同じ。
- 503 overloaded を可用性から除く: 新しい計測が要る。容量不足は SLO で見えるべき。
- balance にもアラートを作る: ADR-0407 §2 と同じ理由。

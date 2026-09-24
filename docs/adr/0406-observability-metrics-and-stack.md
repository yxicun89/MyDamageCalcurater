# ADR-0406: メトリクス計測と監視スタック(kube-prometheus-stack / Loki)導入(P7-1)

- 状態: 採用(2026-09-24。docs/plan.md M4「運用」P7-1。別セッションからの依頼をユーザーが承認)
- 日付: 2026-09-24
- 関連: docs/plan.md M4、docs/requirements.md(技術スタック表「監視 | kube-prometheus-stack, Loki」)、
  ADR-0018(balance の Argo CD 導入。ローカル k3d 先行検証の流儀)、ADR-0405(install物のハッシュ/digest固定)、
  ADR-0407(P7-2。SLO・ダッシュボード。本ADRの上に乗る)

## 背景
M1〜M3(計算・タイプバランス・素早さ・判定・iOS)は完了し、各サービスに `/metrics` などの計測エンドポイントは一つも無い
(`prometheus/client_golang` への依存も無し)。requirements.md は監視ツールとして kube-prometheus-stack・Loki を名指ししているが、
詳細設計は無い。P7-3(Argo CD/GitOps)はタイプバランスレーンがすでに ADR-0018・ADR-0405 で導入済みで、その流儀
(ローカル k3d 先行検証・イメージ digest 固定・repoURL を Git に書かない・manual sync)を踏襲する。

サービス構成は2種類に分かれる: `gateway`・`pokedex`・`calc`(`services/go.mod` 配下、Go module 共有)と、
`balance`・`speed`・`judge`(各自独立した go.mod。ADR-0012 のサービス境界どおり互いに依存しない)。
`record`・`team` はまだ実装されていない(M2未着手)ので対象外(実装時に本ADRの型を適用する)。

## 決定

### 1. メトリクス計測(各サービス共通の最小セット)
各サービスに Prometheus text format の `GET /metrics` を追加する(認証なし。private overlay 内のみ露出。issue #148 の
「私設サービスを維持する」決定に従い、public Ingress には出さない)。

計測する指標は最小限にする(必要になったら増やす。将来の拡張を見込んで先回りしない):
- `http_requests_total{method,path,status}`(counter)
- `http_request_duration_seconds{method,path}`(histogram。デフォルトの prometheus/client_golang バケットをまず使う。
  P7-2 の SLO 評価で足りなければ ADR-0407 側でバケットを調整する)
- `path` ラベルは実際に来たパスそのものではなく、ルーティングパターン(例 `/api/pokedex/species/:key`)を使う
  (未知のIDがラベル値に紛れ込みカーディナリティが爆発するのを防ぐ)。どのルートにも当たらない 404 はルーティング
  パターンが無いため `path=""` になる(Prometheus では空文字ラベルは「そのラベルを付けない」のと同じ扱いなので、
  他の `path` 値と混同はしないが、集計時はこの系列を「ルート不明」として読む必要がある)。
- `method` ラベルも同様の理由で正規化する。`net/http` はトークン文字列であれば任意のメソッド名をそのまま受け付ける
  ため、正規化しないと未知のメソッドを送りつけられるたびにラベル値が増え続ける(カーディナリティ爆発)。
  `http.MethodGet/Head/Post/Put/Patch/Delete/Options/Connect/Trace` の9つだけをそのまま使い、それ以外は
  `"OTHER"` にまとめる(`methodLabel` 関数。§2 の複製ファイルすべてに同じ実装を置く)。

### 2. 実装コードの置き場所(モジュール境界に合わせて分ける。共有Goモジュール依存は作らない)
- `services/go.mod` 配下(gateway・pokedex・calc): 新設 `services/internal/httpmetrics`(1パッケージ)を3サービス共通で使う。
  同じ module 内なので依存追加は不要。
- balance・speed・judge(独立 go.mod): それぞれの `internal/httpmetrics`(同じ実装を1ファイルだけ複製)を持つ。
  ADR-0405 の spec-writer 調査で確認した「balance/speed はスクリプトを写経する」既存慣習(ADR-0605 §2)に倣う。
  `services` モジュールへの `go.work` 経由の依存追加はしない(ADR-0012 のサービス境界「兄弟サービスは互いに依存しない」の
  精神を Go module の依存関係にも適用し、1レーンの変更が他レーンの `go.mod` を揺らさないようにするため)。
- 複製したファイルはどれも1ファイル・100行未満の小さなコードなので、二重管理のコストは小さいと判断する。

### 3. ミドルウェアの適用方法
- Echo を使うサービス(gateway・balance・speed・judge・pokedex の一部想定)は Echo middleware として登録する。
- 標準 `net/http` の場合は `http.Handler` をラップする。
- 既存のルーティング・エラー整形・CORS 等のミドルウェア順序は変更しない。`/metrics` 自体は計測対象に含めない
  (自己参照でヒストグラムが汚れるのを防ぐ)。
- ヘルスチェック(`/healthz` 等)は計測してよい(可用性 SLO の分母に使うため。P7-2 で参照)。

### 4. kube-prometheus-stack・Loki の導入(ローカル k3d 先行検証。ADR-0018・ADR-0405 の流儀を踏襲)
- 配布は公式 Helm chart(`prometheus-community/kube-prometheus-stack`・`grafana/loki`・`grafana/alloy`)を使う。バージョンは
  「最新の安定版を正確に固定する」という既存のユーザー方針(latest-versions policy)に従い、2026-09-24 時点で取得した最新安定版を
  `--version` で厳密固定する(`^`/`~` のような範囲指定はしない)。ADR-0405 の manifest ハッシュ固定と同じ精神で、chart の取得物
  (`.tgz`)の SHA-256 も固定する(Helm の repo index は署名が無く、取得内容そのものの検証手段が chart version pin だけでは
  無いため)。
  - `prometheus-community/kube-prometheus-stack` バージョン `91.5.1`(app v0.94.1)。tgz SHA-256:
    `b6bcf53edcb86fd1385b677d343c3f7366f0ed0d43fa7e7f7988eba28261a69f`
  - `grafana/loki` バージョン `7.3.0`(app 3.6.12)。tgz SHA-256:
    `04a339f712d770a1f599f05fc0a5a3cde18e43914e49ae6a49f7171be86bcc09`
  - `grafana/alloy` バージョン `1.12.1`(app v1.19.2)。tgz SHA-256:
    `cdd1ec39f99c3c506d5b521156d72236ea143c089f50da6b64398f831734829c`
  - `grafana/alloy` を追加した理由: Loki 単体はログを受け取るサーバーでしかなく、各 Pod のコンテナログを収集して Loki へ送る
    エージェントが別途要る。旧来の `grafana/promtail` は上流で非推奨になっているため、後継の Alloy を使う
    (DaemonSet でノード上のコンテナログを読み、Loki へ push する構成のみを使う。メトリクス収集・OTel 等の他機能は有効化しない)。
- `scripts/observability-bootstrap.sh`(リポジトリルート。ADR-0405 の `scripts/argocd-bootstrap.sh` と同じ置き場所・同じ流儀)を新設する:
  - `cd "$(git rev-parse --show-toplevel)"` から始める。
  - `helm repo add`/`helm repo update` で公式リポジトリを追加し、`helm pull <chart> --version <固定版> -d <一時ディレクトリ>` で
    一度ローカルへ取得し、取得した `.tgz` の SHA-256 を上記の期待値と比較する(不一致ならインストール前に非0で終了。
    ADR-0405 と同じ「取得→検証→適用」の順序)。検証を通った `.tgz` から `helm upgrade --install --version <固定版>` する
    (`helm repo` 経由の再解決はさせず、検証済みのローカル chart を使う)。
  - values は `deploy/k8s/base/observability/values/{kube-prometheus-stack,loki,alloy}.yaml` にコミットする(秘密値を含めない。
    Grafana の管理者パスワードはユーザーが手動で Secret に登録する。ADR-0018 §5 の「AI はトークンを見ない」と同じ扱い)。
  - namespace(`observability`)は冪等に作成する。
  - Argo CD と同じく **manual** な運用にする(sync/upgrade は都度このスクリプトを実行。自動アップグレードはしない)。バージョンを
    上げるときは、このスクリプトの3つの版・3つのハッシュを明示的に更新する(ADR-0405 と同じ「無検証追従を防ぐ」ため)。
- **Loki は `deploymentMode: SingleBinary`(chart の既定は `SimpleScalable` で、素のままではオブジェクトストレージ〈S3/GCS等〉
  を前提にする)。個人利用・小規模〈chart のコメントにある "up to a few tens of GB/day"〉に合うシングルバイナリ構成にし、
  ストレージはローカル PVC(filesystem)にする**(クラウドの object storage には依存しない。ADR-0018 §2 のローカル完結の方針と一致)。
- ローカル k3d では Grafana・Prometheus・Alertmanager の UI は `kubectl port-forward` でのみアクセスする(Ingress を作らない。
  issue #148 の決定と一貫させる)。
- 各サービスの `/metrics` は Kubernetes の `ServiceMonitor`(kube-prometheus-stack の CRD)で Prometheus に発見させる。
  `/metrics` は各サービスの既存 HTTP サーバー・既存 Service ポート(`http`)上でそのまま動く(P7-1 §1〜3 の実装のとおり、
  別ポート・別サーバーを立てていないため)。ServiceMonitor は `deploy/k8s/base/observability/servicemonitors/`(6サービス分)に
  まとめて置く(balance/speed/judge の Kustomize base を書き換えず、observability 側からラベルセレクタで既存 Service を参照する)。
- kube-prometheus-stack・Loki はデフォルトで多くのコンポーネントを含み、個人の k3d(servers:1・agents:0)では
  Pending のまま残るものや意味を持たないものがある。無効化は2種類に分ける。
  (a) **`deploymentMode: SingleBinary` にするために必須のもの**(`gateway.enabled: false`・`monitoring.serviceMonitor`/
  `selfMonitoring`・`test.enabled: false`・backend/read/write のレプリカ0など。SimpleScalable 用のコンポーネントを
  SingleBinary構成では使わないため)。
  (b) **実際に `helm template` で描画を確認して問題が見えたので追加で無効化したもの**(critic レビューで見つかった
  Loki の chunks-cache/results-cache〈既定 memory request がそれぞれ 9830Mi/1229Mi の StatefulSet。k3d では Pending の
  まま残る〉と `lokiCanary`〈個人利用では不要な自己監視用 DaemonSet〉を `loki.yaml` で無効化した)。
  **それ以外は先回りして削らない**: kube-prometheus-stack 側の node-exporter・kube-state-metrics・alertmanager や、
  Grafana 同梱ダッシュボードのうち etcd/scheduler 等クラウド
  マネージド環境では取得できない metrics に依存するものは、今回は無効化していない(実クラスタで動かして問題が
  出た時点で間引く。動く前提で先に削らない)。
- balance-registry のような「クラスタ内レジストリを経由する自前ビルドイメージ」は不要(kube-prometheus-stack・Loki・Alloy は
  すべて公式配布イメージをそのまま使うため)。

### 5. cloud overlay への配慮(まだ実クラウドは無い。issue #149 は保留)
`deploy/k8s/overlays/cloud` には触れない。observability 一式は `deploy/k8s/base/observability` に閉じ、ローカル k3d 専用の
overlay(または base 直下)で完結させる。クラウド選定(issue #149)が決まった時点で provider 別 overlay を足す。

## 影響と制約
- 6サービス(gateway・pokedex・calc・balance・speed・judge)それぞれに `/metrics` を追加するので、変更が全レーンの
  デプロイ定義(Service に port を1つ追加する程度)にまたがる。既存のポート・Ingress 設定は変更しない。
- `services/internal/httpmetrics` と balance/speed/judge の複製3つ、計4箇所で同じロジックを持つ。将来メトリクス項目を
  増やすときは4箇所すべてに反映する必要がある(§2 で許容した二重管理のコスト)。
- kube-prometheus-stack はデフォルトで CPU/メモリを多く使う(alertmanager・node-exporter 等を含む)。ローカル k3d の
  リソースが厳しい場合は values で不要なコンポーネント(node-exporter は k3d では意味が薄い等)を無効化する
  (実装時に spec-writer/implementer が判断し、ADR に追記)。

## 却下した案
- OpenTelemetry Collector 経由でメトリクスを送る: 個人利用規模でこの層を挟む理由がない(直接 scrape で十分)。将来分散トレーシングが
  要るようになったら別ADRで検討する。
- 各サービスにメトリクス用の別ポート・別サーバーを立てる: Echo/net/http に1ルート足すだけで十分。運用(Service定義)を複雑にしない。
- 共有 Go module(`services/internal`)を balance/speed/judge からも `go.work` 経由で直接 import する: レーン間の Go module 依存を
  作ってしまい、1レーンの変更が他レーンの `go.mod`/`go.sum` に影響しうる。ADR-0012 のサービス境界の精神に反するため複製を選んだ。
- Grafana/Prometheus を Ingress で公開する: issue #148(私設サービスを維持する)の決定に反する。

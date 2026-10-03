## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: なし(2026-10-03 時点で担当 issue と未実装機能は実装・main 統合済み。次は下記 Next の人間確認項目待ち)
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: 設計書(docs/type-balance-design.md)の TB0〜TB6 はすべて main に統合済み(TB6: 技範囲チェッカー、PR #65)。P2-3b(特性の無効・吸収)の実データ確認を完了(2026-09-23): データレーンが再生成した export(348 pokemon・moves・216 abilities)で `make balance-k3d-deploy-readmodel && make balance-smoke-readmodel` を実行し、`POST .../team-balance/analyze` でチリーン(levitate)への ground 攻撃が `{"category":"immune","effect":"immune","multiplier":"0","source":"ability"}` になること、`POST .../move-range/analyze`(thunderbolt)の `walledByAbility` にエモンガ(motordrive)が正しく含まれることを実データで確認済み。メガフォームの nameJa が英語表記のままの件はデータレーンへ確認候補として残る(ブロッカーではない)。
Codexレビュー issue #105(Argo CD導入のハッシュ・digest固定)対応完了(2026-09-23。ADR-0405。PR #140)。
**別セッションからの依頼(ユーザー承認済み)で M4 P7-1(kube-prometheus-stack / Loki、各サービスのメトリクス)に着手・完了**
(2026-09-24〜25。ADR-0406。PR #201・#336): 6サービス(gateway・pokedex・calc・balance・speed・judge)に `GET /metrics`
(Prometheus text format、method/pathはカーディナリティ対策で正規化)、`scripts/observability-bootstrap.sh` で
kube-prometheus-stack・Loki(SingleBinary)・Alloy を版・SHA-256固定で導入。実クラスタ(k3d-pokecalc)で実行し、
全Pod起動・PVC Bound・6 ServiceMonitor適用・balance/calc/gatewayのscrapeがup・GrafanaのLokiデータソースで
実ログ取得まで確認済み(judge/pokedex/speedは`/metrics`追加前の古いイメージのため404。各レーン再デプロイで解消見込み)。
Status(追記): 2026-10-01〜02、タイプバランス担当 issue を処理。#263・#292(ADR-0408。AppProject pokecalc 限定・GitOps スクリプトを scripts/gitops/ に共通化・レジストリ PVC 化。PR #422)、#298(ADR-0409。recommendations のアロケーション削減 3.1→0.53MB/op・同時実行上限〈既定4。超過は 503 overloaded〉・GOMEMLIMIT。PR #426)、#260(type-balance-design.md を現在の設計に書き換え、旧版を ADR-0410 へ。PR #429)、#276(ADR-0411。API専用画面は計算モードに関係なくオンラインのマスタ・エラー日本語化。PR #448)、#237(ADR-0412。gitops overlay の pokedex export initContainer。PR #452。実クラスタ未適用)。P7-2(SLO。ADR-0407)は実装済み・実クラスタ未確認。
Status(追記): 2026-10-02〜03、タイプバランスレーンの残件をすべて main へ統合。#236 balance 分(ADR-0413。PR #458)・NetworkPolicy に balance・speed を追加(PR #477)・#284 balance 分=直結 Ingress 撤去で gateway 経由に一本化(ADR-0414。PR #478)・balance の SLO とダッシュボード(ADR-0420。PR #479)・#230 の tb 系ブランチ 19 本の削除(PR #474)・iOS のタイプバランス画面 第1〜3段(防御相性・集計・攻撃範囲・仮想敵・おすすめタイプ・技範囲。ADR-0415。PR #482・#493。swift test 780 件・simulator build 済み)。#259・#263 は理由を書いてクローズ。#237 は initContainer 方式をコード化済み(ADR-0412)。
Status(追記): 2026-10-03、k3d に最新コードを反映して確認済み: `make pokedex-export-k3d`(新設)→ `make deploy-latest`(balance・speed は実データ)→ 旧 `Ingress/balance` を削除 → `web-k3d-smoke`・`API_SMOKE_STRICT=1 make api-smoke`(balance=200 を gateway 経由で)・`balance-smoke-readmodel`・`speed-smoke-readmodel` が緑。promtool で calc・balance の記録ルールを検査(SUCCESS)。確認手順書 M1〜M4 は docs/verify-all.md。`make web-k3d-e2e` は record 未デプロイ(API レーンの P5-3b)で 503 が出て 2 件失敗する既知の状態。
Next(人間の作業・共有クラスタ。AI の自動判定が止める領域): ①NetworkPolicy を `kubectl apply -n pokecalc -k deploy/k8s/base/networkpolicy` で適用(balance・speed → mysql。GitOps の initContainer 用)、②Grafana 管理者パスワードの Secret を作って監視スタックを導入(docs/runbooks/observability.md。balance-slo・calc-slo の実クラスタ確認)、③Argo CD のリポジトリ認証(PAT)登録 → `make pokedex-registry-push`(POKEDEX_REGISTRY_PUSH_CONFIRM=1)で実 digest を確定 → overlay へ PR → sync(#237 はここまで済むまでクローズしない)、④iOS のシミュレータ・実機での見た目確認と `make ios-test`。speed・judge の直結 Ingress 撤去は各レーン。
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code(2026-10-02 再開。#263・#292・#298・#260・#276・#237 を実装。残りは下記 Next)
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
Next: #237 の残り(人間確認が必要): ①`make pokedex-registry-push`(POKEDEX_REGISTRY_PUSH_CONFIRM=1)で pokedex image を balance-registry へ push し実 digest を overlay へ PR、②`deploy/k8s/base/networkpolicy/allow-mysql-ingress.yaml` に balance・speed を足す別 PR(ADR-0132 の許可表も)、③Argo CD sync と `make balance-smoke-readmodel`。①は データレーンへ依頼可。#259 は ADR-0118(DECISIONS 2026-09-25)により必須でなくなったので、タイプバランス側で閉じてよいか判断する(embedded と golden の一致テストは既存)。#236(balance/speed の端末ID検証・エラーコード不一致)と #284(balance・speed・judge を gateway の後ろへ)は別途。判定画面の補助行に英語 message が残る点は別 issue 候補(#276 の ADR-0411)。
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

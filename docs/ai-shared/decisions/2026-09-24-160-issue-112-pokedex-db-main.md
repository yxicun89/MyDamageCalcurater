## 2026-09-24: issue #112(pokedexのDB接続プールに上限と寿命を設定)を main へ統合(データレーン)
Decision: PR #180(`feat/claude-p1-engine` → `main`)をマージした。`services/pokedex/db.OpenPool`/
`PoolConfig`/`ForExport()`(ADR-0112)。critic PASS(1往復。軽微指摘1件〈idle==openの境界値テスト〉は
`TestLoadConfigPoolAllowsIdleEqualToOpen`を追加して反映済み)。
Reason: 独立レビュー PASS・`make test`(953件)/`test-db`(実MySQL)/`lint`/`build`/`k8s-render`すべて
green。実クラスタで4環境変数の設定・api-smoke正常応答を確認済み。
Impact: これでデータレーン主担当のCodexレビューissue(#104・#106・#109・#112)はすべてmain統合済み。
他レーンへの影響なし。
